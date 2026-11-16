package service

import (
	"LabPanel/models"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode"
)

var (
	ErrPortForwardInvalid     = errors.New("转发参数错误")
	ErrPortForwardConflict    = errors.New("转发规则冲突")
	ErrPortForwardNotFound    = errors.New("转发规则不存在")
	portForwardServiceMu      sync.RWMutex
	defaultPortForwardService *PortForwardService
)

type PortForwardService struct {
	mu       sync.Mutex
	path     string
	rules    []models.PortForward
	runtimes map[string]*portForwardRuntime
	errors   map[string]string
	closed   bool
}

type portForwardStore struct {
	Forwards []models.PortForward `json:"forwards"`
}

func InitPortForwardService(path string) error {
	portForwardServiceMu.Lock()
	defer portForwardServiceMu.Unlock()
	if defaultPortForwardService != nil {
		defaultPortForwardService.Close()
		defaultPortForwardService = nil
	}
	s, err := NewPortForwardService(path)
	if err != nil {
		return err
	}
	defaultPortForwardService = s
	return nil
}

func GetPortForwardService() *PortForwardService {
	portForwardServiceMu.RLock()
	defer portForwardServiceMu.RUnlock()
	return defaultPortForwardService
}

// NewPortForwardService restores enabled listeners. A bind failure affects only
// that rule, so the panel remains available to fix or disable it.
func NewPortForwardService(path string) (*PortForwardService, error) {
	s := &PortForwardService{
		path: path, rules: []models.PortForward{},
		runtimes: make(map[string]*portForwardRuntime), errors: make(map[string]string),
	}
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("读取端口转发配置失败: %w", err)
	}
	if err == nil {
		var store portForwardStore
		if err := json.Unmarshal(data, &store); err != nil {
			return nil, fmt.Errorf("解析端口转发配置失败: %w", err)
		}
		ids, names := map[string]bool{}, map[string]bool{}
		for _, rule := range store.Forwards {
			rule, err = normalizePortForward(rule)
			if err != nil {
				return nil, fmt.Errorf("端口转发配置无效: %w", err)
			}
			if rule.ID == "" || ids[rule.ID] || names[strings.ToLower(rule.Name)] {
				return nil, fmt.Errorf("端口转发配置包含空 ID、重复 ID 或重复名称")
			}
			ids[rule.ID], names[strings.ToLower(rule.Name)] = true, true
			s.rules = append(s.rules, rule)
		}
	}
	for _, rule := range s.rules {
		if rule.Enabled {
			runtime, err := startPortForward(rule)
			if err != nil {
				s.errors[rule.ID] = err.Error()
				log.Printf("恢复端口转发 %q 失败: %v", rule.Name, err)
			} else {
				s.runtimes[rule.ID] = runtime
			}
		}
	}
	return s, nil
}

func (s *PortForwardService) List() []models.PortForwardStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]models.PortForwardStatus, 0, len(s.rules))
	for _, rule := range s.rules {
		result = append(result, s.statusLocked(rule))
	}
	return result
}

func (s *PortForwardService) statusLocked(rule models.PortForward) models.PortForwardStatus {
	status := models.PortForwardStatus{PortForward: rule, Status: "stopped"}
	if rule.Enabled && !s.closed {
		status.Status = "error"
		status.Error = s.errors[rule.ID]
		if runtime := s.runtimes[rule.ID]; runtime != nil {
			if err := runtime.Err(); err != nil {
				status.Error = err.Error()
			} else {
				status.Status, status.Error = "running", ""
			}
		}
	}
	return status
}

// Save creates a rule when id is empty, otherwise replaces that rule. Configuration
// is saved atomically, and failed updates preserve the previous listener and rule.
func (s *PortForwardService) Save(id string, rule models.PortForward) (models.PortForwardStatus, error) {
	rule, err := normalizePortForward(rule)
	if err != nil {
		return models.PortForwardStatus{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return models.PortForwardStatus{}, fmt.Errorf("端口转发服务已停止")
	}
	index := -1
	for i, existing := range s.rules {
		if existing.ID == id {
			index = i
		}
	}
	if id != "" && index == -1 {
		return models.PortForwardStatus{}, ErrPortForwardNotFound
	}
	for _, existing := range s.rules {
		if existing.ID != id && strings.EqualFold(existing.Name, rule.Name) {
			return models.PortForwardStatus{}, fmt.Errorf("%w: 名称 %q 已存在", ErrPortForwardConflict, rule.Name)
		}
	}
	if id == "" {
		buf := make([]byte, 16)
		if _, err := rand.Read(buf); err != nil {
			return models.PortForwardStatus{}, fmt.Errorf("生成转发 ID 失败: %w", err)
		}
		id = hex.EncodeToString(buf)
	}
	rule.ID = id
	var old models.PortForward
	if index >= 0 {
		old = s.rules[index]
	}
	oldRuntime := s.runtimes[id]
	oldHealthy := oldRuntime != nil && oldRuntime.Err() == nil
	sameListener := old.Protocol == rule.Protocol && old.ListenAddress == rule.ListenAddress && old.ListenPort == rule.ListenPort
	reuse := rule.Enabled && oldHealthy && sameListener
	stoppedOld := false
	var candidate *portForwardRuntime
	if rule.Enabled && !reuse {
		// Overlapping old/new binds (e.g. changing 127.0.0.1 to 0.0.0.0)
		// require releasing the old socket first. Restore it on any failure.
		if oldRuntime != nil && old.Protocol == rule.Protocol && old.ListenPort == rule.ListenPort {
			oldRuntime.Close()
			delete(s.runtimes, id)
			stoppedOld = true
		}
		candidate, err = startPortForward(rule)
		if err != nil {
			err = fmt.Errorf("%w: 无法监听宿主机端口: %v", ErrPortForwardConflict, err)
			return models.PortForwardStatus{}, s.restoreLocked(old, stoppedOld && oldHealthy, err)
		}
	}
	next := append([]models.PortForward(nil), s.rules...)
	if index < 0 {
		next = append(next, rule)
	} else {
		next[index] = rule
	}
	if err := s.saveLocked(next); err != nil {
		if candidate != nil {
			candidate.Close()
		}
		return models.PortForwardStatus{}, s.restoreLocked(old, stoppedOld && oldHealthy, err)
	}
	if reuse {
		oldRuntime.SetTarget(rule.TargetAddress, rule.TargetPort)
	} else {
		if oldRuntime != nil && !stoppedOld {
			oldRuntime.Close()
		}
		delete(s.runtimes, id)
		if candidate != nil {
			s.runtimes[id] = candidate
		}
	}
	delete(s.errors, id)
	s.rules = next
	return s.statusLocked(rule), nil
}

func (s *PortForwardService) restoreLocked(old models.PortForward, needed bool, cause error) error {
	if needed {
		runtime, err := startPortForward(old)
		if err != nil {
			s.errors[old.ID] = err.Error()
			return fmt.Errorf("%w；恢复原转发失败: %v", cause, err)
		}
		s.runtimes[old.ID] = runtime
	}
	return cause
}

func (s *PortForwardService) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("端口转发服务已停止")
	}
	for i, rule := range s.rules {
		if rule.ID != id {
			continue
		}
		next := make([]models.PortForward, 0, len(s.rules)-1)
		next = append(next, s.rules[:i]...)
		next = append(next, s.rules[i+1:]...)
		if err := s.saveLocked(next); err != nil {
			return err
		}
		if runtime := s.runtimes[id]; runtime != nil {
			runtime.Close()
		}
		delete(s.runtimes, id)
		delete(s.errors, id)
		s.rules = next
		return nil
	}
	return ErrPortForwardNotFound
}

func (s *PortForwardService) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	for id, runtime := range s.runtimes {
		runtime.Close()
		delete(s.runtimes, id)
	}
}

func (s *PortForwardService) saveLocked(rules []models.PortForward) error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建端口转发配置目录失败: %w", err)
	}
	file, err := os.CreateTemp(dir, ".port-forwards-*.tmp")
	if err != nil {
		return fmt.Errorf("创建端口转发配置文件失败: %w", err)
	}
	defer os.Remove(file.Name())
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err = encoder.Encode(portForwardStore{Forwards: rules}); err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(file.Name(), s.path)
	}
	if err != nil {
		return fmt.Errorf("保存端口转发配置失败: %w", err)
	}
	return nil
}

func normalizePortForward(rule models.PortForward) (models.PortForward, error) {
	rule.Name = strings.TrimSpace(rule.Name)
	rule.Protocol = strings.ToLower(strings.TrimSpace(rule.Protocol))
	rule.ListenAddress = strings.TrimSpace(rule.ListenAddress)
	rule.TargetAddress = strings.TrimSpace(rule.TargetAddress)
	rule.Comment = strings.TrimSpace(rule.Comment)
	invalid := func(message string) (models.PortForward, error) {
		return rule, fmt.Errorf("%w: %s", ErrPortForwardInvalid, message)
	}
	if rule.Name == "" || len(rule.Name) > 128 || len(rule.Comment) > 2000 {
		return invalid("名称必填且不能超过 128 字节，备注不能超过 2000 字节")
	}
	if rule.Protocol != "tcp" && rule.Protocol != "udp" {
		return invalid("协议必须为 TCP 或 UDP")
	}
	if rule.ListenAddress == "" {
		rule.ListenAddress = "0.0.0.0"
	}
	// Accept bracketed IPv6 input as well as bare IPv6 addresses.
	rule.ListenAddress = strings.TrimSuffix(strings.TrimPrefix(rule.ListenAddress, "["), "]")
	listen, err := netip.ParseAddr(rule.ListenAddress)
	if err != nil {
		return invalid("宿主机监听地址必须为 IPv4 或 IPv6 地址")
	}
	rule.ListenAddress = listen.Unmap().String()
	rule.TargetAddress = strings.TrimSuffix(strings.TrimPrefix(rule.TargetAddress, "["), "]")
	if rule.ListenPort < 1 || rule.ListenPort > 65535 || rule.TargetPort < 1 || rule.TargetPort > 65535 {
		return invalid("端口必须在 1–65535 之间")
	}
	target, targetErr := netip.ParseAddr(rule.TargetAddress)
	if targetErr == nil {
		if target.IsUnspecified() {
			return invalid("目标地址不能为通配地址，请填写实际 IP 或域名")
		}
		rule.TargetAddress = target.Unmap().String()
	} else {
		if len(rule.TargetAddress) == 0 || len(rule.TargetAddress) > 253 || strings.ContainsAny(rule.TargetAddress, ":/\\?#@[]") || strings.ContainsFunc(rule.TargetAddress, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) {
			return invalid("目标地址必须为 IP 或域名，不能包含协议、路径或端口")
		}
		for _, label := range strings.Split(strings.TrimSuffix(rule.TargetAddress, "."), ".") {
			if len(label) == 0 || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
				return invalid("目标域名格式无效")
			}
			for _, ch := range label {
				if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '-' || ch == '_') {
					return invalid("目标域名格式无效，请使用 ASCII 或 Punycode 域名")
				}
			}
		}
	}
	if rule.ListenPort == rule.TargetPort && pointsToListener(listen.Unmap(), target.Unmap(), rule.TargetAddress) {
		return invalid("目标端口不能指向转发规则自身")
	}
	return rule, nil
}

func pointsToListener(listen, target netip.Addr, targetHost string) bool {
	if strings.EqualFold(strings.TrimSuffix(targetHost, "."), "localhost") {
		return listen.IsLoopback() || listen.IsUnspecified()
	}
	if target.IsValid() && target == listen {
		return true
	}
	if !listen.IsUnspecified() {
		return false
	}
	if !target.IsValid() || listen.Is4() != target.Is4() {
		return false
	}
	if target.IsLoopback() {
		return true
	}
	addresses, _ := net.InterfaceAddrs()
	for _, address := range addresses {
		ip, _, err := net.ParseCIDR(address.String())
		if err == nil && ip.String() == target.String() {
			return true
		}
	}
	return false
}
