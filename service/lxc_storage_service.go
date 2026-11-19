package service

import (
	"LabPanel/models"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

var storageNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,62}$`)
var storageWork = struct {
	sync.Mutex
	moves map[string]models.LxcStorageMove
}{moves: make(map[string]models.LxcStorageMove)}

type storagePoolInfoJSON struct {
	Name   string
	Driver string
	Status string
	Config map[string]string
}

type storageInstance struct {
	Type            string
	Status          string
	Config          map[string]string
	ExpandedDevices map[string]map[string]string `json:"expanded_devices"`
}

func rootStoragePool(devices map[string]map[string]string) string {
	for _, device := range devices {
		if device["type"] == "disk" && device["path"] == "/" {
			return device["pool"]
		}
	}
	return ""
}

func lxcStorageCommand(timeout time.Duration, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "lxc", append([]string{"--force-local"}, args...)...)
	cmd.WaitDelay = 2 * time.Second
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return nil, fmt.Errorf("LXD 请求超时: %w", ctx.Err())
	}
	if err != nil {
		return nil, fmt.Errorf("LXD 操作失败: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return output, nil
}

func readStorageJSON(target any, args ...string) error {
	output, err := lxcStorageCommand(10*time.Second, args...)
	if err != nil {
		return err
	}
	return json.Unmarshal(output, target)
}

func ListLxcStoragePools() ([]models.LxcStoragePool, error) {
	var entries []storagePoolInfoJSON
	if err := readStorageJSON(&entries, "storage", "list", "--format=json"); err != nil {
		return nil, err
	}
	pools := make([]models.LxcStoragePool, 0, len(entries))
	for _, entry := range entries {
		if entry.Status != "Created" {
			continue
		}
		pool := models.LxcStoragePool{Name: entry.Name, Driver: entry.Driver, Source: entry.Config["source"]}
		var resources struct{ Space struct{ Total, Used uint64 } }
		if readStorageJSON(&resources, "query", "/1.0/storage-pools/"+url.PathEscape(entry.Name)+"/resources") == nil && resources.Space.Total > 0 && resources.Space.Used <= resources.Space.Total {
			available := resources.Space.Total - resources.Space.Used
			pool.AvailableBytes = &available
			pool.TotalBytes = &resources.Space.Total
		}
		pools = append(pools, pool)
	}
	sort.Slice(pools, func(left, right int) bool { return pools[left].Name < pools[right].Name })
	return pools, nil
}

type storageFilesystem struct {
	Target string
	FSType string
}

func LxcStorageMountpoints() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "findmnt", "--json", "--list", "--real", "--output", "TARGET,FSTYPE").Output()
	if err != nil {
		return nil, fmt.Errorf("读取宿主机挂载点失败: %w", err)
	}
	var mounts struct{ Filesystems []storageFilesystem }
	if err := json.Unmarshal(output, &mounts); err != nil {
		return nil, err
	}
	paths := make([]string, 0)
	for _, mount := range mounts.Filesystems {
		path := mount.Target
		if !strings.HasPrefix(path, "/mnt/") && !strings.HasPrefix(path, "/media/") && !strings.HasPrefix(path, "/srv/") {
			continue
		}
		if mount.FSType == "ext4" || mount.FSType == "xfs" || mount.FSType == "btrfs" || mount.FSType == "zfs" {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	return paths, nil
}

func CreateLxcStoragePool(name, mount string) error {
	if !storageNamePattern.MatchString(name) {
		return fmt.Errorf("存储池名称须为 1–63 个字母、数字、点、横线或下划线，并以字母或数字开头")
	}
	mounts, err := LxcStorageMountpoints()
	if err != nil {
		return err
	}
	found := false
	for _, candidate := range mounts {
		if mount == candidate {
			found = true
			break
		}
	}
	resolved, err := filepath.EvalSymlinks(mount)
	if !found || err != nil || resolved != mount {
		return fmt.Errorf("请选择已挂载的本地磁盘")
	}
	base := filepath.Join(mount, "labpanel-pools")
	if err := os.Mkdir(base, 0700); err != nil && !os.IsExist(err) {
		return err
	}
	info, err := os.Lstat(base)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0022 != 0 {
		return fmt.Errorf("存储池父目录必须是权限受限的普通目录")
	}
	source := filepath.Join(base, name)
	if err := os.Mkdir(source, 0700); err != nil {
		return fmt.Errorf("创建存储池目录失败: %w", err)
	}
	if _, err := lxcStorageCommand(30*time.Second, "storage", "create", name, "dir", "source="+source); err != nil {
		_ = os.Remove(source)
		return err
	}
	return nil
}

func readStorageInstance(name string) (storageInstance, error) {
	var instance storageInstance
	if !storageNamePattern.MatchString(name) {
		return instance, fmt.Errorf("无效的容器名称")
	}
	err := readStorageJSON(&instance, "query", "/1.0/instances/"+url.PathEscape(name))
	return instance, err
}

func LxcStorageMounts(name string) ([]models.LxcStorageMount, error) {
	instance, err := readStorageInstance(name)
	if err != nil {
		return nil, err
	}
	mounts := make([]models.LxcStorageMount, 0)
	for name, device := range instance.ExpandedDevices {
		if device["type"] == "disk" && device["path"] != "/" {
			mounts = append(mounts, models.LxcStorageMount{Name: name, Source: device["source"], Path: device["path"], Pool: device["pool"]})
		}
	}
	sort.Slice(mounts, func(left, right int) bool { return mounts[left].Name < mounts[right].Name })
	return mounts, nil
}

func validateStoragePool(name string) error {
	if !storageNamePattern.MatchString(name) {
		return fmt.Errorf("无效的目标存储池名称")
	}
	var pool storagePoolInfoJSON
	if err := readStorageJSON(&pool, "query", "/1.0/storage-pools/"+url.PathEscape(name)); err != nil {
		return err
	}
	if pool.Status != "Created" {
		return fmt.Errorf("目标存储池尚未就绪")
	}
	source := pool.Config["source"]
	if pool.Driver == "dir" && filepath.Base(filepath.Dir(source)) == "labpanel-pools" {
		mount := filepath.Dir(filepath.Dir(source))
		mounts, err := LxcStorageMountpoints()
		if err != nil {
			return err
		}
		for _, candidate := range mounts {
			if candidate == mount {
				return nil
			}
		}
		return fmt.Errorf("存储池所在磁盘 %s 未挂载", mount)
	}
	return nil
}

func ListLxcStorageMoves() []models.LxcStorageMove {
	storageWork.Lock()
	defer storageWork.Unlock()
	moves := make([]models.LxcStorageMove, 0, len(storageWork.moves))
	for _, move := range storageWork.moves {
		moves = append(moves, move)
	}
	sort.Slice(moves, func(left, right int) bool { return moves[left].UpdatedAt.After(moves[right].UpdatedAt) })
	return moves
}

func StartLxcStorageMove(name, target string) (models.LxcStorageMove, error) {
	if !storageNamePattern.MatchString(name) {
		return models.LxcStorageMove{}, fmt.Errorf("无效的容器名称")
	}
	release, err := lockLxcOperation(name)
	if err != nil {
		return models.LxcStorageMove{}, err
	}
	instance, err := readStorageInstance(name)
	source := rootStoragePool(instance.ExpandedDevices)
	if err == nil && (instance.Type != "container" || source == "" || source == target || (instance.Status != "Stopped" && instance.Status != "Running")) {
		err = fmt.Errorf("请选择一个可迁移的容器和不同的目标存储池")
	}
	if err == nil {
		err = validateStoragePool(target)
	}
	if err != nil {
		release()
		return models.LxcStorageMove{}, err
	}
	move := models.LxcStorageMove{Name: name, SourcePool: source, TargetPool: target, Status: "running"}
	move = publishStorageMove(move, "checking", "正在核对容量和快照")
	go func() {
		defer release()
		if err := moveLxcStorage(move, instance); err != nil {
			move.Status = "failed"
			publishStorageMove(move, "failed", err.Error())
			return
		}
		move.Status = "completed"
		publishStorageMove(move, "completed", "容器已迁移")
	}()
	return move, nil
}

func publishStorageMove(move models.LxcStorageMove, stage, message string) models.LxcStorageMove {
	storageWork.Lock()
	defer storageWork.Unlock()
	move.Stage, move.Message, move.UpdatedAt = stage, message, time.Now()
	storageWork.moves[move.Name] = move
	return move
}

func storageSnapshotNames(name string) ([]string, error) {
	var snapshots []string
	err := readStorageJSON(&snapshots, "query", "/1.0/instances/"+url.PathEscape(name)+"/snapshots")
	sort.Strings(snapshots)
	return snapshots, err
}

func checkStorageCapacity(move models.LxcStorageMove) error {
	var resources struct{ Space struct{ Total, Used uint64 } }
	resourcePath := "/1.0/storage-pools/" + url.PathEscape(move.TargetPool) + "/resources"
	if err := readStorageJSON(&resources, "query", resourcePath); err != nil {
		return fmt.Errorf("无法核对目标存储池容量: %w", err)
	}
	var state lxdStorageVolumeState
	statePath := "/1.0/storage-pools/" + url.PathEscape(move.SourcePool) + "/volumes/container/" + url.PathEscape(move.Name) + "/state"
	_ = readStorageJSON(&state, "query", statePath)
	usage := firstNonZeroInt64(state.Usage, state.Disk.Usage)
	if usage > 0 && resources.Space.Total > 0 && (resources.Space.Used >= resources.Space.Total || uint64(usage) > resources.Space.Total-resources.Space.Used) {
		return fmt.Errorf("目标存储池可用空间不足")
	}
	return nil
}

func moveLxcStorage(move models.LxcStorageMove, before storageInstance) error {
	if err := checkStorageCapacity(move); err != nil {
		return err
	}
	snapshots, err := storageSnapshotNames(move.Name)
	if err != nil {
		return err
	}
	wasRunning := before.Status == "Running"
	if wasRunning {
		publishStorageMove(move, "stopping", "正在正常关闭容器")
		if _, err := lxcStorageCommand(time.Minute, "stop", move.Name, "--timeout=45"); err != nil {
			return err
		}
	}
	publishStorageMove(move, "moving", "正在迁移容器和快照")
	_, err = lxcStorageCommand(24*time.Hour, "move", move.Name, "--storage="+move.TargetPool)
	if err != nil {
		current, inspectErr := readStorageInstance(move.Name)
		if wasRunning && inspectErr == nil && rootStoragePool(current.ExpandedDevices) == move.SourcePool && current.Status == "Stopped" && !strings.Contains(err.Error(), "超时") {
			if _, restartErr := lxcStorageCommand(time.Minute, "start", move.Name); restartErr != nil {
				return fmt.Errorf("迁移失败: %v；原容器启动失败: %w", err, restartErr)
			}
		}
		return err
	}
	publishStorageMove(move, "verifying", "正在核对存储池、快照和设备")
	after, err := readStorageInstance(move.Name)
	if err != nil || rootStoragePool(after.ExpandedDevices) != move.TargetPool {
		return fmt.Errorf("无法确认容器已切换到 %s", move.TargetPool)
	}
	actualSnapshots, err := storageSnapshotNames(move.Name)
	if err != nil || !reflect.DeepEqual(snapshots, actualSnapshots) {
		return fmt.Errorf("无法确认快照完整，容器保持停止")
	}
	for name, device := range before.ExpandedDevices {
		if device["type"] == "disk" && device["path"] == "/" {
			continue
		}
		if !reflect.DeepEqual(device, after.ExpandedDevices[name]) {
			return fmt.Errorf("设备 %s 配置发生变化，容器保持停止", name)
		}
	}
	for key, value := range before.Config {
		if strings.HasSuffix(key, ".hwaddr") && after.Config[key] != value {
			return fmt.Errorf("容器网卡 MAC 发生变化，容器保持停止")
		}
	}
	if wasRunning {
		publishStorageMove(move, "starting", "正在恢复容器运行")
		if _, err := lxcStorageCommand(time.Minute, "start", move.Name); err != nil {
			return fmt.Errorf("存储已迁移，启动容器失败: %w", err)
		}
	}
	return nil
}

var lxcOperationLocks sync.Map

func lockLxcOperation(name string) (func(), error) {
	if _, busy := lxcOperationLocks.LoadOrStore(name, struct{}{}); busy {
		return nil, fmt.Errorf("容器 %s 正在执行独占操作", name)
	}
	return func() { lxcOperationLocks.Delete(name) }, nil
}
