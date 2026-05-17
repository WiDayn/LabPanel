package service

import (
	"LabPanel/config"
	"LabPanel/models"
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type EnvironmentCheckService struct {
	cfg *config.Config
}

func NewEnvironmentCheckService() *EnvironmentCheckService {
	cfg, _ := config.Load()
	return &EnvironmentCheckService{cfg: cfg}
}

func (s *EnvironmentCheckService) Check() models.EnvironmentCheckResponse {
	return models.EnvironmentCheckResponse{
		FRP: s.checkFRP(),
		LXC: s.checkLXC(),
	}
}

func (s *EnvironmentCheckService) checkFRP() models.ComponentCheck {
	check := models.ComponentCheck{
		Name: "FRP",
	}

	frpcInstalled := s.commandExists(s.cfg.FrpcPath, "frpc")
	configExists := s.fileExists(s.cfg.TomlPath)
	serviceExists := s.systemdUnitExists(s.cfg.ServiceName)

	if frpcInstalled {
		check.Installed = true
	} else {
		check.MissingItems = append(check.MissingItems, "frpc 可执行文件")
	}

	if !configExists {
		check.MissingItems = append(check.MissingItems, "frpc 配置文件")
	}

	if !serviceExists {
		check.MissingItems = append(check.MissingItems, "systemd 服务 "+s.cfg.ServiceName)
	}

	check.Ready = frpcInstalled && configExists && serviceExists
	if check.Ready {
		check.Message = "FRP 环境已就绪"
		return check
	}

	if !frpcInstalled {
		check.Message = "未检测到 FRP 客户端，请先安装 frpc 并创建服务"
	} else {
		check.Message = "FRP 未完全配置，请补齐配置文件或 systemd 服务"
	}

	check.Guides = s.buildFRPGuides()
	return check
}

func (s *EnvironmentCheckService) checkLXC() models.ComponentCheck {
	check := models.ComponentCheck{
		Name: "LXC/LXD",
	}

	lxcInstalled := s.commandExists("", "lxc")
	lxdServiceExists := s.systemdUnitExists("snap.lxd.daemon")
	lxdServiceAltExists := s.systemdUnitExists("lxd")

	if lxcInstalled {
		check.Installed = true
		check.Ready = true
		check.Message = "LXC 命令已安装"
		return check
	}

	check.MissingItems = append(check.MissingItems, "lxc 命令行工具")
	if !lxdServiceExists && !lxdServiceAltExists {
		check.MissingItems = append(check.MissingItems, "LXD 服务")
	}

	check.Message = "未检测到 LXC/LXD，请先安装后再使用容器管理"
	check.Guides = s.buildLXCGuides()
	return check
}

func (s *EnvironmentCheckService) commandExists(configuredPath, fallbackCommand string) bool {
	if strings.TrimSpace(configuredPath) != "" {
		if info, err := os.Stat(configuredPath); err == nil && !info.IsDir() {
			return true
		}
	}

	_, err := exec.LookPath(fallbackCommand)
	return err == nil
}

func (s *EnvironmentCheckService) fileExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}

	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func (s *EnvironmentCheckService) systemdUnitExists(unit string) bool {
	if strings.TrimSpace(unit) == "" {
		return false
	}

	cmd := exec.Command("systemctl", "status", unit, "--no-pager")
	err := cmd.Run()
	if err == nil {
		return true
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode() != 4
	}

	return false
}

func (s *EnvironmentCheckService) buildFRPGuides() []models.InstallGuide {
	osID := s.detectOSID()
	commands := []models.InstallCommand{
		{
			Label:   "创建配置目录",
			Command: "sudo mkdir -p " + filepath.Dir(s.cfg.TomlPath),
		},
	}

	if osID == "ubuntu" || osID == "debian" {
		commands = append(commands,
			models.InstallCommand{
				Label:   "安装下载工具",
				Command: "sudo apt-get update && sudo apt-get install -y curl tar",
			},
		)
	} else if osID == "centos" || osID == "rhel" || osID == "rocky" || osID == "almalinux" || osID == "fedora" {
		commands = append(commands,
			models.InstallCommand{
				Label:   "安装下载工具",
				Command: "sudo dnf install -y curl tar || sudo yum install -y curl tar",
			},
		)
	}

	commands = append(commands,
		models.InstallCommand{
			Label:   "下载并安装 frpc",
			Command: "下载官方 frp 压缩包后，将 frpc 放到 /usr/local/bin/frpc，并执行 sudo chmod +x /usr/local/bin/frpc",
		},
		models.InstallCommand{
			Label:   "创建配置文件",
			Command: "sudo editor " + s.cfg.TomlPath,
		},
		models.InstallCommand{
			Label:   "创建 systemd 服务",
			Command: "sudo editor /etc/systemd/system/" + s.cfg.ServiceName + ".service",
		},
		models.InstallCommand{
			Label:   "启用并启动服务",
			Command: "sudo systemctl daemon-reload && sudo systemctl enable --now " + s.cfg.ServiceName,
		},
	)

	return []models.InstallGuide{
		{
			Title:       "FRP 安装与初始化",
			Description: "面板依赖 frpc 可执行文件、配置文件以及 systemd 服务。补齐后即可在首页继续管理代理与状态。",
			Commands:    commands,
		},
	}
}

func (s *EnvironmentCheckService) buildLXCGuides() []models.InstallGuide {
	osID := s.detectOSID()
	commands := []models.InstallCommand{}

	switch osID {
	case "ubuntu", "debian":
		commands = append(commands,
			models.InstallCommand{
				Label:   "安装 snapd",
				Command: "sudo apt-get update && sudo apt-get install -y snapd",
			},
			models.InstallCommand{
				Label:   "通过 snap 安装 LXD",
				Command: "sudo snap install lxd",
			},
			models.InstallCommand{
				Label:   "初始化 LXD",
				Command: "sudo lxd init --auto",
			},
			models.InstallCommand{
				Label:   "授予当前用户访问权限",
				Command: "getent group lxd | grep -qwF \"$USER\" || sudo usermod -aG lxd \"$USER\"",
			},
		)
	case "centos", "rhel", "rocky", "almalinux", "fedora":
		commands = append(commands,
			models.InstallCommand{
				Label:   "安装 LXC/LXD",
				Command: "sudo dnf install -y lxc lxc-templates || sudo yum install -y lxc lxc-templates",
			},
			models.InstallCommand{
				Label:   "启动服务",
				Command: "sudo systemctl enable --now lxd || sudo systemctl enable --now lxc",
			},
		)
	default:
		commands = append(commands,
			models.InstallCommand{
				Label:   "安装 LXC/LXD",
				Command: "请按当前发行版安装 lxc 或 lxd，并确认 `lxc` 命令可直接执行",
			},
		)
	}

	return []models.InstallGuide{
		{
			Title:       "LXC/LXD 安装",
			Description: "容器管理页面依赖 `lxc` 命令。Ubuntu 24.04 等新版本已不再通过 apt 提供 lxd/lxd-client，推荐使用 snap 安装 LXD。安装完成后刷新页面即可读取容器列表。",
			Commands:    commands,
		},
	}
}

func (s *EnvironmentCheckService) detectOSID() string {
	file, err := os.Open("/etc/os-release")
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "ID=") {
			continue
		}

		value := strings.TrimPrefix(line, "ID=")
		return strings.Trim(value, "\"")
	}

	return ""
}
