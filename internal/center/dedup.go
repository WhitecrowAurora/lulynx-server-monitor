package center

import (
	"sync"
	"time"
)

type dedupStore struct {
	mu          sync.Mutex
	nextCleanup time.Time
	ttl         time.Duration
	byAgent     map[string]map[string]int64
}

func newDedupStore(ttl time.Duration) *dedupStore {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	return &dedupStore{
		ttl:     ttl,
		byAgent: map[string]map[string]int64{},
	}
}

// Seen returns true if msgID has been seen recently for that agent; otherwise stores it and returns false.
func (d *dedupStore) Seen(agentID, msgID string, now time.Time) bool {
	if agentID == "" || msgID == "" {
		return false
	}
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.nextCleanup.IsZero() || now.After(d.nextCleanup) {
		d.cleanupLocked(now)
		d.nextCleanup = now.Add(1 * time.Minute)
	}

	m := d.byAgent[agentID]
	if m == nil {
		m = map[string]int64{}
		d.byAgent[agentID] = m
	}
	if exp, ok := m[msgID]; ok && exp >= now.UnixMilli() {
		return true
	}
	m[msgID] = now.Add(d.ttl).UnixMilli()
	return false
}

func (d *dedupStore) cleanupLocked(now time.Time) {
	nowMS := now.UnixMilli()
	for agentID, m := range d.byAgent {
		for k, exp := range m {
			if exp < nowMS {
				delete(m, k)
			}
		}
		if len(m) == 0 {
			delete(d.byAgent, agentID)
		}
	}
}

