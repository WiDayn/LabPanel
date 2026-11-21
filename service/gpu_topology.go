package service

import (
	"LabPanel/models"
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	nvidiaANSI         = regexp.MustCompile("\\x1b\\[[0-9;]*[A-Za-z]")
	nvidiaNVPath       = regexp.MustCompile(`^NV[0-9]+$`)
	nvidiaNVGPU        = regexp.MustCompile(`^GPU\s+(\d+):.*\(UUID:\s*(GPU-[^\s)]+)\)`)
	nvidiaNVActiveLink = regexp.MustCompile(`^Link\s+\d+:\s+[0-9.]+\s+(KB|MB|GB|TB)/s\b`)
	nvidiaNVData       = regexp.MustCompile(`^Link\s+(\d+):\s+Data (Rx|Tx):\s+(\d+)\s+KiB\b`)
)

type gpuIndexPair struct{ from, to int }

func parseGPUIndex(value string) (int, bool) {
	number, ok := strings.CutPrefix(value, "GPU")
	index, err := strconv.Atoi(number)
	return index, ok && err == nil && index >= 0
}

func parseNvidiaMatrix(output []byte) map[gpuIndexPair]string {
	cells := make(map[gpuIndexPair]string)
	var columns []int
	for _, line := range strings.Split(nvidiaANSI.ReplaceAllString(string(output), ""), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if len(columns) == 0 {
			for _, field := range fields {
				index, ok := parseGPUIndex(field)
				if !ok {
					break
				}
				columns = append(columns, index)
			}
			continue
		}
		source, ok := parseGPUIndex(fields[0])
		if !ok || len(fields) <= len(columns) {
			continue
		}
		for i, target := range columns {
			if source != target {
				cells[gpuIndexPair{source, target}] = fields[i+1]
			}
		}
	}
	return cells
}

func nvidiaP2PStatus(code string) string {
	switch code {
	case "OK":
		return "supported"
	case "CNS", "GNS", "TNS", "NS":
		return "unsupported"
	default:
		return "unknown"
	}
}

func buildNvidiaTopology(gpus []models.GPUDevice, topo, read, write []byte) *models.GPUTopology {
	paths, reads, writes := parseNvidiaMatrix(topo), parseNvidiaMatrix(read), parseNvidiaMatrix(write)
	result := &models.GPUTopology{}
	for _, source := range gpus {
		for _, target := range gpus {
			if source.UUID == target.UUID {
				continue
			}
			key := gpuIndexPair{source.Index, target.Index}
			path, rd, wr := paths[key], reads[key], writes[key]
			if path == "N/A" || path == "?" || path == "X" {
				path = ""
			}
			if path == "" && rd == "" && wr == "" {
				continue
			}
			result.Pairs = append(result.Pairs, models.GPUTopologyPair{
				SourceUUID: source.UUID, TargetUUID: target.UUID, Path: path,
				P2PRead: nvidiaP2PStatus(rd), P2PWrite: nvidiaP2PStatus(wr),
				P2PReadCode: rd, P2PWriteCode: wr,
			})
		}
	}
	if len(result.Pairs) == 0 {
		return nil
	}
	return result
}

type nvidiaCounterPair struct {
	rx *uint64
	tx *uint64
}

type nvidiaNVCounters map[string]map[int]nvidiaCounterPair

func nvidiaActiveLinks(output []byte) map[string]int {
	counts := make(map[string]int)
	var uuid string
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "GPU ") {
			uuid = ""
			if match := nvidiaNVGPU.FindStringSubmatch(line); match != nil {
				uuid = match[2]
			}
		} else if uuid != "" && nvidiaNVActiveLink.MatchString(line) {
			counts[uuid]++
		}
	}
	return counts
}

func parseNvidiaNVCounters(output []byte) nvidiaNVCounters {
	counters := make(nvidiaNVCounters)
	var uuid string
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "GPU ") {
			uuid = ""
			if match := nvidiaNVGPU.FindStringSubmatch(line); match != nil {
				uuid = match[2]
			}
			continue
		}
		match := nvidiaNVData.FindStringSubmatch(line)
		if uuid == "" || match == nil {
			continue
		}
		link, err := strconv.Atoi(match[1])
		raw, rawErr := strconv.ParseUint(match[3], 10, 64)
		if err != nil || rawErr != nil {
			continue
		}
		if counters[uuid] == nil {
			counters[uuid] = make(map[int]nvidiaCounterPair)
		}
		values := counters[uuid][link]
		if match[2] == "Rx" {
			values.rx = &raw
		} else {
			values.tx = &raw
		}
		counters[uuid][link] = values
	}
	return counters
}

func nvidiaNVRate(previous, current map[int]nvidiaCounterPair, count int, elapsed float64, rx bool) *float64 {
	if elapsed <= 0 || elapsed > 20 || count <= 0 || len(previous) != count || len(current) != count {
		return nil
	}
	var rate float64
	for link, now := range current {
		before := previous[link]
		a, b := before.tx, now.tx
		if rx {
			a, b = before.rx, now.rx
		}
		if a == nil || b == nil || *b < *a {
			return nil
		}
		rate += float64(*b-*a) * 1024 / elapsed
	}
	return &rate
}

// The probe is owned by the collector. Topology is reused between device samples.
type gpuTopologyProbe struct {
	signature string
	checkedAt time.Time
	topology  *models.GPUTopology
	active    map[string]int
	counters  nvidiaNVCounters
	counterAt time.Time
}

func (p *gpuTopologyProbe) sample(ctx context.Context, response *models.GPUMonitorResponse) {
	var signature strings.Builder
	for _, gpu := range response.GPUs {
		fmt.Fprintf(&signature, "%d:%s;", gpu.Index, gpu.UUID)
	}
	if key := signature.String(); key != p.signature || time.Since(p.checkedAt) >= time.Minute {
		if key != p.signature {
			*p = gpuTopologyProbe{signature: key}
		}
		args := [][]string{{"topo", "-m"}, {"topo", "-p2p", "r"}, {"topo", "-p2p", "w"}, {"nvlink", "--status"}}
		outputs := make([][]byte, len(args))
		probeCtx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
		var wait sync.WaitGroup
		for i, command := range args {
			i, command := i, command
			wait.Add(1)
			go func() {
				defer wait.Done()
				outputs[i], _ = nvidiaQuery(probeCtx, command...)
			}()
		}
		wait.Wait()
		cancel()
		p.topology = buildNvidiaTopology(response.GPUs, outputs[0], outputs[1], outputs[2])
		p.active = nvidiaActiveLinks(outputs[3])
		p.checkedAt = time.Now()
	}
	response.Topology = p.topology
	nvGPU := make(map[string]bool)
	for uuid := range p.active {
		nvGPU[uuid] = true
	}
	if p.topology != nil {
		for _, pair := range p.topology.Pairs {
			if nvidiaNVPath.MatchString(pair.Path) {
				nvGPU[pair.SourceUUID] = true
				nvGPU[pair.TargetUUID] = true
			}
		}
	}
	if len(nvGPU) == 0 {
		p.counters = nil
		return
	}
	now := time.Now()
	data, err := nvidiaQuery(ctx, "nvlink", "--getthroughput", "d")
	var readings nvidiaNVCounters
	if err == nil {
		readings = parseNvidiaNVCounters(data)
	}
	for i := range response.GPUs {
		gpu := &response.GPUs[i]
		if !nvGPU[gpu.UUID] {
			continue
		}
		link := &models.GPULink{Detected: true}
		if count := p.active[gpu.UUID]; count > 0 {
			link.ActiveLinks = &count
			elapsed := now.Sub(p.counterAt).Seconds()
			link.RxBytesPerSecond = nvidiaNVRate(p.counters[gpu.UUID], readings[gpu.UUID], count, elapsed, true)
			link.TxBytesPerSecond = nvidiaNVRate(p.counters[gpu.UUID], readings[gpu.UUID], count, elapsed, false)
		}
		gpu.NVLink = link
	}
	p.counters, p.counterAt = readings, now
}
