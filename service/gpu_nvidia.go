package service

import (
	"LabPanel/models"
	"context"
	"encoding/xml"
	"math"
	"os/exec"
	"strconv"
	"strings"
)

type nvidiaXML struct {
	XMLName xml.Name `xml:"nvidia_smi_log"`
	Driver  string   `xml:"driver_version"`
	KMD     string   `xml:"kmd_version"`
	CUDA    string   `xml:"cuda_version"`
	UMD     string   `xml:"cuda_umd_version"`
	GPUs    []struct {
		UUID        string `xml:"uuid"`
		Fan         string `xml:"fan_speed"`
		Temperature string `xml:"temperature>gpu_temp"`
		TempLimit   string `xml:"temperature>gpu_temp_max_gpu_threshold"`
		TempTarget  string `xml:"temperature>gpu_target_temperature"`
		MemoryTotal string `xml:"fb_memory_usage>total"`
		MemoryUsed  string `xml:"fb_memory_usage>used"`
		Load        string `xml:"utilization>gpu_util"`
		Power       struct {
			Average string `xml:"average_power_draw"`
			Instant string `xml:"instant_power_draw"`
			Limit   string `xml:"current_power_limit"`
		} `xml:"gpu_power_readings"`
		LegacyPower struct {
			Draw  string `xml:"power_draw"`
			Limit string `xml:"enforced_power_limit"`
			Ceil  string `xml:"power_limit"`
		} `xml:"power_readings"`
		PCIGen   string `xml:"pci>pci_gpu_link_info>pcie_gen>current_link_gen"`
		PCIWidth string `xml:"pci>pci_gpu_link_info>link_widths>current_link_width"`
		Rx       string `xml:"pci>rx_util"`
		Tx       string `xml:"pci>tx_util"`
	} `xml:"gpu"`
}

func nvidiaQuery(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "nvidia-smi", args...)
	cmd.Env = append(cmd.Environ(), "LC_ALL=C", "NO_COLOR=1")
	return cmd.Output()
}

func gpuNumber(raw string) *float64 {
	fields := strings.Fields(raw)
	if len(fields) == 0 {
		return nil
	}
	value, err := strconv.ParseFloat(strings.TrimSuffix(fields[0], "x"), 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return nil
	}
	return &value
}

func gpuPositiveInteger(raw string) *int {
	if n := gpuNumber(raw); n != nil && *n > 0 && *n < 1024 && math.Trunc(*n) == *n {
		value := int(*n)
		return &value
	}
	return nil
}

func gpuRateBytes(raw string) *float64 {
	fields := strings.Fields(raw)
	value := gpuNumber(raw)
	if len(fields) != 2 || value == nil || *value < 0 {
		return nil
	}
	units := map[string]float64{"B/s": 1, "KB/s": 1000, "MB/s": 1e6, "GB/s": 1e9, "KiB/s": 1024, "MiB/s": 1 << 20, "GiB/s": 1 << 30}
	multiplier, ok := units[fields[1]]
	if !ok {
		return nil
	}
	*value *= multiplier
	return value
}

func firstGPUReading(current *float64, candidates ...string) *float64 {
	for _, raw := range candidates {
		if value := gpuNumber(raw); value != nil {
			return value
		}
	}
	return current
}

func firstGPUVersion(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && value != "N/A" && !strings.HasPrefix(value, "Not Supported") {
			return value
		}
	}
	return ""
}

func applyNvidiaXML(response *models.GPUMonitorResponse, data []byte) error {
	var report nvidiaXML
	if err := xml.Unmarshal(data, &report); err != nil {
		return err
	}
	response.DriverVersion = firstGPUVersion(report.KMD, report.Driver)
	response.CUDASupportedVersion = firstGPUVersion(report.UMD, report.CUDA)
	byUUID := make(map[string]*models.GPUDevice, len(response.GPUs))
	for i := range response.GPUs {
		byUUID[response.GPUs[i].UUID] = &response.GPUs[i]
	}
	for _, row := range report.GPUs {
		gpu := byUUID[strings.TrimSpace(row.UUID)]
		if gpu == nil {
			continue
		}
		gpu.Temperature = firstGPUReading(gpu.Temperature, row.Temperature)
		gpu.MemoryTotalMiB = firstGPUReading(gpu.MemoryTotalMiB, row.MemoryTotal)
		gpu.MemoryUsedMiB = firstGPUReading(gpu.MemoryUsedMiB, row.MemoryUsed)
		gpu.Utilization = firstGPUReading(gpu.Utilization, row.Load)
		gpu.MaxTemperatureCelsius = gpuNumber(row.TempLimit)
		gpu.TemperatureTargetCelsius = gpuNumber(row.TempTarget)
		gpu.FanPercent = gpuNumber(row.Fan)
		gpu.PowerWatts = firstGPUReading(nil, row.Power.Instant, row.Power.Average, row.LegacyPower.Draw)
		gpu.MaxPowerWatts = firstGPUReading(nil, row.Power.Limit, row.LegacyPower.Limit, row.LegacyPower.Ceil)
		link := models.GPULink{
			LinkGeneration: gpuPositiveInteger(row.PCIGen), LinkWidth: gpuPositiveInteger(row.PCIWidth),
			RxBytesPerSecond: gpuRateBytes(row.Rx), TxBytesPerSecond: gpuRateBytes(row.Tx),
		}
		if link.LinkGeneration != nil || link.LinkWidth != nil || link.RxBytesPerSecond != nil || link.TxBytesPerSecond != nil {
			gpu.PCIe = &link
		}
	}
	return nil
}
