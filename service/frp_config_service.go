package service

import (
	"LabPanel/config"
	"LabPanel/models"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

type FrpConfigService struct {
	cfg *config.Config
}

func NewFrpConfigService() *FrpConfigService {
	cfg, _ := config.Load()
	return &FrpConfigService{cfg: cfg}
}

// getCommentsPath 获取备注文件路径
func (s *FrpConfigService) getCommentsPath() string {
	dir := filepath.Dir(s.cfg.TomlPath)
	return filepath.Join(dir, "proxy_comments.json")
}

// loadComments 加载备注
func (s *FrpConfigService) loadComments() (models.ProxyComments, error) {
	commentsPath := s.getCommentsPath()
	data, err := os.ReadFile(commentsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(models.ProxyComments), nil
		}
		return nil, fmt.Errorf("读取备注文件失败: %v", err)
	}

	var comments models.ProxyComments
	if err := json.Unmarshal(data, &comments); err != nil {
		return nil, fmt.Errorf("解析备注文件失败: %v", err)
	}

	return comments, nil
}

// saveComments 保存备注
func (s *FrpConfigService) saveComments(comments models.ProxyComments) error {
	commentsPath := s.getCommentsPath()
	data, err := json.MarshalIndent(comments, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化备注失败: %v", err)
	}

	if err := os.WriteFile(commentsPath, data, 0644); err != nil {
		return fmt.Errorf("写入备注文件失败: %v", err)
	}

	return nil
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

	// 加载备注并合并到代理中
	comments, err := s.loadComments()
	if err != nil {
		// 备注加载失败不影响主配置，只记录错误
		_ = err
	} else {
		for i := range frpConfig.Proxies {
			if comment, ok := comments[frpConfig.Proxies[i].Name]; ok {
				frpConfig.Proxies[i].Comment = comment
			}
		}
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

	// 提取备注并保存到单独文件
	comments := make(models.ProxyComments)
	for _, proxy := range frpConfig.Proxies {
		if proxy.Comment != "" {
			comments[proxy.Name] = proxy.Comment
		}
	}
	if err := s.saveComments(comments); err != nil {
		return fmt.Errorf("保存备注失败: %v", err)
	}

	// 创建不包含comment的配置用于保存到toml
	type FrpConfigForToml struct {
		ServerAddr string          `toml:"serverAddr"`
		ServerPort int             `toml:"serverPort"`
		Auth       models.AuthConfig      `toml:"auth"`
		WebServer  models.WebServerConfig `toml:"webServer"`
		Proxies    []models.ProxyForToml  `toml:"proxies"`
	}

	configForToml := FrpConfigForToml{
		ServerAddr: frpConfig.ServerAddr,
		ServerPort: frpConfig.ServerPort,
		Auth:       frpConfig.Auth,
		WebServer:  frpConfig.WebServer,
		Proxies:    make([]models.ProxyForToml, len(frpConfig.Proxies)),
	}

	for i, proxy := range frpConfig.Proxies {
		configForToml.Proxies[i] = models.ProxyForToml{
			Name:      proxy.Name,
			Type:      proxy.Type,
			LocalIP:   proxy.LocalIP,
			LocalPort: proxy.LocalPort,
			RemotePort: proxy.RemotePort,
		}
	}

	data, err := toml.Marshal(configForToml)
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

	// 删除代理前，先删除对应的备注
	deletedProxyName := frpConfig.Proxies[index].Name
	frpConfig.Proxies = append(frpConfig.Proxies[:index], frpConfig.Proxies[index+1:]...)
	
	// 保存配置（这会更新备注文件，删除已删除代理的备注）
	if err := s.SaveAndReload(frpConfig); err != nil {
		return err
	}

	// 确保删除备注（如果SaveAndReload没有处理）
	comments, err := s.loadComments()
	if err == nil {
		delete(comments, deletedProxyName)
		s.saveComments(comments)
	}

	return nil
}

