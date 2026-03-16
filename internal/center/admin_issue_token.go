package center

import (
	"encoding/json"
	"net/http"
	"strings"
)

type adminIssueAgentTokenReq struct {
	AgentID string `json:"agent_id"`
	Name    string `json:"name,omitempty"`
}

func (s *Service) handleAdminIssueAgentToken(w http.ResponseWriter, r *http.Request) {
	if !s.authorizeAdmin(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req adminIssueAgentTokenReq
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

	writeJSON(w, map[string]any{
		"ok":          true,
		"agent_id":    req.AgentID,
		"ingest_token": tok,
	})
}

