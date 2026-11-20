package service

import (
	"LabPanel/models"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func editableDirectoryMount(device map[string]string) bool {
	return device["type"] == "disk" && device["pool"] == "" && device["path"] != "" && device["path"] != "/" && filepath.IsAbs(device["source"])
}

func ChangeLxcMount(instanceName, deviceName string, mount *models.LxcStorageMount, create bool) error {
	if !storageNamePattern.MatchString(deviceName) {
		return fmt.Errorf("设备名须以字母或数字开头，仅允许字母、数字、点、横线和下划线，最长 63 个字符")
	}
	release, err := lockLxcOperation(instanceName)
	if err != nil {
		return err
	}
	defer release()
	instance, err := readStorageInstance(instanceName)
	if err != nil {
		return err
	}
	if instance.Type != "container" {
		return fmt.Errorf("目录挂载仅支持 LXD 容器")
	}
	device, exists := instance.ExpandedDevices[deviceName]
	if create && exists {
		return fmt.Errorf("设备 %s 已存在", deviceName)
	}
	if !create && (!exists || !editableDirectoryMount(device) || !editableDirectoryMount(instance.Devices[deviceName])) {
		return fmt.Errorf("只能修改容器自有的宿主机目录挂载")
	}
	if mount == nil {
		for _, profileName := range instance.Profiles {
			var profile struct{ Devices map[string]map[string]string }
			if err := readStorageJSON(&profile, "query", "/1.0/profiles/"+url.PathEscape(profileName)); err != nil {
				return err
			}
			if _, inherited := profile.Devices[deviceName]; inherited {
				return fmt.Errorf("移除此设备会恢复配置档 %s 的同名设备，请在配置档中处理", profileName)
			}
		}
		_, err = lxcStorageCommand(20*time.Second, "config", "device", "remove", instanceName, deviceName)
		return err
	}
	if err := validateDirectoryMount(instance, deviceName, *mount); err != nil {
		return err
	}
	args := []string{"config", "device", "set", instanceName, deviceName}
	if create {
		args[2] = "add"
		args = append(args, "disk")
	}
	args = append(args, "source="+mount.Source, "path="+mount.Path, fmt.Sprintf("readonly=%t", mount.Readonly))
	_, err = lxcStorageCommand(20*time.Second, args...)
	return err
}

func validateDirectoryMount(instance storageInstance, deviceName string, mount models.LxcStorageMount) error {
	if !filepath.IsAbs(mount.Path) || mount.Path == "/" || filepath.Clean(mount.Path) != mount.Path {
		return fmt.Errorf("容器路径必须是规范的绝对路径，且不能为根目录")
	}
	for name, device := range instance.ExpandedDevices {
		if name != deviceName && device["type"] == "disk" && device["path"] == mount.Path {
			return fmt.Errorf("容器路径已由设备 %s 使用", name)
		}
	}
	if !filepath.IsAbs(mount.Source) || filepath.Clean(mount.Source) != mount.Source {
		return fmt.Errorf("宿主机目录必须是规范的绝对路径")
	}
	resolved, err := filepath.EvalSymlinks(mount.Source)
	if err != nil || resolved != mount.Source {
		return fmt.Errorf("宿主机目录不存在或含有符号链接")
	}
	info, err := os.Stat(mount.Source)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("宿主机路径必须是存在的目录")
	}
	mountpoints, err := LxcStorageMountpoints()
	if err != nil {
		return err
	}
	for _, path := range mountpoints {
		if strings.HasPrefix(mount.Source, path+"/") {
			return nil
		}
	}
	return fmt.Errorf("宿主机目录需位于 /mnt、/media 或 /srv 下已挂载的本地磁盘内")
}
