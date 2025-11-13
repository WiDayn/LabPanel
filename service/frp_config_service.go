package service

import (
	"LabPanel/config"
	"LabPanel/models"
	"fmt"
	"os"
	"os/exec"

	"github.com/pelletier/go-toml/v2"
)

type FrpConfigService struct {
	cfg *config.Config
}

func NewFrpConfigService() *FrpConfigService {
	cfg, _ := config.Load()
	return &FrpConfigService{cfg: cfg}
}

func (s *FrpConfigService) GetConfig() (*models.FrpConfig, error) {
	data, err := os.ReadFile(s.cfg.TomlPath)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %v", err)
	}

	var frpConfig models.FrpConfig
	if err := toml.Unmarshal(data, &frpConfig); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %v", err)
	}

	return &frpConfig, nil
}

func (s *FrpConfigService) SaveConfig(frpConfig *models.FrpConfig) error {
	// 确保WebServer配置存在（热重载必需）
	if frpConfig.WebServer.Addr == "" {
		frpConfig.WebServer.Addr = "127.0.0.1"
	}
	if frpConfig.WebServer.Port == 0 {
		frpConfig.WebServer.Port = 7400
	}

	data, err := toml.Marshal(frpConfig)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %v", err)
	}

	if err := os.WriteFile(s.cfg.TomlPath, data, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %v", err)
	}

	return nil
}

// VerifyConfig 验证配置文件
func (s *FrpConfigService) VerifyConfig() error {
	cmd := exec.Command(s.cfg.FrpcPath, "verify", "-c", s.cfg.TomlPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("配置验证失败: %v, 输出: %s", err, string(output))
	}
	return nil
}

// ReloadConfig 热重载配置
func (s *FrpConfigService) ReloadConfig() error {
	cmd := exec.Command(s.cfg.FrpcPath, "reload", "-c", s.cfg.TomlPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("配置重载失败: %v, 输出: %s", err, string(output))
	}
	return nil
}

// SaveAndReload 保存配置并热重载，如果失败则回滚
func (s *FrpConfigService) SaveAndReload(frpConfig *models.FrpConfig) error {
	// 备份当前配置
	backupConfig, err := s.GetConfig()
	if err != nil {
		return fmt.Errorf("读取当前配置失败: %v", err)
	}

	// 保存新配置
	if err := s.SaveConfig(frpConfig); err != nil {
		return err
	}

	// 验证配置
	if err := s.VerifyConfig(); err != nil {
		// 验证失败，回滚配置
		if rollbackErr := s.SaveConfig(backupConfig); rollbackErr != nil {
			return fmt.Errorf("配置验证失败且回滚失败: 验证错误: %v, 回滚错误: %v", err, rollbackErr)
		}
		return fmt.Errorf("配置验证失败，已回滚: %v", err)
	}

	// 重载配置
	if err := s.ReloadConfig(); err != nil {
		// 重载失败，回滚配置
		if rollbackErr := s.SaveConfig(backupConfig); rollbackErr != nil {
			return fmt.Errorf("配置重载失败且回滚失败: 重载错误: %v, 回滚错误: %v", err, rollbackErr)
		}
		return fmt.Errorf("配置重载失败，已回滚: %v", err)
	}

	return nil
}

func (s *FrpConfigService) AddProxy(proxy models.Proxy) error {
	frpConfig, err := s.GetConfig()
	if err != nil {
		return err
	}

	// 冲突检测：检查名称和远程端口是否已存在
	for _, existingProxy := range frpConfig.Proxies {
		if existingProxy.Name == proxy.Name {
			return fmt.Errorf("代理名称 '%s' 已存在", proxy.Name)
		}
		if existingProxy.RemotePort == proxy.RemotePort {
			return fmt.Errorf("远程端口 %d 已被代理 '%s' 使用", proxy.RemotePort, existingProxy.Name)
		}
	}

	frpConfig.Proxies = append(frpConfig.Proxies, proxy)
	return s.SaveAndReload(frpConfig)
}

func (s *FrpConfigService) UpdateProxy(index int, proxy models.Proxy) error {
	frpConfig, err := s.GetConfig()
	if err != nil {
		return err
	}

	if index < 0 || index >= len(frpConfig.Proxies) {
		return fmt.Errorf("代理索引超出范围")
	}

	// 冲突检测：检查名称和远程端口是否与其他代理冲突（排除当前代理）
	for i, existingProxy := range frpConfig.Proxies {
		if i == index {
			continue // 跳过当前正在编辑的代理
		}
		if existingProxy.Name == proxy.Name {
			return fmt.Errorf("代理名称 '%s' 已存在", proxy.Name)
		}
		if existingProxy.RemotePort == proxy.RemotePort {
			return fmt.Errorf("远程端口 %d 已被代理 '%s' 使用", proxy.RemotePort, existingProxy.Name)
		}
	}

	frpConfig.Proxies[index] = proxy
	return s.SaveAndReload(frpConfig)
}

func (s *FrpConfigService) DeleteProxy(index int) error {
	frpConfig, err := s.GetConfig()
	if err != nil {
		return err
	}

	if index < 0 || index >= len(frpConfig.Proxies) {
		return fmt.Errorf("代理索引超出范围")
	}

	frpConfig.Proxies = append(frpConfig.Proxies[:index], frpConfig.Proxies[index+1:]...)
	return s.SaveAndReload(frpConfig)
}

