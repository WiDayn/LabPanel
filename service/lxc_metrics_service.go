package service

import (
	"LabPanel/models"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	lxcMetricsSampleInterval = time.Minute
	lxcMetricsRetention      = 30 * 24 * time.Hour
	lxcMetricsMaxPoints      = 240
)

type LxcMetricsService struct {
	mu        sync.RWMutex
	history   map[string][]lxcMetricsSample
	lastRaw   map[string]lxcMetricsRaw
	lastSweep time.Time
	started   bool
	collectMu sync.Mutex
}

type lxcMetricsRaw struct {
	Name           string
	State          string
	Timestamp      time.Time
	CPUUsageNs     int64
	MemoryBytes    int64
	Processes      int64
	NetworkRxBytes int64
	NetworkTxBytes int64
	DiskUsageBytes int64
	DiskReadBytes  int64
	DiskWriteBytes int64
}

type lxcMetricsSample struct {
	models.LxcMetricsPoint
	Name  string
	State string
}

type lxdState struct {
	Status string `json:"status"`
	CPU    struct {
		Usage int64 `json:"usage"`
	} `json:"cpu"`
	Memory struct {
		Usage int64 `json:"usage"`
	} `json:"memory"`
	Processes int64 `json:"processes"`
	Network   map[string]struct {
		Counters struct {
			BytesReceived int64 `json:"bytes_received"`
			BytesSent     int64 `json:"bytes_sent"`
		} `json:"counters"`
	} `json:"network"`
	Disk map[string]struct {
		Usage         int64 `json:"usage"`
		BytesRead     int64 `json:"bytes_read"`
		BytesWritten  int64 `json:"bytes_written"`
		ReadBytes     int64 `json:"read_bytes"`
		WrittenBytes  int64 `json:"written_bytes"`
		IoReadBytes   int64 `json:"io_read_bytes"`
		IoWriteBytes  int64 `json:"io_write_bytes"`
		DiskReadBytes int64 `json:"disk_read_bytes"`
		DiskWritBytes int64 `json:"disk_write_bytes"`
	} `json:"disk"`
}

type lxdInstance struct {
	Devices         map[string]map[string]string `json:"devices"`
	ExpandedDevices map[string]map[string]string `json:"expanded_devices"`
}

type lxdStorageVolumeState struct {
	Usage int64 `json:"usage"`
	Disk  struct {
		Usage int64 `json:"usage"`
	} `json:"disk"`
}

type lxdStoragePool struct {
	Driver string            `json:"driver"`
	Config map[string]string `json:"config"`
}

var defaultLxcMetricsService = NewLxcMetricsService()

func NewLxcMetricsService() *LxcMetricsService {
	return &LxcMetricsService{
		history: make(map[string][]lxcMetricsSample),
		lastRaw: make(map[string]lxcMetricsRaw),
	}
}

func GetLxcMetricsService() *LxcMetricsService {
	return defaultLxcMetricsService
}

func (s *LxcMetricsService) Start() {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return
	}
	s.started = true
	s.mu.Unlock()

	go func() {
		s.Collect()
		ticker := time.NewTicker(lxcMetricsSampleInterval)
		defer ticker.Stop()
		for range ticker.C {
			s.Collect()
		}
	}()
}

func (s *LxcMetricsService) Collect() {
	if !s.collectMu.TryLock() {
		return
	}
	defer s.collectMu.Unlock()

	lxcService := NewLxcService()
	containers, err := lxcService.ListContainers()
	if err != nil {
		return
	}

	now := time.Now()
	seen := make(map[string]struct{}, len(containers))
	for _, container := range containers {
		name := strings.TrimSpace(container.Name)
		if name == "" {
			continue
		}
		seen[name] = struct{}{}

		raw, err := collectLxcMetricsRaw(name, now)
		if err != nil {
			raw = lxcMetricsRaw{
				Name:      name,
				State:     container.State,
				Timestamp: now,
			}
		}
		if raw.State == "" {
			raw.State = container.State
		}

		s.storeRaw(raw)
	}

	s.pruneMissing(seen)

	s.mu.Lock()
	s.lastSweep = now
	s.mu.Unlock()
}

func (s *LxcMetricsService) Response(name, rangeKey string) models.LxcMetricsResponse {
	s.collectIfStale()

	window := metricsRangeDuration(rangeKey)
	if rangeKey == "" {
		rangeKey = "1h"
	}
	since := time.Now().Add(-window)

	s.mu.RLock()
	summaries := make([]models.LxcMetricsContainerSummary, 0, len(s.history))
	for containerName, samples := range s.history {
		if len(samples) == 0 {
			continue
		}
		latest := samples[len(samples)-1]
		summaries = append(summaries, models.LxcMetricsContainerSummary{
			Name:           containerName,
			State:          latest.State,
			UpdatedAt:      latest.Timestamp,
			CPUPercent:     latest.CPUPercent,
			MemoryBytes:    latest.MemoryBytes,
			Processes:      latest.Processes,
			NetworkRxBps:   latest.NetworkRxBps,
			NetworkTxBps:   latest.NetworkTxBps,
			DiskReadBps:    latest.DiskReadBps,
			DiskWriteBps:   latest.DiskWriteBps,
			DiskUsageBytes: latest.DiskUsageBytes,
			NetworkRxBytes: latest.NetworkRxBytes,
			NetworkTxBytes: latest.NetworkTxBytes,
			DiskReadBytes:  latest.DiskReadBytes,
			DiskWriteBytes: latest.DiskWriteBytes,
		})
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].Name < summaries[j].Name
	})

	if name == "" && len(summaries) > 0 {
		name = summaries[0].Name
	}

	var selected *models.LxcMetricsSeries
	if name != "" {
		rawPoints := filterMetricsSince(s.history[name], since)
		points := downsampleMetrics(rawPoints, lxcMetricsMaxPoints)
		selected = &models.LxcMetricsSeries{Name: name, Points: points}
	}
	s.mu.RUnlock()

	return models.LxcMetricsResponse{
		Range:           rangeKey,
		IntervalSeconds: int(lxcMetricsSampleInterval.Seconds()),
		Containers:      summaries,
		Selected:        selected,
	}
}

func (s *LxcMetricsService) collectIfStale() {
	s.mu.RLock()
	lastSweep := s.lastSweep
	s.mu.RUnlock()

	if time.Since(lastSweep) >= lxcMetricsSampleInterval/2 {
		s.Collect()
	}
}

func (s *LxcMetricsService) storeRaw(raw lxcMetricsRaw) {
	s.mu.Lock()
	defer s.mu.Unlock()

	previous, hasPrevious := s.lastRaw[raw.Name]
	s.lastRaw[raw.Name] = raw

	sample := lxcMetricsSample{
		Name:  raw.Name,
		State: raw.State,
		LxcMetricsPoint: models.LxcMetricsPoint{
			Timestamp:      raw.Timestamp,
			MemoryBytes:    raw.MemoryBytes,
			Processes:      raw.Processes,
			DiskUsageBytes: raw.DiskUsageBytes,
			NetworkRxBytes: raw.NetworkRxBytes,
			NetworkTxBytes: raw.NetworkTxBytes,
			DiskReadBytes:  raw.DiskReadBytes,
			DiskWriteBytes: raw.DiskWriteBytes,
		},
	}

	if hasPrevious && raw.Timestamp.After(previous.Timestamp) {
		elapsed := raw.Timestamp.Sub(previous.Timestamp).Seconds()
		if elapsed > 0 {
			cpuDelta := raw.CPUUsageNs - previous.CPUUsageNs
			if cpuDelta >= 0 {
				sample.CPUPercent = clampFloat(float64(cpuDelta)/1e9/elapsed/float64(runtime.NumCPU())*100, 0, 100)
			}
			sample.NetworkRxBps = positiveRate(raw.NetworkRxBytes, previous.NetworkRxBytes, elapsed)
			sample.NetworkTxBps = positiveRate(raw.NetworkTxBytes, previous.NetworkTxBytes, elapsed)
			sample.DiskReadBps = positiveRate(raw.DiskReadBytes, previous.DiskReadBytes, elapsed)
			sample.DiskWriteBps = positiveRate(raw.DiskWriteBytes, previous.DiskWriteBytes, elapsed)
		}
	}

	history := s.history[raw.Name]
	if len(history) > 0 && sameLxcMetricsSample(history[len(history)-1], sample) {
		return
	}

	history = append(history, sample)
	cutoff := time.Now().Add(-lxcMetricsRetention)
	start := 0
	for start < len(history) && history[start].Timestamp.Before(cutoff) {
		start++
	}
	s.history[raw.Name] = history[start:]
}

func sameLxcMetricsSample(a, b lxcMetricsSample) bool {
	return a.State == b.State &&
		a.CPUPercent == b.CPUPercent &&
		a.MemoryBytes == b.MemoryBytes &&
		a.Processes == b.Processes &&
		a.NetworkRxBps == b.NetworkRxBps &&
		a.NetworkTxBps == b.NetworkTxBps &&
		a.DiskReadBps == b.DiskReadBps &&
		a.DiskWriteBps == b.DiskWriteBps &&
		a.DiskUsageBytes == b.DiskUsageBytes &&
		a.NetworkRxBytes == b.NetworkRxBytes &&
		a.NetworkTxBytes == b.NetworkTxBytes &&
		a.DiskReadBytes == b.DiskReadBytes &&
		a.DiskWriteBytes == b.DiskWriteBytes
}

func (s *LxcMetricsService) pruneMissing(seen map[string]struct{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for name := range s.history {
		if _, ok := seen[name]; !ok {
			delete(s.history, name)
			delete(s.lastRaw, name)
		}
	}
}

func collectLxcMetricsRaw(name string, timestamp time.Time) (lxcMetricsRaw, error) {
	output, err := exec.Command("lxc", "query", "/1.0/instances/"+name+"/state").CombinedOutput()
	if err != nil {
		return lxcMetricsRaw{}, fmt.Errorf("读取容器状态失败: %v, 输出: %s", err, string(output))
	}

	var state lxdState
	if err := json.Unmarshal(output, &state); err != nil {
		return lxcMetricsRaw{}, fmt.Errorf("解析容器状态失败: %v, 原始输出: %s", err, string(output))
	}

	raw := lxcMetricsRaw{
		Name:        name,
		State:       state.Status,
		Timestamp:   timestamp,
		CPUUsageNs:  state.CPU.Usage,
		MemoryBytes: state.Memory.Usage,
		Processes:   state.Processes,
	}

	for ifaceName, network := range state.Network {
		if ifaceName == "lo" {
			continue
		}
		raw.NetworkRxBytes += network.Counters.BytesReceived
		raw.NetworkTxBytes += network.Counters.BytesSent
	}

	for _, disk := range state.Disk {
		raw.DiskUsageBytes += disk.Usage
		raw.DiskReadBytes += firstNonZeroInt64(disk.BytesRead, disk.ReadBytes, disk.IoReadBytes, disk.DiskReadBytes)
		raw.DiskWriteBytes += firstNonZeroInt64(disk.BytesWritten, disk.WrittenBytes, disk.IoWriteBytes, disk.DiskWritBytes)
	}
	if raw.DiskUsageBytes == 0 {
		raw.DiskUsageBytes = fallbackDiskUsageBytes(name)
	}

	return raw, nil
}

func fallbackDiskUsageBytes(name string) int64 {
	pool, ok := instanceRootDiskPool(name)
	if !ok {
		return 0
	}

	if usage := storageVolumeStateUsage(pool, name); usage > 0 {
		return usage
	}
	if usage := storageVolumeInfoUsage(pool, name); usage > 0 {
		return usage
	}
	if usage := dirStorageVolumeUsage(pool, name); usage > 0 {
		return usage
	}
	return 0
}

func instanceRootDiskPool(name string) (string, bool) {
	output, err := exec.Command("lxc", "query", "/1.0/instances/"+url.PathEscape(name)).CombinedOutput()
	if err != nil {
		return "", false
	}

	var instance lxdInstance
	if err := json.Unmarshal(output, &instance); err != nil {
		return "", false
	}

	for _, devices := range []map[string]map[string]string{instance.ExpandedDevices, instance.Devices} {
		for _, device := range devices {
			if device["type"] == "disk" && device["path"] == "/" && strings.TrimSpace(device["pool"]) != "" {
				return strings.TrimSpace(device["pool"]), true
			}
		}
	}
	return "", false
}

func storageVolumeStateUsage(pool, name string) int64 {
	apiPath := fmt.Sprintf(
		"/1.0/storage-pools/%s/volumes/container/%s/state",
		url.PathEscape(pool),
		url.PathEscape(name),
	)
	output, err := exec.Command("lxc", "query", apiPath).CombinedOutput()
	if err != nil {
		return 0
	}

	var state lxdStorageVolumeState
	if err := json.Unmarshal(output, &state); err != nil {
		return 0
	}
	return firstNonZeroInt64(state.Usage, state.Disk.Usage)
}

func storageVolumeInfoUsage(pool, name string) int64 {
	output, err := exec.Command("lxc", "storage", "volume", "info", pool, "container/"+name).CombinedOutput()
	if err != nil {
		return 0
	}

	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		lower := strings.ToLower(line)
		if !strings.HasPrefix(lower, "usage:") && !strings.HasPrefix(lower, "used:") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		if usage := parseByteSize(strings.TrimSpace(parts[1])); usage > 0 {
			return usage
		}
	}
	return 0
}

func dirStorageVolumeUsage(pool, name string) int64 {
	storagePool, ok := storagePoolInfo(pool)
	if !ok || storagePool.Driver != "dir" {
		return 0
	}

	candidates := []string{}
	if source := strings.TrimSpace(storagePool.Config["source"]); source != "" {
		candidates = append(candidates, filepath.Join(source, "containers", name))
	}
	candidates = append(candidates, filepath.Join("/var/snap/lxd/common/lxd/storage-pools", pool, "containers", name))
	candidates = append(candidates, filepath.Join("/var/lib/lxd/storage-pools", pool, "containers", name))

	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err != nil || !info.IsDir() {
			continue
		}
		if usage := duBytes(candidate); usage > 0 {
			return usage
		}
	}
	return 0
}

func storagePoolInfo(pool string) (lxdStoragePool, bool) {
	output, err := exec.Command("lxc", "query", "/1.0/storage-pools/"+url.PathEscape(pool)).CombinedOutput()
	if err != nil {
		return lxdStoragePool{}, false
	}

	var storagePool lxdStoragePool
	if err := json.Unmarshal(output, &storagePool); err != nil {
		return lxdStoragePool{}, false
	}
	return storagePool, true
}

func duBytes(path string) int64 {
	output, err := exec.Command("du", "-sb", path).CombinedOutput()
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(output))
	if len(fields) == 0 {
		return 0
	}
	value, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil {
		return 0
	}
	return value
}

func metricsRangeDuration(rangeKey string) time.Duration {
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

func filterMetricsSince(samples []lxcMetricsSample, since time.Time) []lxcMetricsSample {
	points := make([]lxcMetricsSample, 0, len(samples))
	var carry *lxcMetricsSample
	for _, sample := range samples {
		if sample.Timestamp.Before(since) {
			sampleCopy := sample
			carry = &sampleCopy
			continue
		}
		points = append(points, sample)
	}
	if carry != nil {
		carryCopy := *carry
		carryCopy.Timestamp = since
		points = append([]lxcMetricsSample{carryCopy}, points...)
	}
	if len(points) > 0 {
		last := points[len(points)-1]
		now := time.Now()
		if last.Timestamp.Before(now) {
			last.Timestamp = now
			points = append(points, last)
		}
	}
	return points
}

func downsampleMetrics(samples []lxcMetricsSample, maxPoints int) []models.LxcMetricsPoint {
	if len(samples) <= maxPoints {
		points := make([]models.LxcMetricsPoint, 0, len(samples))
		for _, sample := range samples {
			points = append(points, sample.LxcMetricsPoint)
		}
		return points
	}

	bucketSize := int(math.Ceil(float64(len(samples)) / float64(maxPoints)))
	points := make([]models.LxcMetricsPoint, 0, maxPoints)
	for i := 0; i < len(samples); i += bucketSize {
		end := i + bucketSize
		if end > len(samples) {
			end = len(samples)
		}
		points = append(points, averageMetricsBucket(samples[i:end]))
	}
	return points
}

func averageMetricsBucket(samples []lxcMetricsSample) models.LxcMetricsPoint {
	if len(samples) == 0 {
		return models.LxcMetricsPoint{}
	}

	var point models.LxcMetricsPoint
	point.Timestamp = samples[len(samples)-1].Timestamp
	for _, sample := range samples {
		point.CPUPercent += sample.CPUPercent
		point.MemoryBytes += sample.MemoryBytes
		point.Processes += sample.Processes
		point.NetworkRxBps += sample.NetworkRxBps
		point.NetworkTxBps += sample.NetworkTxBps
		point.DiskReadBps += sample.DiskReadBps
		point.DiskWriteBps += sample.DiskWriteBps
		point.DiskUsageBytes += sample.DiskUsageBytes
		point.NetworkRxBytes = sample.NetworkRxBytes
		point.NetworkTxBytes = sample.NetworkTxBytes
		point.DiskReadBytes = sample.DiskReadBytes
		point.DiskWriteBytes = sample.DiskWriteBytes
	}
	count := float64(len(samples))
	point.CPUPercent /= count
	point.MemoryBytes = int64(float64(point.MemoryBytes) / count)
	point.Processes = int64(float64(point.Processes) / count)
	point.NetworkRxBps /= count
	point.NetworkTxBps /= count
	point.DiskReadBps /= count
	point.DiskWriteBps /= count
	point.DiskUsageBytes = int64(float64(point.DiskUsageBytes) / count)
	return point
}

func positiveRate(current, previous int64, elapsed float64) float64 {
	if current < previous || elapsed <= 0 {
		return 0
	}
	return float64(current-previous) / elapsed
}

func clampFloat(value, minValue, maxValue float64) float64 {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

var byteSizePattern = regexp.MustCompile(`(?i)^([0-9]+(?:\.[0-9]+)?)\s*([KMGTPE]?i?B?|B)?$`)

func parseByteSize(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}

	matches := byteSizePattern.FindStringSubmatch(value)
	if len(matches) != 3 {
		return 0
	}

	number, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0
	}

	unit := strings.ToUpper(matches[2])
	unit = strings.TrimSuffix(unit, "B")
	unit = strings.TrimSuffix(unit, "I")

	multiplier := float64(1)
	switch unit {
	case "K":
		multiplier = 1024
	case "M":
		multiplier = 1024 * 1024
	case "G":
		multiplier = 1024 * 1024 * 1024
	case "T":
		multiplier = 1024 * 1024 * 1024 * 1024
	case "P":
		multiplier = 1024 * 1024 * 1024 * 1024 * 1024
	case "E":
		multiplier = 1024 * 1024 * 1024 * 1024 * 1024 * 1024
	}

	return int64(number * multiplier)
}

func firstNonZeroInt64(values ...int64) int64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}
