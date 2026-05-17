package service

import (
	"LabPanel/models"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"LabPanel/config"
)

func TestExtractJSONArrayFromLxdEmptyListHint(t *testing.T) {
	output := []byte(`To start your first container, try:

  lxc launch ubuntu:24.04

Or for a virtual machine:

  lxc launch ubuntu:24.04 --vm

[]`)

	jsonOutput, err := extractJSONArray(output)
	if err != nil {
		t.Fatalf("expected JSON array to be extracted: %v", err)
	}

	var containers []map[string]interface{}
	if err := json.Unmarshal(jsonOutput, &containers); err != nil {
		t.Fatalf("expected extracted output to be valid JSON: %v", err)
	}
	if len(containers) != 0 {
		t.Fatalf("expected empty container list, got %d", len(containers))
	}
}

func TestExtractJSONArrayKeepsNormalJSON(t *testing.T) {
	output := []byte(`[{"name":"web","status":"Running"}]`)

	jsonOutput, err := extractJSONArray(output)
	if err != nil {
		t.Fatalf("expected normal JSON array to be accepted: %v", err)
	}
	if string(jsonOutput) != string(output) {
		t.Fatalf("unexpected JSON output: %s", string(jsonOutput))
	}
}

func TestDeleteBackupArchive(t *testing.T) {
	backupDir := t.TempDir()
	archiveName := "ubuntu2404-20260517-180629.tar.gz"
	archivePath := filepath.Join(backupDir, archiveName)
	if err := os.WriteFile(archivePath, []byte("backup"), 0o644); err != nil {
		t.Fatalf("failed to write backup archive: %v", err)
	}

	service := &LxcService{cfg: &config.Config{LxcBackupDir: backupDir}}
	if err := service.DeleteBackupArchive(archiveName); err != nil {
		t.Fatalf("DeleteBackupArchive returned error: %v", err)
	}

	if _, err := os.Stat(archivePath); !os.IsNotExist(err) {
		t.Fatalf("expected archive to be deleted, stat err: %v", err)
	}
}

func TestDeleteBackupArchiveRejectsTraversal(t *testing.T) {
	service := &LxcService{cfg: &config.Config{LxcBackupDir: t.TempDir()}}

	if err := service.DeleteBackupArchive("../outside.tar.gz"); err == nil {
		t.Fatal("expected traversal archive name to be rejected")
	}
}

func TestParseByteSize(t *testing.T) {
	cases := map[string]int64{
		"1024":     1024,
		"1 KB":     1024,
		"1KiB":     1024,
		"1.5 MB":   1572864,
		"2 GiB":    2147483648,
		"0 B":      0,
		"bad data": 0,
	}

	for input, expected := range cases {
		if got := parseByteSize(input); got != expected {
			t.Fatalf("parseByteSize(%q) = %d, expected %d", input, got, expected)
		}
	}
}

func TestContainerNameFromCgroup(t *testing.T) {
	cases := map[string]string{
		"0::/lxc.payload.instance1/init.scope":                                  "instance1",
		"0::/system.slice/lxc.payload.web-01.scope":                             "web-01",
		"12:memory:/machine.slice/lxc.payload.instance_YueQ.scope/some/process": "instance_YueQ",
	}

	for input, expected := range cases {
		got, ok := containerNameFromCgroup(input)
		if !ok {
			t.Fatalf("expected cgroup to match container name for %q", input)
		}
		if got != expected {
			t.Fatalf("containerNameFromCgroup(%q) = %q, expected %q", input, got, expected)
		}
	}
}

func TestFilterMetricsSinceCarriesPreviousValue(t *testing.T) {
	base := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	samples := []lxcMetricsSample{
		{
			LxcMetricsPoint: models.LxcMetricsPoint{
				Timestamp:   base,
				MemoryBytes: 1024,
			},
		},
	}

	points := filterMetricsSince(samples, base.Add(time.Hour))
	if len(points) != 2 {
		t.Fatalf("expected carried point plus tail point, got %d points", len(points))
	}
	if !points[0].Timestamp.Equal(base.Add(time.Hour)) {
		t.Fatalf("expected carried timestamp to match range start, got %s", points[0].Timestamp)
	}
	if points[0].MemoryBytes != 1024 {
		t.Fatalf("expected carried memory value, got %d", points[0].MemoryBytes)
	}
	if !points[1].Timestamp.After(points[0].Timestamp) {
		t.Fatalf("expected tail point to extend to now, got %s after %s", points[1].Timestamp, points[0].Timestamp)
	}
	if points[1].MemoryBytes != 1024 {
		t.Fatalf("expected tail point to keep memory value, got %d", points[1].MemoryBytes)
	}
}

func TestSameGPUMemorySample(t *testing.T) {
	a := gpuMemorySample{
		Owners: map[string]gpuOwnerMemory{
			"container:web": {
				OwnerType:     "container",
				ContainerName: "web",
				Label:         "web",
				UsedMemoryMiB: 2048,
			},
		},
	}
	b := gpuMemorySample{
		Owners: map[string]gpuOwnerMemory{
			"container:web": {
				OwnerType:     "container",
				ContainerName: "web",
				Label:         "web",
				UsedMemoryMiB: 2048,
			},
		},
	}

	if !sameGPUMemorySample(a, b) {
		t.Fatal("expected equivalent GPU memory samples to be equal")
	}

	b.Owners["container:web"] = gpuOwnerMemory{
		OwnerType:     "container",
		ContainerName: "web",
		Label:         "web",
		UsedMemoryMiB: 4096,
	}
	if sameGPUMemorySample(a, b) {
		t.Fatal("expected changed GPU memory sample to be different")
	}
}

func TestStaleImportZFSDataset(t *testing.T) {
	output := "Failed to run zfs create -o mountpoint=legacy -o canmount=noauto new-pool/containers/test: exit status 1 (cannot create 'new-pool/containers/test': dataset already exists)"

	dataset, ok := staleImportZFSDataset(output, "test")
	if !ok {
		t.Fatal("expected ZFS dataset residue to be detected")
	}
	if dataset != "new-pool/containers/test" {
		t.Fatalf("unexpected dataset: %s", dataset)
	}
}

func TestStaleImportZFSDatasetWithoutQuotes(t *testing.T) {
	output := "Failed to run zfs create -o mountpoint=legacy -o canmount=noauto new-pool/containers/test: exit status 1 (cannot create new-pool/containers/test: dataset already exists)"

	dataset, ok := staleImportZFSDataset(output, "test")
	if !ok {
		t.Fatal("expected ZFS dataset residue to be detected")
	}
	if dataset != "new-pool/containers/test" {
		t.Fatalf("unexpected dataset: %s", dataset)
	}
}

func TestStaleImportZFSDatasetRejectsDifferentTarget(t *testing.T) {
	output := "cannot create new-pool/containers/prod: dataset already exists"

	if dataset, ok := staleImportZFSDataset(output, "test"); ok {
		t.Fatalf("unexpected dataset match: %s", dataset)
	}
}

func TestMissingImportZFSDataset(t *testing.T) {
	output := "Failed to run zfs create -o mountpoint=legacy -o canmount=noauto new-pool/containers/admin: exit status 1 (cannot open 'new-pool/containers/admin': dataset does not exist)"

	dataset, ok := missingImportZFSDataset(output, "admin")
	if !ok {
		t.Fatal("expected missing ZFS dataset to be detected")
	}
	if dataset != "new-pool/containers/admin" {
		t.Fatalf("unexpected dataset: %s", dataset)
	}
}

func TestStaleImportVolumeExists(t *testing.T) {
	cases := []string{
		"Cannot restore volume, already exists on target",
		"Cannot restore volume already exists on target",
		"volume already exists",
	}

	for _, output := range cases {
		if !staleImportVolumeExists(output) {
			t.Fatalf("expected volume residue to be detected for: %s", output)
		}
	}
}

func TestStaleImportMountPath(t *testing.T) {
	output := "Failed to create mount directory /var/snap/lxd/common/lxd/storage-pools/default/containers/test: mkdir /var/snap/lxd/common/lxd/storage-pools/default/containers/test: file exists"

	stalePath, ok := staleImportMountPath(output, "test")
	if !ok {
		t.Fatal("expected mount path residue to be detected")
	}
	if stalePath != "/var/snap/lxd/common/lxd/storage-pools/default/containers/test" {
		t.Fatalf("unexpected path: %s", stalePath)
	}
}
