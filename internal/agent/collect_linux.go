//go:build linux

package agent

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type sample struct {
	metrics map[string]float64
}

type collector struct {
	diskMount string
	netIface  string
	cores     int

	prevCPUTotal uint64
	prevCPUIdle  uint64
	haveCPUPrev  bool

	prevNetRX uint64
	prevNetTX uint64
	prevNetAt time.Time
	haveNet   bool
}

func newCollector(diskMount, netIface string) *collector {
	return &collector{
		diskMount: diskMount,
		netIface:  netIface,
		cores:     runtime.NumCPU(),
	}
}

func (c *collector) collect(includeTCP bool) (sample, error) {
	now := time.Now()
	out := sample{metrics: map[string]float64{}}

	if err := c.collectCPU(out.metrics); err != nil {
		return sample{}, err
	}
	if err := collectMem(out.metrics); err != nil {
		return sample{}, err
	}
	if err := c.collectDisk(out.metrics); err != nil {
		return sample{}, err
	}
	if err := c.collectNet(now, out.metrics); err != nil {
		return sample{}, err
	}
	collectLoad(out.metrics)
	collectUptime(out.metrics)
	if includeTCP {
		collectTCP(out.metrics)
	}
	return out, nil
}

func (c *collector) collectCPU(m map[string]float64) error {
	b, err := os.ReadFile("/proc/stat")
	if err != nil {
		return err
	}
	lineEnd := bytes.IndexByte(b, '\n')
	if lineEnd <= 0 {
		return errors.New("proc/stat: missing first line")
	}
	fields := strings.Fields(string(b[:lineEnd]))
	if len(fields) < 5 || fields[0] != "cpu" {
		return errors.New("proc/stat: invalid first line")
	}
	var total uint64
	var idle uint64
	for i := 1; i < len(fields); i++ {
		v, err := strconv.ParseUint(fields[i], 10, 64)
		if err != nil {
			return fmt.Errorf("proc/stat: %w", err)
		}
		total += v
		if i == 4 || i == 5 { // idle + iowait
			idle += v
		}
	}

	if c.haveCPUPrev && total > c.prevCPUTotal {
		dTotal := total - c.prevCPUTotal
		dIdle := idle - c.prevCPUIdle
		if dTotal > 0 && dIdle <= dTotal {
			usage := (1 - (float64(dIdle) / float64(dTotal))) * 100
			if usage < 0 {
				usage = 0
			}
			if usage > 100 {
				usage = 100
			}
			m["cpu_pct"] = usage
		}
	} else {
		m["cpu_pct"] = 0
	}
	c.prevCPUTotal = total
	c.prevCPUIdle = idle
	c.haveCPUPrev = true
	m["cpu_cores"] = float64(c.cores)
	return nil
}

func collectMem(m map[string]float64) error {
	b, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return err
	}
	var memTotal, memAvail, swapTotal, swapFree uint64
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			memTotal = parseMeminfoKB(line)
		} else if strings.HasPrefix(line, "MemAvailable:") {
			memAvail = parseMeminfoKB(line)
		} else if strings.HasPrefix(line, "SwapTotal:") {
			swapTotal = parseMeminfoKB(line)
		} else if strings.HasPrefix(line, "SwapFree:") {
			swapFree = parseMeminfoKB(line)
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	memUsed := uint64(0)
	if memTotal > memAvail {
		memUsed = memTotal - memAvail
	}
	swapUsed := uint64(0)
	if swapTotal > swapFree {
		swapUsed = swapTotal - swapFree
	}
	m["mem_total_bytes"] = float64(memTotal)
	m["mem_used_bytes"] = float64(memUsed)
	m["swap_total_bytes"] = float64(swapTotal)
	m["swap_used_bytes"] = float64(swapUsed)
	return nil
}

func parseMeminfoKB(line string) uint64 {
	// example: "MemTotal:       16367472 kB"
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0
	}
	v, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return 0
	}
	return v * 1024
}

func (c *collector) collectDisk(m map[string]float64) error {
	var st syscall.Statfs_t
	if err := syscall.Statfs(c.diskMount, &st); err != nil {
		return err
	}
	total := uint64(st.Blocks) * uint64(st.Bsize)
	free := uint64(st.Bavail) * uint64(st.Bsize)
	used := uint64(0)
	if total > free {
		used = total - free
	}
	m["disk_total_bytes"] = float64(total)
	m["disk_used_bytes"] = float64(used)
	return nil
}

func (c *collector) collectNet(now time.Time, m map[string]float64) error {
	rx, tx, err := readNetBytes(c.netIface)
	if err != nil {
		return err
	}
	m["net_rx_total_bytes"] = float64(rx)
	m["net_tx_total_bytes"] = float64(tx)

	if c.haveNet {
		dt := now.Sub(c.prevNetAt).Seconds()
		if dt > 0 {
			drx := float64(0)
			dtx := float64(0)
			if rx >= c.prevNetRX {
				drx = float64(rx-c.prevNetRX) / dt
			}
			if tx >= c.prevNetTX {
				dtx = float64(tx-c.prevNetTX) / dt
			}
			m["net_rx_bps"] = drx
			m["net_tx_bps"] = dtx
		}
	} else {
		m["net_rx_bps"] = 0
		m["net_tx_bps"] = 0
	}
	c.prevNetRX = rx
	c.prevNetTX = tx
	c.prevNetAt = now
	c.haveNet = true
	return nil
}

func readNetBytes(iface string) (rx uint64, tx uint64, err error) {
	b, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return 0, 0, err
	}
	sc := bufio.NewScanner(bytes.NewReader(b))
	// Skip first two header lines.
	for i := 0; i < 2 && sc.Scan(); i++ {
	}
	var bestIface string
	var bestSum uint64
	var bestRX, bestTX uint64
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		if name == "lo" {
			continue
		}
		fields := strings.Fields(strings.TrimSpace(parts[1]))
		// rx bytes is field[0], tx bytes is field[8]
		if len(fields) < 9 {
			continue
		}
		rxb, err1 := strconv.ParseUint(fields[0], 10, 64)
		txb, err2 := strconv.ParseUint(fields[8], 10, 64)
		if err1 != nil || err2 != nil {
			continue
		}
		if iface != "" {
			if name == iface {
				return rxb, txb, nil
			}
			continue
		}
		sum := rxb + txb
		if sum >= bestSum {
			bestSum = sum
			bestIface = name
			bestRX = rxb
			bestTX = txb
		}
	}
	if err := sc.Err(); err != nil {
		return 0, 0, err
	}
	if iface != "" {
		return 0, 0, fmt.Errorf("net iface %q not found", iface)
	}
	if bestIface == "" {
		return 0, 0, errors.New("no net iface found")
	}
	return bestRX, bestTX, nil
}

func collectLoad(m map[string]float64) {
	b, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return
	}
	fields := strings.Fields(string(b))
	if len(fields) < 3 {
		return
	}
	if v, err := strconv.ParseFloat(fields[0], 64); err == nil {
		m["load1"] = v
	}
	if v, err := strconv.ParseFloat(fields[1], 64); err == nil {
		m["load5"] = v
	}
	if v, err := strconv.ParseFloat(fields[2], 64); err == nil {
		m["load15"] = v
	}
}

func collectUptime(m map[string]float64) {
	b, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return
	}
	fields := strings.Fields(string(b))
	if len(fields) < 1 {
		return
	}
	if v, err := strconv.ParseFloat(fields[0], 64); err == nil {
		m["uptime_seconds"] = v
	}
}

func collectTCP(m map[string]float64) {
	total, established := countTCP("/proc/net/tcp")
	total6, established6 := countTCP("/proc/net/tcp6")
	m["tcp_conn_total"] = float64(total + total6)
	m["tcp_conn_established"] = float64(established + established6)
}

func countTCP(path string) (total int, established int) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	first := true
	for sc.Scan() {
		if first {
			first = false
			continue
		}
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		state := fields[3]
		total++
		if state == "01" {
			established++
		}
	}
	return total, established
}

func probePorts(host string, ports []int) []PortStatus {
	if host == "" {
		host = "127.0.0.1"
	}
	out := make([]PortStatus, 0, len(ports))
	dialer := &net.Dialer{Timeout: 500 * time.Millisecond}
	for _, p := range ports {
		if p <= 0 || p > 65535 {
			continue
		}
		addr := net.JoinHostPort(host, strconv.Itoa(p))
		start := time.Now()
		conn, err := dialer.Dial("tcp", addr)
		lat := time.Since(start).Milliseconds()
		ok := err == nil
		if conn != nil {
			_ = conn.Close()
		}
		out = append(out, PortStatus{Port: p, OK: ok, LatencyMS: lat})
	}
	return out
}

