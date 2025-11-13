package service

import (
	"LabPanel/models"
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type LxcService struct{}

func NewLxcService() *LxcService {
	return &LxcService{}
}

// waitForContainerRunning 等待容器运行，最多等待60秒
func (s *LxcService) waitForContainerRunning(name string, timeoutSeconds int) error {
	maxAttempts := timeoutSeconds / 2 // 每2秒检查一次
	for i := 0; i < maxAttempts; i++ {
		cmd := exec.Command("lxc", "exec", name, "--", "true")
		if err := cmd.Run(); err == nil {
			// 容器可以执行，说明正在运行
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	// 超时后尝试启动容器
	cmd := exec.Command("lxc", "start", name)
	cmd.Run() // 忽略启动错误
	// 再等待10秒
	for i := 0; i < 5; i++ {
		cmd := exec.Command("lxc", "exec", name, "--", "true")
		if err := cmd.Run(); err == nil {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("等待容器运行超时")
}

func (s *LxcService) ListContainers() ([]models.LxcContainer, error) {
	// 执行 lxc list --format json 命令
	cmd := exec.Command("lxc", "list", "--format", "json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("执行lxc命令失败: %v, 输出: %s", err, string(output))
	}

	// 解析JSON输出
	var rawContainers []map[string]interface{}
	if err := json.Unmarshal(output, &rawContainers); err != nil {
		return nil, fmt.Errorf("解析lxc输出失败: %v, 原始输出: %s", err, string(output))
	}

	// 转换为模型
	containers := make([]models.LxcContainer, 0, len(rawContainers))
	for _, raw := range rawContainers {
		container := models.LxcContainer{
			Name:     getString(raw, "name"),
			State:    getString(raw, "status"),
			Type:     getString(raw, "type"),
			Arch:     getString(raw, "architecture"),
			Profiles: getStringSlice(raw, "profiles"),
		}

		// 提取IP地址 - LXC的JSON格式中，网络信息可能在state字段下
		if state, ok := raw["state"].(map[string]interface{}); ok {
			if network, ok := state["network"].(map[string]interface{}); ok {
				// 遍历所有网络接口
				for _, iface := range network {
					if ifaceMap, ok := iface.(map[string]interface{}); ok {
						if addresses, ok := ifaceMap["addresses"].([]interface{}); ok {
							for _, addr := range addresses {
								if addrMap, ok := addr.(map[string]interface{}); ok {
									family := getString(addrMap, "family")
									address := getString(addrMap, "address")
									if family == "inet" && container.IPV4 == "" {
										container.IPV4 = address
									} else if family == "inet6" && container.IPV6 == "" {
										container.IPV6 = address
									}
								}
							}
						}
					}
				}
			}
		}

		containers = append(containers, container)
	}

	return containers, nil
}

func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func getStringSlice(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if arr, ok := val.([]interface{}); ok {
			strs := make([]string, 0, len(arr))
			for _, v := range arr {
				if str, ok := v.(string); ok {
					strs = append(strs, str)
				}
			}
			return strings.Join(strs, ", ")
		}
	}
	return ""
}

func (s *LxcService) DeleteContainer(name string) error {
	cmd := exec.Command("lxc", "delete", name, "--force")
	output, err := cmd.CombinedOutput()
	if err != nil {
		outputStr := string(output)
		// 如果容器不存在，认为删除成功（可能已经被删除或本来就不存在）
		if strings.Contains(outputStr, "Instance not found") || 
		   strings.Contains(outputStr, "not found") ||
		   strings.Contains(outputStr, "does not exist") {
			return nil
		}
		return fmt.Errorf("删除容器失败: %v, 输出: %s", err, outputStr)
	}
	return nil
}

func (s *LxcService) RestartContainer(name string) error {
	cmd := exec.Command("lxc", "restart", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("重启容器失败: %v, 输出: %s", err, string(output))
	}
	return nil
}

func (s *LxcService) CreateContainer(name, password string) error {
	// 1. 创建容器
	cmd := exec.Command("lxc", "launch", "ubuntu:22.04", name, "-c", "nvidia.runtime=true")
	output, err := cmd.CombinedOutput()
	if err != nil {
		outputStr := string(output)
		// 如果容器正在创建中或已存在，等待并继续
		if strings.Contains(outputStr, "Instance is busy") || 
		   strings.Contains(outputStr, "already exists") ||
		   strings.Contains(outputStr, "already exist") {
			// 等待容器创建完成
			s.waitForContainerRunning(name, 60)
		} else {
			return fmt.Errorf("创建容器失败: %v, 输出: %s", err, outputStr)
		}
	}

	// 2. 等待容器启动
	s.waitForContainerRunning(name, 60)

	// 3. 添加GPU设备
	cmd = exec.Command("lxc", "config", "device", "add", name, "gpu", "gpu")
	output, err = cmd.CombinedOutput()
	if err != nil {
		outputStr := string(output)
		// GPU设备可能已存在，忽略相关错误
		if !strings.Contains(outputStr, "already exists") && 
		   !strings.Contains(outputStr, "already exist") {
			// 如果不是"已存在"的错误，记录但不中断流程
			_ = outputStr
		}
	}

	// 4. 配置SSH
	if err := s.configureSSH(name, password); err != nil {
		return fmt.Errorf("配置SSH失败: %v", err)
	}

	return nil
}

func (s *LxcService) configureSSH(name, password string) error {
	// 等待容器启动
	if err := s.waitForContainerRunning(name, 60); err != nil {
		// 如果等待失败，尝试启动容器
		cmd := exec.Command("lxc", "start", name)
		cmd.Run() // 忽略启动错误
		// 再次等待
		s.waitForContainerRunning(name, 30)
	}
	
	// 在设置密码前，最后确认容器正在运行
	cmd := exec.Command("lxc", "exec", name, "--", "true")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("容器无法执行命令，可能未运行: %v", err)
	}
	
	// 设置root密码 - 使用passwd命令，通过标准输入传递两次密码
	// passwd命令需要输入两次密码进行确认
	cmd = exec.Command("lxc", "exec", name, "--", "passwd", "root")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("创建stdin管道失败: %v", err)
	}
	
	// 设置输出管道
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	
	// 启动命令
	if err := cmd.Start(); err != nil {
		stdin.Close()
		return fmt.Errorf("启动命令失败: %v", err)
	}
	
	// 写入两次密码（passwd需要输入两次进行确认）
	passwordInput := fmt.Sprintf("%s\n%s\n", password, password)
	_, err = stdin.Write([]byte(passwordInput))
	if err != nil {
		stdin.Close()
		cmd.Wait()
		return fmt.Errorf("写入密码失败: %v", err)
	}
	stdin.Close()
	
	// 等待命令完成
	if err := cmd.Wait(); err != nil {
		output := stdout.String() + stderr.String()
		return fmt.Errorf("设置root密码失败: %v, 输出: %s", err, output)
	}

	// 启动SSH服务（如果还没启动）
	cmd = exec.Command("lxc", "exec", name, "--", "bash", "-c", "systemctl enable ssh && systemctl start ssh")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// 忽略启动错误，可能已经启动
		_ = output
	}

	// 配置SSH允许root登录和密码登录
	// 使用更全面的sed命令来处理各种配置情况
	sshConfig := `sed -i 's/^#*PermitRootLogin.*/PermitRootLogin yes/' /etc/ssh/sshd_config
sed -i 's/^#*PasswordAuthentication.*/PasswordAuthentication yes/' /etc/ssh/sshd_config
sed -i 's/^#*KbdInteractiveAuthentication.*/KbdInteractiveAuthentication yes/' /etc/ssh/sshd_config
grep -q "^PermitRootLogin" /etc/ssh/sshd_config || echo "PermitRootLogin yes" >> /etc/ssh/sshd_config
grep -q "^PasswordAuthentication" /etc/ssh/sshd_config || echo "PasswordAuthentication yes" >> /etc/ssh/sshd_config
grep -q "^KbdInteractiveAuthentication" /etc/ssh/sshd_config || echo "KbdInteractiveAuthentication yes" >> /etc/ssh/sshd_config
systemctl restart ssh || service ssh restart || /etc/init.d/ssh restart`

	cmd = exec.Command("lxc", "exec", name, "--", "bash", "-c", sshConfig)
	output, err = cmd.CombinedOutput()
	if err != nil {
		// SSH重启可能失败，但不影响配置
		if !strings.Contains(string(output), "restart") {
			return fmt.Errorf("配置SSH失败: %v, 输出: %s", err, string(output))
		}
	}

	return nil
}

// ChangePassword 修改容器的root密码
func (s *LxcService) ChangePassword(name, password string) error {
	// 确保容器正在运行
	if err := s.waitForContainerRunning(name, 30); err != nil {
		return fmt.Errorf("容器未运行或无法访问: %v", err)
	}

	// 设置root密码 - 使用passwd命令，通过标准输入传递两次密码
	cmd := exec.Command("lxc", "exec", name, "--", "passwd", "root")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("创建stdin管道失败: %v", err)
	}

	// 设置输出管道
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// 启动命令
	if err := cmd.Start(); err != nil {
		stdin.Close()
		return fmt.Errorf("启动命令失败: %v", err)
	}

	// 写入两次密码（passwd需要输入两次进行确认）
	passwordInput := fmt.Sprintf("%s\n%s\n", password, password)
	_, err = stdin.Write([]byte(passwordInput))
	if err != nil {
		stdin.Close()
		cmd.Wait()
		return fmt.Errorf("写入密码失败: %v", err)
	}
	stdin.Close()

	// 等待命令完成
	if err := cmd.Wait(); err != nil {
		output := stdout.String() + stderr.String()
		return fmt.Errorf("修改root密码失败: %v, 输出: %s", err, output)
	}

	return nil
}

