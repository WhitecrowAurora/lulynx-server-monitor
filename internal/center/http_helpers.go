package center

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/WhitecrowAurora/lulynx-server-monitor/internal/agent"
)

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	_ = enc.Encode(v)
}

func copyMetrics(in map[string]float64) map[string]float64 {
	if in == nil {
		return map[string]float64{}
	}
	out := make(map[string]float64, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func computeRenew(nowMS int64, cfg ServerConfig) (dateDisplay string, days *int) {
	if strings.TrimSpace(cfg.ExpiresDate) == "" {
		if strings.TrimSpace(cfg.ExpiresText) == "" {
			return "", nil
		}
		return "", nil
	}
	t, err := time.ParseInLocation("2006-01-02", cfg.ExpiresDate, time.UTC)
	if err != nil {
		return "", nil
	}
	dateDisplay = t.Format("2006/01/02")
	now := time.UnixMilli(nowMS).UTC()
	nowDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	delta := int(t.Sub(nowDate).Hours() / 24)
	days = &delta
	return dateDisplay, days
}

func computeTrafficRenew(nowMS int64, cfg ServerConfig) (dateDisplay string, days *int) {
	if strings.TrimSpace(cfg.TrafficRenewDate) == "" {
		return "", nil
	}
	t, err := time.ParseInLocation("2006-01-02", cfg.TrafficRenewDate, time.UTC)
	if err != nil {
		return "", nil
	}
	dateDisplay = t.Format("2006/01/02")
	now := time.UnixMilli(nowMS).UTC()
	nowDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	delta := int(t.Sub(nowDate).Hours() / 24)
	days = &delta
	return dateDisplay, days
}

func monthMinFor(retentionDays int) int64 {
	monthMin := int64(43200)
	if retentionDays > 0 {
		retMin := int64(retentionDays) * 1440
		if retMin < monthMin {
			monthMin = retMin
		}
	}
	return monthMin
}

type settingsPatch struct {
	DefaultCollectIntervalSeconds *int      `json:"default_collect_interval_seconds,omitempty"`
	RetentionDays                 *int      `json:"retention_days,omitempty"`
	DashboardPollSeconds          *int      `json:"dashboard_poll_seconds,omitempty"`
	EnableGrouping                *bool     `json:"enable_grouping,omitempty"`
	TapeFields                    *[]string `json:"tape_fields,omitempty"`
}

type snapshotResp struct {
	OK       bool         `json:"ok"`
	NowMS    int64        `json:"now_ms"`
	Settings Settings     `json:"settings"`
	Totals   snapshotTop  `json:"totals"`
	Traffic  trafficTop   `json:"traffic"`
	Servers  []serverView `json:"servers"`
}

type snapshotTop struct {
	TotalServers  int `json:"total_servers"`
	OnlineServers int `json:"online_servers"`
	RegionCount   int `json:"region_count"`
}

type trafficTop struct {
	NowRXBps float64           `json:"now_rx_bps"`
	NowTXBps float64           `json:"now_tx_bps"`
	Windows  map[string]aggWin `json:"windows"`
}

type aggWin struct {
	RXBytes  uint64  `json:"rx_bytes"`
	TXBytes  uint64  `json:"tx_bytes"`
	AvgRXBps float64 `json:"avg_rx_bps"`
	AvgTXBps float64 `json:"avg_tx_bps"`
	Minutes  int64   `json:"minutes"`
}

type serverView struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Region     string   `json:"region,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	Online     bool     `json:"online"`
	LastSeenMS int64    `json:"last_seen_ms"`

	DashboardWidgets []string `json:"dashboard_widgets,omitempty"`

	ControlMode string `json:"control_mode,omitempty"` // passive|active
	ControlPort int    `json:"control_port,omitempty"`
	ControlOK   bool   `json:"control_ok,omitempty"`

	ExpiresText string `json:"expires_text,omitempty"`
	ExpiresDate string `json:"expires_date,omitempty"` // YYYY/MM/DD
	RenewDays   *int   `json:"renew_days,omitempty"`

	TrafficTotalBytes uint64 `json:"traffic_total_bytes,omitempty"`
	TrafficUsedBytes  uint64 `json:"traffic_used_bytes,omitempty"`
	TrafficRenewDate  string `json:"traffic_renew_date,omitempty"` // YYYY/MM/DD
	TrafficRenewDays  *int   `json:"traffic_renew_days,omitempty"`

	Meta    serverMeta         `json:"meta"`
	Metrics map[string]float64 `json:"metrics"`
	Ports   []agent.PortStatus `json:"ports,omitempty"`
}
