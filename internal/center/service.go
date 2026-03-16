package center

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/WhitecrowAurora/lulynx-server-monitor/internal/agent"
	"github.com/WhitecrowAurora/lulynx-server-monitor/internal/common"
)

type Service struct {
	cfg Config
	fs  embed.FS

	mux *http.ServeMux

	mu       sync.RWMutex
	settings Settings
	servers  map[string]*serverState

	series *SeriesStore
	roll   *globalRolling
	dedup  *dedupStore

	ingestKey       [32]byte
	secrets         map[string]string
	enrollBans      *banStore
	adminBans       *banStore
	adminSessionsMu sync.Mutex
	adminSessions   map[string]adminSession

	closed chan struct{}
}

const adminSessionCookieName = "tz_admin_session"
const adminSessionTTL = 24 * time.Hour

type serverState struct {
	cfg ServerConfig

	lastSeenMS int64
	lastIP     string
	meta       serverMeta

	metrics map[string]float64
	ports   []agent.PortStatus

	roll *globalRolling

	lastNetRXTotal uint64
	lastNetTXTotal uint64
	haveNetTotals  bool

	controlOK          bool
	controlLastCheckMS int64
}

type serverMeta struct {
	Cores int    `json:"cores,omitempty"`
	OS    string `json:"os,omitempty"`
	Arch  string `json:"arch,omitempty"`
}

type adminSession struct {
	User      string
	ExpiresAt time.Time
}

func NewService(cfg Config, webFS embed.FS) (*Service, error) {
	s := &Service{
		cfg:           cfg,
		fs:            webFS,
		mux:           http.NewServeMux(),
		servers:       map[string]*serverState{},
		series:        NewSeriesStore(filepath.Join(cfg.DataDir, "series")),
		roll:          newGlobalRolling(43200), // 30 days * 1440 minutes
		dedup:         newDedupStore(10 * time.Minute),
		secrets:       map[string]string{},
		enrollBans:    newBanStore(cfg.EnrollMaxFails, time.Duration(cfg.EnrollBanHours)*time.Hour, "enroll_bans.json"),
		adminBans:     newBanStore(cfg.EnrollMaxFails, time.Duration(cfg.EnrollBanHours)*time.Hour, "admin_bans.json"),
		adminSessions: map[string]adminSession{},
		closed:        make(chan struct{}),
	}
	s.ingestKey = common.DeriveKeySHA256(cfg.IngestToken)

	if err := s.loadState(); err != nil {
		return nil, err
	}

	s.routes()
	s.startCleanupLoop()
	s.startControlProbeLoop()
	return s, nil
}

func (s *Service) Close() {
	select {
	case <-s.closed:
		return
	default:
		close(s.closed)
	}
	s.series.Close()
}

func (s *Service) Handler() http.Handler {
	return s.mux
}

func (s *Service) routes() {
	s.mux.HandleFunc("/api/ingest", s.handleIngest)
	s.mux.HandleFunc("/api/enroll", s.handleEnroll)
	s.mux.HandleFunc("/api/snapshot", s.handleSnapshot)
	s.mux.HandleFunc("/api/series", s.handleSeries)

	s.mux.HandleFunc("/api/admin/login", s.handleAdminLogin)
	s.mux.HandleFunc("/api/admin/logout", s.handleAdminLogout)
	s.mux.HandleFunc("/api/admin/session", s.handleAdminSession)
	s.mux.HandleFunc("/api/admin/settings", s.handleAdminSettings)
	s.mux.HandleFunc("/api/admin/servers", s.handleAdminServers)
	s.mux.HandleFunc("/api/admin/server", s.handleAdminServerUpsert)
	s.mux.HandleFunc("/api/admin/server_patch", s.handleAdminServerPatch)
	s.mux.HandleFunc("/api/admin/issue_agent_token", s.handleAdminIssueAgentToken)
	s.mux.HandleFunc("/api/admin/bans", s.handleAdminBans)
	s.mux.HandleFunc("/api/admin/admin_bans", s.handleAdminAdminBans)

	sub, _ := fs.Sub(s.fs, "web")
	static := http.FileServer(http.FS(sub))
	s.mux.HandleFunc("/admin", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !s.isAdminAuthedBySession(r) {
			http.Redirect(w, r, "/admin/login?next="+url.QueryEscape("/admin"), http.StatusTemporaryRedirect)
			return
		}
		r2 := r.Clone(r.Context())
		u := *r.URL
		u.Path = "/admin.html"
		r2.URL = &u
		static.ServeHTTP(w, r2)
	})
	s.mux.HandleFunc("/admin/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.isAdminAuthedBySession(r) {
			http.Redirect(w, r, "/admin", http.StatusTemporaryRedirect)
			return
		}
		r2 := r.Clone(r.Context())
		u := *r.URL
		u.Path = "/login.html"
		r2.URL = &u
		static.ServeHTTP(w, r2)
	})
	s.mux.HandleFunc("/admin/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin", http.StatusPermanentRedirect)
	})
	s.mux.HandleFunc("/admin.html", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin", http.StatusTemporaryRedirect)
	})
	s.mux.Handle("/", static)
}

func (s *Service) loadState() error {
	if err := ensureDir(s.cfg.DataDir); err != nil {
		return err
	}

	// settings
	settingsPath := filepath.Join(s.cfg.DataDir, "settings.json")
	var st Settings
	if err := loadJSONFile(settingsPath, &st); err == nil {
		s.settings = st
	} else {
		s.settings = Settings{
			DefaultCollectIntervalSeconds: s.cfg.DefaultCollectIntervalSeconds,
			RetentionDays:                 s.cfg.DefaultRetentionDays,
			DashboardPollSeconds:          s.cfg.DashboardPollSeconds,
		}
		_ = saveJSONFile(settingsPath, s.settings)
	}
	if s.settings.DefaultCollectIntervalSeconds <= 0 {
		s.settings.DefaultCollectIntervalSeconds = s.cfg.DefaultCollectIntervalSeconds
	}
	if s.settings.RetentionDays <= 0 {
		s.settings.RetentionDays = s.cfg.DefaultRetentionDays
	}
	if s.settings.DashboardPollSeconds <= 0 {
		s.settings.DashboardPollSeconds = s.cfg.DashboardPollSeconds
	}
	s.settings.TapeFields = normalizeTapeFields(s.settings.TapeFields)

	// servers
	serversPath := filepath.Join(s.cfg.DataDir, "servers.json")
	var list []ServerConfig
	if err := loadJSONFile(serversPath, &list); err == nil {
		for _, c := range list {
			if c.ID == "" {
				continue
			}
			c = applyServerDefaults(c)
			c.DashboardWidgets = normalizeWidgets(c.DashboardWidgets)
			c.Tags = normalizeTags(c.Tags)
			s.servers[c.ID] = &serverState{cfg: c, metrics: map[string]float64{}, roll: newGlobalRolling(43200)}
		}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read %s: %w", serversPath, err)
	}

	// secrets (per-agent ingest tokens)
	if err := s.loadSecrets(); err != nil {
		return err
	}

	// bans (enroll brute-force protection)
	if s.enrollBans != nil || s.adminBans != nil {
		_ = s.loadBans()
	}
	return nil
}

func (s *Service) persistServersLocked() {
	serversPath := filepath.Join(s.cfg.DataDir, "servers.json")
	out := make([]ServerConfig, 0, len(s.servers))
	for _, ss := range s.servers {
		out = append(out, ss.cfg)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	_ = saveJSONFile(serversPath, out)
}

func (s *Service) persistSettingsLocked() {
	settingsPath := filepath.Join(s.cfg.DataDir, "settings.json")
	_ = saveJSONFile(settingsPath, s.settings)
}

func applyServerDefaults(c ServerConfig) ServerConfig {
	if c.Name == "" {
		c.Name = c.ID
	}
	mode := strings.ToLower(strings.TrimSpace(c.ControlMode))
	switch mode {
	case "", "passive":
		c.ControlMode = ""
		c.ControlPort = 0
	case "active":
		c.ControlMode = "active"
		if c.ControlPort <= 0 || c.ControlPort > 65535 {
			c.ControlPort = 38088
		}
	default:
		c.ControlMode = ""
		c.ControlPort = 0
	}
	if c.PortProbeHost == "" {
		c.PortProbeHost = "127.0.0.1"
	}
	return c
}

func isVisible(c ServerConfig) bool {
	if c.Visible == nil {
		return true
	}
	return *c.Visible
}

func boolPtr(v bool) *bool { return &v }

func ensureDir(path string) error {
	return os.MkdirAll(path, 0o755)
}

func (s *Service) startCleanupLoop() {
	ticker := time.NewTicker(6 * time.Hour)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-s.closed:
				return
			case <-ticker.C:
				s.mu.RLock()
				retDays := s.settings.RetentionDays
				s.mu.RUnlock()
				cutoff := time.Now().UTC().Add(-time.Duration(retDays) * 24 * time.Hour)
				_ = s.series.CleanupOlderThan(cutoff)
			}
		}
	}()
}

func (s *Service) authorize(w http.ResponseWriter, r *http.Request, headerName, expected string) bool {
	got := r.Header.Get(headerName)
	if got == "" {
		// allow query for convenience in private deployments
		got = r.URL.Query().Get(strings.ToLower(headerName))
	}
	if got != expected {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return false
	}
	return true
}

func (s *Service) authorizeAdmin(w http.ResponseWriter, r *http.Request) bool {
	if s == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return false
	}
	if s.isAdminAuthedBySession(r) {
		return true
	}

	expected := s.adminExpectedPassword()
	got := strings.TrimSpace(r.Header.Get("X-Admin-Token"))
	if got == "" {
		got = strings.TrimSpace(r.URL.Query().Get("x-admin-token"))
	}
	if got == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return false
	}
	if got == expected {
		if s.adminBans != nil {
			ip := clientIP(r, s.cfg.TrustProxy)
			s.adminBans.Reset(ip)
		}
		return true
	}

	// Wrong token: count attempts and temporarily ban this IP.
	ip := clientIP(r, s.cfg.TrustProxy)
	if s.adminBans != nil {
		now := time.Now()
		if banned, until := s.adminBans.IsBanned(ip, now); banned {
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(http.StatusTooManyRequests)
			writeJSON(w, map[string]any{
				"ok":              false,
				"error":           "banned",
				"banned_until_ms": until.UnixMilli(),
			})
			return false
		}
		if banned, until := s.adminBans.RegisterFail(ip, now); banned {
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(http.StatusTooManyRequests)
			writeJSON(w, map[string]any{
				"ok":              false,
				"error":           "banned",
				"banned_until_ms": until.UnixMilli(),
			})
			return false
		}
	}

	http.Error(w, "unauthorized", http.StatusUnauthorized)
	return false
}

func (s *Service) adminExpectedPassword() string {
	if s == nil {
		return ""
	}
	if strings.TrimSpace(s.cfg.AdminPassword) != "" {
		return strings.TrimSpace(s.cfg.AdminPassword)
	}
	return strings.TrimSpace(s.cfg.AdminToken)
}

func (s *Service) isAdminAuthedBySession(r *http.Request) bool {
	if s == nil || r == nil {
		return false
	}
	c, err := r.Cookie(adminSessionCookieName)
	if err != nil || c == nil {
		return false
	}
	sid := strings.TrimSpace(c.Value)
	if sid == "" {
		return false
	}
	now := time.Now()
	s.adminSessionsMu.Lock()
	defer s.adminSessionsMu.Unlock()
	sess, ok := s.adminSessions[sid]
	if !ok {
		return false
	}
	if !sess.ExpiresAt.IsZero() && now.After(sess.ExpiresAt) {
		delete(s.adminSessions, sid)
		return false
	}
	return true
}

func (s *Service) newAdminSession(username string) (string, time.Time, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", time.Time{}, err
	}
	sid := hex.EncodeToString(b)
	exp := time.Now().Add(adminSessionTTL)
	s.adminSessionsMu.Lock()
	s.adminSessions[sid] = adminSession{User: username, ExpiresAt: exp}
	s.adminSessionsMu.Unlock()
	return sid, exp, nil
}

func (s *Service) deleteAdminSession(r *http.Request) {
	if s == nil || r == nil {
		return
	}
	c, err := r.Cookie(adminSessionCookieName)
	if err != nil || c == nil {
		return
	}
	sid := strings.TrimSpace(c.Value)
	if sid == "" {
		return
	}
	s.adminSessionsMu.Lock()
	delete(s.adminSessions, sid)
	s.adminSessionsMu.Unlock()
}

func (s *Service) stealthIngestUnauthorizedEnabled() bool {
	return s.cfg.StealthIngestUnauthorized == nil || *s.cfg.StealthIngestUnauthorized
}

func (s *Service) ingestReject(w http.ResponseWriter, r *http.Request) {
	if !s.stealthIngestUnauthorizedEnabled() {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	// Best-effort: close the underlying connection without writing any response.
	if hj, ok := w.(http.Hijacker); ok {
		conn, _, err := hj.Hijack()
		if err == nil {
			_ = conn.Close()
			return
		}
	}
	// Fallback (e.g. HTTP/2): return an unhelpful status without a body.
	w.WriteHeader(http.StatusNotFound)
}
