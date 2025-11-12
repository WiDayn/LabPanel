package service

import (
	"LabPanel/config"
	"fmt"
	"os"
)

type TomlService struct {
	cfg *config.Config
}

func NewTomlService() *TomlService {
	cfg, _ := config.Load()
	return &TomlService{cfg: cfg}
}

func (s *TomlService) ReadConfig() (string, error) {
	content, err := os.ReadFile(s.cfg.TomlPath)
	if err != nil {
		return "", fmt.Errorf("读取配置文件失败: %v", err)
	}
	return string(content), nil
}

func (s *TomlService) WriteConfig(content string) error {
	if err := os.WriteFile(s.cfg.TomlPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %v", err)
	}
	return nil
}

