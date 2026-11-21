package service

import (
	"LabPanel/models"
	"context"
	"database/sql"
	"sort"
	"time"
)

type gpuTelemetryCache struct {
	expires time.Time
	points  map[string][]models.GPUDevicePoint
}

func (s *MetricsStore) insertGPUDeviceSamples(at time.Time, gpus []models.GPUDevice) error {
	return s.enqueueWrite("gpu_device_sample", func(db *sql.DB) error {
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback()
		for _, gpu := range gpus {
			if _, err := tx.Exec(
				`INSERT OR IGNORE INTO gpu_device_samples
				(timestamp_ns, gpu_uuid, used_memory_mib, power_watts, utilization, temperature, fan_percent)
				VALUES (?, ?, ?, ?, ?, ?, ?)`,
				at.UnixNano(), gpu.UUID, gpu.MemoryUsedMiB, gpu.PowerWatts, gpu.Utilization, gpu.Temperature, gpu.FanPercent,
			); err != nil {
				return err
			}
		}
		return tx.Commit()
	})
}

func (s *MetricsStore) loadGPUDeviceSamples(ctx context.Context, since time.Time, bucket time.Duration) (map[string][]models.GPUDevicePoint, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT gpu_uuid, (timestamp_ns / ?) * ? AS bucket_ns,
		       avg(used_memory_mib), avg(power_watts), avg(utilization), avg(temperature), avg(fan_percent)
		FROM gpu_device_samples
		WHERE timestamp_ns >= ?
		GROUP BY gpu_uuid, bucket_ns
		ORDER BY gpu_uuid, bucket_ns`, int64(bucket), int64(bucket), since.UnixNano())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]models.GPUDevicePoint)
	for rows.Next() {
		var uuid string
		var at int64
		var point models.GPUDevicePoint
		if err := rows.Scan(&uuid, &at, &point.UsedMemoryMiB, &point.PowerWatts, &point.Utilization, &point.Temperature, &point.FanPercent); err != nil {
			return nil, err
		}
		point.Timestamp = time.Unix(0, at).UTC()
		result[uuid] = append(result[uuid], point)
	}
	return result, rows.Err()
}

func (s *GPUMonitorService) addDeviceTelemetry(response *models.GPUMonitorResponse, since time.Time, window time.Duration) {
	bucket := (window + gpuMaxPoints*gpuSampleInterval - 1) / (gpuMaxPoints * gpuSampleInterval) * gpuSampleInterval
	cacheTTL := bucket
	if cacheTTL > 2*time.Minute {
		cacheTTL = 2 * time.Minute
	}
	var history map[string][]models.GPUDevicePoint
	if store := GetMetricsStore(); store != nil {
		s.telemetryMu.Lock()
		entry := s.telemetryCache[response.Range]
		if now := time.Now(); !now.Before(entry.expires) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			points, err := store.loadGPUDeviceSamples(ctx, since.Truncate(bucket), bucket)
			cancel()
			entry.expires = now.Add(cacheTTL)
			if err == nil {
				entry.points = points
			} else {
				entry.expires = now.Add(gpuSampleInterval)
			}
			s.telemetryCache[response.Range] = entry
		}
		history = entry.points
		s.telemetryMu.Unlock()
	}
	for i := range response.GPUs {
		gpu := &response.GPUs[i]
		points := history[gpu.UUID]
		from := sort.Search(len(points), func(i int) bool { return !points[i].Timestamp.Before(since.Truncate(bucket)) })
		to := sort.Search(len(points), func(i int) bool { return !points[i].Timestamp.Before(response.UpdatedAt.Truncate(bucket)) })
		if to < from {
			to = from
		}
		gpu.Telemetry = make([]models.GPUDevicePoint, 0, to-from+1)
		gpu.Telemetry = append(gpu.Telemetry, points[from:to]...)
		gpu.Telemetry = append(gpu.Telemetry, models.GPUDevicePoint{
			Timestamp: response.UpdatedAt, UsedMemoryMiB: gpu.MemoryUsedMiB, PowerWatts: gpu.PowerWatts,
			Utilization: gpu.Utilization, Temperature: gpu.Temperature, FanPercent: gpu.FanPercent,
		})
	}
}
