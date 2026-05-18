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
	cmd := exec.Command("sudo", "systemctl", "restart", s.cfg.FrpService)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("重启服务失败: %v, 输出: %s", err, string(output))
	}
	return nil
}

func (s *SystemctlService) GetStatus() (bool, string, string, error) {
	cmd := exec.Command("systemctl", "is-active", s.cfg.FrpService)
	output, err := cmd.CombinedOutput()
	status := strings.TrimSpace(string(output))
	isActive := status == "active"

	detailCommand := fmt.Sprintf("systemctl status %s", s.cfg.FrpService)
	cmd = exec.Command("systemctl", "status", s.cfg.FrpService, "--no-pager", "-l")
	detailOutput, _ := cmd.CombinedOutput()

	if err != nil {
		return false, strings.TrimRight(string(detailOutput), "\n"), detailCommand, nil
	}

	return isActive, strings.TrimRight(string(detailOutput), "\n"), detailCommand, nil
}
