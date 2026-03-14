package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type enrollReq struct {
	AgentID string `json:"agent_id"`
	Name    string `json:"name,omitempty"`
}

type enrollResp struct {
	OK          bool   `json:"ok"`
	AgentID     string `json:"agent_id,omitempty"`
	IngestToken string `json:"ingest_token,omitempty"`
	Error       string `json:"error,omitempty"`
}

// EnrollAndPersist requests a per-agent ingest_token from the center and writes it into the config file.
// It is a bootstrap step to avoid embedding long-lived shared tokens in agent configs.
func EnrollAndPersist(ctx context.Context, configPath string, cfg Config) (Config, error) {
	if strings.TrimSpace(cfg.IngestToken) != "" {
		return cfg, nil
	}
	if strings.TrimSpace(cfg.EnrollToken) == "" {
		return cfg, errors.New("enroll_token required")
	}
	central := stringsTrimRightSlash(cfg.CentralURL)
	if central == "" {
		return cfg, errors.New("central_url required")
	}

	body, _ := json.Marshal(enrollReq{AgentID: cfg.AgentID, Name: cfg.Name})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, central+"/api/enroll", bytes.NewReader(body))
	if err != nil {
		return cfg, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Enroll-Token", cfg.EnrollToken)

	client := &http.Client{Timeout: 10 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return cfg, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return cfg, fmt.Errorf("enroll failed: %s", res.Status)
	}
	var out enrollResp
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return cfg, fmt.Errorf("decode enroll response: %w", err)
	}
	if !out.OK || strings.TrimSpace(out.IngestToken) == "" {
		msg := strings.TrimSpace(out.Error)
		if msg == "" {
			msg = "enroll rejected"
		}
		return cfg, errors.New(msg)
	}

	if err := writeConfigIngestToken(configPath, out.IngestToken); err != nil {
		return cfg, err
	}
	cfg.IngestToken = out.IngestToken
	return cfg, nil
}

func writeConfigIngestToken(path string, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return errors.New("ingest_token empty")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	m["ingest_token"] = token
	// Drop bootstrap secret after successful enroll to avoid leaving shared secrets on disk.
	delete(m, "enroll_token")
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	dir := filepath.Dir(path)
	tmp := filepath.Join(dir, fmt.Sprintf(".%s.tmp", filepath.Base(path)))
	if err := os.WriteFile(tmp, out, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
