package service

import (
	"LabPanel/models"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReadHostNetworkPorts(t *testing.T) {
	root := t.TempDir()
	write := func(path, value string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, path), []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
	}
	makeDir := func(path string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(root, path), 0700); err != nil {
			t.Fatal(err)
		}
	}
	for i, iface := range []string{"enp1s0", "enp2s0", "br0", "bond0", "lo", "veth0", "enp1s0.10", "wlan0", "enp1s0v0"} {
		makeDir(iface)
		write(iface+"/type", "1\n")
		write(iface+"/ifindex", fmt.Sprint(i+1))
		write(iface+"/carrier", "1\n")
	}
	write("enp2s0/carrier", "0\n")
	write("enp1s0/carrier_changes", "12\n")
	for _, path := range []string{"enp1s0/device", "enp2s0/device", "wlan0/device", "wlan0/phy80211", "enp1s0v0/device", "enp1s0v0/device/physfn"} {
		makeDir(path)
	}
	const counters = ": 120 0 0 0 0 0 0 0 240 0 0 0 0 0 0 0\n"
	write("netdev", "enp1s0"+counters+"enp2s0"+counters+"br0"+counters+"bond0"+counters+
		"lo"+counters+"veth0"+counters+"enp1s0.10"+counters+"wlan0"+counters+"enp1s0v0"+counters)
	ports := readHostNetworkPorts(filepath.Join(root, "netdev"), root)
	if len(ports) != 2 || ports["enp1s0"] != (hostNetworkCounters{"1", "12", true, 120, 240}) ||
		ports["enp2s0"].LinkUp {
		t.Fatalf("unexpected ports: %+v", ports)
	}
}

func TestHostNetworkRatesAndInterruptions(t *testing.T) {
	before := map[string]hostNetworkCounters{
		"eth0": {IfIndex: "1", LinkUp: true, CarrierChanges: "1", RxBytes: 1000, TxBytes: 2000},
		"eth1": {IfIndex: "2", LinkUp: true, CarrierChanges: "1", RxBytes: 500, TxBytes: 900},
	}
	current := map[string]hostNetworkCounters{
		"eth0": {IfIndex: "1", LinkUp: true, CarrierChanges: "1", RxBytes: 1600, TxBytes: 2040},
		"eth1": {IfIndex: "2", LinkUp: true, CarrierChanges: "1", RxBytes: 900, TxBytes: 930},
	}
	var normal models.HostMetricsPoint
	addHostNetworkMetrics(&normal, current, before, 10)
	if normal.NetworkRxBps != 100 || normal.NetworkTxBps != 7 || normal.NetworkRxBytes != 2500 ||
		normal.NetworkTxBytes != 2970 || *normal.NetworkPorts[0].RxBps != 60 || *normal.NetworkPorts[1].TxBps != 3 {
		t.Fatalf("unexpected aggregate or port rates: %+v", normal)
	}
	current["eth0"] = hostNetworkCounters{IfIndex: "1", CarrierChanges: "2", RxBytes: 1610, TxBytes: 2040}
	current["eth1"] = hostNetworkCounters{IfIndex: "2", LinkUp: true, CarrierChanges: "3", RxBytes: 100, TxBytes: 1000}
	current["eth2"] = hostNetworkCounters{IfIndex: "3", LinkUp: true, RxBytes: 1000000, TxBytes: 5000000}
	var interrupted models.HostMetricsPoint
	addHostNetworkMetrics(&interrupted, current, before, 10)
	if interrupted.NetworkRxBps != 61 || interrupted.NetworkTxBps != 14 {
		t.Fatalf("unexpected rates after disconnect, reset and new port: %+v", interrupted)
	}
	for _, port := range interrupted.NetworkPorts {
		if port.RxBps != nil || port.TxBps != nil {
			t.Fatalf("unstable link has a plotted rate: %+v", port)
		}
	}
	var initial models.HostMetricsPoint
	addHostNetworkMetrics(&initial, current, nil, 10)
	if initial.NetworkRxBps != 0 || initial.NetworkPorts[0].RxBps != nil {
		t.Fatalf("unexpected first sample rate: %+v", initial)
	}
	averaged := averageHostMetricsBucket([]models.HostMetricsPoint{normal, interrupted})
	if averaged.NetworkPorts[0].LinkUp || averaged.NetworkPorts[0].RxBps != nil || averaged.NetworkPorts[1].RxBps != nil {
		t.Fatalf("downsampling filled a disconnect gap: %+v", averaged.NetworkPorts)
	}
	averaged = averageHostMetricsBucket([]models.HostMetricsPoint{normal, normal})
	if *averaged.NetworkPorts[0].RxBps != 60 || *averaged.NetworkPorts[1].TxBps != 3 {
		t.Fatalf("unexpected downsampling: %+v", averaged.NetworkPorts)
	}
}

func TestHostNetworkMetricsUpgradeAndRestore(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "metrics.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, q := range []string{
		`CREATE TABLE host_metric_points (timestamp_ns INTEGER PRIMARY KEY, cpu_percent REAL NOT NULL,
			cpu_mhz REAL NOT NULL, temperature_c REAL NOT NULL, memory_used_bytes INTEGER NOT NULL,
			memory_total_bytes INTEGER NOT NULL, network_rx_bps REAL NOT NULL, network_tx_bps REAL NOT NULL,
			network_rx_bytes INTEGER NOT NULL, network_tx_bytes INTEGER NOT NULL)`,
		`CREATE TABLE host_metric_raw_state (id INTEGER PRIMARY KEY, timestamp_ns INTEGER NOT NULL,
			boot_id TEXT NOT NULL, cpu_idle INTEGER NOT NULL, cpu_total INTEGER NOT NULL,
			network_rx_bytes INTEGER NOT NULL, network_tx_bytes INTEGER NOT NULL)`,
		`INSERT INTO host_metric_points VALUES (100, 5, 7, 8, 9, 10, 11, 12, 13, 14)`,
		`INSERT INTO host_metric_raw_state VALUES (1, 100, 'boot', 20, 50, 13, 14)`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	store := &MetricsStore{db: db, writes: make(chan metricsWriteJob, 3)}
	for i := 0; i < 2; i++ {
		if err := store.initSchema(); err != nil {
			t.Fatal(err)
		}
	}
	old, ok, err := store.LoadHostRaw()
	if err != nil || !ok || old.NetworkPorts != nil || old.CPUTotal != 50 {
		t.Fatalf("legacy raw state should preserve CPU baseline only: %v %t %v", old, ok, err)
	}
	raw := hostMetricsRaw{Timestamp: time.Unix(0, 200), BootID: "boot",
		NetworkPorts: map[string]hostNetworkCounters{"eth0": {IfIndex: "2", LinkUp: true, RxBytes: 200, TxBytes: 300}}}
	rate := 25.0
	point := models.HostMetricsPoint{Timestamp: raw.Timestamp,
		NetworkPorts: []models.HostNetworkPort{{Name: "eth0", LinkUp: true, RxBps: &rate}}}
	if err := store.SaveHostRaw(raw); err != nil {
		t.Fatal(err)
	}
	if err := store.InsertHostMetric(point); err != nil {
		t.Fatal(err)
	}
	for len(store.writes) > 0 {
		if err := (<-store.writes).run(db); err != nil {
			t.Fatal(err)
		}
	}
	got, ok, err := store.LoadHostRaw()
	if err != nil || !ok || got.NetworkPorts["eth0"] != raw.NetworkPorts["eth0"] {
		t.Fatalf("raw counters were not restored: %v %t %v", got, ok, err)
	}
	points, err := store.LoadHostMetricsSince(time.Unix(0, 0))
	if err != nil || len(points) != 2 || points[0].NetworkPorts != nil || points[0].NetworkRxBps != 11 ||
		!sameHostMetricsPoint(points[1], point) {
		t.Fatalf("legacy and new history did not round trip: %+v, %v", points, err)
	}
}
