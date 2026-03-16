package center

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	ListenAddr string `json:"listen_addr"`
	DataDir    string `json:"data_dir"`

	IngestToken   string `json:"ingest_token"`
	AdminUser     string `json:"admin_user,omitempty"`
	AdminPassword string `json:"admin_password,omitempty"`
	// AdminToken is a legacy alias for AdminPassword (kept for backward compatibility).
	AdminToken string `json:"admin_token,omitempty"`
	// EnrollToken is an optional bootstrap secret for issuing per-agent ingest tokens.
	// If empty, /api/enroll is disabled.
	EnrollToken    string `json:"enroll_token,omitempty"`
	EnrollMaxFails int    `json:"enroll_max_fails,omitempty"`
	EnrollBanHours int    `json:"enroll_ban_hours,omitempty"`
	// TrustProxy controls whether to use X-Forwarded-For for ban/rate-limit IP accounting.
	// Only enable if the center is behind a trusted reverse proxy.
	TrustProxy bool `json:"trust_proxy,omitempty"`

	AllowAutoRegister bool `json:"allow_auto_register"`

	// StealthIngestUnauthorized controls how the center responds to unauthorized ingest requests.
	// If nil or true, the server will try to close the connection without sending an HTTP error
	// (reduces fingerprinting; makes misconfig harder to debug).
	StealthIngestUnauthorized *bool `json:"stealth_ingest_unauthorized"`

	DefaultCollectIntervalSeconds int `json:"default_collect_interval_seconds"`
	DefaultRetentionDays          int `json:"default_retention_days"`
	DashboardPollSeconds          int `json:"dashboard_poll_seconds"`
}

func (c *Config) applyDefaults() {
	if c.ListenAddr == "" {
		c.ListenAddr = ":38088"
	}
	if c.DataDir == "" {
		c.DataDir = "./data"
	}
	if c.DefaultCollectIntervalSeconds <= 0 {
		c.DefaultCollectIntervalSeconds = 5
	}
	if c.DefaultRetentionDays <= 0 {
		c.DefaultRetentionDays = 30
	}
	if c.DashboardPollSeconds <= 0 {
		c.DashboardPollSeconds = 3
	}
	if c.EnrollMaxFails <= 0 {
		c.EnrollMaxFails = 5
	}
	if c.EnrollBanHours <= 0 {
		c.EnrollBanHours = 8
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
	if cfg.IngestToken == "" {
		return Config{}, fmt.Errorf("%s: ingest_token is required", path)
	}

	cfg.AdminUser = strings.TrimSpace(cfg.AdminUser)
	if cfg.AdminUser == "" {
		cfg.AdminUser = "admin"
	}
	cfg.AdminPassword = strings.TrimSpace(cfg.AdminPassword)
	cfg.AdminToken = strings.TrimSpace(cfg.AdminToken)
	if cfg.AdminPassword == "" {
		cfg.AdminPassword = cfg.AdminToken
	}
	// Default admin password to ingest_token in simple deployments.
	if cfg.AdminPassword == "" {
		cfg.AdminPassword = cfg.IngestToken
	}
	if cfg.AdminPassword == "" {
		return Config{}, fmt.Errorf("%s: admin_password (or legacy admin_token) is required", path)
	}
	if cfg.DataDir != "" {
		cfg.DataDir = filepath.Clean(cfg.DataDir)
	}
	return cfg, nil
}
