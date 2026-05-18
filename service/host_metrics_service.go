package service

import (
	"LabPanel/models"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	hostMetricsSampleInterval = time.Minute
	hostMetricsRetention      = 30 * 24 * time.Hour
	hostMetricsMaxPoints      = 240
	hostProcessLimit          = 40
)

type HostMetricsService struct {
	mu        sync.RWMutex
	history   []models.HostMetricsPoint
	lastRaw   hostMetricsRaw
	hasRaw    bool
	lastSweep time.Time
	started   bool
	collectMu sync.Mutex
}

type hostMetricsRaw struct {
	Timestamp        time.Time
	CPUIdle          uint64
	CPUTotal         uint64
	CPUMHz           float64
	TemperatureC     float64
	MemoryUsedBytes  int64
	MemoryTotalBytes int64
	NetworkRxBytes   int64
	NetworkTxBytes   int64
}

type hostMemoryOverview struct {
	TotalBytes int64
	Modules    []models.HostMemoryModule
}

type hostCPUPackage struct {
	PhysicalID    string
	Model         string
	CoreKeys      map[string]struct{}
	ThreadCount   int
	TotalMHz      float64
	MHzCount      int
	FallbackCores int
}

var defaultHostMetricsService = NewHostMetricsService()

func NewHostMetricsService() *HostMetricsService {
	return &HostMetricsService{}
}

func GetHostMetricsService() *HostMetricsService {
	return defaultHostMetricsService
}

func (s *HostMetricsService) Start() {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return
	}
	s.started = true
	s.mu.Unlock()

	go func() {
		s.Collect()
		time.Sleep(2 * time.Second)
		s.Collect()
		ticker := time.NewTicker(hostMetricsSampleInterval)
		defer ticker.Stop()
		for range ticker.C {
			s.Collect()
		}
	}()
}

func (s *HostMetricsService) Collect() {
	if !s.collectMu.TryLock() {
		return
	}
	defer s.collectMu.Unlock()

	raw := collectHostMetricsRaw(time.Now())

	s.mu.Lock()
	defer s.mu.Unlock()

	point := models.HostMetricsPoint{
		Timestamp:        raw.Timestamp,
		CPUMHz:           raw.CPUMHz,
		TemperatureC:     raw.TemperatureC,
		MemoryUsedBytes:  raw.MemoryUsedBytes,
		MemoryTotalBytes: raw.MemoryTotalBytes,
		NetworkRxBytes:   raw.NetworkRxBytes,
		NetworkTxBytes:   raw.NetworkTxBytes,
		NetworkRxBps:     0,
		NetworkTxBps:     0,
		CPUPercent:       0,
	}

	if s.hasRaw {
		deltaTotal := raw.CPUTotal - s.lastRaw.CPUTotal
		deltaIdle := raw.CPUIdle - s.lastRaw.CPUIdle
		if deltaTotal > 0 && deltaIdle <= deltaTotal {
			point.CPUPercent = clampFloat((1-float64(deltaIdle)/float64(deltaTotal))*100, 0, 100)
		}

		elapsed := raw.Timestamp.Sub(s.lastRaw.Timestamp).Seconds()
		if elapsed > 0 {
			point.NetworkRxBps = positiveRate(raw.NetworkRxBytes, s.lastRaw.NetworkRxBytes, elapsed)
			point.NetworkTxBps = positiveRate(raw.NetworkTxBytes, s.lastRaw.NetworkTxBytes, elapsed)
		}
	}

	s.lastRaw = raw
	s.hasRaw = true
	s.lastSweep = raw.Timestamp

	if len(s.history) > 0 && sameHostMetricsPoint(s.history[len(s.history)-1], point) {
		return
	}
	s.history = append(s.history, point)
	cutoff := raw.Timestamp.Add(-hostMetricsRetention)
	start := 0
	for start < len(s.history) && s.history[start].Timestamp.Before(cutoff) {
		start++
	}
	s.history = s.history[start:]
}

func (s *HostMetricsService) Response(rangeKey string) models.HostMetricsResponse {
	s.collectIfStale()

	if rangeKey == "" {
		rangeKey = "1h"
	}
	window := hostRangeDuration(rangeKey)
	since := time.Now().Add(-window)

	s.mu.RLock()
	points := filterHostMetricsSince(s.history, since)
	points = downsampleHostMetrics(points, hostMetricsMaxPoints)
	summary := models.HostMetricsPoint{}
	if len(s.history) > 0 {
		summary = s.history[len(s.history)-1]
	}
	updatedAt := summary.Timestamp
	s.mu.RUnlock()

	return models.HostMetricsResponse{
		Range:           rangeKey,
		IntervalSeconds: int(hostMetricsSampleInterval.Seconds()),
		UpdatedAt:       updatedAt,
		System:          collectHostSystemOverview(summary),
		Summary:         summary,
		Points:          points,
		Processes:       collectHostProcesses(),
	}
}

func (s *HostMetricsService) collectIfStale() {
	s.mu.RLock()
	lastSweep := s.lastSweep
	s.mu.RUnlock()

	if time.Since(lastSweep) >= hostMetricsSampleInterval/2 {
		s.Collect()
	}
}

func collectHostMetricsRaw(now time.Time) hostMetricsRaw {
	idle, total := readCPUStat()
	memoryUsed, memoryTotal := readMemoryUsage()
	networkRx, networkTx := readNetworkTotals()

	return hostMetricsRaw{
		Timestamp:        now,
		CPUIdle:          idle,
		CPUTotal:         total,
		CPUMHz:           readCPUMHz(),
		TemperatureC:     readTemperatureC(),
		MemoryUsedBytes:  memoryUsed,
		MemoryTotalBytes: memoryTotal,
		NetworkRxBytes:   networkRx,
		NetworkTxBytes:   networkTx,
	}
}

func collectHostSystemOverview(summary models.HostMetricsPoint) models.HostSystemOverview {
	hostname, _ := os.Hostname()
	cpuModel, cpuCores, cpuThreads := readCPUOverview()
	runtimeMemoryTotal := summary.MemoryTotalBytes
	if runtimeMemoryTotal == 0 {
		_, runtimeMemoryTotal = readMemoryUsage()
	}
	memoryOverview := readMemoryOverview(runtimeMemoryTotal)
	cpuMHz := summary.CPUMHz
	if cpuMHz == 0 {
		cpuMHz = readCPUMHz()
	}
	cpuPackages := readCPUPackages()
	if len(cpuPackages) > 0 && cpuModel == "" {
		cpuModel = cpuPackages[0].Model
	}

	return models.HostSystemOverview{
		Hostname:             hostname,
		OS:                   readOSPrettyName(),
		UptimeSeconds:        readUptimeSeconds(),
		CPUModel:             cpuModel,
		CPUCores:             cpuCores,
		CPUThreads:           cpuThreads,
		CPUMHz:               cpuMHz,
		CPUs:                 cpuPackages,
		GPUs:                 readGPUNames(),
		MemoryTotalBytes:     runtimeMemoryTotal,
		MemoryInstalledBytes: memoryOverview.TotalBytes,
		MemoryModules:        memoryOverview.Modules,
	}
}

func readCPUStat() (uint64, uint64) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			return 0, 0
		}
		var values []uint64
		for _, field := range fields[1:] {
			value, err := strconv.ParseUint(field, 10, 64)
			if err != nil {
				value = 0
			}
			values = append(values, value)
		}
		var total uint64
		for _, value := range values {
			total += value
		}
		idle := values[3]
		if len(values) > 4 {
			idle += values[4]
		}
		return idle, total
	}
	return 0, 0
}

func readMemoryUsage() (int64, int64) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0
	}
	values := map[string]int64{}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSuffix(fields[0], ":")
		value, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			continue
		}
		values[key] = value * 1024
	}
	total := values["MemTotal"]
	available := values["MemAvailable"]
	if available == 0 {
		available = values["MemFree"] + values["Buffers"] + values["Cached"]
	}
	used := total - available
	if used < 0 {
		used = 0
	}
	return used, total
}

func readNetworkTotals() (int64, int64) {
	data, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return 0, 0
	}
	var rxTotal int64
	var txTotal int64
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.Contains(line, ":") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		iface := strings.TrimSpace(parts[0])
		if iface == "" || iface == "lo" {
			continue
		}
		fields := strings.Fields(parts[1])
		if len(fields) < 16 {
			continue
		}
		rxTotal += parseInt64Default(fields[0], 0)
		txTotal += parseInt64Default(fields[8], 0)
	}
	return rxTotal, txTotal
}

func readCPUMHz() float64 {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return 0
	}
	var total float64
	var count float64
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "cpu MHz") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		value, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err != nil {
			continue
		}
		total += value
		count++
	}
	if count == 0 {
		return 0
	}
	return total / count
}

func readCPUOverview() (string, int, int) {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		threads := runtime.NumCPU()
		return "", threads, threads
	}

	blocks := strings.Split(string(data), "\n\n")
	coreKeys := map[string]struct{}{}
	physicalIDs := map[string]struct{}{}
	modelName := ""
	threadCount := 0
	fallbackCores := 0

	for _, block := range blocks {
		lines := strings.Split(block, "\n")
		fields := map[string]string{}
		for _, line := range lines {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) != 2 {
				continue
			}
			fields[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
		if _, ok := fields["processor"]; ok {
			threadCount++
		}
		if modelName == "" {
			modelName = fields["model name"]
		}
		if value := atoiDefault(fields["cpu cores"], 0); value > fallbackCores {
			fallbackCores = value
		}
		physicalID := fields["physical id"]
		coreID := fields["core id"]
		if physicalID != "" {
			physicalIDs[physicalID] = struct{}{}
		}
		if physicalID != "" && coreID != "" {
			coreKeys[physicalID+":"+coreID] = struct{}{}
		}
	}

	if threadCount == 0 {
		threadCount = runtime.NumCPU()
	}
	coreCount := len(coreKeys)
	if coreCount == 0 && fallbackCores > 0 {
		socketCount := len(physicalIDs)
		if socketCount == 0 {
			socketCount = 1
		}
		coreCount = fallbackCores * socketCount
	}
	if coreCount == 0 {
		coreCount = threadCount
	}

	return modelName, coreCount, threadCount
}

func readCPUPackages() []models.HostCPUInfo {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		threads := runtime.NumCPU()
		return []models.HostCPUInfo{{
			Index:      1,
			Cores:      threads,
			Threads:    threads,
			CurrentMHz: readCPUMHz(),
			MaxMHz:     readCPUMaxMHz(),
		}}
	}

	packages := map[string]*hostCPUPackage{}
	for _, block := range strings.Split(string(data), "\n\n") {
		fields := map[string]string{}
		for _, line := range strings.Split(block, "\n") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) != 2 {
				continue
			}
			fields[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
		if _, ok := fields["processor"]; !ok {
			continue
		}

		physicalID := fields["physical id"]
		if physicalID == "" {
			physicalID = "0"
		}
		pkg := packages[physicalID]
		if pkg == nil {
			pkg = &hostCPUPackage{
				PhysicalID: physicalID,
				CoreKeys:   map[string]struct{}{},
			}
			packages[physicalID] = pkg
		}
		if pkg.Model == "" {
			pkg.Model = fields["model name"]
		}
		if coreID := fields["core id"]; coreID != "" {
			pkg.CoreKeys[coreID] = struct{}{}
		}
		if value := atoiDefault(fields["cpu cores"], 0); value > pkg.FallbackCores {
			pkg.FallbackCores = value
		}
		if mhz := parseFloatDefault(fields["cpu MHz"], 0); mhz > 0 {
			pkg.TotalMHz += mhz
			pkg.MHzCount++
		}
		pkg.ThreadCount++
	}

	ids := make([]string, 0, len(packages))
	for id := range packages {
		ids = append(ids, id)
	}
	sort.SliceStable(ids, func(i, j int) bool {
		left, leftErr := strconv.Atoi(ids[i])
		right, rightErr := strconv.Atoi(ids[j])
		if leftErr == nil && rightErr == nil {
			return left < right
		}
		return ids[i] < ids[j]
	})

	maxMHz := readCPUMaxMHz()
	cpus := make([]models.HostCPUInfo, 0, len(ids))
	for index, id := range ids {
		pkg := packages[id]
		cores := len(pkg.CoreKeys)
		if cores == 0 {
			cores = pkg.FallbackCores
		}
		if cores == 0 {
			cores = pkg.ThreadCount
		}
		currentMHz := float64(0)
		if pkg.MHzCount > 0 {
			currentMHz = pkg.TotalMHz / float64(pkg.MHzCount)
		}
		cpus = append(cpus, models.HostCPUInfo{
			Index:      index + 1,
			Model:      pkg.Model,
			Cores:      cores,
			Threads:    pkg.ThreadCount,
			CurrentMHz: currentMHz,
			MaxMHz:     maxMHz,
		})
	}
	return cpus
}

func readCPUMaxMHz() float64 {
	if data, err := os.ReadFile("/sys/devices/system/cpu/cpu0/cpufreq/cpuinfo_max_freq"); err == nil {
		if khz := parseFloatDefault(string(data), 0); khz > 0 {
			return khz / 1000
		}
	}

	output, err := exec.Command("lscpu").CombinedOutput()
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "CPU max MHz:") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		return parseFloatDefault(parts[1], 0)
	}
	return 0
}

func readOSPrettyName() string {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return runtime.GOOS
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "PRETTY_NAME=") {
			continue
		}
		value := strings.TrimPrefix(line, "PRETTY_NAME=")
		value = strings.TrimSpace(value)
		if unquoted, err := strconv.Unquote(value); err == nil {
			value = unquoted
		}
		return value
	}
	return runtime.GOOS
}

func readUptimeSeconds() int64 {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0
	}
	value, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0
	}
	return int64(value)
}

func readGPUNames() []string {
	output, err := exec.Command(
		"nvidia-smi",
		"--query-gpu=name",
		"--format=csv,noheader",
	).CombinedOutput()
	if err != nil {
		return []string{}
	}
	gpus := []string{}
	for _, line := range nonEmptyLines(string(output)) {
		name := strings.TrimSpace(line)
		if name == "" {
			continue
		}
		gpus = append(gpus, name)
	}
	return gpus
}

func readMemoryOverview(fallbackBytes int64) hostMemoryOverview {
	modules := readMemoryModulesFromDMI()
	var totalBytes int64
	for _, module := range modules {
		totalBytes += module.SizeBytes
	}
	if totalBytes == 0 {
		totalBytes = roundInstalledMemoryBytes(fallbackBytes)
	}
	return hostMemoryOverview{
		TotalBytes: totalBytes,
		Modules:    modules,
	}
}

func readMemoryModulesFromDMI() []models.HostMemoryModule {
	output, err := exec.Command("dmidecode", "--type", "memory").CombinedOutput()
	if err != nil {
		return []models.HostMemoryModule{}
	}

	modules := []models.HostMemoryModule{}
	var current map[string]string
	flush := func() {
		if current == nil {
			return
		}
		sizeBytes := parseMemorySizeBytes(current["Size"])
		if sizeBytes > 0 {
			speed := cleanDMIValue(current["Configured Memory Speed"])
			if speed == "" {
				speed = cleanDMIValue(current["Speed"])
			}
			modules = append(modules, models.HostMemoryModule{
				Locator:      cleanDMIValue(current["Locator"]),
				BankLocator:  cleanDMIValue(current["Bank Locator"]),
				SizeBytes:    sizeBytes,
				Manufacturer: cleanDMIValue(current["Manufacturer"]),
				PartNumber:   cleanDMIValue(current["Part Number"]),
				Speed:        speed,
			})
		}
		current = nil
	}

	for _, rawLine := range strings.Split(string(output), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "Memory Device" {
			flush()
			current = map[string]string{}
			continue
		}
		if current == nil {
			continue
		}
		if line == "" {
			flush()
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		current[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	flush()
	return modules
}

func parseMemorySizeBytes(value string) int64 {
	value = cleanDMIValue(value)
	if value == "" {
		return 0
	}
	if strings.Contains(strings.ToLower(value), "no module") {
		return 0
	}
	fields := strings.Fields(value)
	if len(fields) < 2 {
		return 0
	}
	amount := parseFloatDefault(fields[0], 0)
	if amount <= 0 {
		return 0
	}
	unit := strings.ToUpper(strings.TrimSpace(fields[1]))
	switch {
	case strings.HasPrefix(unit, "KB"):
		return int64(amount * 1024)
	case strings.HasPrefix(unit, "MB"):
		return int64(amount * 1024 * 1024)
	case strings.HasPrefix(unit, "GB"):
		return int64(amount * 1024 * 1024 * 1024)
	case strings.HasPrefix(unit, "TB"):
		return int64(amount * 1024 * 1024 * 1024 * 1024)
	case unit == "B" || strings.HasPrefix(unit, "BYTE"):
		return int64(amount)
	default:
		return 0
	}
}

func roundInstalledMemoryBytes(fallbackBytes int64) int64 {
	if fallbackBytes <= 0 {
		return 0
	}
	const gib = int64(1024 * 1024 * 1024)
	totalGiB := int64(math.Ceil(float64(fallbackBytes) / float64(gib)))
	if totalGiB <= 0 {
		return fallbackBytes
	}
	quantumGiB := int64(1)
	switch {
	case totalGiB > 128:
		quantumGiB = 16
	case totalGiB > 32:
		quantumGiB = 8
	case totalGiB > 8:
		quantumGiB = 4
	}
	roundedGiB := ((totalGiB + quantumGiB - 1) / quantumGiB) * quantumGiB
	return roundedGiB * gib
}

func cleanDMIValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	lower := strings.ToLower(value)
	emptyMarkers := []string{
		"unknown",
		"not specified",
		"not provided",
		"no module installed",
		"to be filled by o.e.m.",
		"default string",
		"none",
	}
	for _, marker := range emptyMarkers {
		if strings.Contains(lower, marker) {
			return ""
		}
	}
	return value
}

func firstNonEmptyDMIValue(values ...string) string {
	for _, value := range values {
		if cleaned := cleanDMIValue(value); cleaned != "" {
			return cleaned
		}
	}
	return ""
}

func readTemperatureC() float64 {
	patterns := []string{
		"/sys/class/thermal/thermal_zone*/temp",
		"/sys/class/hwmon/hwmon*/temp*_input",
	}
	var maxTemp float64
	for _, pattern := range patterns {
		paths, _ := filepath.Glob(pattern)
		for _, path := range paths {
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			value, err := strconv.ParseFloat(strings.TrimSpace(string(data)), 64)
			if err != nil || value <= 0 {
				continue
			}
			if value > 1000 {
				value = value / 1000
			}
			if value > 0 && value < 200 && value > maxTemp {
				maxTemp = value
			}
		}
	}
	return maxTemp
}

func collectHostProcesses() []models.HostProcess {
	output, err := exec.Command(
		"ps",
		"-eo",
		"pid=,user=,pcpu=,pmem=,rss=,comm=",
		"--sort=-pcpu",
	).CombinedOutput()
	if err != nil {
		return []models.HostProcess{}
	}

	processes := make([]models.HostProcess, 0, hostProcessLimit)
	groupsByContainer, _ := NewLxcGroupService().GroupsByContainer()
	for _, line := range nonEmptyLines(string(output)) {
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}
		pid := atoiDefault(fields[0], 0)
		if pid <= 0 {
			continue
		}
		ownerType, containerName := processOwner(pid)
		groups := groupsByContainer[containerName]
		if groups == nil {
			groups = []models.LxcGroup{}
		}
		processes = append(processes, models.HostProcess{
			PID:           pid,
			User:          fields[1],
			CPUPercent:    parseFloatDefault(fields[2], 0),
			MemoryPercent: parseFloatDefault(fields[3], 0),
			MemoryBytes:   parseInt64Default(fields[4], 0) * 1024,
			Command:       strings.Join(fields[5:], " "),
			OwnerType:     ownerType,
			ContainerName: containerName,
			Groups:        groups,
		})
		if len(processes) >= hostProcessLimit {
			break
		}
	}

	sort.SliceStable(processes, func(i, j int) bool {
		if processes[i].CPUPercent == processes[j].CPUPercent {
			return processes[i].MemoryBytes > processes[j].MemoryBytes
		}
		return processes[i].CPUPercent > processes[j].CPUPercent
	})
	return processes
}

func filterHostMetricsSince(samples []models.HostMetricsPoint, since time.Time) []models.HostMetricsPoint {
	points := make([]models.HostMetricsPoint, 0, len(samples))
	var carry *models.HostMetricsPoint
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
		points = append([]models.HostMetricsPoint{carryCopy}, points...)
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

func downsampleHostMetrics(points []models.HostMetricsPoint, maxPoints int) []models.HostMetricsPoint {
	if len(points) <= maxPoints {
		return points
	}
	bucketSize := int(math.Ceil(float64(len(points)) / float64(maxPoints)))
	result := make([]models.HostMetricsPoint, 0, maxPoints)
	for i := 0; i < len(points); i += bucketSize {
		end := i + bucketSize
		if end > len(points) {
			end = len(points)
		}
		result = append(result, averageHostMetricsBucket(points[i:end]))
	}
	return result
}

func averageHostMetricsBucket(points []models.HostMetricsPoint) models.HostMetricsPoint {
	if len(points) == 0 {
		return models.HostMetricsPoint{}
	}
	result := models.HostMetricsPoint{Timestamp: points[len(points)-1].Timestamp}
	for _, point := range points {
		result.CPUPercent += point.CPUPercent
		result.CPUMHz += point.CPUMHz
		result.TemperatureC += point.TemperatureC
		result.MemoryUsedBytes += point.MemoryUsedBytes
		result.MemoryTotalBytes += point.MemoryTotalBytes
		result.NetworkRxBps += point.NetworkRxBps
		result.NetworkTxBps += point.NetworkTxBps
		result.NetworkRxBytes += point.NetworkRxBytes
		result.NetworkTxBytes += point.NetworkTxBytes
	}
	divisor := float64(len(points))
	result.CPUPercent /= divisor
	result.CPUMHz /= divisor
	result.TemperatureC /= divisor
	result.MemoryUsedBytes /= int64(len(points))
	result.MemoryTotalBytes /= int64(len(points))
	result.NetworkRxBps /= divisor
	result.NetworkTxBps /= divisor
	result.NetworkRxBytes /= int64(len(points))
	result.NetworkTxBytes /= int64(len(points))
	return result
}

func sameHostMetricsPoint(a, b models.HostMetricsPoint) bool {
	return a.CPUPercent == b.CPUPercent &&
		a.CPUMHz == b.CPUMHz &&
		a.TemperatureC == b.TemperatureC &&
		a.MemoryUsedBytes == b.MemoryUsedBytes &&
		a.MemoryTotalBytes == b.MemoryTotalBytes &&
		a.NetworkRxBps == b.NetworkRxBps &&
		a.NetworkTxBps == b.NetworkTxBps &&
		a.NetworkRxBytes == b.NetworkRxBytes &&
		a.NetworkTxBytes == b.NetworkTxBytes
}

func hostRangeDuration(rangeKey string) time.Duration {
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

func parseInt64Default(value string, defaultValue int64) int64 {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return defaultValue
	}
	return parsed
}

func parseFloatDefault(value string, defaultValue float64) float64 {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return defaultValue
	}
	return parsed
}
