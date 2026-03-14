package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	AgentID string `json:"agent_id"`
	Name    string `json:"name"`

	CentralURL  string `json:"central_url"`
	IngestToken string `json:"ingest_token"`
	EnrollToken string `json:"enroll_token,omitempty"`
	EncryptEnabled bool `json:"encrypt_enabled"`

	CollectIntervalSeconds int `json:"collect_interval_seconds"`

	DiskMount string `json:"disk_mount"`
	NetIface  string `json:"net_iface"`

	PortProbeEnabled bool   `json:"port_probe_enabled"`
	PortProbeHost    string `json:"port_probe_host"`
	Ports            []int  `json:"ports"`

	TCPConnEnabled bool `json:"tcp_conn_enabled"`
}

func (c *Config) applyDefaults() {
	if strings.TrimSpace(c.AgentID) == "" {
		c.AgentID = defaultAgentID()
	}
	if strings.TrimSpace(c.Name) == "" {
		c.Name = c.AgentID
	}
	if u := strings.TrimSpace(c.CentralURL); u != "" {
		if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
			c.CentralURL = "http://" + u
		}
	}
	if c.CollectIntervalSeconds <= 0 {
		c.CollectIntervalSeconds = 5
	}
	if c.DiskMount == "" {
		c.DiskMount = "/"
	}
	if c.PortProbeHost == "" {
		c.PortProbeHost = "127.0.0.1"
	}
}

func LoadConfig(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	cfg.applyDefaults()
	if cfg.CentralURL == "" {
		return Config{}, fmt.Errorf("%s: central_url is required", path)
	}
	if cfg.IngestToken == "" && cfg.EnrollToken == "" {
		return Config{}, fmt.Errorf("%s: ingest_token is required (or set enroll_token)", path)
	}
	return cfg, nil
}

func defaultAgentID() string {
	h, _ := os.Hostname()
	h = strings.ToLower(strings.TrimSpace(h))
	if h == "" {
		return fmt.Sprintf("probe-%d", time.Now().Unix())
	}
	var b strings.Builder
	lastDash := false
	for _, r := range h {
		isOK := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-'
		if isOK {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return fmt.Sprintf("probe-%d", time.Now().Unix())
	}
	return out
}
