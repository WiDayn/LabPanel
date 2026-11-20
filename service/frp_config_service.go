package service

import (
	"LabPanel/config"
	"LabPanel/models"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pelletier/go-toml/v2"
)

const frpMetadataPrefix = "# LabPanel routing v1 "

var frpConfigMu sync.Mutex

type FrpConfigService struct {
	cfg *config.Config
}

type frpRouteState struct {
	Comment string               `json:"comment,omitempty"`
	Binding *models.ProxyBinding `json:"binding,omitempty"`
	Paused  string               `json:"paused,omitempty"`
}

type frpDocument struct {
	original []byte
	root     map[string]any
	rules    []map[string]any
	state    map[string]frpRouteState
}

func NewFrpConfigService() *FrpConfigService {
	cfg, _ := config.Load()
	return &FrpConfigService{cfg: cfg}
}

func (s *FrpConfigService) readDocument() (*frpDocument, error) {
	data, err := os.ReadFile(s.cfg.TomlPath)
	if err != nil {
		return nil, err
	}
	doc := &frpDocument{original: data, state: make(map[string]frpRouteState)}
	if err := toml.Unmarshal(data, &doc.root); err != nil {
		return nil, err
	}
	var rows struct{ Proxies []map[string]any }
	if err := toml.Unmarshal(data, &rows); err != nil {
		return nil, err
	}
	doc.rules = rows.Proxies
	seen := make(map[string]bool)
	for _, proxy := range doc.rules {
		name, _ := proxy["name"].(string)
		if name == "" || seen[name] {
			return nil, fmt.Errorf("FRP 配置包含空名称或重复规则 %q", name)
		}
		seen[name] = true
	}
	for _, line := range bytes.Split(data, []byte{'\n'}) {
		encoded, found := strings.CutPrefix(string(line), frpMetadataPrefix)
		if !found {
			continue
		}
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
		if err != nil {
			return nil, fmt.Errorf("解析 FRP 绑定记录: %w", err)
		}
		if err := json.Unmarshal(decoded, &doc.state); err != nil || doc.state == nil {
			return nil, fmt.Errorf("FRP 绑定记录格式错误")
		}
		names := make([]string, 0)
		for name, state := range doc.state {
			if state.Paused != "" {
				names = append(names, name)
			}
		}
		sort.Strings(names)
		for _, name := range names {
			var proxy map[string]any
			if err := toml.Unmarshal([]byte(doc.state[name].Paused), &proxy); err != nil || proxy["name"] != name || seen[name] {
				return nil, fmt.Errorf("暂停规则 %s 的记录不一致", name)
			}
			doc.rules = append(doc.rules, proxy)
		}
		return doc, nil
	}
	legacy, err := os.ReadFile(filepath.Join(filepath.Dir(s.cfg.TomlPath), "proxy_comments.json"))
	if err == nil {
		var comments models.ProxyComments
		if err := json.Unmarshal(legacy, &comments); err != nil {
			return nil, fmt.Errorf("解析原 FRP 备注: %w", err)
		}
		for name, comment := range comments {
			if seen[name] {
				doc.state[name] = frpRouteState{Comment: comment}
			}
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	return doc, nil
}

func (s *FrpConfigService) GetConfig() (*models.FrpConfig, error) {
	frpConfigMu.Lock()
	defer frpConfigMu.Unlock()
	doc, err := s.readDocument()
	if err != nil {
		return nil, err
	}
	var result models.FrpConfig
	if err := toml.Unmarshal(doc.original, &result); err != nil {
		return nil, err
	}
	result.Proxies = make([]models.Proxy, 0, len(doc.rules))
	instances, _ := listFRPInstances()
	for _, row := range doc.rules {
		var proxy models.Proxy
		data, err := toml.Marshal(row)
		if err != nil {
			return nil, err
		}
		if err := toml.Unmarshal(data, &proxy); err != nil {
			return nil, err
		}
		state := doc.state[proxy.Name]
		proxy.Comment, proxy.Binding, proxy.Paused = state.Comment, state.Binding, state.Paused != ""
		if proxy.Binding == nil {
			proxy.Candidate = uniqueFRPCandidate(instances, proxy.LocalIP)
		}
		result.Proxies = append(result.Proxies, proxy)
	}
	return &result, nil
}

func (s *FrpConfigService) SaveAndReload(settings *models.FrpConfig) error {
	frpConfigMu.Lock()
	defer frpConfigMu.Unlock()
	doc, err := s.readDocument()
	if err != nil {
		return err
	}
	doc.root["serverAddr"], doc.root["serverPort"] = settings.ServerAddr, settings.ServerPort
	updateFRPTable(doc.root, "auth", map[string]any{"method": settings.Auth.Method, "token": settings.Auth.Token})
	updateFRPTable(doc.root, "webServer", map[string]any{"addr": settings.WebServer.Addr, "port": settings.WebServer.Port})
	return s.apply(doc)
}

func updateFRPTable(root map[string]any, name string, values map[string]any) {
	section, _ := root[name].(map[string]any)
	if section == nil {
		section = make(map[string]any)
		root[name] = section
	}
	for key, value := range values {
		section[key] = value
	}
}

func (s *FrpConfigService) AddProxy(proxy models.Proxy) error {
	return s.editProxy(-1, "", &proxy)
}

func (s *FrpConfigService) UpdateProxy(index int, originalName string, proxy models.Proxy) error {
	if index < 0 && originalName == "" {
		return fmt.Errorf("缺少原规则名称")
	}
	return s.editProxy(index, originalName, &proxy)
}

func (s *FrpConfigService) DeleteProxy(index int, name string) error {
	return s.editProxy(index, name, nil)
}

func (s *FrpConfigService) ConfirmBinding(name string, binding models.ProxyBinding) (models.ProxyBinding, error) {
	frpConfigMu.Lock()
	defer frpConfigMu.Unlock()
	doc, err := s.readDocument()
	if err != nil {
		return binding, err
	}
	for _, row := range doc.rules {
		if row["name"] != name {
			continue
		}
		state := doc.state[name]
		if state.Binding != nil && *state.Binding == binding && row["localIP"] == binding.Address {
			return binding, nil
		}
		if state.Binding != nil || row["localIP"] != binding.Address || (row["type"] != "tcp" && row["type"] != "udp") {
			return binding, fmt.Errorf("规则目标或绑定状态已变化，请刷新后核对")
		}
		release, err := lockLxcOperation(binding.Container)
		if err != nil {
			return binding, err
		}
		defer release()
		verified, rotated, err := confirmFRPTarget(doc, binding)
		if err != nil {
			return binding, err
		}
		state.Binding = &verified
		doc.state[name] = state
		if err := s.apply(doc); err != nil {
			if rotated {
				err = errors.Join(err, setLxcUUID(binding.Container, binding.InstanceID, false))
			}
			return binding, err
		}
		return verified, nil
	}
	return binding, fmt.Errorf("规则已变化，请刷新后重试")
}

func (s *FrpConfigService) editProxy(index int, name string, proxy *models.Proxy) error {
	frpConfigMu.Lock()
	defer frpConfigMu.Unlock()
	doc, err := s.readDocument()
	if err != nil {
		return err
	}
	if name != "" {
		index = -1
		for candidateIndex, candidate := range doc.rules {
			if candidate["name"] == name {
				index = candidateIndex
				break
			}
		}
	}
	if index >= len(doc.rules) || (index < 0 && (name != "" || proxy == nil)) {
		return fmt.Errorf("规则已变化，请刷新后重试")
	}
	if proxy == nil {
		oldName := doc.rules[index]["name"].(string)
		delete(doc.state, oldName)
		doc.rules = append(doc.rules[:index], doc.rules[index+1:]...)
		return s.apply(doc)
	}
	proxy.Name, proxy.Type, proxy.LocalIP = strings.TrimSpace(proxy.Name), strings.ToLower(strings.TrimSpace(proxy.Type)), strings.TrimSpace(proxy.LocalIP)
	if proxy.Name == "" || proxy.Type == "" || proxy.LocalPort < 1 || proxy.LocalPort > 65535 || proxy.RemotePort < 1 || proxy.RemotePort > 65535 {
		return fmt.Errorf("请检查规则名称、协议和端口")
	}
	for candidateIndex, candidate := range doc.rules {
		if candidateIndex == index {
			continue
		}
		if candidate["name"] == proxy.Name || (candidate["type"] == proxy.Type && candidate["remotePort"] == int64(proxy.RemotePort)) {
			return fmt.Errorf("规则名称或协议端口已被使用")
		}
	}
	state := frpRouteState{Comment: proxy.Comment}
	row := make(map[string]any)
	if index >= 0 {
		row = doc.rules[index]
		delete(doc.state, row["name"].(string))
	}
	if proxy.Binding != nil {
		if proxy.Type != "tcp" && proxy.Type != "udp" {
			return fmt.Errorf("容器绑定支持 TCP 和 UDP")
		}
		release, err := lockLxcOperation(proxy.Binding.Container)
		if err != nil {
			return err
		}
		defer release()
		binding, err := reserveFRPAddress(*proxy.Binding)
		if err != nil {
			return err
		}
		state.Binding = &binding
		proxy.LocalIP = binding.Address
	} else if proxy.LocalIP == "" {
		return fmt.Errorf("请输入目标地址")
	}
	row["name"], row["type"], row["localIP"] = proxy.Name, proxy.Type, proxy.LocalIP
	row["localPort"], row["remotePort"] = int64(proxy.LocalPort), int64(proxy.RemotePort)
	if index < 0 {
		doc.rules = append(doc.rules, row)
	}
	doc.state[proxy.Name] = state
	return s.apply(doc)
}

func (s *FrpConfigService) apply(doc *frpDocument) error {
	active := make([]map[string]any, 0, len(doc.rules))
	for _, row := range doc.rules {
		if doc.state[row["name"].(string)].Paused == "" {
			active = append(active, row)
		}
	}
	doc.root["proxies"] = active
	configBytes, err := toml.Marshal(doc.root)
	if err != nil {
		return err
	}
	state, err := json.Marshal(doc.state)
	if err != nil {
		return err
	}
	data := append([]byte(frpMetadataPrefix+base64.StdEncoding.EncodeToString(state)+"\n"), configBytes...)
	if bytes.Equal(data, doc.original) {
		return nil
	}
	return s.applyRaw(doc.original, data)
}

func (s *FrpConfigService) applyRaw(previous, data []byte) error {
	var oldConfig, newConfig map[string]any
	oldParsed := toml.Unmarshal(previous, &oldConfig) == nil
	newParsed := toml.Unmarshal(data, &newConfig) == nil
	reload := !oldParsed || !newParsed || !reflect.DeepEqual(oldConfig, newConfig)
	candidate, err := writeFRPCandidate(s.cfg.TomlPath, data)
	if err != nil {
		return err
	}
	defer os.Remove(candidate)
	if err := s.frpc("verify", candidate); err != nil {
		return err
	}
	live, err := os.ReadFile(s.cfg.TomlPath)
	if err != nil {
		return err
	}
	if !bytes.Equal(live, previous) {
		return fmt.Errorf("FRP 配置已从其他位置修改，请刷新后重试")
	}
	if !reload {
		return os.Rename(candidate, s.cfg.TomlPath)
	}
	backup, err := writeFRPCandidate(s.cfg.TomlPath, previous)
	if err != nil {
		return err
	}
	defer func() {
		if backup != "" {
			_ = os.Remove(backup)
		}
	}()
	if err := os.Rename(candidate, s.cfg.TomlPath); err != nil {
		return err
	}
	if err := s.frpc("reload", s.cfg.TomlPath); err != nil {
		if rollbackErr := os.Rename(backup, s.cfg.TomlPath); rollbackErr != nil {
			backupPath := backup
			backup = ""
			return errors.Join(err, fmt.Errorf("恢复配置失败，备份位于 %s: %w", backupPath, rollbackErr))
		}
		return errors.Join(err, s.frpc("reload", s.cfg.TomlPath))
	}
	return nil
}

func writeFRPCandidate(path string, data []byte) (string, error) {
	file, err := os.CreateTemp(filepath.Dir(path), ".labpanel-frp-*.toml")
	if err != nil {
		return "", err
	}
	name := file.Name()
	if info, err := os.Stat(path); err == nil {
		if err := file.Chmod(info.Mode().Perm()); err != nil {
			_ = file.Close()
			_ = os.Remove(name)
			return "", err
		}
	}
	_, writeErr := file.Write(data)
	if err := errors.Join(writeErr, file.Sync(), file.Close()); err != nil {
		_ = os.Remove(name)
		return "", err
	}
	return name, nil
}

func (s *FrpConfigService) frpc(action, path string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, s.cfg.FrpcPath, action, "-c", path).CombinedOutput()
	if err != nil {
		return fmt.Errorf("FRP %s 失败: %w: %s", action, err, strings.TrimSpace(string(output)))
	}
	return nil
}
