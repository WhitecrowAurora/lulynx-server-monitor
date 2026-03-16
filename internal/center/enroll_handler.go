package center

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"
)

type enrollReq struct {
	AgentID string `json:"agent_id"`
	Name    string `json:"name,omitempty"`
}

type enrollResp struct {
	OK            bool   `json:"ok"`
	AgentID       string `json:"agent_id,omitempty"`
	IngestToken   string `json:"ingest_token,omitempty"`
	Error         string `json:"error,omitempty"`
	BannedUntilMS int64  `json:"banned_until_ms,omitempty"`
}

func (s *Service) enrollExpectedSecret() string {
	if s == nil {
		return ""
	}
	return strings.TrimSpace(s.cfg.EnrollToken)
}

func (s *Service) handleEnroll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	secret := s.enrollExpectedSecret()
	if secret == "" {
		http.NotFound(w, r)
		return
	}

	now := time.Now()
	ip := clientIP(r, s.cfg.TrustProxy)
	if s.enrollBans != nil {
		if banned, until := s.enrollBans.IsBanned(ip, now); banned {
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(http.StatusTooManyRequests)
			writeJSON(w, enrollResp{OK: false, Error: "banned", BannedUntilMS: until.UnixMilli()})
			return
		}
	}

	var req enrollReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	req.AgentID = strings.TrimSpace(req.AgentID)
	req.Name = strings.TrimSpace(req.Name)
	if req.AgentID == "" {
		http.Error(w, "agent_id required", http.StatusBadRequest)
		return
	}

	got := r.Header.Get("X-Enroll-Token")
	if got == "" {
		got = r.URL.Query().Get("x-enroll-token")
	}
	if got == "" {
		// Convenience: allow reusing admin password as enroll token by setting enroll_token == admin_password.
		got = r.Header.Get("X-Admin-Token")
		if got == "" {
			got = r.URL.Query().Get("x-admin-token")
		}
	}

	if got != secret {
		if s.enrollBans != nil {
			banned, until := s.enrollBans.RegisterFail(ip, now)
			if banned {
				w.Header().Set("Cache-Control", "no-store")
				w.WriteHeader(http.StatusTooManyRequests)
				writeJSON(w, enrollResp{OK: false, Error: "banned", BannedUntilMS: until.UnixMilli()})
				return
			}
		}
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if s.enrollBans != nil {
		s.enrollBans.Reset(ip)
	}

	s.mu.Lock()
	tok, err := s.ensureAgentTokenLocked(req.AgentID)
	if err != nil {
		s.mu.Unlock()
		http.Error(w, "failed to issue token", http.StatusInternalServerError)
		return
	}
	if s.servers[req.AgentID] == nil {
		visible := boolPtr(false)
		c := ServerConfig{
			ID:             req.AgentID,
			Name:           req.Name,
			Visible:        visible,
			ExpiresText:    "长期",
			TCPConnEnabled: true,
		}
		c = applyServerDefaults(c)
		c.DashboardWidgets = normalizeWidgets(c.DashboardWidgets)
		c.Tags = normalizeTags(c.Tags)
		s.servers[req.AgentID] = &serverState{cfg: c, metrics: map[string]float64{}, roll: newGlobalRolling(43200)}
		s.persistServersLocked()
	} else if req.Name != "" && s.servers[req.AgentID].cfg.Name != req.Name {
		ss := s.servers[req.AgentID]
		ss.cfg.Name = req.Name
		s.persistServersLocked()
	}
	s.mu.Unlock()

	writeJSON(w, enrollResp{OK: true, AgentID: req.AgentID, IngestToken: tok})
}

func clientIP(r *http.Request, trustProxy bool) string {
	if r == nil {
		return ""
	}
	if trustProxy {
		xff := r.Header.Get("X-Forwarded-For")
		if xff != "" {
			parts := strings.Split(xff, ",")
			if len(parts) > 0 {
				if ip := strings.TrimSpace(parts[0]); ip != "" {
					return ip
				}
			}
		}
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}
