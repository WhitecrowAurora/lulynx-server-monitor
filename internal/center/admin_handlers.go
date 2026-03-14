package center

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

func (s *Service) handleAdminSettings(w http.ResponseWriter, r *http.Request) {
	if !s.authorizeAdmin(w, r) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		s.mu.RLock()
		st := s.settings
		s.mu.RUnlock()
		writeJSON(w, map[string]any{"ok": true, "settings": st})
	case http.MethodPost:
		var patch settingsPatch
		if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		s.mu.Lock()
		if patch.DefaultCollectIntervalSeconds != nil && *patch.DefaultCollectIntervalSeconds > 0 {
			s.settings.DefaultCollectIntervalSeconds = *patch.DefaultCollectIntervalSeconds
		}
		if patch.RetentionDays != nil && *patch.RetentionDays > 0 {
			s.settings.RetentionDays = *patch.RetentionDays
		}
		if patch.DashboardPollSeconds != nil && *patch.DashboardPollSeconds > 0 {
			s.settings.DashboardPollSeconds = *patch.DashboardPollSeconds
		}
		if patch.EnableGrouping != nil {
			s.settings.EnableGrouping = *patch.EnableGrouping
		}
		if patch.TapeFields != nil {
			s.settings.TapeFields = normalizeTapeFields(*patch.TapeFields)
		}
		s.persistSettingsLocked()
		out := s.settings
		s.mu.Unlock()
		writeJSON(w, map[string]any{"ok": true, "settings": out})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Service) handleAdminServers(w http.ResponseWriter, r *http.Request) {
	if !s.authorizeAdmin(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	nowMS := time.Now().UnixMilli()
	s.mu.RLock()
	monthMin := monthMinFor(s.settings.RetentionDays)
	out := make([]map[string]any, 0, len(s.servers))
	for _, ss := range s.servers {
		intervalSec := s.effectiveCollectIntervalSecondsLocked(ss.cfg)
		thresholdSec := intervalSec * 3
		if thresholdSec < 30 {
			thresholdSec = 30
		}
		isOnline := ss.lastSeenMS > 0 && (nowMS-ss.lastSeenMS) <= int64(thresholdSec)*1000
		used := s.serverTrafficUsedBytes(nowMS, monthMin, ss)
		out = append(out, map[string]any{
			"config":             ss.cfg,
			"online":             isOnline,
			"last_seen_ms":       ss.lastSeenMS,
			"traffic_used_bytes": used,
		})
	}
	s.mu.RUnlock()
	writeJSON(w, map[string]any{"ok": true, "servers": out})
}

func (s *Service) handleAdminServerUpsert(w http.ResponseWriter, r *http.Request) {
	if !s.authorizeAdmin(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var c ServerConfig
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	c.ID = strings.TrimSpace(c.ID)
	if c.ID == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}
	c = applyServerDefaults(c)
	c.DashboardWidgets = normalizeWidgets(c.DashboardWidgets)
	c.Tags = normalizeTags(c.Tags)

	s.mu.Lock()
	ss := s.servers[c.ID]
	if ss == nil {
		ss = &serverState{cfg: c, metrics: map[string]float64{}, roll: newGlobalRolling(43200)}
		s.servers[c.ID] = ss
	} else {
		ss.cfg = c
	}
	s.persistServersLocked()
	s.mu.Unlock()
	go s.maybePushControlConfig(c.ID)
	writeJSON(w, map[string]any{"ok": true})
}

type serverPatch struct {
	ID          string  `json:"id"`
	ControlMode *string `json:"control_mode,omitempty"`
	ControlPort *int    `json:"control_port,omitempty"`
}

func (s *Service) handleAdminServerPatch(w http.ResponseWriter, r *http.Request) {
	if !s.authorizeAdmin(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var patch serverPatch
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	patch.ID = strings.TrimSpace(patch.ID)
	if patch.ID == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}

	var probeID, probeIP, probeTok string
	var probePort int
	s.mu.Lock()
	ss := s.servers[patch.ID]
	if ss == nil {
		s.mu.Unlock()
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if patch.ControlMode != nil {
		ss.cfg.ControlMode = strings.TrimSpace(*patch.ControlMode)
	}
	if patch.ControlPort != nil {
		ss.cfg.ControlPort = *patch.ControlPort
	}
	ss.cfg = applyServerDefaults(ss.cfg)
	ss.controlOK = false
	ss.controlLastCheckMS = 0
	if strings.ToLower(strings.TrimSpace(ss.cfg.ControlMode)) == "active" && strings.TrimSpace(ss.lastIP) != "" {
		probeID = patch.ID
		probeIP = ss.lastIP
		probePort = effectiveControlPort(ss.cfg)
		probeTok = strings.TrimSpace(s.secrets[patch.ID])
		if probeTok == "" {
			probeTok = strings.TrimSpace(s.cfg.IngestToken)
		}
	}
	s.persistServersLocked()
	s.mu.Unlock()

	if probeID != "" && probeIP != "" && probeTok != "" && probePort > 0 {
		go func() {
			client := &http.Client{Timeout: 900 * time.Millisecond}
			ok := s.probeOneControl(client, probeID, probeIP, probePort, probeTok)
			s.mu.Lock()
			if ss := s.servers[probeID]; ss != nil {
				ss.controlOK = ok
				ss.controlLastCheckMS = time.Now().UnixMilli()
			}
			s.mu.Unlock()
		}()
	}

	writeJSON(w, map[string]any{"ok": true})
}
