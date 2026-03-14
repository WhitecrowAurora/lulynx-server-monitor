package agent

type PortStatus struct {
	Port      int   `json:"port"`
	OK        bool  `json:"ok"`
	LatencyMS int64 `json:"latency_ms,omitempty"`
}

type IngestPayload struct {
	AgentID string `json:"agent_id"`
	Name    string `json:"name,omitempty"`
	TSMS    int64  `json:"ts_ms"`

	Meta struct {
		Cores int    `json:"cores,omitempty"`
		OS    string `json:"os,omitempty"`
		Arch  string `json:"arch,omitempty"`
	} `json:"meta"`

	Metrics map[string]float64 `json:"metrics"`
	Ports   []PortStatus       `json:"ports,omitempty"`
}

type ConfigResponse struct {
	OK     bool `json:"ok"`
	Config struct {
		CollectIntervalSeconds int   `json:"collect_interval_seconds"`
		PortProbeEnabled       bool  `json:"port_probe_enabled"`
		PortProbeHost          string `json:"port_probe_host"`
		Ports                  []int `json:"ports"`
		TCPConnEnabled         bool  `json:"tcp_conn_enabled"`
		ControlMode            string `json:"control_mode,omitempty"` // passive|active
		ControlPort            int    `json:"control_port,omitempty"`
	} `json:"config"`
}
