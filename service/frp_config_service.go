package service

import (
	"LabPanel/config"
	"LabPanel/models"
	"fmt"
	"os"

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
	data, err := toml.Marshal(frpConfig)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %v", err)
	}

	if err := os.WriteFile(s.cfg.TomlPath, data, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %v", err)
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
	return s.SaveConfig(frpConfig)
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
	return s.SaveConfig(frpConfig)
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
	return s.SaveConfig(frpConfig)
}

