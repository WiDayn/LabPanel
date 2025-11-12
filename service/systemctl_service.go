package service

import (
	"LabPanel/config"
	"fmt"
	"os/exec"
	"strings"
)

type SystemctlService struct {
	cfg *config.Config
}

func NewSystemctlService() *SystemctlService {
	cfg, _ := config.Load()
	return &SystemctlService{cfg: cfg}
}

func (s *SystemctlService) Restart() error {
	cmd := exec.Command("sudo", "systemctl", "restart", s.cfg.ServiceName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("重启服务失败: %v, 输出: %s", err, string(output))
	}
	return nil
}

func (s *SystemctlService) GetStatus() (bool, string, error) {
	cmd := exec.Command("systemctl", "is-active", s.cfg.ServiceName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, "unknown", nil
	}

	status := strings.TrimSpace(string(output))
	isActive := status == "active"

	// 获取详细状态
	cmd = exec.Command("systemctl", "status", s.cfg.ServiceName, "--no-pager", "-l")
	statusOutput, _ := cmd.CombinedOutput()
	statusDetail := string(statusOutput)

	return isActive, statusDetail, nil
}

