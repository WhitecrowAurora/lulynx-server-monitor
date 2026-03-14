package center

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (s *Service) handleSeries(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	serverID := strings.TrimSpace(r.URL.Query().Get("server"))
	metric := strings.TrimSpace(r.URL.Query().Get("metric"))
	rangeStr := strings.TrimSpace(r.URL.Query().Get("range"))
	maxPoints := 600
	if mp := strings.TrimSpace(r.URL.Query().Get("max_points")); mp != "" {
		if v, err := strconv.Atoi(mp); err == nil && v > 10 && v <= 5000 {
			maxPoints = v
		}
	}
	if serverID == "" || metric == "" {
		http.Error(w, "server and metric required", http.StatusBadRequest)
		return
	}
	now := time.Now().UTC()
	start := now.Add(-1 * time.Hour)
	switch rangeStr {
	case "", "1h":
		start = now.Add(-1 * time.Hour)
	case "6h":
		start = now.Add(-6 * time.Hour)
	case "12h":
		start = now.Add(-12 * time.Hour)
	case "1d":
		start = now.Add(-24 * time.Hour)
	case "7d":
		start = now.Add(-7 * 24 * time.Hour)
	case "30d":
		start = now.Add(-30 * 24 * time.Hour)
	default:
		http.Error(w, "invalid range", http.StatusBadRequest)
		return
	}
	res, err := s.series.Query(serverID, metric, start, now, maxPoints)
	if err != nil {
		http.Error(w, "query failed", http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "result": res})
}

