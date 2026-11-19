package service

import (
	"LabPanel/config"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func CheckFRPUpgrade(cfg *config.Config) error {
	data, err := os.ReadFile(cfg.PortForwardsPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var legacy struct {
		Forwards []struct {
			Name          string
			ListenAddress string
			ListenPort    int
			Enabled       bool
		}
	}
	if err := json.Unmarshal(data, &legacy); err != nil {
		return fmt.Errorf("读取旧端口转发规则 %s: %w", cfg.PortForwardsPath, err)
	}
	var enabled []string
	for _, rule := range legacy.Forwards {
		if rule.Enabled {
			enabled = append(enabled, fmt.Sprintf("%s (%s:%d)", rule.Name, rule.ListenAddress, rule.ListenPort))
		}
	}
	if len(enabled) != 0 {
		return fmt.Errorf("旧端口转发仍在启用，请先确认替代入口并停用这些规则: %s", strings.Join(enabled, "、"))
	}
	return nil
}
