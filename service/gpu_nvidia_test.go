package service

import (
	"LabPanel/models"
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

func TestApplyNvidiaXMLPreservesUnknownAndZero(t *testing.T) {
	report := []byte(`<nvidia_smi_log>
		<driver_version>580.119.02</driver_version><cuda_version>13.0</cuda_version>
		<gpu><uuid>GPU-b</uuid><fan_speed>N/A</fan_speed><fb_memory_usage><used>0 MiB</used></fb_memory_usage>
			<temperature><gpu_temp>40 C</gpu_temp><gpu_temp_max_gpu_threshold>93 C</gpu_temp_max_gpu_threshold><gpu_target_temperature>83 C</gpu_target_temperature></temperature>
			<gpu_power_readings><average_power_draw>18.20 W</average_power_draw><instant_power_draw>N/A</instant_power_draw><current_power_limit>300.00 W</current_power_limit></gpu_power_readings>
			<pci><pci_gpu_link_info><pcie_gen><current_link_gen>1</current_link_gen><max_link_gen>3</max_link_gen><max_device_link_gen>4</max_device_link_gen></pcie_gen><link_widths><current_link_width>16x</current_link_width><max_link_width>16x</max_link_width></link_widths></pci_gpu_link_info><rx_util>0 KB/s</rx_util><tx_util>585 KB/s</tx_util></pci>
		</gpu>
		<gpu><uuid>GPU-a</uuid><fan_speed>0 %</fan_speed><power_readings><power_draw>9.5 W</power_draw><enforced_power_limit>100 W</enforced_power_limit></power_readings></gpu>
	</nvidia_smi_log>`)
	response := models.GPUMonitorResponse{GPUs: []models.GPUDevice{{Index: 4, UUID: "GPU-a"}, {Index: 7, UUID: "GPU-b"}}}
	if err := applyNvidiaXML(&response, report); err != nil {
		t.Fatal(err)
	}
	requireNumber := func(value *float64, want float64) {
		t.Helper()
		if value == nil || *value != want {
			t.Fatalf("wanted %v, got %v", want, value)
		}
	}
	if response.DriverVersion != "580.119.02" || response.CUDASupportedVersion != "13.0" {
		t.Fatal("missing host versions")
	}
	a, b := response.GPUs[0], response.GPUs[1]
	requireNumber(a.FanPercent, 0)
	requireNumber(a.PowerWatts, 9.5)
	requireNumber(b.PowerWatts, 18.2)
	requireNumber(b.MaxPowerWatts, 300)
	requireNumber(b.TemperatureTargetCelsius, 83)
	requireNumber(b.MaxTemperatureCelsius, 93)
	requireNumber(b.MemoryUsedMiB, 0)
	if b.FanPercent != nil || b.PCIe == nil || *b.PCIe.LinkWidth != 16 || *b.PCIe.LinkGeneration != 1 {
		t.Fatalf("wrong or missing readings: %+v", b)
	}
	if b.PCIe.MaxLinkGeneration == nil || *b.PCIe.MaxLinkGeneration != 3 ||
		b.PCIe.MaxLinkWidth == nil || *b.PCIe.MaxLinkWidth != 16 {
		t.Fatalf("unexpected maximum PCIe link: %+v", b.PCIe)
	}
	requireNumber(b.PCIe.RxBytesPerSecond, 0)
	requireNumber(b.PCIe.TxBytesPerSecond, 585000)
}

func TestNvidiaTopologyPreservesDirectionsAndReasons(t *testing.T) {
	devices := []models.GPUDevice{{Index: 0, UUID: "GPU-a"}, {Index: 2, UUID: "GPU-b"}}
	paths := []byte("\t\x1b[4mGPU0\tGPU2\tCPU Affinity\x1b[0m\nGPU0 X SYS 0-15\nGPU2 SYS X 16-31\n")
	read := []byte("\tGPU0\tGPU2\nGPU0 X CNS\nGPU2 OK X\n")
	write := []byte("\tGPU0\tGPU2\nGPU0 X U\nGPU2 TNS X\n")
	result := buildNvidiaTopology(devices, paths, read, write)
	if result == nil || len(result.Pairs) != 2 {
		t.Fatalf("unexpected topology: %+v", result)
	}
	a, b := result.Pairs[0], result.Pairs[1]
	if a.TargetUUID != "GPU-b" || a.Path != "SYS" || a.P2PRead != "unsupported" || a.P2PReadCode != "CNS" || a.P2PWrite != "unknown" ||
		b.P2PRead != "supported" || b.P2PWriteCode != "TNS" {
		t.Fatalf("direction or status lost: %+v %+v", a, b)
	}
}

func TestNvidiaNVLinkRatesRejectIncompleteOrResetCounters(t *testing.T) {
	const a = "GPU 0: A100 (UUID: GPU-a)"
	previous := parseNvidiaNVCounters([]byte(a + "\nLink 0: Data Rx: 8 KiB\nLink 0: Data Tx: 10 KiB\n"))
	current := parseNvidiaNVCounters([]byte(a + "\nLink 0: Data Rx: 18 KiB\nLink 0: Data Tx: 1 KiB\n"))
	if got := nvidiaActiveLinks([]byte(a + "\nLink 0: 25.000 GB/s\nLink 1: <inactive>\n")); got["GPU-a"] != 1 {
		t.Fatalf("active links: %v", got)
	}
	if rate := nvidiaNVRate(previous["GPU-a"], current["GPU-a"], 1, 5, true); rate == nil || *rate != 2048 {
		t.Fatalf("wrong RX rate: %v", rate)
	}
	if nvidiaNVRate(previous["GPU-a"], current["GPU-a"], 1, 5, false) != nil ||
		nvidiaNVRate(previous["GPU-a"], current["GPU-a"], 2, 5, true) != nil {
		t.Fatal("reported a rate from incomplete or reset counters")
	}
}

func TestGPUDeviceTelemetryUsesSharedTimeBucketsAndPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metrics.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	store := &MetricsStore{db: db, writes: make(chan metricsWriteJob, 1)}
	if err := store.initSchema(); err != nil {
		t.Fatal(err)
	}
	start := time.Now().Truncate(15 * time.Second)
	power := 100.0
	for _, step := range []time.Duration{5 * time.Second, 10 * time.Second} {
		if err := store.insertGPUDeviceSamples(start.Add(step), []models.GPUDevice{{UUID: "GPU-a", PowerWatts: &power}, {UUID: "GPU-b"}}); err != nil {
			t.Fatal(err)
		}
		if err := (<-store.writes).run(db); err != nil {
			t.Fatal(err)
		}
		power += 100
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	store.db = db
	history, err := store.loadGPUDeviceSamples(context.Background(), start, 15*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(history["GPU-a"]) != 1 || len(history["GPU-b"]) != 1 {
		t.Fatalf("wrong time buckets: %+v", history)
	}
	a, b := history["GPU-a"][0], history["GPU-b"][0]
	if !a.Timestamp.Equal(start) || !b.Timestamp.Equal(start) || a.PowerWatts == nil || *a.PowerWatts != 150 || b.PowerWatts != nil {
		t.Fatalf("unexpected persisted data: %+v %+v", a, b)
	}
}
