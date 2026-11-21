package models

import "time"

type GPUProcess struct {
	GPUUUID       string     `json:"gpuUuid"`
	PID           int        `json:"pid"`
	User          string     `json:"user"`
	ProcessName   string     `json:"processName"`
	UsedMemoryMiB int64      `json:"usedMemoryMiB"`
	OwnerType     string     `json:"ownerType"`
	ContainerName string     `json:"containerName"`
	Groups        []LxcGroup `json:"groups"`
}

type GPUDevice struct {
	Index                    int                    `json:"index"`
	UUID                     string                 `json:"uuid"`
	Name                     string                 `json:"name"`
	MemoryTotalMiB           *float64               `json:"memoryTotalMiB"`
	MemoryUsedMiB            *float64               `json:"memoryUsedMiB"`
	Utilization              *float64               `json:"utilization"`
	Temperature              *float64               `json:"temperature"`
	MaxTemperatureCelsius    *float64               `json:"maxTemperatureCelsius"`
	TemperatureTargetCelsius *float64               `json:"temperatureTargetCelsius"`
	PowerWatts               *float64               `json:"powerWatts"`
	MaxPowerWatts            *float64               `json:"maxPowerWatts"`
	FanPercent               *float64               `json:"fanPercent"`
	PCIe                     *GPULink               `json:"pcie,omitempty"`
	NVLink                   *GPULink               `json:"nvlink,omitempty"`
	Processes                []GPUProcess           `json:"processes"`
	MemorySeries             []GPUOwnerMemorySeries `json:"memorySeries"`
	Telemetry                []GPUDevicePoint       `json:"telemetry"`
}

type GPULink struct {
	Detected         bool     `json:"detected,omitempty"`
	LinkGeneration   *int     `json:"linkGeneration,omitempty"`
	LinkWidth        *int     `json:"linkWidth,omitempty"`
	ActiveLinks      *int     `json:"activeLinks,omitempty"`
	RxBytesPerSecond *float64 `json:"rxBytesPerSecond"`
	TxBytesPerSecond *float64 `json:"txBytesPerSecond"`
}

type GPUDevicePoint struct {
	Timestamp     time.Time `json:"timestamp"`
	UsedMemoryMiB *float64  `json:"usedMemoryMiB"`
	PowerWatts    *float64  `json:"powerWatts"`
	Utilization   *float64  `json:"utilization"`
	Temperature   *float64  `json:"temperature"`
	FanPercent    *float64  `json:"fanPercent"`
}

type GPUTopology struct {
	Pairs []GPUTopologyPair `json:"pairs"`
}

type GPUTopologyPair struct {
	SourceUUID   string `json:"sourceUuid"`
	TargetUUID   string `json:"targetUuid"`
	Path         string `json:"path,omitempty"`
	P2PRead      string `json:"p2pRead"`
	P2PWrite     string `json:"p2pWrite"`
	P2PReadCode  string `json:"p2pReadCode,omitempty"`
	P2PWriteCode string `json:"p2pWriteCode,omitempty"`
}

type GPUOwnerMemoryPoint struct {
	Timestamp     time.Time `json:"timestamp"`
	UsedMemoryMiB float64   `json:"usedMemoryMiB"`
}

type GPUOwnerMemorySeries struct {
	OwnerType     string                `json:"ownerType"`
	ContainerName string                `json:"containerName"`
	Label         string                `json:"label"`
	Points        []GPUOwnerMemoryPoint `json:"points"`
}

type GPUMonitorResponse struct {
	Available            bool         `json:"available"`
	Message              string       `json:"message"`
	UpdatedAt            time.Time    `json:"updatedAt"`
	Range                string       `json:"range"`
	IntervalSeconds      int          `json:"intervalSeconds"`
	GPUs                 []GPUDevice  `json:"gpus"`
	DriverVersion        string       `json:"driverVersion,omitempty"`
	CUDASupportedVersion string       `json:"cudaSupportedVersion,omitempty"`
	Topology             *GPUTopology `json:"topology,omitempty"`
}
