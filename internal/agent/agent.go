package agent

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/WhitecrowAurora/lulynx-server-monitor/internal/common"
)

type runtimeConfig struct {
	mu           sync.RWMutex
	collectEvery time.Duration
	portEnabled  bool
	portHost     string
	ports        []int
	tcpEnabled   bool
}

func Run(ctx context.Context, cfg Config) error {
	rc := &runtimeConfig{
		collectEvery: time.Duration(cfg.CollectIntervalSeconds) * time.Second,
		portEnabled:  cfg.PortProbeEnabled,
		portHost:     cfg.PortProbeHost,
		ports:        append([]int(nil), cfg.Ports...),
		tcpEnabled:   cfg.TCPConnEnabled,
	}
	if rc.collectEvery <= 0 {
		rc.collectEvery = 5 * time.Second
	}

	collector := newCollector(cfg.DiskMount, cfg.NetIface)

	client := &http.Client{
		Timeout: 8 * time.Second,
	}
	ingestURL := stringsTrimRightSlash(cfg.CentralURL) + "/api/ingest"
	encKey := common.DeriveKeySHA256(cfg.IngestToken)

	intervalCh := make(chan time.Duration, 1)
	ctrl := newControlManager(cfg.AgentID, cfg.IngestToken, rc, intervalCh)
	defer ctrl.Stop()

	ticker := time.NewTicker(rc.collectEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case d := <-intervalCh:
			if d > 0 {
				ticker.Reset(d)
			}
		case <-ticker.C:
			ts := time.Now()
			rc.mu.RLock()
			portEnabled := rc.portEnabled
			portHost := rc.portHost
			ports := append([]int(nil), rc.ports...)
			tcpEnabled := rc.tcpEnabled
			rc.mu.RUnlock()

			payload := IngestPayload{
				AgentID: cfg.AgentID,
				Name:    cfg.Name,
				TSMS:    ts.UnixMilli(),
				Metrics: map[string]float64{},
			}
			payload.Meta.Cores = collector.cores
			payload.Meta.OS = runtime.GOOS
			payload.Meta.Arch = runtime.GOARCH

			sample, err := collector.collect(tcpEnabled)
			if err != nil {
				continue
			}
			for k, v := range sample.metrics {
				payload.Metrics[k] = v
			}
			if portEnabled && len(ports) > 0 {
				payload.Ports = probePorts(portHost, ports)
			}

			respCfg, err := pushOnce(ctx, client, ingestURL, cfg.IngestToken, cfg.EncryptEnabled, encKey, payload)
			if err == nil {
				rc.mu.Lock()
				if respCfg.Config.PortProbeHost != "" {
					rc.portHost = respCfg.Config.PortProbeHost
				}
				rc.portEnabled = respCfg.Config.PortProbeEnabled
				rc.tcpEnabled = respCfg.Config.TCPConnEnabled
				if respCfg.Config.Ports != nil {
					rc.ports = append([]int(nil), respCfg.Config.Ports...)
				}
				nextEvery := rc.collectEvery
				if respCfg.Config.CollectIntervalSeconds > 0 {
					nextEvery = time.Duration(respCfg.Config.CollectIntervalSeconds) * time.Second
				}
				changedInterval := nextEvery > 0 && nextEvery != rc.collectEvery
				if changedInterval {
					rc.collectEvery = nextEvery
				}
				rc.mu.Unlock()

				if changedInterval {
					notifyInterval(intervalCh, nextEvery)
				}
				ctrl.SetDesired(respCfg.Config.ControlMode, respCfg.Config.ControlPort)
			}
		}
	}
}

func notifyInterval(ch chan time.Duration, d time.Duration) {
	// Best-effort: keep only the latest interval.
	select {
	case ch <- d:
		return
	default:
	}
	select {
	case <-ch:
	default:
	}
	select {
	case ch <- d:
	default:
	}
}

func pushOnce(ctx context.Context, client *http.Client, url, token string, encrypt bool, key [32]byte, payload IngestPayload) (ConfigResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return ConfigResponse{}, err
	}
	if !encrypt {
		b, err := json.Marshal(payload)
		if err != nil {
			return ConfigResponse{}, err
		}
		req.Body = io.NopCloser(bytes.NewReader(b))
		req.ContentLength = int64(len(b))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Ingest-Token", token)
		req.Header.Set("X-Agent-ID", payload.AgentID)
	} else {
		plain, err := json.Marshal(payload)
		if err != nil {
			return ConfigResponse{}, err
		}
		msgID := make([]byte, 16)
		if _, err := rand.Read(msgID); err != nil {
			return ConfigResponse{}, err
		}
		nonce, ct, err := common.EncryptAESGCM(key, []byte(payload.AgentID), plain)
		if err != nil {
			return ConfigResponse{}, err
		}
		req.Body = io.NopCloser(bytes.NewReader(ct))
		req.ContentLength = int64(len(ct))
		req.Header.Set("Content-Type", "application/octet-stream")
		req.Header.Set("X-Ingest-Enc", "aesgcm")
		req.Header.Set("X-Agent-ID", payload.AgentID)
		req.Header.Set("X-Nonce", base64.RawStdEncoding.EncodeToString(nonce))
		req.Header.Set("X-Msg-ID", base64.RawStdEncoding.EncodeToString(msgID))
	}

	resp, err := client.Do(req)
	if err != nil {
		return ConfigResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ConfigResponse{}, errors.New(resp.Status)
	}
	var cr ConfigResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return ConfigResponse{}, fmt.Errorf("decode response: %w", err)
	}
	if !cr.OK {
		return ConfigResponse{}, errors.New("server rejected ingest")
	}
	return cr, nil
}

func stringsTrimRightSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}
