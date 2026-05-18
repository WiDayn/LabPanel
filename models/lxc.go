package models

import "time"

const (
	LxcCreateSourceDefaultImage = "default_image"
	LxcCreateSourceCustomImage  = "custom_image"
	LxcCreateSourceBackup       = "backup"
)

type LxcContainer struct {
	Name     string     `json:"name"`
	State    string     `json:"state"`
	IPV4     string     `json:"ipv4"`
	IPV6     string     `json:"ipv6"`
	Type     string     `json:"type"`
	Arch     string     `json:"arch"`
	Profiles string     `json:"profiles"`
	Groups   []LxcGroup `json:"groups"`
}

type LxcListResponse struct {
	Containers []LxcContainer `json:"containers"`
}

type CreateLxcRequest struct {
	Name       string   `json:"name" binding:"required"`
	Password   string   `json:"password" binding:"required"`
	SourceType string   `json:"sourceType"`
	Image      string   `json:"image"`
	BackupFile string   `json:"backupFile"`
	GroupIDs   []string `json:"groupIds"`
}

type LxcGroup struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Color     string    `json:"color"`
	CreatedAt time.Time `json:"createdAt"`
}

type LxcGroupsResponse struct {
	Groups          []LxcGroup          `json:"groups"`
	ContainerGroups map[string][]string `json:"containerGroups"`
}

type CreateLxcGroupRequest struct {
	Name string `json:"name" binding:"required"`
}

type LxcGroupResponse struct {
	Group LxcGroup `json:"group"`
}

type UpdateLxcContainerGroupsRequest struct {
	ContainerName string   `json:"containerName" binding:"required"`
	GroupIDs      []string `json:"groupIds"`
}

type DeleteLxcRequest struct {
	Name string `json:"name" binding:"required"`
}

type RestartLxcRequest struct {
	Name string `json:"name" binding:"required"`
}

type ChangePasswordLxcRequest struct {
	Name     string `json:"name" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type StartLxcRequest struct {
	Name string `json:"name" binding:"required"`
}

type StopLxcRequest struct {
	Name string `json:"name" binding:"required"`
}

type ForceStopLxcRequest struct {
	Name string `json:"name" binding:"required"`
}

type BackupLxcRequest struct {
	Name string `json:"name" binding:"required"`
}

const (
	LxcBackupStatusQueued    = "queued"
	LxcBackupStatusRunning   = "running"
	LxcBackupStatusCompleted = "completed"
	LxcBackupStatusFailed    = "failed"
)

type LxcBackupStatus struct {
	Name          string     `json:"name"`
	TaskID        string     `json:"taskId"`
	Status        string     `json:"status"`
	Stage         string     `json:"stage"`
	Progress      int        `json:"progress"`
	Message       string     `json:"message"`
	ArchivePath   string     `json:"archivePath"`
	ExportedFiles []string   `json:"exportedFiles"`
	StartedAt     time.Time  `json:"startedAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
	FinishedAt    *time.Time `json:"finishedAt"`
}

type BackupLxcResponse struct {
	Message string          `json:"message"`
	Backup  LxcBackupStatus `json:"backup"`
}

type LxcBackupStatusListResponse struct {
	Backups []LxcBackupStatus `json:"backups"`
}

type LxcRestoreStatus struct {
	Name       string     `json:"name"`
	TaskID     string     `json:"taskId"`
	Status     string     `json:"status"`
	Stage      string     `json:"stage"`
	Progress   int        `json:"progress"`
	Message    string     `json:"message"`
	BackupFile string     `json:"backupFile"`
	StartedAt  time.Time  `json:"startedAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
	FinishedAt *time.Time `json:"finishedAt"`
}

type RestoreLxcResponse struct {
	Message string           `json:"message"`
	Restore LxcRestoreStatus `json:"restore"`
}

type LxcRestoreStatusListResponse struct {
	Restores []LxcRestoreStatus `json:"restores"`
}

type LxcBackupArchive struct {
	Name         string    `json:"name"`
	Path         string    `json:"path"`
	SizeBytes    int64     `json:"sizeBytes"`
	ModifiedAt   time.Time `json:"modifiedAt"`
	DisplayLabel string    `json:"displayLabel"`
}

type LxcBackupArchiveListResponse struct {
	Archives []LxcBackupArchive `json:"archives"`
}

type DeleteLxcBackupArchiveRequest struct {
	Name string `json:"name" binding:"required"`
}

type GetLxcConfigRequest struct {
	Name string `uri:"name" binding:"required"`
}

type GetLxcConfigResponse struct {
	Config string `json:"config"`
}

type UpdateLxcConfigRequest struct {
	Name   string `json:"name" binding:"required"`
	Config string `json:"config" binding:"required"`
}

type LxcMetricsPoint struct {
	Timestamp      time.Time `json:"timestamp"`
	CPUPercent     float64   `json:"cpuPercent"`
	MemoryBytes    int64     `json:"memoryBytes"`
	Processes      int64     `json:"processes"`
	NetworkRxBps   float64   `json:"networkRxBps"`
	NetworkTxBps   float64   `json:"networkTxBps"`
	DiskReadBps    float64   `json:"diskReadBps"`
	DiskWriteBps   float64   `json:"diskWriteBps"`
	DiskUsageBytes int64     `json:"diskUsageBytes"`
	NetworkRxBytes int64     `json:"networkRxBytes"`
	NetworkTxBytes int64     `json:"networkTxBytes"`
	DiskReadBytes  int64     `json:"diskReadBytes"`
	DiskWriteBytes int64     `json:"diskWriteBytes"`
}

type LxcMetricsContainerSummary struct {
	Name           string    `json:"name"`
	State          string    `json:"state"`
	UpdatedAt      time.Time `json:"updatedAt"`
	CPUPercent     float64   `json:"cpuPercent"`
	MemoryBytes    int64     `json:"memoryBytes"`
	Processes      int64     `json:"processes"`
	NetworkRxBps   float64   `json:"networkRxBps"`
	NetworkTxBps   float64   `json:"networkTxBps"`
	DiskReadBps    float64   `json:"diskReadBps"`
	DiskWriteBps   float64   `json:"diskWriteBps"`
	DiskUsageBytes int64     `json:"diskUsageBytes"`
	NetworkRxBytes int64     `json:"networkRxBytes"`
	NetworkTxBytes int64     `json:"networkTxBytes"`
	DiskReadBytes  int64     `json:"diskReadBytes"`
	DiskWriteBytes int64     `json:"diskWriteBytes"`
}

type LxcMetricsSeries struct {
	Name   string            `json:"name"`
	Points []LxcMetricsPoint `json:"points"`
}

type LxcMetricsResponse struct {
	Range           string                       `json:"range"`
	IntervalSeconds int                          `json:"intervalSeconds"`
	Containers      []LxcMetricsContainerSummary `json:"containers"`
	Selected        *LxcMetricsSeries            `json:"selected,omitempty"`
}
