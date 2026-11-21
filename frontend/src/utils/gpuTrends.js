export const formatGpuCapacity = value => value >= 1024 ? `${(value / 1024).toFixed(1)} GiB` : `${Math.round(value)} MiB`
export const formatGpuPercent = value => `${Math.round(value)}%`
export const formatGpuTemperature = value => `${Math.round(value)}°C`
export const readFanPercent = value => Number.isFinite(value) && value >= 0 && value <= 100 ? value : null

export const gpuTrendMetrics = [
  { id: 'memory', label: '显存', valueKey: 'usedMemoryMiB', limitKey: 'memoryTotalMiB', sum: true, format: formatGpuCapacity },
  { id: 'power', label: '功率', valueKey: 'powerWatts', limitKey: 'maxPowerWatts', sum: true, format: value => `${value.toFixed(1)} W` },
  { id: 'utilization', label: '负载', valueKey: 'utilization', limit: 100, format: formatGpuPercent },
  { id: 'temperature', label: '温度', valueKey: 'temperature', limitKey: 'maxTemperatureCelsius', format: formatGpuTemperature },
  { id: 'fan', label: '风扇', valueKey: 'fanPercent', limit: 100, format: formatGpuPercent },
]

export function gpuTrendLimit(metric, gpus) {
  if (metric.limit !== undefined) return metric.limit
  const limits = gpus.map(gpu => gpu[metric.limitKey])
  if (!limits.length || limits.some(limit => !Number.isFinite(limit) || limit <= 0)) return undefined
  return metric.sum ? limits.reduce((sum, limit) => sum + limit, 0) : Math.max(...limits)
}

const gpuColors = ['#2563eb', '#047857', '#b45309', '#7c3aed', '#be123c', '#0e7490', '#4d7c0f', '#9333ea']
export const gpuTrendColor = index => gpuColors[index] || `hsl(${(index * 137.5) % 360} 62% 36%)`

export function gpuTotalPoints(series, valueKey) {
  const byTime = new Map()
  for (const { points } of series) {
    // Compare the same millisecond across devices, including different ISO offsets.
    const samples = new Map()
    for (const sample of points) {
      const stamp = sample.timestamp ? Date.parse(sample.timestamp) : NaN
      if (Number.isFinite(stamp)) samples.set(stamp, sample[valueKey])
    }
    for (const [stamp, value] of samples) {
      const point = byTime.get(stamp) || { value: 0, count: 0 }
      if (Number.isFinite(value)) {
        point.value += value
        point.count++
      }
      byTime.set(stamp, point)
    }
  }
  return [...byTime].sort(([a], [b]) => a - b).map(([stamp, point]) => ({
    timestamp: new Date(stamp).toISOString(),
    [valueKey]: point.count === series.length ? point.value : null,
  }))
}
