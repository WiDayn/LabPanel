package service

import (
	"LabPanel/models"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

const (
	metricsHistoryLoadWindow   = 30 * 24 * time.Hour
	metricsWriteQueueSize      = 2048
	metricsWriteEnqueueTimeout = 2 * time.Second
)

type MetricsStore struct {
	db            *sql.DB
	retentionDays int
	writes        chan metricsWriteJob
}

type metricsWriteJob struct {
	name string
	run  func(*sql.DB) error
}

var (
	defaultMetricsStore *MetricsStore
	metricsStoreMu      sync.RWMutex
)

func InitMetricsStore(dbPath string, retentionDays int) error {
	dbPath = strings.TrimSpace(dbPath)
	if dbPath == "" {
		dbPath = "./data/metrics.db"
	}
	if retentionDays < 0 {
		retentionDays = 0
	}
	if dir := filepath.Dir(dbPath); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("创建监控数据目录失败: %w", err)
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return err
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(0)
	store := &MetricsStore{
		db:            db,
		retentionDays: retentionDays,
		writes:        make(chan metricsWriteJob, metricsWriteQueueSize),
	}
	if err := store.initSchema(); err != nil {
		db.Close()
		return err
	}
	go store.runWriter()

	metricsStoreMu.Lock()
	if defaultMetricsStore != nil {
		_ = defaultMetricsStore.db.Close()
	}
	defaultMetricsStore = store
	metricsStoreMu.Unlock()
	return nil
}

func (s *MetricsStore) runWriter() {
	for job := range s.writes {
		if err := job.run(s.db); err != nil {
			log.Printf("监控数据写入失败[%s]: %v", job.name, err)
		}
	}
}

func (s *MetricsStore) enqueueWrite(name string, run func(*sql.DB) error) error {
	if s == nil || run == nil {
		return nil
	}

	job := metricsWriteJob{name: name, run: run}
	timer := time.NewTimer(metricsWriteEnqueueTimeout)
	defer timer.Stop()

	select {
	case s.writes <- job:
		return nil
	case <-timer.C:
		err := fmt.Errorf("监控数据写入队列已满，放弃写入: %s", name)
		log.Print(err)
		return err
	}
}

func GetMetricsStore() *MetricsStore {
	metricsStoreMu.RLock()
	defer metricsStoreMu.RUnlock()
	return defaultMetricsStore
}

func StartMetricsCleanup() {
	store := GetMetricsStore()
	if store == nil || store.retentionDays == 0 {
		return
	}
	go func() {
		_ = store.PruneExpired()
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			_ = store.PruneExpired()
		}
	}()
}

func (s *MetricsStore) initSchema() error {
	statements := []string{
		`PRAGMA journal_mode=WAL`,
		`PRAGMA busy_timeout=5000`,
		`PRAGMA foreign_keys=ON`,
		`CREATE TABLE IF NOT EXISTS host_metric_points (
			timestamp_ns INTEGER PRIMARY KEY,
			cpu_percent REAL NOT NULL,
			cpu_mhz REAL NOT NULL,
			temperature_c REAL NOT NULL,
			memory_used_bytes INTEGER NOT NULL,
			memory_total_bytes INTEGER NOT NULL,
			network_rx_bps REAL NOT NULL,
			network_tx_bps REAL NOT NULL,
			network_rx_bytes INTEGER NOT NULL,
			network_tx_bytes INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS host_metric_raw_state (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			timestamp_ns INTEGER NOT NULL,
			boot_id TEXT NOT NULL DEFAULT '',
			cpu_idle INTEGER NOT NULL,
			cpu_total INTEGER NOT NULL,
			network_rx_bytes INTEGER NOT NULL,
			network_tx_bytes INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS lxc_metric_points (
			container_name TEXT NOT NULL,
			timestamp_ns INTEGER NOT NULL,
			state TEXT NOT NULL,
			cpu_percent REAL NOT NULL,
			memory_bytes INTEGER NOT NULL,
			processes INTEGER NOT NULL,
			network_rx_bps REAL NOT NULL,
			network_tx_bps REAL NOT NULL,
			disk_read_bps REAL NOT NULL,
			disk_write_bps REAL NOT NULL,
			disk_usage_bytes INTEGER NOT NULL,
			network_rx_bytes INTEGER NOT NULL,
			network_tx_bytes INTEGER NOT NULL,
			disk_read_bytes INTEGER NOT NULL,
			disk_write_bytes INTEGER NOT NULL,
			PRIMARY KEY (container_name, timestamp_ns)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_lxc_metric_points_time ON lxc_metric_points(timestamp_ns)`,
		`CREATE TABLE IF NOT EXISTS lxc_metric_raw_state (
			container_name TEXT PRIMARY KEY,
			state TEXT NOT NULL,
			timestamp_ns INTEGER NOT NULL,
			cpu_usage_ns INTEGER NOT NULL,
			network_rx_bytes INTEGER NOT NULL,
			network_tx_bytes INTEGER NOT NULL,
			disk_read_bytes INTEGER NOT NULL,
			disk_write_bytes INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS gpu_memory_samples (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			gpu_uuid TEXT NOT NULL,
			timestamp_ns INTEGER NOT NULL,
			UNIQUE (gpu_uuid, timestamp_ns)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_gpu_memory_samples_uuid_time ON gpu_memory_samples(gpu_uuid, timestamp_ns)`,
		`CREATE TABLE IF NOT EXISTS gpu_memory_sample_owners (
			sample_id INTEGER NOT NULL,
			owner_key TEXT NOT NULL,
			owner_type TEXT NOT NULL,
			container_name TEXT NOT NULL DEFAULT '',
			label TEXT NOT NULL,
			used_memory_mib INTEGER NOT NULL,
			PRIMARY KEY (sample_id, owner_key),
			FOREIGN KEY (sample_id) REFERENCES gpu_memory_samples(id) ON DELETE CASCADE
		)`,
	}
	for _, statement := range statements {
		if _, err := s.db.Exec(statement); err != nil {
			return err
		}
	}
	return nil
}

func (s *MetricsStore) SaveHostRaw(raw hostMetricsRaw) error {
	return s.enqueueWrite("host_raw", func(db *sql.DB) error {
		_, err := db.Exec(
			`INSERT INTO host_metric_raw_state
				(id, timestamp_ns, boot_id, cpu_idle, cpu_total, network_rx_bytes, network_tx_bytes)
			 VALUES (1, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT(id) DO UPDATE SET
				timestamp_ns=excluded.timestamp_ns,
				boot_id=excluded.boot_id,
				cpu_idle=excluded.cpu_idle,
				cpu_total=excluded.cpu_total,
				network_rx_bytes=excluded.network_rx_bytes,
				network_tx_bytes=excluded.network_tx_bytes`,
			timeToNano(raw.Timestamp),
			raw.BootID,
			uint64ToInt64(raw.CPUIdle),
			uint64ToInt64(raw.CPUTotal),
			raw.NetworkRxBytes,
			raw.NetworkTxBytes,
		)
		return err
	})
}

func (s *MetricsStore) LoadHostRaw() (hostMetricsRaw, bool, error) {
	var raw hostMetricsRaw
	var timestampNS int64
	var cpuIdle int64
	var cpuTotal int64
	err := s.db.QueryRow(
		`SELECT timestamp_ns, boot_id, cpu_idle, cpu_total, network_rx_bytes, network_tx_bytes
		 FROM host_metric_raw_state WHERE id = 1`,
	).Scan(&timestampNS, &raw.BootID, &cpuIdle, &cpuTotal, &raw.NetworkRxBytes, &raw.NetworkTxBytes)
	if err == sql.ErrNoRows {
		return raw, false, nil
	}
	if err != nil {
		return raw, false, err
	}
	raw.Timestamp = nanoToTime(timestampNS)
	raw.CPUIdle = uint64(cpuIdle)
	raw.CPUTotal = uint64(cpuTotal)
	return raw, true, nil
}

func (s *MetricsStore) InsertHostMetric(point models.HostMetricsPoint) error {
	return s.enqueueWrite("host_metric", func(db *sql.DB) error {
		_, err := db.Exec(
			`INSERT OR REPLACE INTO host_metric_points
				(timestamp_ns, cpu_percent, cpu_mhz, temperature_c, memory_used_bytes, memory_total_bytes,
				 network_rx_bps, network_tx_bps, network_rx_bytes, network_tx_bytes)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			timeToNano(point.Timestamp),
			point.CPUPercent,
			point.CPUMHz,
			point.TemperatureC,
			point.MemoryUsedBytes,
			point.MemoryTotalBytes,
			point.NetworkRxBps,
			point.NetworkTxBps,
			point.NetworkRxBytes,
			point.NetworkTxBytes,
		)
		return err
	})
}

func (s *MetricsStore) LoadHostMetricsSince(since time.Time) ([]models.HostMetricsPoint, error) {
	rows, err := s.db.Query(
		`SELECT timestamp_ns, cpu_percent, cpu_mhz, temperature_c, memory_used_bytes, memory_total_bytes,
		        network_rx_bps, network_tx_bps, network_rx_bytes, network_tx_bytes
		   FROM host_metric_points
		  WHERE timestamp_ns >= ?
		  ORDER BY timestamp_ns ASC`,
		timeToNano(since),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	points := []models.HostMetricsPoint{}
	for rows.Next() {
		var point models.HostMetricsPoint
		var timestampNS int64
		if err := rows.Scan(
			&timestampNS,
			&point.CPUPercent,
			&point.CPUMHz,
			&point.TemperatureC,
			&point.MemoryUsedBytes,
			&point.MemoryTotalBytes,
			&point.NetworkRxBps,
			&point.NetworkTxBps,
			&point.NetworkRxBytes,
			&point.NetworkTxBytes,
		); err != nil {
			return nil, err
		}
		point.Timestamp = nanoToTime(timestampNS)
		points = append(points, point)
	}
	return points, rows.Err()
}

func (s *MetricsStore) SaveLxcRaw(raw lxcMetricsRaw) error {
	return s.enqueueWrite("lxc_raw", func(db *sql.DB) error {
		_, err := db.Exec(
			`INSERT INTO lxc_metric_raw_state
				(container_name, state, timestamp_ns, cpu_usage_ns, network_rx_bytes, network_tx_bytes, disk_read_bytes, disk_write_bytes)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT(container_name) DO UPDATE SET
				state=excluded.state,
				timestamp_ns=excluded.timestamp_ns,
				cpu_usage_ns=excluded.cpu_usage_ns,
				network_rx_bytes=excluded.network_rx_bytes,
				network_tx_bytes=excluded.network_tx_bytes,
				disk_read_bytes=excluded.disk_read_bytes,
				disk_write_bytes=excluded.disk_write_bytes`,
			raw.Name,
			raw.State,
			timeToNano(raw.Timestamp),
			raw.CPUUsageNs,
			raw.NetworkRxBytes,
			raw.NetworkTxBytes,
			raw.DiskReadBytes,
			raw.DiskWriteBytes,
		)
		return err
	})
}

func (s *MetricsStore) LoadLxcRawStates() (map[string]lxcMetricsRaw, error) {
	rows, err := s.db.Query(
		`SELECT container_name, state, timestamp_ns, cpu_usage_ns, network_rx_bytes, network_tx_bytes, disk_read_bytes, disk_write_bytes
		   FROM lxc_metric_raw_state`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := map[string]lxcMetricsRaw{}
	for rows.Next() {
		var raw lxcMetricsRaw
		var timestampNS int64
		if err := rows.Scan(
			&raw.Name,
			&raw.State,
			&timestampNS,
			&raw.CPUUsageNs,
			&raw.NetworkRxBytes,
			&raw.NetworkTxBytes,
			&raw.DiskReadBytes,
			&raw.DiskWriteBytes,
		); err != nil {
			return nil, err
		}
		raw.Timestamp = nanoToTime(timestampNS)
		result[raw.Name] = raw
	}
	return result, rows.Err()
}

func (s *MetricsStore) DeleteLxcRaw(name string) error {
	return s.enqueueWrite("lxc_raw_delete", func(db *sql.DB) error {
		_, err := db.Exec(`DELETE FROM lxc_metric_raw_state WHERE container_name = ?`, name)
		return err
	})
}

func (s *MetricsStore) InsertLxcMetric(sample lxcMetricsSample) error {
	point := sample.LxcMetricsPoint
	return s.enqueueWrite("lxc_metric", func(db *sql.DB) error {
		_, err := db.Exec(
			`INSERT OR REPLACE INTO lxc_metric_points
				(container_name, timestamp_ns, state, cpu_percent, memory_bytes, processes,
				 network_rx_bps, network_tx_bps, disk_read_bps, disk_write_bps, disk_usage_bytes,
				 network_rx_bytes, network_tx_bytes, disk_read_bytes, disk_write_bytes)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			sample.Name,
			timeToNano(point.Timestamp),
			sample.State,
			point.CPUPercent,
			point.MemoryBytes,
			point.Processes,
			point.NetworkRxBps,
			point.NetworkTxBps,
			point.DiskReadBps,
			point.DiskWriteBps,
			point.DiskUsageBytes,
			point.NetworkRxBytes,
			point.NetworkTxBytes,
			point.DiskReadBytes,
			point.DiskWriteBytes,
		)
		return err
	})
}

func (s *MetricsStore) LoadLxcMetricsSince(since time.Time) (map[string][]lxcMetricsSample, error) {
	rows, err := s.db.Query(
		`SELECT container_name, timestamp_ns, state, cpu_percent, memory_bytes, processes,
		        network_rx_bps, network_tx_bps, disk_read_bps, disk_write_bps, disk_usage_bytes,
		        network_rx_bytes, network_tx_bytes, disk_read_bytes, disk_write_bytes
		   FROM lxc_metric_points
		  WHERE timestamp_ns >= ?
		  ORDER BY container_name ASC, timestamp_ns ASC`,
		timeToNano(since),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := map[string][]lxcMetricsSample{}
	for rows.Next() {
		var sample lxcMetricsSample
		var timestampNS int64
		point := &sample.LxcMetricsPoint
		if err := rows.Scan(
			&sample.Name,
			&timestampNS,
			&sample.State,
			&point.CPUPercent,
			&point.MemoryBytes,
			&point.Processes,
			&point.NetworkRxBps,
			&point.NetworkTxBps,
			&point.DiskReadBps,
			&point.DiskWriteBps,
			&point.DiskUsageBytes,
			&point.NetworkRxBytes,
			&point.NetworkTxBytes,
			&point.DiskReadBytes,
			&point.DiskWriteBytes,
		); err != nil {
			return nil, err
		}
		point.Timestamp = nanoToTime(timestampNS)
		result[sample.Name] = append(result[sample.Name], sample)
	}
	return result, rows.Err()
}

func (s *MetricsStore) InsertGPUMemorySample(gpuUUID string, sample gpuMemorySample) error {
	sample = cloneGPUMemorySample(sample)
	return s.enqueueWrite("gpu_memory_sample", func(db *sql.DB) error {
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback()

		timestampNS := timeToNano(sample.Timestamp)
		if _, err := tx.Exec(
			`INSERT OR IGNORE INTO gpu_memory_samples (gpu_uuid, timestamp_ns) VALUES (?, ?)`,
			gpuUUID,
			timestampNS,
		); err != nil {
			return err
		}

		var sampleID int64
		if err := tx.QueryRow(
			`SELECT id FROM gpu_memory_samples WHERE gpu_uuid = ? AND timestamp_ns = ?`,
			gpuUUID,
			timestampNS,
		).Scan(&sampleID); err != nil {
			return err
		}

		if _, err := tx.Exec(`DELETE FROM gpu_memory_sample_owners WHERE sample_id = ?`, sampleID); err != nil {
			return err
		}

		keys := make([]string, 0, len(sample.Owners))
		for key := range sample.Owners {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			owner := sample.Owners[key]
			if _, err := tx.Exec(
				`INSERT INTO gpu_memory_sample_owners
					(sample_id, owner_key, owner_type, container_name, label, used_memory_mib)
				 VALUES (?, ?, ?, ?, ?, ?)`,
				sampleID,
				key,
				owner.OwnerType,
				owner.ContainerName,
				owner.Label,
				owner.UsedMemoryMiB,
			); err != nil {
				return err
			}
		}

		return tx.Commit()
	})
}

func (s *MetricsStore) LoadGPUMemoryHistorySince(since time.Time) (map[string][]gpuMemorySample, error) {
	rows, err := s.db.Query(
		`SELECT s.gpu_uuid, s.timestamp_ns, o.owner_key, o.owner_type, o.container_name, o.label, o.used_memory_mib
		   FROM gpu_memory_samples s
		   LEFT JOIN gpu_memory_sample_owners o ON o.sample_id = s.id
		  WHERE s.timestamp_ns >= ?
		  ORDER BY s.gpu_uuid ASC, s.timestamp_ns ASC, o.owner_key ASC`,
		timeToNano(since),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := map[string][]gpuMemorySample{}
	indexByKey := map[string]int{}
	for rows.Next() {
		var gpuUUID string
		var timestampNS int64
		var ownerKey sql.NullString
		var ownerType sql.NullString
		var containerName sql.NullString
		var label sql.NullString
		var usedMemoryMiB sql.NullInt64
		if err := rows.Scan(&gpuUUID, &timestampNS, &ownerKey, &ownerType, &containerName, &label, &usedMemoryMiB); err != nil {
			return nil, err
		}

		sampleKey := fmt.Sprintf("%s/%d", gpuUUID, timestampNS)
		idx, ok := indexByKey[sampleKey]
		if !ok {
			result[gpuUUID] = append(result[gpuUUID], gpuMemorySample{
				Timestamp: nanoToTime(timestampNS),
				Owners:    map[string]gpuOwnerMemory{},
			})
			idx = len(result[gpuUUID]) - 1
			indexByKey[sampleKey] = idx
		}

		if ownerKey.Valid && ownerKey.String != "" {
			result[gpuUUID][idx].Owners[ownerKey.String] = gpuOwnerMemory{
				OwnerType:     ownerType.String,
				ContainerName: containerName.String,
				Label:         label.String,
				UsedMemoryMiB: usedMemoryMiB.Int64,
			}
		}
	}
	return result, rows.Err()
}

func (s *MetricsStore) PruneExpired() error {
	if s.retentionDays == 0 {
		return nil
	}
	cutoff := timeToNano(time.Now().Add(-time.Duration(s.retentionDays) * 24 * time.Hour))
	return s.enqueueWrite("metrics_prune", func(db *sql.DB) error {
		statements := []string{
			`DELETE FROM host_metric_points WHERE timestamp_ns < ?`,
			`DELETE FROM lxc_metric_points WHERE timestamp_ns < ?`,
			`DELETE FROM gpu_memory_samples WHERE timestamp_ns < ?`,
			`DELETE FROM host_metric_raw_state WHERE timestamp_ns < ?`,
			`DELETE FROM lxc_metric_raw_state WHERE timestamp_ns < ?`,
		}
		for _, statement := range statements {
			if _, err := db.Exec(statement, cutoff); err != nil {
				return err
			}
		}
		return nil
	})
}

func cloneGPUMemorySample(sample gpuMemorySample) gpuMemorySample {
	cloned := gpuMemorySample{
		Timestamp: sample.Timestamp,
		Owners:    make(map[string]gpuOwnerMemory, len(sample.Owners)),
	}
	for key, owner := range sample.Owners {
		cloned.Owners[key] = owner
	}
	return cloned
}

func timeToNano(value time.Time) int64 {
	return value.UnixNano()
}

func nanoToTime(value int64) time.Time {
	return time.Unix(0, value)
}

func uint64ToInt64(value uint64) int64 {
	if value > uint64(^uint64(0)>>1) {
		return int64(^uint64(0) >> 1)
	}
	return int64(value)
}
