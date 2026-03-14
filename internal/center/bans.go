package center

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type banEntry struct {
	IP            string `json:"ip"`
	FailCount     int    `json:"fail_count"`
	BannedUntilMS int64  `json:"banned_until_ms"`
	LastFailMS    int64  `json:"last_fail_ms"`
}

type bansFile struct {
	Entries []banEntry `json:"entries"`
}

type banStore struct {
	mu        sync.Mutex
	maxFails  int
	banFor    time.Duration
	entries   map[string]*banEntry
	dataDir   string
	fileName  string
}

func newBanStore(maxFails int, banFor time.Duration, fileName string) *banStore {
	if maxFails <= 0 {
		maxFails = 5
	}
	if banFor <= 0 {
		banFor = 8 * time.Hour
	}
	fileName = strings.TrimSpace(fileName)
	if fileName == "" {
		fileName = "bans.json"
	}
	return &banStore{
		maxFails: maxFails,
		banFor:   banFor,
		entries:  map[string]*banEntry{},
		fileName: fileName,
	}
}

func (b *banStore) path() string {
	if strings.TrimSpace(b.dataDir) == "" {
		return ""
	}
	return filepath.Join(b.dataDir, b.fileName)
}

func (b *banStore) BindDataDir(dataDir string) {
	b.mu.Lock()
	b.dataDir = dataDir
	b.mu.Unlock()
}

func (b *banStore) Load() error {
	path := b.path()
	if path == "" {
		return nil
	}
	var f bansFile
	if err := loadJSONFile(path, &f); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, e := range f.Entries {
		ip := strings.TrimSpace(e.IP)
		if ip == "" {
			continue
		}
		cp := e
		b.entries[ip] = &cp
	}
	return nil
}

func (b *banStore) SaveLocked() {
	path := b.path()
	if path == "" {
		return
	}
	out := bansFile{Entries: make([]banEntry, 0, len(b.entries))}
	for _, e := range b.entries {
		if e == nil || strings.TrimSpace(e.IP) == "" {
			continue
		}
		out.Entries = append(out.Entries, *e)
	}
	_ = saveJSONFilePrivate(path, out)
}

func (b *banStore) IsBanned(ip string, now time.Time) (bool, time.Time) {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return false, time.Time{}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	e := b.entries[ip]
	if e == nil || e.BannedUntilMS <= 0 {
		return false, time.Time{}
	}
	until := time.UnixMilli(e.BannedUntilMS)
	if now.Before(until) {
		return true, until
	}
	return false, time.Time{}
}

func (b *banStore) RegisterFail(ip string, now time.Time) (banned bool, until time.Time) {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return false, time.Time{}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	e := b.entries[ip]
	if e == nil {
		e = &banEntry{IP: ip}
		b.entries[ip] = e
	}
	e.FailCount++
	e.LastFailMS = now.UnixMilli()
	if e.FailCount >= b.maxFails {
		u := now.Add(b.banFor)
		e.BannedUntilMS = u.UnixMilli()
		b.SaveLocked()
		return true, u
	}
	b.SaveLocked()
	return false, time.Time{}
}

func (b *banStore) Reset(ip string) {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.entries, ip)
	b.SaveLocked()
}

func (b *banStore) List(now time.Time) []banEntry {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]banEntry, 0, len(b.entries))
	for _, e := range b.entries {
		if e == nil || strings.TrimSpace(e.IP) == "" {
			continue
		}
		// Keep expired entries; UI can clear.
		out = append(out, *e)
	}
	return out
}

func (s *Service) loadBans() error {
	if s == nil || s.enrollBans == nil {
		return nil
	}
	s.enrollBans.BindDataDir(s.cfg.DataDir)
	_ = s.enrollBans.Load()
	if s.adminBans != nil {
		s.adminBans.BindDataDir(s.cfg.DataDir)
		_ = s.adminBans.Load()
	}
	return nil
}
