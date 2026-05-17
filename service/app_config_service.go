package service

import (
	"LabPanel/config"
	"LabPanel/models"
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type AppConfigService struct {
	envPath string
}

func NewAppConfigService() *AppConfigService {
	return &AppConfigService{
		envPath: ".env",
	}
}

func (s *AppConfigService) GetConfig() (*models.AppConfigResponse, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("读取应用配置失败: %v", err)
	}

	return &models.AppConfigResponse{
		LxcImage:     cfg.LxcImage,
		LxcBackupDir: cfg.LxcBackupDir,
	}, nil
}

func (s *AppConfigService) UpdateConfig(req models.UpdateAppConfigRequest) error {
	lxcImage := strings.TrimSpace(req.LxcImage)
	if lxcImage == "" {
		return fmt.Errorf("容器镜像不能为空")
	}
	lxcBackupDir := strings.TrimSpace(req.LxcBackupDir)
	if lxcBackupDir == "" {
		return fmt.Errorf("容器备份目录不能为空")
	}

	envMap, err := godotenv.Read(s.envPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("读取 .env 失败: %v", err)
	}
	if envMap == nil {
		envMap = make(map[string]string)
	}

	envMap["LXC_IMAGE"] = lxcImage
	envMap["LXC_BACKUP_DIR"] = lxcBackupDir

	if err := godotenv.Write(envMap, s.envPath); err != nil {
		return fmt.Errorf("写入 .env 失败: %v", err)
	}

	if err := os.Setenv("LXC_IMAGE", lxcImage); err != nil {
		return fmt.Errorf("更新当前进程 LXC_IMAGE 失败: %v", err)
	}
	if err := os.Setenv("LXC_BACKUP_DIR", lxcBackupDir); err != nil {
		return fmt.Errorf("更新当前进程 LXC_BACKUP_DIR 失败: %v", err)
	}

	return nil
}
