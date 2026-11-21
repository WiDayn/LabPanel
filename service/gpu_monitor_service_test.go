package service

import (
	"LabPanel/models"
	"testing"
	"time"
)

func TestGPUMonitorResponseReusesSnapshot(t *testing.T) {
	s := NewGPUMonitorService()
	now := time.Now().Add(-10 * time.Second)
	const uuid = "GPU-test"
	s.snapshot = models.GPUMonitorResponse{
		Available: true,
		UpdatedAt: now,
		GPUs:      []models.GPUDevice{{UUID: uuid}},
	}
	s.history[uuid] = []gpuMemorySample{{
		Timestamp: now,
		Owners: map[string]gpuOwnerMemory{
			"host": {OwnerType: "host", Label: "host", UsedMemoryMiB: 64},
		},
	}}

	for _, rangeKey := range []string{"1h", "24h"} {
		got := s.Response(rangeKey)
		if !got.Available || !got.UpdatedAt.Equal(now) || got.IntervalSeconds != 5 || got.Range != rangeKey {
			t.Fatalf("unexpected snapshot response: %+v", got)
		}
		if len(got.GPUs) != 1 || len(got.GPUs[0].MemorySeries) != 2 {
			t.Fatalf("missing history on response: %+v", got.GPUs)
		}
	}
	if len(s.snapshot.GPUs[0].MemorySeries) != 0 {
		t.Fatal("attaching history modified the cached GPU snapshot")
	}
}

func TestGPUMonitorCachesUnavailableSnapshot(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	s := NewGPUMonitorService()
	first := s.Response("")
	second := s.Response("")
	if first.Available || first.UpdatedAt.IsZero() || !first.UpdatedAt.Equal(second.UpdatedAt) {
		t.Fatalf("unavailable sample should be cached: first=%+v second=%+v", first, second)
	}
}
