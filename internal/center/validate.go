package center

import "strings"

var allowedWidgets = map[string]struct{}{
	"meta":         {},
	"expiry":       {},
	"lastseen":     {},
	"region":       {},
	"traffic_renew": {},
	"cpu":          {},
	"mem":          {},
	"swap":         {},
	"disk":         {},
	"net":          {},
	"traffic":      {},
	"quota":        {},
	"load":         {},
	"uptime":       {},
	"ports":        {},
}

var allowedTapeFields = map[string]struct{}{
	"time":              {},
	"traffic_today":     {},
	"speed_now":         {},
	"conn_total":        {},
	"online":            {},
	"regions":           {},
	"traffic_window":    {},
	"offline":           {},
	"expire_soon":       {},
	"traffic_renew_soon": {},
}

func normalizeWidgets(in []string) []string {
	return normalizeKnownList(in, allowedWidgets, 32)
}

func normalizeTapeFields(in []string) []string {
	return normalizeKnownList(in, allowedTapeFields, 24)
}

func normalizeTags(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		s = strings.ToLower(s)
		if len(s) > 32 {
			s = s[:32]
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
		if len(out) >= 12 {
			break
		}
	}
	return out
}

func normalizeKnownList(in []string, allow map[string]struct{}, limit int) []string {
	if limit <= 0 {
		limit = 32
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := allow[s]; !ok {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
		if len(out) >= limit {
			break
		}
	}
	return out
}
