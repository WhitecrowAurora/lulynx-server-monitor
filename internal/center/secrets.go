package center

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type secretsFile struct {
	Tokens map[string]string `json:"tokens"`
}

func (s *Service) secretsPath() string {
	return filepath.Join(s.cfg.DataDir, "secrets.json")
}

func (s *Service) loadSecrets() error {
	path := s.secretsPath()
	var f secretsFile
	if err := loadJSONFile(path, &f); err == nil {
		if f.Tokens != nil {
			for k, v := range f.Tokens {
				k = strings.TrimSpace(k)
				v = strings.TrimSpace(v)
				if k == "" || v == "" {
					continue
				}
				s.secrets[k] = v
			}
		}
		return nil
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read %s: %w", path, err)
	}
	return nil
}

func (s *Service) persistSecretsLocked() {
	path := s.secretsPath()
	out := secretsFile{Tokens: map[string]string{}}
	for k, v := range s.secrets {
		if strings.TrimSpace(k) == "" || strings.TrimSpace(v) == "" {
			continue
		}
		out.Tokens[k] = v
	}
	_ = saveJSONFilePrivate(path, out)
}

func randomTokenHex(bytes int) (string, error) {
	if bytes <= 0 {
		bytes = 24
	}
	b := make([]byte, bytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *Service) ensureAgentTokenLocked(agentID string) (string, error) {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return "", errors.New("agent_id required")
	}
	if v := strings.TrimSpace(s.secrets[agentID]); v != "" {
		return v, nil
	}
	tok, err := randomTokenHex(24)
	if err != nil {
		return "", err
	}
	s.secrets[agentID] = tok
	s.persistSecretsLocked()
	return tok, nil
}
