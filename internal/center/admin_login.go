package center

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type adminLoginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Service) handleAdminSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Cache-Control", "no-store")

	c, err := r.Cookie(adminSessionCookieName)
	if err != nil || c == nil || strings.TrimSpace(c.Value) == "" {
		writeJSON(w, map[string]any{"ok": true, "authed": false})
		return
	}
	sid := strings.TrimSpace(c.Value)

	now := time.Now()
	s.adminSessionsMu.Lock()
	sess, ok := s.adminSessions[sid]
	if ok && !sess.ExpiresAt.IsZero() && now.After(sess.ExpiresAt) {
		delete(s.adminSessions, sid)
		ok = false
	}
	s.adminSessionsMu.Unlock()

	if !ok {
		writeJSON(w, map[string]any{"ok": true, "authed": false})
		return
	}
	writeJSON(w, map[string]any{
		"ok":         true,
		"authed":     true,
		"user":       sess.User,
		"expires_ms": sess.ExpiresAt.UnixMilli(),
	})
}

func (s *Service) handleAdminLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Cache-Control", "no-store")

	now := time.Now()
	ip := clientIP(r, s.cfg.TrustProxy)
	if s.adminBans != nil {
		if banned, until := s.adminBans.IsBanned(ip, now); banned {
			w.WriteHeader(http.StatusTooManyRequests)
			writeJSON(w, map[string]any{
				"ok":              false,
				"error":           "banned",
				"banned_until_ms": until.UnixMilli(),
			})
			return
		}
	}

	var req adminLoginReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	user := strings.TrimSpace(req.Username)
	pass := strings.TrimSpace(req.Password)
	if user == "" || pass == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	expUser := strings.TrimSpace(s.cfg.AdminUser)
	expPass := s.adminExpectedPassword()
	userOK := subtle.ConstantTimeCompare([]byte(user), []byte(expUser)) == 1
	passOK := subtle.ConstantTimeCompare([]byte(pass), []byte(expPass)) == 1
	if !userOK || !passOK {
		if s.adminBans != nil {
			if banned, until := s.adminBans.RegisterFail(ip, now); banned {
				w.WriteHeader(http.StatusTooManyRequests)
				writeJSON(w, map[string]any{
					"ok":              false,
					"error":           "banned",
					"banned_until_ms": until.UnixMilli(),
				})
				return
			}
		}
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if s.adminBans != nil {
		s.adminBans.Reset(ip)
	}

	sid, exp, err := s.newAdminSession(expUser)
	if err != nil {
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     adminSessionCookieName,
		Value:    sid,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
		Expires:  exp,
	})
	writeJSON(w, map[string]any{"ok": true, "user": expUser, "expires_ms": exp.UnixMilli()})
}

func (s *Service) handleAdminLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Cache-Control", "no-store")

	s.deleteAdminSession(r)
	http.SetCookie(w, &http.Cookie{
		Name:     adminSessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
	writeJSON(w, map[string]any{"ok": true})
}
