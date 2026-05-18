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
	Index          int                    `json:"index"`
	UUID           string                 `json:"uuid"`
	Name           string                 `json:"name"`
	MemoryTotalMiB int64                  `json:"memoryTotalMiB"`
	MemoryUsedMiB  int64                  `json:"memoryUsedMiB"`
	Utilization    int                    `json:"utilization"`
	Temperature    int                    `json:"temperature"`
	Processes      []GPUProcess           `json:"processes"`
	MemorySeries   []GPUOwnerMemorySeries `json:"memorySeries"`
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
	Available       bool        `json:"available"`
	Message         string      `json:"message"`
	UpdatedAt       time.Time   `json:"updatedAt"`
	Range           string      `json:"range"`
	IntervalSeconds int         `json:"intervalSeconds"`
	GPUs            []GPUDevice `json:"gpus"`
}
