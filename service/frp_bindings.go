package service

import (
	"LabPanel/config"
	"LabPanel/models"
	"fmt"
	"log"
	"net/netip"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

type frpInstance struct {
	Name            string
	Type            string
	Status          string
	Config          map[string]string
	Devices         map[string]map[string]string
	ExpandedDevices map[string]map[string]string `json:"expanded_devices"`
	State           struct {
		Network map[string]struct {
			Hwaddr    string
			Addresses []struct{ Family, Address, Scope string }
		}
	}
}

type FRPTarget struct {
	InstanceID string `json:"instanceId"`
	Container  string `json:"container"`
	Device     string `json:"device"`
	Address    string `json:"address"`
}

func listFRPInstances() ([]frpInstance, error) {
	var instances []frpInstance
	err := readStorageJSON(&instances, "list", "--project=default", "--format=json")
	return instances, err
}

func frpTargets(instances []frpInstance) []FRPTarget {
	targets := make([]FRPTarget, 0)
	for _, instance := range instances {
		id := instance.Config["volatile.uuid"]
		if id == "" || instance.Type != "container" || !strings.EqualFold(instance.Status, "running") {
			continue
		}
		for deviceName, device := range instance.ExpandedDevices {
			if device["type"] != "nic" || device["network"] == "" {
				continue
			}
			iface := instance.State.Network[device["name"]]
			for _, entry := range iface.Addresses {
				address, err := netip.ParseAddr(entry.Address)
				if err != nil || !address.Is4() || !address.IsGlobalUnicast() || entry.Scope != "global" {
					continue
				}
				targets = append(targets, FRPTarget{InstanceID: id, Container: instance.Name, Device: deviceName, Address: entry.Address})
			}
		}
	}
	return targets
}

func ListFRPTargets() ([]FRPTarget, error) {
	instances, err := listFRPInstances()
	if err != nil {
		return nil, err
	}
	return frpTargets(instances), nil
}

func uniqueFRPCandidate(instances []frpInstance, address string) string {
	var candidate string
	for _, target := range frpTargets(instances) {
		if target.Address != address {
			continue
		}
		if candidate != "" {
			return ""
		}
		candidate = target.Container
	}
	return candidate
}

func validateFRPTarget(instances []frpInstance, binding models.ProxyBinding) (frpInstance, error) {
	if binding.InstanceID == "" || !storageNamePattern.MatchString(binding.Device) {
		return frpInstance{}, fmt.Errorf("请选择容器和网卡")
	}
	var instance frpInstance
	for _, candidate := range instances {
		if candidate.Config["volatile.uuid"] != binding.InstanceID {
			continue
		}
		if instance.Name != "" {
			return frpInstance{}, fmt.Errorf("发现重复容器身份")
		}
		instance = candidate
	}
	ownerCount := 0
	expectedOwner := false
	for _, candidate := range instances {
		for interfaceName, iface := range candidate.State.Network {
			for _, entry := range iface.Addresses {
				if entry.Address != binding.Address {
					continue
				}
				ownerCount++
				expectedOwner = expectedOwner || (candidate.Name == instance.Name && interfaceName == instance.ExpandedDevices[binding.Device]["name"])
			}
		}
	}
	if ownerCount != 1 || !expectedOwner {
		return frpInstance{}, fmt.Errorf("地址 %s 的归属已变化或存在冲突", binding.Address)
	}
	for _, candidate := range instances {
		for name, device := range candidate.ExpandedDevices {
			if device["ipv4.address"] == binding.Address && (candidate.Name != instance.Name || name != binding.Device) {
				return frpInstance{}, fmt.Errorf("地址 %s 已由其他网卡保留", binding.Address)
			}
		}
	}
	for _, target := range frpTargets([]frpInstance{instance}) {
		if target.Device == binding.Device && target.Address == binding.Address {
			return instance, nil
		}
	}
	return frpInstance{}, fmt.Errorf("所选容器网卡没有可验证的 IPv4 地址")
}

func verifyFRPLease(instance frpInstance, binding models.ProxyBinding) error {
	device := instance.ExpandedDevices[binding.Device]
	network := device["network"]
	if !storageNamePattern.MatchString(network) {
		return fmt.Errorf("目标网卡必须连接到 LXD 管理的网桥")
	}
	var bridge struct {
		Type    string
		Managed bool
	}
	if err := readStorageJSON(&bridge, "query", "/1.0/networks/"+network+"?project=default"); err != nil {
		return err
	}
	if bridge.Type != "bridge" || !bridge.Managed {
		return fmt.Errorf("目标网络不是 LXD 管理的网桥")
	}
	hardwareAddress := device["hwaddr"]
	if hardwareAddress == "" {
		hardwareAddress = instance.Config["volatile."+binding.Device+".hwaddr"]
	}
	if hardwareAddress == "" || !strings.EqualFold(hardwareAddress, instance.State.Network[device["name"]].Hwaddr) {
		return fmt.Errorf("当前 MAC 与 LXD 网卡配置不一致")
	}
	var leases []struct{ Address, Hwaddr, Type string }
	if err := readStorageJSON(&leases, "network", "list-leases", network, "--project=default", "--format=json"); err != nil {
		return err
	}
	matched := false
	for _, lease := range leases {
		if lease.Address == binding.Address {
			if !strings.EqualFold(lease.Hwaddr, hardwareAddress) {
				return fmt.Errorf("DHCP 租约属于另一块网卡")
			}
			matched = true
		}
	}
	if !matched {
		return fmt.Errorf("没有找到该网卡的 DHCP 租约")
	}
	return nil
}

func reserveFRPAddress(binding models.ProxyBinding) (models.ProxyBinding, error) {
	instances, err := listFRPInstances()
	if err != nil {
		return binding, err
	}
	instance, err := validateFRPTarget(instances, binding)
	if err != nil {
		return binding, err
	}
	if instance.Name != binding.Container {
		return binding, fmt.Errorf("容器名称已变化，请重新选择")
	}
	if err := verifyFRPLease(instance, binding); err != nil {
		return binding, err
	}
	binding.Container = instance.Name
	reserved := instance.ExpandedDevices[binding.Device]["ipv4.address"]
	if reserved == binding.Address {
		return binding, nil
	}
	if reserved != "" {
		return binding, fmt.Errorf("网卡已有不同的 DHCP 保留地址 %s", reserved)
	}
	args := []string{"config", "device", "override", instance.Name, binding.Device, "ipv4.address=" + binding.Address, "--project=default"}
	if _, local := instance.Devices[binding.Device]; local {
		args[2] = "set"
	}
	_, err = lxcStorageCommand(10*time.Second, args...)
	return binding, err
}

func (s *FrpConfigService) reconcileBindings() error {
	frpConfigMu.Lock()
	defer frpConfigMu.Unlock()
	doc, err := s.readDocument()
	if err != nil {
		return err
	}
	hasBindings := false
	for _, state := range doc.state {
		hasBindings = hasBindings || state.Binding != nil
	}
	if !hasBindings {
		return nil
	}
	instances, inventoryErr := listFRPInstances()
	changed := false
	for _, row := range doc.rules {
		name := row["name"].(string)
		state := doc.state[name]
		if state.Binding == nil {
			continue
		}
		instance, ownerErr := validateFRPTarget(instances, *state.Binding)
		valid := ownerErr == nil && instance.ExpandedDevices[state.Binding.Device]["ipv4.address"] == state.Binding.Address
		if !valid && state.Paused == "" {
			data, err := toml.Marshal(row)
			if err != nil {
				return err
			}
			state.Paused = string(data)
			changed = true
		}
		if valid && (state.Paused != "" || state.Binding.Container != instance.Name || row["localIP"] != state.Binding.Address) {
			state.Binding.Container = instance.Name
			state.Paused = ""
			row["localIP"] = state.Binding.Address
			changed = true
		}
		doc.state[name] = state
	}
	if !changed {
		return inventoryErr
	}
	return s.apply(doc)
}

func (s *FrpConfigService) RequireUnboundContainer(name string) error {
	if s.cfg.FrpcPath == "" {
		return nil
	}
	frpConfigMu.Lock()
	defer frpConfigMu.Unlock()
	doc, err := s.readDocument()
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for rule, state := range doc.state {
		if state.Binding != nil && state.Binding.Container == name {
			return fmt.Errorf("请先解除 FRP 规则 %s 与该容器的绑定", rule)
		}
	}
	var instance frpInstance
	for rule, state := range doc.state {
		if state.Binding == nil {
			continue
		}
		if instance.Config == nil {
			if err := readStorageJSON(&instance, "query", "/1.0/instances/"+url.PathEscape(name)+"?project=default"); err != nil {
				return err
			}
		}
		if state.Binding.InstanceID == instance.Config["volatile.uuid"] {
			return fmt.Errorf("请先解除 FRP 规则 %s 与该容器的绑定", rule)
		}
	}
	return nil
}

func StartFRPReconciliation(cfg *config.Config) {
	if cfg.FrpcPath == "" {
		return
	}
	service := &FrpConfigService{cfg: cfg}
	go func() {
		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()
		for {
			if err := service.reconcileBindings(); err != nil {
				log.Printf("检查 FRP 容器绑定失败: %v", err)
			}
			<-ticker.C
		}
	}()
}
