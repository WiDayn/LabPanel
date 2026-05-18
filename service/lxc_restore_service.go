package service

import (
	"LabPanel/models"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type LxcRestoreManager struct {
	mu       sync.RWMutex
	restores map[string]*models.LxcRestoreStatus
}

var defaultLxcRestoreManager = NewLxcRestoreManager()

func NewLxcRestoreManager() *LxcRestoreManager {
	return &LxcRestoreManager{
		restores: make(map[string]*models.LxcRestoreStatus),
	}
}

func GetLxcRestoreManager() *LxcRestoreManager {
	return defaultLxcRestoreManager
}

func (m *LxcRestoreManager) StartRestore(req models.CreateLxcRequest) (*models.LxcRestoreStatus, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("容器名称不能为空")
	}
	password := strings.TrimSpace(req.Password)
	if password == "" {
		return nil, fmt.Errorf("Root 密码不能为空")
	}
	backupFile := strings.TrimSpace(req.BackupFile)
	if backupFile == "" {
		return nil, fmt.Errorf("请选择备份文件")
	}
	req.Name = name
	req.Password = password
	req.BackupFile = backupFile

	m.mu.Lock()
	if existing, ok := m.restores[name]; ok && (existing.Status == models.LxcBackupStatusQueued || existing.Status == models.LxcBackupStatusRunning) {
		statusCopy := *existing
		m.mu.Unlock()
		return &statusCopy, nil
	}

	now := time.Now()
	status := &models.LxcRestoreStatus{
		Name:       name,
		TaskID:     fmt.Sprintf("%s-restore-%d", sanitizeBackupName(name), now.UnixNano()),
		Status:     models.LxcBackupStatusQueued,
		Stage:      "queued",
		Progress:   5,
		Message:    "恢复任务已创建，等待后台导入",
		BackupFile: backupFile,
		StartedAt:  now,
		UpdatedAt:  now,
	}
	m.restores[name] = status
	m.mu.Unlock()

	go m.runRestore(req, status.TaskID)

	statusCopy := *status
	return &statusCopy, nil
}

func (m *LxcRestoreManager) runRestore(req models.CreateLxcRequest, taskID string) {
	name := strings.TrimSpace(req.Name)
	lxcService := NewLxcService()

	m.updateStatus(name, taskID, func(status *models.LxcRestoreStatus) {
		status.Status = models.LxcBackupStatusRunning
		status.Stage = "importing"
		status.Progress = 20
		status.Message = "正在后台导入备份包，20GB 备份可能需要几分钟，请勿重复创建同名容器"
	})

	if err := lxcService.CreateContainer(req); err != nil {
		m.updateStatus(name, taskID, func(status *models.LxcRestoreStatus) {
			status.Status = models.LxcBackupStatusFailed
			status.Stage = "failed"
			status.Progress = 100
			status.Message = err.Error()
			now := time.Now()
			status.FinishedAt = &now
		})
		return
	}

	m.updateStatus(name, taskID, func(status *models.LxcRestoreStatus) {
		status.Status = models.LxcBackupStatusCompleted
		status.Stage = "completed"
		status.Progress = 100
		status.Message = "容器已从备份恢复完成"
		now := time.Now()
		status.FinishedAt = &now
	})
}

func (m *LxcRestoreManager) updateStatus(name, taskID string, update func(status *models.LxcRestoreStatus)) {
	m.mu.Lock()
	defer m.mu.Unlock()

	status, ok := m.restores[name]
	if !ok || status.TaskID != taskID {
		return
	}

	update(status)
	status.UpdatedAt = time.Now()
}

func (m *LxcRestoreManager) ListRestores() []models.LxcRestoreStatus {
	m.mu.RLock()
	restores := make([]models.LxcRestoreStatus, 0, len(m.restores))
	activeNames := make(map[string]struct{}, len(m.restores))
	for _, status := range m.restores {
		restores = append(restores, *status)
		if status.Status == models.LxcBackupStatusQueued || status.Status == models.LxcBackupStatusRunning {
			activeNames[status.Name] = struct{}{}
		}
	}
	m.mu.RUnlock()

	for _, external := range activeImportProcessStatuses() {
		if _, exists := activeNames[external.Name]; exists {
			continue
		}
		restores = append(restores, external)
	}

	sort.Slice(restores, func(i, j int) bool {
		return restores[i].UpdatedAt.After(restores[j].UpdatedAt)
	})

	return restores
}
