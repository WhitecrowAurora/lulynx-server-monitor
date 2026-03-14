package center

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (s *Service) maybePushControlConfig(agentID string) {
	if s == nil {
		return
	}
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return
	}

	var (
		ip     string
		port   int
		token  string
		patch  map[string]any
	)

	s.mu.RLock()
	ss := s.servers[agentID]
	if ss == nil {
		s.mu.RUnlock()
		return
	}
	if strings.ToLower(strings.TrimSpace(ss.cfg.ControlMode)) != "active" || !ss.controlOK {
		s.mu.RUnlock()
		return
	}
	ip = strings.TrimSpace(ss.lastIP)
	port = effectiveControlPort(ss.cfg)
	token = strings.TrimSpace(s.secrets[agentID])
	if token == "" {
		token = strings.TrimSpace(s.cfg.IngestToken)
	}
	interval := s.effectiveCollectIntervalSecondsLocked(ss.cfg)
	patch = map[string]any{
		"collect_interval_seconds": interval,
		"port_probe_enabled":       ss.cfg.PortProbeEnabled,
		"port_probe_host":          ss.cfg.PortProbeHost,
		"ports":                    append([]int(nil), ss.cfg.Ports...),
		"tcp_conn_enabled":         ss.cfg.TCPConnEnabled,
	}
	s.mu.RUnlock()

	if ip == "" || port <= 0 || port > 65535 || token == "" {
		return
	}

	host := ip
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		host = "[" + host + "]"
	}
	u := "http://" + host + ":" + strconv.Itoa(port) + "/api/control/config"
	body, _ := json.Marshal(patch)

	ctx, cancel := context.WithTimeout(context.Background(), 900*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Agent-ID", agentID)
	req.Header.Set("X-Agent-Token", token)

	client := &http.Client{Timeout: 900 * time.Millisecond}
	res, err := client.Do(req)
	if err != nil {
		return
	}
	_ = res.Body.Close()
}

