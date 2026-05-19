package service

import (
	"LabPanel/models"
	"fmt"
	"math"
	"os"
	"os/exec"
	osuser "os/user"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	gpuSampleInterval = time.Minute
	gpuRetention      = 30 * 24 * time.Hour
	gpuMaxPoints      = 240
)

type GPUMonitorService struct {
	mu        sync.RWMutex
	history   map[string][]gpuMemorySample
	lastSweep time.Time
	started   bool
	collectMu sync.Mutex
}

type gpuMemorySample struct {
	Timestamp time.Time
	Owners    map[string]gpuOwnerMemory
}

type gpuOwnerMemory struct {
	OwnerType     string
	ContainerName string
	Label         string
	UsedMemoryMiB int64
}

var defaultGPUMonitorService = NewGPUMonitorService()

func NewGPUMonitorService() *GPUMonitorService {
	return &GPUMonitorService{
		history: make(map[string][]gpuMemorySample),
	}
}

func GetGPUMonitorService() *GPUMonitorService {
	return defaultGPUMonitorService
}

func (s *GPUMonitorService) Start() {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return
	}
	s.started = true
	s.mu.Unlock()

	s.restoreFromStore()

	go func() {
		s.Collect()
		ticker := time.NewTicker(gpuSampleInterval)
		defer ticker.Stop()
		for range ticker.C {
			s.Collect()
		}
	}()
}

func (s *GPUMonitorService) Response(rangeKey string) models.GPUMonitorResponse {
	s.collectIfStale()

	if rangeKey == "" {
		rangeKey = "1h"
	}
	window := gpuRangeDuration(rangeKey)
	since := time.Now().Add(-window)
	response := collectGPUStatus(time.Now())
	response.Range = rangeKey
	response.IntervalSeconds = int(gpuSampleInterval.Seconds())
	if !response.Available {
		return response
	}

	s.mu.RLock()
	for i := range response.GPUs {
		response.GPUs[i].MemorySeries = s.memorySeries(response.GPUs[i].UUID, since)
	}
	s.mu.RUnlock()

	return response
}

func (s *GPUMonitorService) Collect() {
	if !s.collectMu.TryLock() {
		return
	}
	defer s.collectMu.Unlock()

	now := time.Now()
	status := collectGPUStatus(now)

	s.mu.Lock()
	s.lastSweep = now
	if status.Available {
		seen := make(map[string]struct{}, len(status.GPUs))
		for _, gpu := range status.GPUs {
			seen[gpu.UUID] = struct{}{}
			sample := gpuMemorySample{
				Timestamp: now,
				Owners:    ownerMemoryByProcess(gpu.Processes),
			}
			history := s.history[gpu.UUID]
			if len(history) > 0 && sameGPUMemorySample(history[len(history)-1], sample) {
				continue
			}

			history = append(history, sample)
			cutoff := now.Add(-gpuRetention)
			start := 0
			for start < len(history) && history[start].Timestamp.Before(cutoff) {
				start++
			}
			s.history[gpu.UUID] = history[start:]
			if store := GetMetricsStore(); store != nil {
				_ = store.InsertGPUMemorySample(gpu.UUID, sample)
			}
		}
		for uuid := range s.history {
			if _, ok := seen[uuid]; !ok {
				delete(s.history, uuid)
			}
		}
	}
	s.mu.Unlock()
}

func (s *GPUMonitorService) restoreFromStore() {
	store := GetMetricsStore()
	if store == nil {
		return
	}

	history, err := store.LoadGPUMemoryHistorySince(time.Now().Add(-metricsHistoryLoadWindow))
	if err != nil {
		return
	}

	s.mu.Lock()
	s.history = history
	for _, samples := range history {
		if len(samples) == 0 {
			continue
		}
		latest := samples[len(samples)-1].Timestamp
		if latest.After(s.lastSweep) {
			s.lastSweep = latest
		}
	}
	s.mu.Unlock()
}

func sameGPUMemorySample(a, b gpuMemorySample) bool {
	if len(a.Owners) != len(b.Owners) {
		return false
	}
	for key, aOwner := range a.Owners {
		bOwner, ok := b.Owners[key]
		if !ok {
			return false
		}
		if aOwner.OwnerType != bOwner.OwnerType ||
			aOwner.ContainerName != bOwner.ContainerName ||
			aOwner.Label != bOwner.Label ||
			aOwner.UsedMemoryMiB != bOwner.UsedMemoryMiB {
			return false
		}
	}
	return true
}

func (s *GPUMonitorService) collectIfStale() {
	s.mu.RLock()
	lastSweep := s.lastSweep
	s.mu.RUnlock()

	if time.Since(lastSweep) >= gpuSampleInterval/2 {
		s.Collect()
	}
}

func (s *GPUMonitorService) memorySeries(gpuUUID string, since time.Time) []models.GPUOwnerMemorySeries {
	history := s.history[gpuUUID]
	if len(history) == 0 {
		return []models.GPUOwnerMemorySeries{}
	}

	filtered := make([]gpuMemorySample, 0, len(history))
	owners := map[string]gpuOwnerMemory{}
	var carry *gpuMemorySample
	for _, sample := range history {
		if sample.Timestamp.Before(since) {
			sampleCopy := sample
			carry = &sampleCopy
			continue
		}
		filtered = append(filtered, sample)
		for key, owner := range sample.Owners {
			owners[key] = owner
		}
	}
	if carry != nil {
		carryCopy := *carry
		carryCopy.Timestamp = since
		filtered = append([]gpuMemorySample{carryCopy}, filtered...)
		for key, owner := range carryCopy.Owners {
			owners[key] = owner
		}
	}
	if len(filtered) > 0 {
		last := filtered[len(filtered)-1]
		now := time.Now()
		if last.Timestamp.Before(now) {
			last.Timestamp = now
			filtered = append(filtered, last)
		}
	}
	if len(filtered) == 0 {
		return []models.GPUOwnerMemorySeries{}
	}

	totalPoints := make([]models.GPUOwnerMemoryPoint, 0, len(filtered))
	for _, sample := range filtered {
		var total float64
		for _, memory := range sample.Owners {
			total += float64(memory.UsedMemoryMiB)
		}
		totalPoints = append(totalPoints, models.GPUOwnerMemoryPoint{
			Timestamp:     sample.Timestamp,
			UsedMemoryMiB: total,
		})
	}

	keys := make([]string, 0, len(owners))
	for key := range owners {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return owners[keys[i]].Label < owners[keys[j]].Label
	})

	series := []models.GPUOwnerMemorySeries{
		{
			OwnerType: "total",
			Label:     "总占用",
			Points:    downsampleGPUPoints(totalPoints, gpuMaxPoints),
		},
	}
	for _, key := range keys {
		owner := owners[key]
		points := make([]models.GPUOwnerMemoryPoint, 0, len(filtered))
		for _, sample := range filtered {
			value := float64(0)
			if memory, ok := sample.Owners[key]; ok {
				value = float64(memory.UsedMemoryMiB)
			}
			points = append(points, models.GPUOwnerMemoryPoint{
				Timestamp:     sample.Timestamp,
				UsedMemoryMiB: value,
			})
		}
		series = append(series, models.GPUOwnerMemorySeries{
			OwnerType:     owner.OwnerType,
			ContainerName: owner.ContainerName,
			Label:         owner.Label,
			Points:        downsampleGPUPoints(points, gpuMaxPoints),
		})
	}

	return series
}

func collectGPUStatus(now time.Time) models.GPUMonitorResponse {
	if _, err := exec.LookPath("nvidia-smi"); err != nil {
		return models.GPUMonitorResponse{
			Available: false,
			Message:   "未找到 nvidia-smi，当前宿主机未安装 NVIDIA 驱动或没有 NVIDIA 显卡",
			UpdatedAt: now,
			GPUs:      []models.GPUDevice{},
		}
	}

	gpus, err := queryGPUDevices()
	if err != nil {
		return models.GPUMonitorResponse{
			Available: false,
			Message:   fmt.Sprintf("nvidia-smi 无显卡或不可用: %v", err),
			UpdatedAt: now,
			GPUs:      []models.GPUDevice{},
		}
	}
	if len(gpus) == 0 {
		return models.GPUMonitorResponse{
			Available: false,
			Message:   "nvidia-smi 未检测到可用显卡",
			UpdatedAt: now,
			GPUs:      []models.GPUDevice{},
		}
	}

	processes := queryGPUProcesses()
	byUUID := make(map[string][]models.GPUProcess, len(gpus))
	for _, process := range processes {
		byUUID[process.GPUUUID] = append(byUUID[process.GPUUUID], process)
	}
	for i := range gpus {
		gpus[i].Processes = byUUID[gpus[i].UUID]
		if gpus[i].Processes == nil {
			gpus[i].Processes = []models.GPUProcess{}
		}
	}

	return models.GPUMonitorResponse{
		Available: true,
		Message:   "nvidia-smi 已就绪",
		UpdatedAt: now,
		GPUs:      gpus,
	}
}

func queryGPUDevices() ([]models.GPUDevice, error) {
	output, err := exec.Command(
		"nvidia-smi",
		"--query-gpu=index,uuid,name,memory.total,memory.used,utilization.gpu,temperature.gpu",
		"--format=csv,noheader,nounits",
	).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%v, 输出: %s", err, strings.TrimSpace(string(output)))
	}

	lines := nonEmptyLines(string(output))
	gpus := make([]models.GPUDevice, 0, len(lines))
	for _, line := range lines {
		fields := csvLineFields(line)
		if len(fields) < 7 {
			continue
		}
		gpus = append(gpus, models.GPUDevice{
			Index:          atoiDefault(fields[0], -1),
			UUID:           fields[1],
			Name:           fields[2],
			MemoryTotalMiB: int64(atoiDefault(fields[3], 0)),
			MemoryUsedMiB:  int64(atoiDefault(fields[4], 0)),
			Utilization:    atoiDefault(fields[5], 0),
			Temperature:    atoiDefault(fields[6], 0),
			Processes:      []models.GPUProcess{},
			MemorySeries:   []models.GPUOwnerMemorySeries{},
		})
	}
	return gpus, nil
}

func queryGPUProcesses() []models.GPUProcess {
	output, err := exec.Command(
		"nvidia-smi",
		"--query-compute-apps=gpu_uuid,pid,process_name,used_memory",
		"--format=csv,noheader,nounits",
	).CombinedOutput()
	if err != nil {
		return []models.GPUProcess{}
	}

	lines := nonEmptyLines(string(output))
	processes := make([]models.GPUProcess, 0, len(lines))
	groupsByContainer, _ := NewLxcGroupService().GroupsByContainer()
	for _, line := range lines {
		fields := csvLineFields(line)
		if len(fields) < 4 {
			continue
		}
		pid := atoiDefault(fields[1], 0)
		if pid <= 0 {
			continue
		}
		ownerType, containerName := processOwner(pid)
		groups := groupsByContainer[containerName]
		if groups == nil {
			groups = []models.LxcGroup{}
		}
		processes = append(processes, models.GPUProcess{
			GPUUUID:       fields[0],
			PID:           pid,
			User:          processUser(pid),
			ProcessName:   fields[2],
			UsedMemoryMiB: int64(atoiDefault(fields[3], 0)),
			OwnerType:     ownerType,
			ContainerName: containerName,
			Groups:        groups,
		})
	}
	return processes
}

func ownerMemoryByProcess(processes []models.GPUProcess) map[string]gpuOwnerMemory {
	owners := map[string]gpuOwnerMemory{}
	for _, process := range processes {
		key := gpuOwnerKey(process.OwnerType, process.ContainerName)
		owner := owners[key]
		if owner.Label == "" {
			owner = gpuOwnerMemory{
				OwnerType:     process.OwnerType,
				ContainerName: process.ContainerName,
				Label:         gpuOwnerLabel(process.OwnerType, process.ContainerName),
			}
		}
		owner.UsedMemoryMiB += process.UsedMemoryMiB
		owners[key] = owner
	}
	return owners
}

func gpuOwnerKey(ownerType, containerName string) string {
	if ownerType == "container" && strings.TrimSpace(containerName) != "" {
		return "container:" + containerName
	}
	return "host"
}

func gpuOwnerLabel(ownerType, containerName string) string {
	if ownerType == "container" && strings.TrimSpace(containerName) != "" {
		return containerName
	}
	return "宿主机"
}

func downsampleGPUPoints(points []models.GPUOwnerMemoryPoint, maxPoints int) []models.GPUOwnerMemoryPoint {
	if len(points) <= maxPoints {
		return points
	}
	bucketSize := int(math.Ceil(float64(len(points)) / float64(maxPoints)))
	result := make([]models.GPUOwnerMemoryPoint, 0, maxPoints)
	for i := 0; i < len(points); i += bucketSize {
		end := i + bucketSize
		if end > len(points) {
			end = len(points)
		}
		var value float64
		for _, point := range points[i:end] {
			value += point.UsedMemoryMiB
		}
		result = append(result, models.GPUOwnerMemoryPoint{
			Timestamp:     points[end-1].Timestamp,
			UsedMemoryMiB: value / float64(end-i),
		})
	}
	return result
}

func gpuRangeDuration(rangeKey string) time.Duration {
	switch rangeKey {
	case "24h":
		return 24 * time.Hour
	case "7d":
		return 7 * 24 * time.Hour
	case "30d":
		return 30 * 24 * time.Hour
	default:
		return time.Hour
	}
}

func processOwner(pid int) (string, string) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cgroup", pid))
	if err != nil {
		return "host", ""
	}
	if name, ok := containerNameFromCgroup(string(data)); ok {
		return "container", name
	}
	return "host", ""
}

func processUser(pid int) string {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "Uid:" {
			continue
		}
		uid := fields[1]
		if u, err := osuser.LookupId(uid); err == nil {
			return u.Username
		}
		return uid
	}
	return ""
}

var lxcPayloadPattern = regexp.MustCompile(`lxc\.payload\.([^/\s:]+)`)

func containerNameFromCgroup(content string) (string, bool) {
	for _, line := range strings.Split(content, "\n") {
		matches := lxcPayloadPattern.FindStringSubmatch(line)
		if len(matches) != 2 {
			continue
		}
		name := strings.TrimSuffix(matches[1], ".scope")
		name = strings.ReplaceAll(name, `\x2d`, "-")
		name = strings.ReplaceAll(name, `\x5f`, "_")
		name = strings.TrimSpace(name)
		if name != "" {
			return name, true
		}
	}
	return "", false
}

func nonEmptyLines(value string) []string {
	lines := []string{}
	for _, line := range strings.Split(value, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func csvLineFields(line string) []string {
	parts := strings.Split(line, ",")
	fields := make([]string, 0, len(parts))
	for _, part := range parts {
		fields = append(fields, strings.TrimSpace(part))
	}
	return fields
}

func atoiDefault(value string, defaultValue int) int {
	value = strings.TrimSpace(value)
	if value == "" || strings.EqualFold(value, "[not supported]") || strings.EqualFold(value, "N/A") {
		return defaultValue
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return parsed
}
