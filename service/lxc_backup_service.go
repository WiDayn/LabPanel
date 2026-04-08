package service

import (
	"LabPanel/models"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type LxcBackupManager struct {
	mu      sync.RWMutex
	backups map[string]*models.LxcBackupStatus
}

var defaultLxcBackupManager = NewLxcBackupManager()

func NewLxcBackupManager() *LxcBackupManager {
	return &LxcBackupManager{
		backups: make(map[string]*models.LxcBackupStatus),
	}
}

func GetLxcBackupManager() *LxcBackupManager {
	return defaultLxcBackupManager
}

func (m *LxcBackupManager) StartBackup(name string) (*models.LxcBackupStatus, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("容器名称不能为空")
	}

	m.mu.Lock()
	if existing, ok := m.backups[name]; ok && (existing.Status == models.LxcBackupStatusQueued || existing.Status == models.LxcBackupStatusRunning) {
		statusCopy := *existing
		m.mu.Unlock()
		return &statusCopy, nil
	}

	now := time.Now()
	status := &models.LxcBackupStatus{
		Name:      name,
		TaskID:    fmt.Sprintf("%s-%d", sanitizeBackupName(name), now.UnixNano()),
		Status:    models.LxcBackupStatusQueued,
		Stage:     "queued",
		Progress:  5,
		Message:   "备份任务已创建，等待执行",
		StartedAt: now,
		UpdatedAt: now,
	}
	m.backups[name] = status
	m.mu.Unlock()

	go m.runBackup(name, status.TaskID)

	statusCopy := *status
	return &statusCopy, nil
}

func (m *LxcBackupManager) runBackup(name, taskID string) {
	lxcService := NewLxcService()

	m.updateStatus(name, taskID, func(status *models.LxcBackupStatus) {
		status.Status = models.LxcBackupStatusRunning
		status.Stage = "exporting"
		status.Progress = 20
		status.Message = "正在导出容器备份文件"
	})

	exportedFiles, err := lxcService.BackupContainerWithProgress(name, func(stage, message string, progress int) {
		m.updateStatus(name, taskID, func(status *models.LxcBackupStatus) {
			status.Status = models.LxcBackupStatusRunning
			status.Stage = stage
			status.Progress = progress
			status.Message = message
		})
	})
	if err != nil {
		m.updateStatus(name, taskID, func(status *models.LxcBackupStatus) {
			status.Status = models.LxcBackupStatusFailed
			status.Progress = 100
			status.Message = err.Error()
			now := time.Now()
			status.FinishedAt = &now
		})
		return
	}

	m.updateStatus(name, taskID, func(status *models.LxcBackupStatus) {
		status.Status = models.LxcBackupStatusCompleted
		status.Stage = "completed"
		status.Progress = 100
		status.Message = fmt.Sprintf("备份完成，共导出 %d 个文件", len(exportedFiles))
		status.ExportedFiles = exportedFiles
		if len(exportedFiles) > 0 {
			status.ArchivePath = strings.Join(exportedFiles, "\n")
		}
		now := time.Now()
		status.FinishedAt = &now
	})
}

func (m *LxcBackupManager) updateStatus(name, taskID string, update func(status *models.LxcBackupStatus)) {
	m.mu.Lock()
	defer m.mu.Unlock()

	status, ok := m.backups[name]
	if !ok || status.TaskID != taskID {
		return
	}

	update(status)
	status.UpdatedAt = time.Now()
}

func (m *LxcBackupManager) ListBackups() []models.LxcBackupStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	backups := make([]models.LxcBackupStatus, 0, len(m.backups))
	for _, status := range m.backups {
		backups = append(backups, *status)
	}

	sort.Slice(backups, func(i, j int) bool {
		return backups[i].UpdatedAt.After(backups[j].UpdatedAt)
	})

	return backups
}

func (m *LxcBackupManager) GetBackup(name string) (*models.LxcBackupStatus, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status, ok := m.backups[name]
	if !ok {
		return nil, false
	}

	statusCopy := *status
	if len(status.ExportedFiles) > 0 {
		statusCopy.ExportedFiles = append([]string(nil), status.ExportedFiles...)
	}

	return &statusCopy, true
}

func FormatExportedFilesForMessage(files []string) string {
	if len(files) == 0 {
		return ""
	}

	labels := make([]string, 0, len(files))
	for _, file := range files {
		labels = append(labels, filepath.Base(file))
	}
	return strings.Join(labels, ", ")
}
