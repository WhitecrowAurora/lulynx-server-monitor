//go:build !linux

package agent

import (
	"errors"
	"time"
)

type sample struct {
	metrics map[string]float64
}

type collector struct {
	diskMount string
	netIface  string
	cores     int
}

func newCollector(diskMount, netIface string) *collector {
	return &collector{diskMount: diskMount, netIface: netIface, cores: 0}
}

func (c *collector) collect(includeTCP bool) (sample, error) {
	return sample{}, errors.New("agent collector is only supported on linux")
}

func probePorts(host string, ports []int) []PortStatus {
	_ = host
	_ = ports
	return nil
}

func (c *collector) collectNet(now time.Time, m map[string]float64) error {
	_ = now
	_ = m
	return nil
}

