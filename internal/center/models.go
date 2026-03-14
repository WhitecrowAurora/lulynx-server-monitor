package center

type ServerConfig struct {
	ID   string `json:"id"`
	Name string `json:"name"`

	Region string `json:"region,omitempty"`
	Tags   []string `json:"tags,omitempty"`

	// ControlMode controls whether the center actively connects to the agent's control port.
	// Known values: ""/"passive", "active".
	ControlMode string `json:"control_mode,omitempty"`
	// ControlPort is the agent-side control port (TCP). If 0, default is used.
	ControlPort int `json:"control_port,omitempty"`

	// Visible controls whether the server is shown on the public dashboard.
	// nil means true for backward compatibility.
	Visible *bool `json:"visible,omitempty"`

	// DashboardWidgets is an allowlist of widgets to show on dashboard.
	// Empty means default (show all).
	// Known widgets: meta, expiry, lastseen, region, traffic_renew, cpu, mem, swap,
	// disk, net, traffic, quota, load, uptime, ports.
	DashboardWidgets []string `json:"dashboard_widgets,omitempty"`

	ExpiresText string `json:"expires_text"`
	// ExpiresDate is "YYYY-MM-DD" (UTC) when applicable.
	ExpiresDate string `json:"expires_date,omitempty"`

	// TrafficTotalBytes is the traffic quota (RX+TX) used for UI display.
	TrafficTotalBytes uint64 `json:"traffic_total_bytes,omitempty"`
	// TrafficRenewDate is "YYYY-MM-DD" (UTC). Used for "traffic renew soon" hints.
	TrafficRenewDate string `json:"traffic_renew_date,omitempty"`

	CollectIntervalSeconds *int `json:"collect_interval_seconds,omitempty"`

	PortProbeEnabled bool  `json:"port_probe_enabled"`
	PortProbeHost    string `json:"port_probe_host"`
	Ports            []int `json:"ports,omitempty"`

	TCPConnEnabled bool `json:"tcp_conn_enabled"`
}

type Settings struct {
	DefaultCollectIntervalSeconds int `json:"default_collect_interval_seconds"`
	RetentionDays                 int `json:"retention_days"`
	DashboardPollSeconds          int `json:"dashboard_poll_seconds"`
	EnableGrouping                bool `json:"enable_grouping"`

	// TapeFields controls the marquee content on the dashboard.
	// Empty means default.
	// Known fields: time, traffic_today, speed_now, conn_total, online, regions,
	// traffic_window, offline, expire_soon, traffic_renew_soon.
	TapeFields []string `json:"tape_fields,omitempty"`
}
