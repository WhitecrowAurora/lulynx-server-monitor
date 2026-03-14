package agent

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type controlConfigPatch struct {
	CollectIntervalSeconds *int   `json:"collect_interval_seconds,omitempty"`
	PortProbeEnabled       *bool  `json:"port_probe_enabled,omitempty"`
	PortProbeHost          *string `json:"port_probe_host,omitempty"`
	Ports                  *[]int `json:"ports,omitempty"`
	TCPConnEnabled         *bool  `json:"tcp_conn_enabled,omitempty"`
}

type controlManager struct {
	agentID string
	token   string

	rc         *runtimeConfig
	intervalCh chan time.Duration

	mu   sync.Mutex
	mode string
	port int
	srv  *http.Server
	ln   net.Listener
}

func newControlManager(agentID, token string, rc *runtimeConfig, intervalCh chan time.Duration) *controlManager {
	return &controlManager{
		agentID:    strings.TrimSpace(agentID),
		token:      strings.TrimSpace(token),
		rc:         rc,
		intervalCh: intervalCh,
	}
}

func (m *controlManager) Stop() {
	m.mu.Lock()
	srv := m.srv
	ln := m.ln
	m.srv = nil
	m.ln = nil
	m.mode = ""
	m.port = 0
	m.mu.Unlock()

	if ln != nil {
		_ = ln.Close()
	}
	if srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = srv.Shutdown(ctx)
		cancel()
	}
}

func (m *controlManager) SetDesired(mode string, port int) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "passive" || mode == "" {
		mode = ""
	}
	if mode != "active" {
		m.Stop()
		return
	}

	if port <= 0 || port > 65535 {
		port = 38088
	}
	m.mu.Lock()
	curMode := m.mode
	curPort := m.port
	m.mu.Unlock()
	if curMode == "active" && curPort == port {
		return
	}

	// restart
	m.Stop()
	m.start(port)
}

func (m *controlManager) start(port int) {
	if m.rc == nil {
		return
	}
	m.mu.Lock()
	if m.srv != nil || m.ln != nil {
		m.mu.Unlock()
		return
	}
	token := m.token
	m.mu.Unlock()
	if token == "" {
		return
	}

	ln, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		return
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/control/ping":
			m.handlePing(w, r)
		case "/api/control/config":
			m.handleConfig(w, r)
		default:
			controlReject(w)
		}
	})
	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 3 * time.Second,
	}

	m.mu.Lock()
	m.mode = "active"
	m.port = port
	m.srv = srv
	m.ln = ln
	m.mu.Unlock()

	go func() {
		_ = srv.Serve(ln)
	}()
}

func (m *controlManager) authorize(w http.ResponseWriter, r *http.Request) bool {
	got := r.Header.Get("X-Agent-Token")
	if got == "" {
		got = r.URL.Query().Get("x-agent-token")
	}
	if got != m.token || got == "" {
		controlReject(w)
		return false
	}
	gotID := strings.TrimSpace(r.Header.Get("X-Agent-ID"))
	if gotID != "" && gotID != m.agentID {
		controlReject(w)
		return false
	}
	return true
}

func (m *controlManager) handlePing(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		controlReject(w)
		return
	}
	if !m.authorize(w, r) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

func (m *controlManager) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		controlReject(w)
		return
	}
	if !m.authorize(w, r) {
		return
	}
	var patch controlConfigPatch
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		controlReject(w)
		return
	}
	rc := m.rc
	if rc == nil {
		controlReject(w)
		return
	}
	rc.mu.Lock()
	if patch.PortProbeHost != nil && strings.TrimSpace(*patch.PortProbeHost) != "" {
		rc.portHost = strings.TrimSpace(*patch.PortProbeHost)
	}
	if patch.PortProbeEnabled != nil {
		rc.portEnabled = *patch.PortProbeEnabled
	}
	if patch.TCPConnEnabled != nil {
		rc.tcpEnabled = *patch.TCPConnEnabled
	}
	if patch.Ports != nil {
		rc.ports = append([]int(nil), (*patch.Ports)...)
	}
	nextEvery := rc.collectEvery
	if patch.CollectIntervalSeconds != nil && *patch.CollectIntervalSeconds > 0 {
		nextEvery = time.Duration(*patch.CollectIntervalSeconds) * time.Second
	}
	changedInterval := nextEvery > 0 && nextEvery != rc.collectEvery
	if changedInterval {
		rc.collectEvery = nextEvery
	}
	rc.mu.Unlock()

	if changedInterval {
		notifyInterval(m.intervalCh, nextEvery)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

func controlReject(w http.ResponseWriter) {
	// Best-effort: close without response (reduces fingerprinting; similar to center stealth ingest).
	if hj, ok := w.(http.Hijacker); ok {
		conn, _, err := hj.Hijack()
		if err == nil {
			_ = conn.Close()
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
}
