package service

import (
	"encoding/json"
	"testing"
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
