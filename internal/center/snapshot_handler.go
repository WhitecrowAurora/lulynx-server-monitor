package center

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/WhitecrowAurora/lulynx-server-monitor/internal/agent"
)

func (s *Service) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	nowMS := time.Now().UnixMilli()

	s.mu.RLock()
	settings := s.settings
	monthMin := monthMinFor(settings.RetentionDays)

	total := 0
	online := 0
	regionSet := map[string]struct{}{}
	nowRXBps := 0.0
	nowTXBps := 0.0

	out := make([]serverView, 0, len(s.servers))
	for id, ss := range s.servers {
		if !isVisible(ss.cfg) {
			continue
		}
		total++

		intervalSec := s.effectiveCollectIntervalSecondsLocked(ss.cfg)
		thresholdSec := intervalSec * 3
		if thresholdSec < 30 {
			thresholdSec = 30
		}
		isOnline := ss.lastSeenMS > 0 && (nowMS-ss.lastSeenMS) <= int64(thresholdSec)*1000
		if isOnline {
			online++
			nowRXBps += ss.metrics["net_rx_bps"]
			nowTXBps += ss.metrics["net_tx_bps"]
		}
		if r := strings.TrimSpace(ss.cfg.Region); r != "" {
			regionSet[r] = struct{}{}
		}

		v := serverView{
			ID:                id,
			Name:              ss.cfg.Name,
			Region:            ss.cfg.Region,
			Tags:              append([]string(nil), ss.cfg.Tags...),
			Online:            isOnline,
			LastSeenMS:        ss.lastSeenMS,
			DashboardWidgets:  append([]string(nil), ss.cfg.DashboardWidgets...),
			ControlMode:       ss.cfg.ControlMode,
			ControlPort:       ss.cfg.ControlPort,
			ControlOK:         ss.controlOK,
			ExpiresText:       ss.cfg.ExpiresText,
			TrafficTotalBytes: ss.cfg.TrafficTotalBytes,
			Meta:              ss.meta,
			Metrics:           copyMetrics(ss.metrics),
			Ports:             append([]agent.PortStatus(nil), ss.ports...),
		}
		v.ExpiresDate, v.RenewDays = computeRenew(nowMS, ss.cfg)
		v.TrafficRenewDate, v.TrafficRenewDays = computeTrafficRenew(nowMS, ss.cfg)
		v.TrafficUsedBytes = s.serverTrafficUsedBytes(nowMS, monthMin, ss)

		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Online != out[j].Online {
			return out[i].Online
		}
		return out[i].Name < out[j].Name
	})

	windows := map[string]aggWin{}
	addWindow := func(key string, minutes int64) {
		rx, tx := s.roll.SumLastMinutes(nowMS, minutes)
		sec := float64(minutes) * 60
		avgRX := 0.0
		avgTX := 0.0
		if sec > 0 {
			avgRX = float64(rx) / sec
			avgTX = float64(tx) / sec
		}
		windows[key] = aggWin{RXBytes: rx, TXBytes: tx, AvgRXBps: avgRX, AvgTXBps: avgTX, Minutes: minutes}
	}
	addWindow("1d", 1440)
	addWindow("1w", 10080)
	addWindow("1m", monthMin)
	s.mu.RUnlock()

	resp := snapshotResp{
		OK:       true,
		NowMS:    nowMS,
		Settings: settings,
		Totals:   snapshotTop{TotalServers: total, OnlineServers: online, RegionCount: len(regionSet)},
		Traffic:  trafficTop{NowRXBps: nowRXBps, NowTXBps: nowTXBps, Windows: windows},
		Servers:  out,
	}
	writeJSON(w, resp)
}

func (s *Service) serverTrafficUsedBytes(nowMS int64, monthMin int64, ss *serverState) uint64 {
	if ss == nil || ss.roll == nil {
		return 0
	}
	if strings.TrimSpace(ss.cfg.TrafficRenewDate) != "" {
		if t, err := time.ParseInLocation("2006-01-02", ss.cfg.TrafficRenewDate, time.UTC); err == nil {
			startMS := t.UnixMilli()
			if startMS > 0 && startMS < nowMS {
				minutes := int64((nowMS - startMS) / 60000)
				if minutes <= 0 {
					minutes = 1
				}
				if minutes > monthMin {
					minutes = monthMin
				}
				rx, tx := ss.roll.SumLastMinutes(nowMS, minutes)
				return rx + tx
			}
		}
	}
	rx, tx := ss.roll.SumLastMinutes(nowMS, monthMin)
	return rx + tx
}
