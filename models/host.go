package models

import "time"

type HostInfoResponse struct {
	RecommendedContainerHostIP string   `json:"recommendedContainerHostIP"`
	LanIPs                     []string `json:"lanIPs"`
	BridgeIPs                  []string `json:"bridgeIPs"`
}

type HostMetricsPoint struct {
	Timestamp        time.Time `json:"timestamp"`
	CPUPercent       float64   `json:"cpuPercent"`
	CPUMHz           float64   `json:"cpuMhz"`
	TemperatureC     float64   `json:"temperatureC"`
	MemoryUsedBytes  int64     `json:"memoryUsedBytes"`
	MemoryTotalBytes int64     `json:"memoryTotalBytes"`
	NetworkRxBps     float64   `json:"networkRxBps"`
	NetworkTxBps     float64   `json:"networkTxBps"`
	NetworkRxBytes   int64     `json:"networkRxBytes"`
	NetworkTxBytes   int64     `json:"networkTxBytes"`
}

type HostProcess struct {
	PID           int     `json:"pid"`
	User          string  `json:"user"`
	Command       string  `json:"command"`
	CPUPercent    float64 `json:"cpuPercent"`
	MemoryBytes   int64   `json:"memoryBytes"`
	MemoryPercent float64 `json:"memoryPercent"`
	OwnerType     string  `json:"ownerType"`
	ContainerName string  `json:"containerName"`
}

type HostMemoryModule struct {
	Locator      string `json:"locator"`
	SizeBytes    int64  `json:"sizeBytes"`
	Manufacturer string `json:"manufacturer"`
	PartNumber   string `json:"partNumber"`
	Speed        string `json:"speed"`
}

type HostCPUInfo struct {
	Index      int     `json:"index"`
	Model      string  `json:"model"`
	Cores      int     `json:"cores"`
	Threads    int     `json:"threads"`
	CurrentMHz float64 `json:"currentMhz"`
	MaxMHz     float64 `json:"maxMhz"`
}

type HostSystemOverview struct {
	Hostname             string             `json:"hostname"`
	OS                   string             `json:"os"`
	UptimeSeconds        int64              `json:"uptimeSeconds"`
	CPUModel             string             `json:"cpuModel"`
	CPUCores             int                `json:"cpuCores"`
	CPUThreads           int                `json:"cpuThreads"`
	CPUMHz               float64            `json:"cpuMhz"`
	CPUs                 []HostCPUInfo      `json:"cpus"`
	GPUs                 []string           `json:"gpus"`
	MemoryTotalBytes     int64              `json:"memoryTotalBytes"`
	MemoryInstalledBytes int64              `json:"memoryInstalledBytes"`
	MemoryModules        []HostMemoryModule `json:"memoryModules"`
}

type HostMetricsResponse struct {
	Range           string             `json:"range"`
	IntervalSeconds int                `json:"intervalSeconds"`
	UpdatedAt       time.Time          `json:"updatedAt"`
	System          HostSystemOverview `json:"system"`
	Summary         HostMetricsPoint   `json:"summary"`
	Points          []HostMetricsPoint `json:"points"`
	Processes       []HostProcess      `json:"processes"`
}
