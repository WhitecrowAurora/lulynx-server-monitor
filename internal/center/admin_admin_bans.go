package center

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"
)

func (s *Service) handleAdminAdminBans(w http.ResponseWriter, r *http.Request) {
	if !s.authorizeAdmin(w, r) {
		return
	}
	if s.adminBans == nil {
		writeJSON(w, bansAdminResp{OK: true, NowMS: time.Now().UnixMilli(), Entries: nil})
		return
	}

	switch r.Method {
	case http.MethodGet:
		now := time.Now()
		entries := s.adminBans.List(now)
		sort.Slice(entries, func(i, j int) bool { return entries[i].IP < entries[j].IP })
		writeJSON(w, bansAdminResp{OK: true, NowMS: now.UnixMilli(), Entries: entries})
	case http.MethodPost:
		var p bansAdminPost
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		ip := strings.TrimSpace(p.IP)
		if ip == "" {
			http.Error(w, "ip required", http.StatusBadRequest)
			return
		}
		s.adminBans.Reset(ip)
		writeJSON(w, map[string]any{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

