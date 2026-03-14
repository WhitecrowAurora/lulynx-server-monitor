package center

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const defaultControlPort = 38088

type controlPingResp struct {
	OK bool `json:"ok"`
}

func effectiveControlPort(c ServerConfig) int {
	if c.ControlPort > 0 && c.ControlPort <= 65535 {
		return c.ControlPort
	}
	return defaultControlPort
}

func (s *Service) startControlProbeLoop() {
	ticker := time.NewTicker(12 * time.Second)
	client := &http.Client{Timeout: 900 * time.Millisecond}
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-s.closed:
				return
			case <-ticker.C:
				s.probeControlOnce(client)
			}
		}
	}()
}

func (s *Service) probeControlOnce(client *http.Client) {
	if s == nil || client == nil {
		return
	}
	nowMS := time.Now().UnixMilli()
	type item struct {
		id    string
		ip    string
		port  int
		token string
	}
	var items []item

	s.mu.RLock()
	for id, ss := range s.servers {
		if ss == nil {
			continue
		}
		if strings.ToLower(strings.TrimSpace(ss.cfg.ControlMode)) != "active" {
			continue
		}
		ip := strings.TrimSpace(ss.lastIP)
		if ip == "" {
			continue
		}
		intervalSec := s.effectiveCollectIntervalSecondsLocked(ss.cfg)
		thresholdSec := intervalSec * 3
		if thresholdSec < 30 {
			thresholdSec = 30
		}
		isOnline := ss.lastSeenMS > 0 && (nowMS-ss.lastSeenMS) <= int64(thresholdSec)*1000
		if !isOnline {
			continue
		}
		tok := strings.TrimSpace(s.secrets[id])
		if tok == "" {
			tok = strings.TrimSpace(s.cfg.IngestToken)
		}
		if tok == "" {
			continue
		}
		items = append(items, item{
			id:    id,
			ip:    ip,
			port:  effectiveControlPort(ss.cfg),
			token: tok,
		})
	}
	s.mu.RUnlock()

	if len(items) == 0 {
		return
	}

	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	for _, it := range items {
		it := it
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			ok := s.probeOneControl(client, it.id, it.ip, it.port, it.token)
			<-sem

			s.mu.Lock()
			if ss := s.servers[it.id]; ss != nil {
				ss.controlOK = ok
				ss.controlLastCheckMS = time.Now().UnixMilli()
			}
			s.mu.Unlock()
		}()
	}
	wg.Wait()
}

func (s *Service) probeOneControl(client *http.Client, agentID, ip string, port int, token string) bool {
	if client == nil {
		return false
	}
	if strings.TrimSpace(ip) == "" || port <= 0 || port > 65535 || strings.TrimSpace(token) == "" {
		return false
	}
	host := ip
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		host = "[" + host + "]"
	}
	u := "http://" + host + ":" + strconv.Itoa(port) + "/api/control/ping"
	ctx, cancel := context.WithTimeout(context.Background(), 900*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return false
	}
	req.Header.Set("X-Agent-ID", agentID)
	req.Header.Set("X-Agent-Token", token)

	res, err := client.Do(req)
	if err != nil {
		return false
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return false
	}
	var out controlPingResp
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return false
	}
	return out.OK
}

