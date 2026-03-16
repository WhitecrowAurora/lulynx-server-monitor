package center

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/WhitecrowAurora/lulynx-server-monitor/internal/agent"
	"github.com/WhitecrowAurora/lulynx-server-monitor/internal/common"
)

func (s *Service) effectiveCollectIntervalSecondsLocked(c ServerConfig) int {
	if c.CollectIntervalSeconds != nil && *c.CollectIntervalSeconds > 0 {
		return *c.CollectIntervalSeconds
	}
	if s.settings.DefaultCollectIntervalSeconds > 0 {
		return s.settings.DefaultCollectIntervalSeconds
	}
	return 5
}

func (s *Service) handleIngest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.ingestReject(w, r)
		return
	}
	recvMS := time.Now().UnixMilli()
	now := time.Now()
	srcIP := clientIP(r, s.cfg.TrustProxy)

	enc := strings.TrimSpace(r.Header.Get("X-Ingest-Enc"))
	var p agent.IngestPayload
	agentID := ""
	if enc == "" {
		got := r.Header.Get("X-Ingest-Token")
		if got == "" {
			// allow query for convenience in private deployments
			got = r.URL.Query().Get("x-ingest-token")
		}
		agentID = strings.TrimSpace(r.Header.Get("X-Agent-ID"))
		if agentID != "" {
			s.mu.RLock()
			expected := strings.TrimSpace(s.secrets[agentID])
			s.mu.RUnlock()
			// Accept either per-agent token or the global ingest token (for simple deployments / migration).
			if expected != "" {
				if got != expected && got != s.cfg.IngestToken {
					s.ingestReject(w, r)
					return
				}
			} else if got != s.cfg.IngestToken {
				s.ingestReject(w, r)
				return
			}
		} else {
			// Backward-compatible path: validate global token before decoding.
			if got != s.cfg.IngestToken {
				s.ingestReject(w, r)
				return
			}
		}
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		bodyAgentID := strings.TrimSpace(p.AgentID)
		if bodyAgentID == "" {
			s.ingestReject(w, r)
			return
		}
		if agentID == "" {
			agentID = bodyAgentID
		} else if agentID != bodyAgentID {
			s.ingestReject(w, r)
			return
		}
	} else if enc == "aesgcm" {
		agentID = strings.TrimSpace(r.Header.Get("X-Agent-ID"))
		if agentID == "" {
			s.ingestReject(w, r)
			return
		}
		msgID := strings.TrimSpace(r.Header.Get("X-Msg-ID"))
		if msgID == "" {
			s.ingestReject(w, r)
			return
		}
		dup := s.dedup != nil && s.dedup.Seen(agentID, msgID, now)
		nonceB64 := strings.TrimSpace(r.Header.Get("X-Nonce"))
		nonce, err := base64.RawStdEncoding.DecodeString(nonceB64)
		if err != nil {
			s.ingestReject(w, r)
			return
		}
		ct, err := io.ReadAll(r.Body)
		if err != nil {
			s.ingestReject(w, r)
			return
		}
		// Use per-agent key if enrolled; also accept global key for simple deployments / migration.
		s.mu.RLock()
		agentTok := strings.TrimSpace(s.secrets[agentID])
		s.mu.RUnlock()
		var plain []byte
		var decErr error
		if agentTok != "" {
			plain, decErr = common.DecryptAESGCM(common.DeriveKeySHA256(agentTok), []byte(agentID), nonce, ct)
			if decErr != nil {
				plain, decErr = common.DecryptAESGCM(s.ingestKey, []byte(agentID), nonce, ct)
			}
		} else {
			plain, decErr = common.DecryptAESGCM(s.ingestKey, []byte(agentID), nonce, ct)
		}
		if decErr != nil {
			s.ingestReject(w, r)
			return
		}
		if err := json.Unmarshal(plain, &p); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if dup {
			// Drop duplicates silently, but return current config so agent can still self-heal.
			s.mu.RLock()
			ss := s.servers[agentID]
			var resp agent.ConfigResponse
			resp.OK = true
			if ss != nil {
				resp.Config.CollectIntervalSeconds = s.effectiveCollectIntervalSecondsLocked(ss.cfg)
				resp.Config.PortProbeEnabled = ss.cfg.PortProbeEnabled
				resp.Config.PortProbeHost = ss.cfg.PortProbeHost
				resp.Config.Ports = append([]int(nil), ss.cfg.Ports...)
				resp.Config.TCPConnEnabled = ss.cfg.TCPConnEnabled
				resp.Config.ControlMode = ss.cfg.ControlMode
				resp.Config.ControlPort = ss.cfg.ControlPort
			} else {
				resp.Config.CollectIntervalSeconds = s.settings.DefaultCollectIntervalSeconds
			}
			s.mu.RUnlock()
			writeJSON(w, resp)
			return
		}
	} else {
		s.ingestReject(w, r)
		return
	}

	p.AgentID = agentID

	s.mu.Lock()
	ss := s.servers[agentID]
	if ss == nil {
		if !s.cfg.AllowAutoRegister {
			s.mu.Unlock()
			http.Error(w, "unknown agent", http.StatusForbidden)
			return
		}
		visible := boolPtr(false)
		c := ServerConfig{
			ID:             p.AgentID,
			Name:           strings.TrimSpace(p.Name),
			Visible:        visible,
			ExpiresText:    "长期",
			TCPConnEnabled: true,
		}
		c = applyServerDefaults(c)
		ss = &serverState{cfg: c, metrics: map[string]float64{}, roll: newGlobalRolling(43200)}
		s.servers[agentID] = ss
		s.persistServersLocked()
	}

	if name := strings.TrimSpace(p.Name); name != "" && name != ss.cfg.Name {
		ss.cfg.Name = name
		s.persistServersLocked()
	}

	ss.lastSeenMS = recvMS
	if strings.TrimSpace(srcIP) != "" && srcIP != ss.lastIP {
		ss.lastIP = srcIP
		ss.controlOK = false
		ss.controlLastCheckMS = 0
	}
	ss.meta = serverMeta{Cores: p.Meta.Cores, OS: p.Meta.OS, Arch: p.Meta.Arch}

	ss.metrics = make(map[string]float64, len(p.Metrics))
	for k, v := range p.Metrics {
		ss.metrics[k] = v
		s.series.Append(agentID, k, recvMS, v)
	}
	ss.ports = append([]agent.PortStatus(nil), p.Ports...)

	rxTotal := uint64(ss.metrics["net_rx_total_bytes"])
	txTotal := uint64(ss.metrics["net_tx_total_bytes"])
	if ss.haveNetTotals {
		var drx, dtx uint64
		if rxTotal >= ss.lastNetRXTotal {
			drx = rxTotal - ss.lastNetRXTotal
		}
		if txTotal >= ss.lastNetTXTotal {
			dtx = txTotal - ss.lastNetTXTotal
		}
		s.roll.Add(recvMS, drx, dtx)
		if ss.roll != nil {
			ss.roll.Add(recvMS, drx, dtx)
		}
	}
	ss.lastNetRXTotal = rxTotal
	ss.lastNetTXTotal = txTotal
	ss.haveNetTotals = true

	resp := agent.ConfigResponse{OK: true}
	resp.Config.CollectIntervalSeconds = s.effectiveCollectIntervalSecondsLocked(ss.cfg)
	resp.Config.PortProbeEnabled = ss.cfg.PortProbeEnabled
	resp.Config.PortProbeHost = ss.cfg.PortProbeHost
	resp.Config.Ports = append([]int(nil), ss.cfg.Ports...)
	resp.Config.TCPConnEnabled = ss.cfg.TCPConnEnabled
	resp.Config.ControlMode = ss.cfg.ControlMode
	resp.Config.ControlPort = ss.cfg.ControlPort
	s.mu.Unlock()

	writeJSON(w, resp)
}
