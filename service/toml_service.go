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
	frpConfigMu.Lock()
	defer frpConfigMu.Unlock()
	frp := &FrpConfigService{cfg: s.cfg}
	doc, err := frp.readDocument()
	if err != nil {
		return err
	}
	for _, state := range doc.state {
		if state.Binding != nil {
			return fmt.Errorf("已存在容器绑定，请通过 FRP 管理编辑规则")
		}
	}
	return frp.applyRaw(doc.original, []byte(content))
}
