package center

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type appendPoint struct {
	serverID string
	metric   string
	tsMS     int64
	value    float64
}

type SeriesStore struct {
	rootDir string
	ch      chan appendPoint
	wg      sync.WaitGroup
	quit    chan struct{}
}

func NewSeriesStore(rootDir string) *SeriesStore {
	s := &SeriesStore{
		rootDir: rootDir,
		ch:      make(chan appendPoint, 8192),
		quit:    make(chan struct{}),
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.runWriter()
	}()
	return s
}

func (s *SeriesStore) Close() {
	close(s.quit)
	s.wg.Wait()
}

func (s *SeriesStore) Append(serverID, metric string, tsMS int64, value float64) {
	if serverID == "" || metric == "" || tsMS <= 0 {
		return
	}
	select {
	case s.ch <- appendPoint{serverID: serverID, metric: metric, tsMS: tsMS, value: value}:
	default:
		// drop on overload to protect ingest path
	}
}

func (s *SeriesStore) runWriter() {
	buf := make([]byte, 16)
	for {
		select {
		case <-s.quit:
			return
		case p := <-s.ch:
			if p.serverID == "" {
				continue
			}
			dir := filepath.Join(s.rootDir, sanitizePath(p.serverID), sanitizePath(p.metric))
			if err := os.MkdirAll(dir, 0o755); err != nil {
				continue
			}
			t := time.UnixMilli(p.tsMS).UTC()
			seg := t.Format("2006010215") + ".bin"
			path := filepath.Join(dir, seg)
			f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
			if err != nil {
				continue
			}
			binary.LittleEndian.PutUint64(buf[0:8], uint64(p.tsMS))
			binary.LittleEndian.PutUint64(buf[8:16], math.Float64bits(p.value))
			_, _ = f.Write(buf)
			_ = f.Close()
		}
	}
}

func (s *SeriesStore) CleanupOlderThan(cutoff time.Time) error {
	if cutoff.IsZero() {
		return nil
	}
	root := s.rootDir
	entries, err := os.ReadDir(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, srv := range entries {
		if !srv.IsDir() {
			continue
		}
		srvDir := filepath.Join(root, srv.Name())
		metrics, _ := os.ReadDir(srvDir)
		for _, met := range metrics {
			if !met.IsDir() {
				continue
			}
			metDir := filepath.Join(srvDir, met.Name())
			files, _ := os.ReadDir(metDir)
			for _, f := range files {
				name := f.Name()
				if !strings.HasSuffix(name, ".bin") || len(name) < len("2006010215.bin") {
					continue
				}
				tsPart := strings.TrimSuffix(name, ".bin")
				t, err := time.ParseInLocation("2006010215", tsPart, time.UTC)
				if err != nil {
					continue
				}
				// file contains points for that hour; remove if the hour ends before cutoff
				if t.Add(time.Hour).Before(cutoff) {
					_ = os.Remove(filepath.Join(metDir, name))
				}
			}
		}
	}
	return nil
}

type SeriesResult struct {
	StartMS int64     `json:"start_ms"`
	EndMS   int64     `json:"end_ms"`
	Points  []XYPoint `json:"points"`
}

type XYPoint struct {
	TSMS  int64   `json:"ts_ms"`
	Value float64 `json:"value"`
}

func (s *SeriesStore) Query(serverID, metric string, start, end time.Time, maxPoints int) (SeriesResult, error) {
	if maxPoints <= 0 {
		maxPoints = 300
	}
	if end.Before(start) {
		start, end = end, start
	}
	if serverID == "" || metric == "" {
		return SeriesResult{}, fmt.Errorf("missing server or metric")
	}
	dir := filepath.Join(s.rootDir, sanitizePath(serverID), sanitizePath(metric))
	files, err := os.ReadDir(dir)
	if err != nil {
		return SeriesResult{StartMS: start.UnixMilli(), EndMS: end.UnixMilli()}, nil
	}
	var segs []string
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		if strings.HasSuffix(f.Name(), ".bin") {
			segs = append(segs, filepath.Join(dir, f.Name()))
		}
	}
	sort.Strings(segs)

	startMS := start.UnixMilli()
	endMS := end.UnixMilli()
	raw := make([]XYPoint, 0, 1024)

	for _, path := range segs {
		// quick filter by segment hour
		base := filepath.Base(path)
		tsPart := strings.TrimSuffix(base, ".bin")
		t, err := time.ParseInLocation("2006010215", tsPart, time.UTC)
		if err == nil {
			segStart := t.UnixMilli()
			segEnd := t.Add(time.Hour).UnixMilli()
			if segEnd < startMS || segStart > endMS {
				continue
			}
		}

		pts, err := readAllPoints(path, startMS, endMS)
		if err != nil {
			continue
		}
		raw = append(raw, pts...)
	}
	if len(raw) == 0 {
		return SeriesResult{StartMS: startMS, EndMS: endMS, Points: nil}, nil
	}
	sort.Slice(raw, func(i, j int) bool { return raw[i].TSMS < raw[j].TSMS })
	down := downsampleBuckets(raw, startMS, endMS, maxPoints)
	return SeriesResult{StartMS: startMS, EndMS: endMS, Points: down}, nil
}

func readAllPoints(path string, startMS, endMS int64) ([]XYPoint, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf := make([]byte, 16)
	out := make([]XYPoint, 0, 1024)
	for {
		_, err := io.ReadFull(f, buf)
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				break
			}
			return out, err
		}
		ts := int64(binary.LittleEndian.Uint64(buf[0:8]))
		if ts < startMS || ts > endMS {
			continue
		}
		v := math.Float64frombits(binary.LittleEndian.Uint64(buf[8:16]))
		out = append(out, XYPoint{TSMS: ts, Value: v})
	}
	return out, nil
}

func downsampleBuckets(points []XYPoint, startMS, endMS int64, maxPoints int) []XYPoint {
	if len(points) <= maxPoints {
		return points
	}
	if endMS <= startMS {
		return points[:maxPoints]
	}
	buckets := maxPoints
	if buckets < 10 {
		buckets = 10
	}
	span := float64(endMS - startMS)
	step := span / float64(buckets)
	if step <= 0 {
		return points[:maxPoints]
	}
	type agg struct {
		sum float64
		cnt int
		ts  int64
	}
	acc := make([]agg, buckets)
	for _, p := range points {
		idx := int(float64(p.TSMS-startMS) / step)
		if idx < 0 {
			idx = 0
		}
		if idx >= buckets {
			idx = buckets - 1
		}
		acc[idx].sum += p.Value
		acc[idx].cnt++
		acc[idx].ts = p.TSMS
	}
	out := make([]XYPoint, 0, buckets)
	for i := 0; i < buckets; i++ {
		if acc[i].cnt == 0 {
			continue
		}
		out = append(out, XYPoint{TSMS: acc[i].ts, Value: acc[i].sum / float64(acc[i].cnt)})
	}
	if len(out) > maxPoints {
		out = out[len(out)-maxPoints:]
	}
	return out
}

func sanitizePath(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "..", "_")
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "\\", "_")
	if s == "" {
		return "_"
	}
	return s
}
