package service

import (
	"LabPanel/config"
	"LabPanel/models"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

type LxcService struct {
	cfg *config.Config
}

var lxcRestoreTargets sync.Map

func NewLxcService() *LxcService {
	cfg, _ := config.Load()
	return &LxcService{cfg: cfg}
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

	jsonOutput, err := extractJSONArray(output)
	if err != nil {
		return nil, fmt.Errorf("解析lxc输出失败: %v, 原始输出: %s", err, string(output))
	}

	// 解析JSON输出
	var rawContainers []map[string]interface{}
	if err := json.Unmarshal(jsonOutput, &rawContainers); err != nil {
		return nil, fmt.Errorf("解析lxc输出失败: %v, 原始输出: %s", err, string(output))
	}

	// 转换为模型
	groupsByContainer, _ := NewLxcGroupService().GroupsByContainer()
	containers := make([]models.LxcContainer, 0, len(rawContainers))
	for _, raw := range rawContainers {
		name := getString(raw, "name")
		container := models.LxcContainer{
			Name:     name,
			State:    getString(raw, "status"),
			Type:     getString(raw, "type"),
			Arch:     getString(raw, "architecture"),
			Profiles: getStringSlice(raw, "profiles"),
			Groups:   groupsByContainer[name],
		}
		if container.Groups == nil {
			container.Groups = []models.LxcGroup{}
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

func (s *LxcService) listContainerNames() ([]string, error) {
	containers, err := s.ListContainers()
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(containers))
	for _, container := range containers {
		names = append(names, container.Name)
	}
	return names, nil
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

func extractJSONArray(output []byte) ([]byte, error) {
	trimmed := bytes.TrimSpace(output)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("输出为空")
	}

	if json.Valid(trimmed) && trimmed[0] == '[' {
		return trimmed, nil
	}

	for i, b := range trimmed {
		if b != '[' {
			continue
		}

		candidate := bytes.TrimSpace(trimmed[i:])
		if json.Valid(candidate) {
			return candidate, nil
		}
	}

	return nil, fmt.Errorf("未找到 JSON 数组")
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
			_ = NewLxcGroupService().SetContainerGroups(name, nil)
			return nil
		}
		return fmt.Errorf("删除容器失败: %v, 输出: %s", err, outputStr)
	}
	_ = NewLxcGroupService().SetContainerGroups(name, nil)
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

func (s *LxcService) StartContainer(name string) error {
	cmd := exec.Command("lxc", "start", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("启动容器失败: %v, 输出: %s", err, string(output))
	}
	return nil
}

func (s *LxcService) StopContainer(name string) error {
	cmd := exec.Command("lxc", "stop", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("停止容器失败: %v, 输出: %s", err, string(output))
	}
	return nil
}

func (s *LxcService) ForceStopContainer(name string) error {
	cmd := exec.Command("lxc", "stop", name, "--force")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("强制停止容器失败: %v, 输出: %s", err, string(output))
	}
	return nil
}

func (s *LxcService) BackupContainer(name string) (string, error) {
	files, err := s.BackupContainerWithProgress(name, nil)
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", fmt.Errorf("备份完成，但未找到导出文件")
	}
	return files[0], nil
}

func (s *LxcService) BackupContainerWithProgress(name string, progress func(stage, message string, percent int)) ([]string, error) {
	backupDir := "./backups"
	if s.cfg != nil && strings.TrimSpace(s.cfg.LxcBackupDir) != "" {
		backupDir = strings.TrimSpace(s.cfg.LxcBackupDir)
	}

	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建备份目录失败: %v", err)
	}

	timestamp := time.Now().Format("20060102-150405")
	exportTarget := filepath.Join(backupDir, fmt.Sprintf("%s-%s.tar.gz", sanitizeBackupName(name), timestamp))

	if progress != nil {
		progress("exporting", "正在导出容器备份文件", 20)
	}
	exportOutput, err := s.runLxcCommandWithHeartbeat(
		45*time.Minute,
		"exporting",
		20,
		85,
		progress,
		"lxc", "export", name, exportTarget,
	)
	if err != nil {
		return nil, fmt.Errorf("导出容器备份失败: %v, 输出: %s", err, string(exportOutput))
	}

	if progress != nil {
		progress("collecting", "正在整理导出的备份文件", 90)
	}
	exportedFiles, err := resolveExportedArchivePaths(exportTarget)
	if err != nil {
		return nil, err
	}

	return exportedFiles, nil
}

func (s *LxcService) runLxcCommandWithHeartbeat(
	timeout time.Duration,
	stage string,
	startPercent int,
	endPercent int,
	progress func(stage, message string, percent int),
	name string,
	args ...string,
) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	done := make(chan struct{})

	var output []byte
	var err error

	go func() {
		output, err = cmd.CombinedOutput()
		close(done)
	}()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	startedAt := time.Now()

	for {
		select {
		case <-done:
			return output, err
		case <-ticker.C:
			if progress != nil {
				elapsed := time.Since(startedAt).Round(time.Second)
				nextPercent := startPercent + int(elapsed/(15*time.Second))
				if nextPercent > endPercent {
					nextPercent = endPercent
				}
				progress(stage, fmt.Sprintf("正在执行 %s，已耗时 %s", strings.Join(append([]string{name}, args...), " "), elapsed), nextPercent)
			}
		case <-ctx.Done():
			<-done
			if ctx.Err() == context.DeadlineExceeded {
				return output, fmt.Errorf("执行超时（>%s）", timeout)
			}
			return output, ctx.Err()
		}
	}
}

func sanitizeBackupName(name string) string {
	replacer := strings.NewReplacer("/", "-", "\\", "-", " ", "-", ":", "-", "@", "-")
	safeName := replacer.Replace(strings.TrimSpace(name))
	if safeName == "" {
		return "container"
	}
	return safeName
}

func resolveExportedArchivePaths(exportPrefix string) ([]string, error) {
	candidates := []string{
		exportPrefix,
		exportPrefix + ".tar.gz",
		exportPrefix + ".tar.xz",
		exportPrefix + ".zip",
	}

	results := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			results = append(results, candidate)
		}
	}
	if len(results) > 0 {
		return uniquePaths(results), nil
	}

	matches, err := filepath.Glob(exportPrefix + "*")
	if err != nil {
		return nil, fmt.Errorf("查找导出文件失败: %v", err)
	}
	results = results[:0]
	for _, match := range matches {
		info, statErr := os.Stat(match)
		if statErr == nil && !info.IsDir() {
			results = append(results, match)
		}
	}
	if len(results) > 0 {
		return uniquePaths(results), nil
	}

	return nil, fmt.Errorf("镜像已导出，但未找到备份文件，请检查目录: %s", exportPrefix)
}

func uniquePaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		result = append(result, path)
	}
	return result
}

func (s *LxcService) backupDir() string {
	backupDir := "./backups"
	if s.cfg != nil && strings.TrimSpace(s.cfg.LxcBackupDir) != "" {
		backupDir = strings.TrimSpace(s.cfg.LxcBackupDir)
	}
	return backupDir
}

func (s *LxcService) ListBackupArchives() ([]models.LxcBackupArchive, error) {
	backupDir := s.backupDir()
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建备份目录失败: %v", err)
	}

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return nil, fmt.Errorf("读取备份目录失败: %v", err)
	}

	archives := make([]models.LxcBackupArchive, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".tar.gz") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		fullPath := filepath.Join(backupDir, entry.Name())
		archives = append(archives, models.LxcBackupArchive{
			Name:         entry.Name(),
			Path:         fullPath,
			SizeBytes:    info.Size(),
			ModifiedAt:   info.ModTime(),
			DisplayLabel: fmt.Sprintf("%s (%.2f GB)", entry.Name(), float64(info.Size())/1024/1024/1024),
		})
	}

	sort.Slice(archives, func(i, j int) bool {
		return archives[i].ModifiedAt.After(archives[j].ModifiedAt)
	})

	return archives, nil
}

func (s *LxcService) DeleteBackupArchive(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("备份文件名不能为空")
	}
	if filepath.Base(name) != name || strings.Contains(name, string(filepath.Separator)) {
		return fmt.Errorf("备份文件名不合法")
	}
	if !strings.HasSuffix(strings.ToLower(name), ".tar.gz") {
		return fmt.Errorf("只能删除 .tar.gz 备份文件")
	}

	backupDir := s.backupDir()
	fullPath := filepath.Join(backupDir, name)
	cleanBackupDir, err := filepath.Abs(backupDir)
	if err != nil {
		return fmt.Errorf("解析备份目录失败: %v", err)
	}
	cleanTarget, err := filepath.Abs(fullPath)
	if err != nil {
		return fmt.Errorf("解析备份文件路径失败: %v", err)
	}
	if filepath.Dir(cleanTarget) != cleanBackupDir {
		return fmt.Errorf("备份文件路径不合法")
	}

	info, err := os.Stat(cleanTarget)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("备份文件不存在")
		}
		return fmt.Errorf("读取备份文件失败: %v", err)
	}
	if info.IsDir() {
		return fmt.Errorf("不能删除目录")
	}

	if err := os.Remove(cleanTarget); err != nil {
		return fmt.Errorf("删除备份文件失败: %v", err)
	}
	return nil
}

func (s *LxcService) GetContainerConfig(name string) (string, error) {
	cmd := exec.Command("lxc", "config", "show", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("获取容器配置失败: %v, 输出: %s", err, string(output))
	}
	return string(output), nil
}

func (s *LxcService) UpdateContainerConfig(name, config string) error {
	// LXC 配置更新比较复杂，因为 YAML 可能包含嵌套结构
	// 我们使用一个简化的方法：解析 YAML 格式的配置，然后使用 lxc config set 逐个设置

	// 定义只读的配置项（这些不能通过 lxc config set 设置）
	readOnlyKeys := map[string]bool{
		"architecture": true,
		"created_at":   true,
		"description":  true,
		"devices":      true, // devices 需要特殊处理
		"ephemeral":    true,
		"name":         true,
		"profiles":     true,
		"status":       true,
		"status_code":  true,
	}

	// 解析配置行，处理简单的 key: value 格式
	// 对于嵌套结构，使用点号分隔的键名（如 device.eth0.parent）
	lines := strings.Split(config, "\n")
	var errors []string
	indentStack := []int{} // 用于跟踪嵌套层级
	currentPath := ""
	skipConfig := false // 标记是否在 config: 块中（需要去掉 config. 前缀）

	for _, line := range lines {
		originalLine := line
		line = strings.TrimRight(line, " \t")

		// 跳过空行和注释
		if line == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}

		// 计算缩进
		indent := 0
		for _, char := range originalLine {
			if char == ' ' {
				indent++
			} else if char == '\t' {
				indent += 4
			} else {
				break
			}
		}

		// 更新缩进栈
		for len(indentStack) > 0 && indent <= indentStack[len(indentStack)-1] {
			indentStack = indentStack[:len(indentStack)-1]
			// 更新当前路径
			parts := strings.Split(currentPath, ".")
			if len(parts) > 1 {
				currentPath = strings.Join(parts[:len(parts)-1], ".")
			} else {
				currentPath = ""
			}
			// 检查是否退出 config: 块
			if currentPath == "" || !strings.HasPrefix(currentPath, "config") {
				skipConfig = false
			}
		}

		// 解析 key: value
		colonIndex := strings.Index(line, ":")
		if colonIndex == -1 {
			continue
		}

		key := strings.TrimSpace(line[:colonIndex])
		value := strings.TrimSpace(line[colonIndex+1:])

		// 检查是否是 config: 键
		if key == "config" && indent == 0 {
			skipConfig = true
			currentPath = ""
			indentStack = append(indentStack, indent)
			continue
		}

		// 构建完整路径
		var fullKey string
		if skipConfig {
			// 在 config: 块中，去掉 config. 前缀
			if currentPath != "" {
				// 去掉 config. 前缀
				pathWithoutConfig := strings.TrimPrefix(currentPath, "config.")
				if pathWithoutConfig != "" {
					fullKey = pathWithoutConfig + "." + key
				} else {
					fullKey = key
				}
			} else {
				fullKey = key
			}
		} else {
			if currentPath != "" {
				fullKey = currentPath + "." + key
			} else {
				fullKey = key
			}
		}

		// 跳过只读配置项
		if readOnlyKeys[key] && indent == 0 {
			continue
		}

		// 如果值不为空且不是嵌套对象（不以 { 或 - 开头），则设置配置
		if value != "" && !strings.HasPrefix(value, "{") && !strings.HasPrefix(value, "-") && !strings.HasPrefix(value, "[") {
			// 移除引号
			if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
				value = value[1 : len(value)-1]
			}

			// 跳过空值
			if value == "" {
				continue
			}

			// 使用 lxc config set 设置配置
			cmd := exec.Command("lxc", "config", "set", name, fullKey, value)
			output, err := cmd.CombinedOutput()
			if err != nil {
				outputStr := string(output)
				// 只记录真正的错误，忽略"不支持"的错误（这些可能是只读配置）
				if !strings.Contains(outputStr, "isn't supported") && !strings.Contains(outputStr, "read-only") {
					errors = append(errors, fmt.Sprintf("设置 %s 失败: %s", fullKey, strings.TrimSpace(outputStr)))
				}
			}
		} else {
			// 这是一个嵌套对象，更新当前路径
			if skipConfig {
				// 在 config: 块中，构建路径时去掉 config. 前缀
				if currentPath != "" {
					pathWithoutConfig := strings.TrimPrefix(currentPath, "config.")
					if pathWithoutConfig != "" {
						currentPath = "config." + pathWithoutConfig + "." + key
					} else {
						currentPath = "config." + key
					}
				} else {
					currentPath = "config." + key
				}
			} else {
				if currentPath != "" {
					currentPath = currentPath + "." + key
				} else {
					currentPath = key
				}
			}
			indentStack = append(indentStack, indent)
		}
	}

	// 如果有错误，返回错误信息
	if len(errors) > 0 {
		return fmt.Errorf("更新配置时出现错误: %s", strings.Join(errors[:min(5, len(errors))], "; "))
	}

	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (s *LxcService) CreateContainer(req models.CreateLxcRequest) error {
	name := strings.TrimSpace(req.Name)
	password := req.Password
	sourceType := strings.TrimSpace(req.SourceType)
	if sourceType == "" {
		sourceType = models.LxcCreateSourceDefaultImage
	}
	groupService := NewLxcGroupService()
	if err := groupService.ValidateGroupIDs(req.GroupIDs); err != nil {
		return err
	}

	switch sourceType {
	case models.LxcCreateSourceDefaultImage, models.LxcCreateSourceCustomImage:
		image := strings.TrimSpace(req.Image)
		if sourceType == models.LxcCreateSourceDefaultImage || image == "" {
			image = "ubuntu:22.04"
			if s.cfg != nil && strings.TrimSpace(s.cfg.LxcImage) != "" {
				image = strings.TrimSpace(s.cfg.LxcImage)
			}
		}
		if err := s.createContainerFromImage(name, password, image); err != nil {
			return err
		}
		return groupService.SetContainerGroups(name, req.GroupIDs)
	case models.LxcCreateSourceBackup:
		if err := s.createContainerFromBackup(name, password, strings.TrimSpace(req.BackupFile)); err != nil {
			return err
		}
		return groupService.SetContainerGroups(name, req.GroupIDs)
	default:
		return fmt.Errorf("不支持的创建来源: %s", sourceType)
	}
}

func (s *LxcService) createContainerFromImage(name, password, image string) error {
	// 1. 创建容器
	cmd := exec.Command("lxc", "launch", image, name, "-c", "nvidia.runtime=true")
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
			return fmt.Errorf("创建容器失败(镜像: %s): %v, 输出: %s", image, err, outputStr)
		}
	}

	// 2. 等待容器启动
	s.waitForContainerRunning(name, 60)

	// 3. 设置磁盘和 GPU 设备
	configureRootDiskAndGPU(name)

	// 5. 配置SSH
	if err := s.configureSSH(name, password); err != nil {
		return fmt.Errorf("配置SSH失败: %v", err)
	}

	return nil
}

func (s *LxcService) createContainerFromBackup(targetName, password, backupFile string) error {
	targetName = strings.TrimSpace(targetName)
	if targetName == "" {
		return fmt.Errorf("容器名称不能为空")
	}
	if _, loaded := lxcRestoreTargets.LoadOrStore(targetName, struct{}{}); loaded {
		return fmt.Errorf("容器 %s 已有恢复任务正在执行，请等待完成后再试", targetName)
	}
	defer lxcRestoreTargets.Delete(targetName)

	if backupFile == "" {
		return fmt.Errorf("请选择备份文件")
	}

	backupDir := s.backupDir()
	backupPath := filepath.Join(backupDir, filepath.Base(backupFile))
	if _, err := os.Stat(backupPath); err != nil {
		return fmt.Errorf("备份文件不存在: %s", backupPath)
	}

	beforeNames, err := s.listContainerNames()
	if err != nil {
		return fmt.Errorf("读取导入前容器列表失败: %v", err)
	}
	beforeSet := make(map[string]struct{}, len(beforeNames))
	for _, name := range beforeNames {
		beforeSet[name] = struct{}{}
	}
	if _, existed := beforeSet[targetName]; existed {
		return fmt.Errorf("容器名称已存在: %s", targetName)
	}

	if activeOps, err := activeImportOperations(targetName); err == nil && len(activeOps) > 0 {
		return fmt.Errorf("检测到已有导入或清理任务正在占用容器 %s，请等待或手动终止后再试: %s", targetName, strings.Join(activeOps, "; "))
	}

	if err := s.cleanupStaleImportTarget(targetName); err != nil {
		return fmt.Errorf("清理同名残留资源失败: %v", err)
	}

	if err := s.importBackupWithTargetName(backupPath, targetName); err != nil {
		return err
	}

	configureRootDiskAndGPU(targetName)

	if err := s.configureHostname(targetName); err != nil {
		return fmt.Errorf("配置容器 hostname 失败: %v", err)
	}

	if err := s.configureSSH(targetName, password); err != nil {
		return fmt.Errorf("配置SSH失败: %v", err)
	}

	return nil
}

func (s *LxcService) importBackupWithTargetName(backupPath, targetName string) error {
	macAddress, err := randomLxdMACAddress()
	if err != nil {
		return fmt.Errorf("生成新容器 MAC 地址失败: %v", err)
	}

	var lastErr error
	var lastOutput string
	for attempt := 0; attempt < 3; attempt++ {
		output, err := exec.Command("lxc", "import", backupPath, targetName, "--device", "eth0,hwaddr="+macAddress).CombinedOutput()
		if err == nil {
			return nil
		}

		lastErr = err
		lastOutput = string(output)
		recovered, recoverErr := s.recoverStaleImportTarget(lastOutput, targetName)
		if recoverErr != nil {
			return recoverErr
		}
		if !recovered {
			break
		}
	}

	return fmt.Errorf("从备份导入容器失败: %v, 输出: %s", lastErr, lastOutput)
}

func randomLxdMACAddress() (string, error) {
	suffix := make([]byte, 3)
	if _, err := rand.Read(suffix); err != nil {
		return "", err
	}

	return fmt.Sprintf("00:16:3e:%02x:%02x:%02x", suffix[0], suffix[1], suffix[2]), nil
}

func (s *LxcService) recoverStaleImportTarget(output, targetName string) (bool, error) {
	stalePath, hasStalePath := staleImportMountPath(output, targetName)
	staleDataset, hasStaleDataset := staleImportZFSDataset(output, targetName)
	missingDataset, hasMissingDataset := missingImportZFSDataset(output, targetName)
	hasStaleVolume := staleImportVolumeExists(output)
	if !hasStalePath && !hasStaleVolume && !hasStaleDataset && !hasMissingDataset {
		return false, nil
	}

	if exists, checkErr := s.containerNameExists(targetName); checkErr != nil {
		return false, fmt.Errorf("从备份导入容器失败，且检查残留资源前读取容器列表失败: %v, 原始输出: %s", checkErr, output)
	} else if exists {
		return false, fmt.Errorf("容器名称已存在: %s", targetName)
	}

	if hasStalePath {
		if removeErr := removeStaleImportMountDir(stalePath); removeErr != nil {
			return false, fmt.Errorf("从备份导入容器失败，检测到残留目录但清理失败: %v, 残留目录: %s, 原始输出: %s", removeErr, stalePath, output)
		}
	}

	if hasStaleVolume {
		if removeErr := s.removeStaleImportVolume(targetName); removeErr != nil {
			if zfsErr := removePotentialStaleContainerZFSDatasets(targetName); zfsErr != nil {
				return false, fmt.Errorf("从备份导入容器失败，检测到残留存储卷但清理失败: %v; 尝试清理同名 ZFS dataset 也失败: %v, 原始输出: %s", removeErr, zfsErr, output)
			}
		}
	}

	if hasMissingDataset {
		if removeErr := s.cleanupStaleImportTarget(targetName); removeErr != nil {
			return false, fmt.Errorf("从备份导入容器失败，检测到缺失的 ZFS dataset 但清理同名残留失败: %v, dataset: %s, 原始输出: %s", removeErr, missingDataset, output)
		}
	}

	if hasStaleDataset {
		if removeErr := removeStaleImportZFSDataset(staleDataset, targetName); removeErr != nil {
			return false, fmt.Errorf("从备份导入容器失败，检测到残留 ZFS dataset 但清理失败: %v, dataset: %s, 原始输出: %s", removeErr, staleDataset, output)
		}
	}

	return true, nil
}

func (s *LxcService) cleanupStaleImportTarget(targetName string) error {
	var errors []string

	if err := s.removeStaleImportVolume(targetName); err != nil && !isStaleVolumeNotFound(err) {
		errors = append(errors, err.Error())
	}
	if err := removePotentialStaleContainerZFSDatasets(targetName); err != nil && !isStaleZFSDatasetNotFound(err) && !isZFSUnavailable(err) {
		errors = append(errors, err.Error())
	}

	if len(errors) > 0 {
		return fmt.Errorf(strings.Join(errors, "; "))
	}
	return nil
}

func activeImportOperations(targetName string) ([]string, error) {
	lines, err := activeOperationLines()
	if err != nil {
		return nil, err
	}

	var operations []string
	for _, line := range lines {
		if targetNameFromOperation(line) == targetName || operationMentionsTarget(line, targetName) {
			operations = append(operations, compactProcessLine(line))
		}
	}
	if len(operations) > 5 {
		operations = operations[:5]
	}
	return operations, nil
}

func activeOperationLines() ([]string, error) {
	output, err := exec.Command("ps", "-eo", "pid=,stat=,args=").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("读取进程列表失败: %v, 输出: %s", err, string(output))
	}

	var operations []string
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.Contains(line, "lxc import ") ||
			strings.Contains(line, "lxc delete ") ||
			strings.Contains(line, "zfs destroy") ||
			strings.Contains(line, "tar -") {
			operations = append(operations, line)
		}
	}
	return operations, nil
}

func activeImportProcessStatuses() []models.LxcRestoreStatus {
	lines, err := activeOperationLines()
	if err != nil {
		return nil
	}

	now := time.Now()
	statuses := make([]models.LxcRestoreStatus, 0, len(lines))
	for _, line := range lines {
		name := targetNameFromOperation(line)
		if name == "" {
			continue
		}

		statuses = append(statuses, models.LxcRestoreStatus{
			Name:      name,
			TaskID:    "process-" + firstProcessField(line),
			Status:    models.LxcBackupStatusRunning,
			Stage:     "external",
			Progress:  50,
			Message:   "检测到系统中已有 LXC 导入或清理进程：" + compactProcessLine(line),
			StartedAt: now,
			UpdatedAt: now,
		})
	}
	return statuses
}

func targetNameFromOperation(line string) string {
	fields := strings.Fields(line)
	for i, field := range fields {
		switch field {
		case "import":
			if i+2 < len(fields) && !strings.HasPrefix(fields[i+2], "-") {
				return strings.Trim(fields[i+2], `'"`)
			}
		case "delete":
			if i+1 < len(fields) && !strings.HasPrefix(fields[i+1], "-") {
				return strings.Trim(fields[i+1], `'"`)
			}
		}
	}

	re := regexp.MustCompile(`(?:^|[\s/])containers/([^/\s'"]+)`)
	matches := re.FindStringSubmatch(line)
	if len(matches) == 2 {
		return matches[1]
	}

	return ""
}

func firstProcessField(line string) string {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return "unknown"
	}
	return fields[0]
}

func operationMentionsTarget(line, targetName string) bool {
	re := regexp.MustCompile(`(?:^|\s)` + regexp.QuoteMeta(targetName) + `(?:$|\s)`)
	if re.MatchString(line) {
		return true
	}

	pathRe := regexp.MustCompile(`(?:^|[\s/])containers/` + regexp.QuoteMeta(targetName) + `(?:$|[\s/])`)
	return pathRe.MatchString(line)
}

func compactProcessLine(line string) string {
	fields := strings.Fields(line)
	if len(fields) <= 8 {
		return line
	}

	return strings.Join(fields[:8], " ") + " ..."
}

func (s *LxcService) containerNameExists(targetName string) (bool, error) {
	names, err := s.listContainerNames()
	if err != nil {
		return false, err
	}

	for _, name := range names {
		if name == targetName {
			return true, nil
		}
	}
	return false, nil
}

func staleImportMountPath(output, targetName string) (string, bool) {
	lowerOutput := strings.ToLower(output)
	if !strings.Contains(lowerOutput, "file exists") || !strings.Contains(lowerOutput, "create mount directory") {
		return "", false
	}

	re := regexp.MustCompile(`mkdir\s+([^:]+):\s*file exists`)
	matches := re.FindStringSubmatch(output)
	if len(matches) != 2 {
		return "", false
	}

	path := filepath.Clean(strings.TrimSpace(matches[1]))
	if filepath.Base(path) != targetName {
		return "", false
	}
	if !strings.Contains(path, string(filepath.Separator)+"storage-pools"+string(filepath.Separator)) {
		return "", false
	}
	if filepath.Base(filepath.Dir(path)) != "containers" {
		return "", false
	}

	return path, true
}

func staleImportVolumeExists(output string) bool {
	lowerOutput := strings.ToLower(output)
	re := regexp.MustCompile(`volume[,\s]+already exists(?:\s+on\s+target)?`)
	return re.MatchString(lowerOutput)
}

func staleImportZFSDataset(output, targetName string) (string, bool) {
	lowerOutput := strings.ToLower(output)
	if !strings.Contains(lowerOutput, "cannot create") || !strings.Contains(lowerOutput, "dataset already exists") {
		return "", false
	}

	re := regexp.MustCompile(`cannot create\s+'?([^':\s]+)'?:\s*dataset already exists`)
	matches := re.FindStringSubmatch(output)
	if len(matches) != 2 {
		return "", false
	}

	dataset := path.Clean(strings.TrimSpace(matches[1]))
	if !validContainerZFSDataset(dataset, targetName) {
		return "", false
	}

	return dataset, true
}

func missingImportZFSDataset(output, targetName string) (string, bool) {
	lowerOutput := strings.ToLower(output)
	if !strings.Contains(lowerOutput, "cannot open") || !strings.Contains(lowerOutput, "dataset does not exist") {
		return "", false
	}

	re := regexp.MustCompile(`cannot open\s+'?([^':\s]+)'?:\s*dataset does not exist`)
	matches := re.FindStringSubmatch(output)
	if len(matches) != 2 {
		return "", false
	}

	dataset := path.Clean(strings.TrimSpace(matches[1]))
	if !validContainerZFSDataset(dataset, targetName) {
		return "", false
	}

	return dataset, true
}

func validContainerZFSDataset(dataset, targetName string) bool {
	if dataset == "." || strings.HasPrefix(dataset, "/") || strings.Contains(dataset, "@") || strings.Contains(dataset, "#") {
		return false
	}
	if path.Base(dataset) != targetName {
		return false
	}
	if path.Base(path.Dir(dataset)) != "containers" {
		return false
	}
	return strings.Contains(dataset, "/containers/")
}

func (s *LxcService) removeStaleImportVolume(targetName string) error {
	pools, err := s.storagePoolNames()
	if err != nil || len(pools) == 0 {
		pools = []string{"default"}
	}

	var errors []string
	for _, pool := range pools {
		output, deleteErr := exec.Command("lxc", "storage", "volume", "delete", pool, "container/"+targetName).CombinedOutput()
		if deleteErr == nil {
			return nil
		}

		outputStr := string(output)
		if strings.Contains(strings.ToLower(outputStr), "not found") {
			continue
		}
		errors = append(errors, fmt.Sprintf("%s: %v, 输出: %s", pool, deleteErr, outputStr))
	}

	if len(errors) == 0 {
		return fmt.Errorf("未找到残留存储卷 container/%s", targetName)
	}
	return fmt.Errorf(strings.Join(errors, "; "))
}

func isStaleVolumeNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), "未找到残留存储卷")
}

func (s *LxcService) storagePoolNames() ([]string, error) {
	output, err := exec.Command("lxc", "storage", "list", "--format", "json").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("读取存储池列表失败: %v, 输出: %s", err, string(output))
	}

	var rawPools []map[string]interface{}
	jsonOutput, err := extractJSONArray(output)
	if err != nil {
		return nil, fmt.Errorf("解析存储池列表失败: %v, 原始输出: %s", err, string(output))
	}
	if err := json.Unmarshal(jsonOutput, &rawPools); err != nil {
		return nil, fmt.Errorf("解析存储池列表失败: %v, 原始输出: %s", err, string(output))
	}

	pools := make([]string, 0, len(rawPools))
	for _, raw := range rawPools {
		if name := getString(raw, "name"); name != "" {
			pools = append(pools, name)
		}
	}
	return pools, nil
}

func removeStaleImportZFSDataset(dataset, targetName string) error {
	if !validContainerZFSDataset(dataset, targetName) {
		return fmt.Errorf("拒绝清理不匹配的 ZFS dataset")
	}

	output, err := exec.Command("zfs", "destroy", "-r", dataset).CombinedOutput()
	if err != nil {
		outputStr := string(output)
		if strings.Contains(strings.ToLower(outputStr), "dataset does not exist") {
			return nil
		}
		return fmt.Errorf("执行 zfs destroy 失败: %v, 输出: %s", err, string(output))
	}
	return nil
}

func removePotentialStaleContainerZFSDatasets(targetName string) error {
	output, err := exec.Command("zfs", "list", "-H", "-o", "name").CombinedOutput()
	if err != nil {
		return fmt.Errorf("读取 ZFS dataset 列表失败: %v, 输出: %s", err, string(output))
	}

	var candidates []string
	for _, line := range strings.Split(string(output), "\n") {
		dataset := path.Clean(strings.TrimSpace(line))
		if validContainerZFSDataset(dataset, targetName) {
			candidates = append(candidates, dataset)
		}
	}
	if len(candidates) == 0 {
		return fmt.Errorf("未找到匹配的 ZFS dataset */containers/%s", targetName)
	}

	var errors []string
	for _, dataset := range candidates {
		if err := removeStaleImportZFSDataset(dataset, targetName); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", dataset, err))
		}
	}
	if len(errors) > 0 {
		return fmt.Errorf(strings.Join(errors, "; "))
	}

	return nil
}

func isStaleZFSDatasetNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), "未找到匹配的 ZFS dataset")
}

func isZFSUnavailable(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "executable file not found") || strings.Contains(message, "permission denied")
}

func removeStaleImportMountDir(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("拒绝删除符号链接")
	}
	if !info.IsDir() {
		return fmt.Errorf("残留路径不是目录")
	}

	return os.RemoveAll(path)
}

func configureRootDiskAndGPU(name string) {
	cmd := exec.Command("lxc", "config", "device", "set", name, "root", "size", "200GB")
	if output, err := cmd.CombinedOutput(); err != nil {
		_ = output
	}

	cmd = exec.Command("lxc", "config", "device", "add", name, "gpu", "gpu")
	if output, err := cmd.CombinedOutput(); err != nil {
		outputStr := string(output)
		if !strings.Contains(outputStr, "already exists") && !strings.Contains(outputStr, "already exist") {
			_ = output
		}
	}
}

func (s *LxcService) configureHostname(name string) error {
	if err := s.waitForContainerRunning(name, 60); err != nil {
		return fmt.Errorf("容器未运行或无法访问: %v", err)
	}

	hostnameFile := name + "/etc/hostname"
	pushCmd := exec.Command("lxc", "file", "push", "--uid", "0", "--gid", "0", "--mode", "0644", "-", hostnameFile)
	pushCmd.Stdin = strings.NewReader(name + "\n")
	if output, err := pushCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("写入 /etc/hostname 失败: %v, 输出: %s", err, string(output))
	}

	script := `set +e
new_hostname="$1"
if command -v hostnamectl >/dev/null 2>&1; then
  hostnamectl set-hostname "$new_hostname" || true
fi
hostname "$new_hostname" || true
if [ -f /etc/hosts ]; then
  if grep -q '^127\.0\.1\.1[[:space:]]' /etc/hosts; then
    sed -i "s/^127\.0\.1\.1[[:space:]].*/127.0.1.1\t${new_hostname}/" /etc/hosts
  else
    printf '127.0.1.1\t%s\n' "$new_hostname" >> /etc/hosts
  fi
fi
if [ -f /etc/cloud/cloud.cfg ]; then
  if grep -q '^preserve_hostname:' /etc/cloud/cloud.cfg; then
    sed -i 's/^preserve_hostname:.*/preserve_hostname: true/' /etc/cloud/cloud.cfg
  else
    printf '\npreserve_hostname: true\n' >> /etc/cloud/cloud.cfg
  fi
fi`

	cmd := exec.Command("lxc", "exec", name, "--", "sh", "-c", script, "sh", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v, 输出: %s", err, string(output))
	}

	verifyOutput, err := exec.Command("lxc", "file", "pull", hostnameFile, "-").CombinedOutput()
	if err != nil {
		return fmt.Errorf("验证 /etc/hostname 失败: %v, 输出: %s", err, string(verifyOutput))
	}
	if strings.TrimSpace(string(verifyOutput)) != name {
		return fmt.Errorf("/etc/hostname 未更新，当前值: %s", strings.TrimSpace(string(verifyOutput)))
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
