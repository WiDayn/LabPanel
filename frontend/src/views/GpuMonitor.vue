<template>
  <div class="min-h-screen bg-gray-50">
    <AppHeader />

    <main class="mx-auto max-w-7xl space-y-6 px-4 py-6 sm:px-6 lg:px-8">
      <div class="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-3 gap-y-1">
        <div class="flex flex-wrap items-center gap-x-2 gap-y-1">
          <h2 class="text-xl font-semibold text-gray-900">显卡监控</h2>
          <span v-if="status.available" class="rounded border border-gray-200 bg-white px-1.5 py-0.5 font-mono text-xs text-gray-600">nvidia-smi</span>
        </div>
        <Button variant="outline" size="sm" :disabled="loading" @click="loadGpuStatus">
          {{ loading ? '刷新中…' : '刷新' }}
        </Button>
        <dl v-if="status.driverVersion || status.cudaSupportedVersion" aria-label="GPU 软件版本"
          class="col-span-full flex flex-wrap gap-x-4 gap-y-1 text-xs text-gray-600">
          <div class="flex items-baseline gap-1.5">
            <dt class="whitespace-nowrap">NVIDIA 驱动</dt>
            <dd class="font-mono font-medium tabular-nums text-gray-800">{{ status.driverVersion || 'N/A' }}</dd>
          </div>
          <div class="flex items-baseline gap-1.5">
            <dt class="whitespace-nowrap">CUDA 支持上限</dt>
            <dd class="font-mono font-medium tabular-nums text-gray-800">{{ status.cudaSupportedVersion || 'N/A' }}</dd>
          </div>
        </dl>
        <p v-if="preview" class="col-span-full text-xs text-gray-600">{{ appVersion }} 预览 · 模拟数据<template v-if="status.updatedAt"> · 采样于 {{ formatTime(status.updatedAt) }}</template></p>
      </div>

      <nav class="flex gap-5 border-b border-gray-200" aria-label="显卡监控视图">
        <RouterLink v-for="tab in monitorTabs" :key="tab.id" :to="tabLocation(tab.id)" custom v-slot="{ href, navigate }">
          <a :href="href" :aria-current="activeTab === tab.id ? 'page' : undefined" @click="navigate"
            class="-mb-px inline-flex h-9 items-center border-b-2 px-1 text-sm font-medium focus-visible:outline focus-visible:outline-2 focus-visible:outline-blue-500"
            :class="activeTab === tab.id ? 'border-blue-600 text-blue-700' : 'border-transparent text-gray-600 hover:text-gray-900'">
            {{ tab.label }}
          </a>
        </RouterLink>
      </nav>

      <div v-if="error" role="alert" class="rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">{{ error }}</div>
      <div v-if="!loaded && loading" role="status" class="rounded-lg border border-gray-200 bg-white p-6 text-sm text-gray-500">正在加载显卡数据…</div>
      <div v-else-if="loaded && !gpuList.length" class="rounded-lg border border-gray-200 bg-white p-6">
        <h3 class="text-base font-semibold text-gray-900">未检测到可用显卡</h3>
        <p class="mt-1 text-sm text-gray-600">{{ status.message || '当前宿主机没有可用的 NVIDIA 显卡。' }}</p>
      </div>

      <template v-else-if="gpuList.length">
        <GpuInterconnectPanel v-show="activeTab === 'interconnect'" :gpus="gpuList" :topology="status.topology" />
        <GpuStatusTable v-show="activeTab === 'monitor'" :gpus="gpuList" :selected-uuid="selectedUuid" @select="toggleGpu" />

        <section v-show="activeTab === 'monitor'" class="overflow-hidden rounded-lg border border-gray-200 bg-white">
          <div class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-200 px-4 py-3 sm:px-5">
            <div class="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-2">
              <h3 class="text-sm font-semibold text-gray-900">趋势</h3>
              <div class="inline-flex flex-wrap gap-0.5 rounded-md bg-gray-100 p-0.5" role="group" aria-label="趋势指标">
                <button v-for="metric in availableTrendMetrics" :key="metric.id" type="button"
                  class="h-7 whitespace-nowrap rounded px-2.5 text-xs font-medium focus-visible:outline focus-visible:outline-2 focus-visible:outline-blue-500"
                  :class="metricKey === metric.id ? 'bg-white text-blue-700 shadow-sm' : 'text-gray-600 hover:text-gray-900'"
                  :aria-pressed="metricKey === metric.id" @click="metricKey = metric.id">{{ metric.label }}</button>
              </div>
            </div>
            <div class="inline-flex items-center gap-1 rounded-md border border-gray-300 bg-white p-1" role="group" aria-label="趋势时间范围">
              <button v-for="option in rangeOptions" :key="option.value" type="button"
                :aria-pressed="rangeKey === option.value"
                class="h-7 min-w-11 rounded px-2 text-xs font-medium focus-visible:outline focus-visible:outline-2 focus-visible:outline-blue-500"
                :class="rangeKey === option.value ? 'bg-blue-600 text-white' : 'text-gray-600 hover:bg-gray-100'"
                @click="setRange(option.value)">{{ option.label }}</button>
            </div>
          </div>
          <div class="p-4 sm:px-5">
            <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
              <div class="flex flex-wrap items-center gap-1.5" role="group" aria-label="趋势 GPU 筛选">
                <ContainerTag v-for="gpu in gpuList" :key="gpu.uuid" :label="`GPU ${gpu.index}`"
                  :tone="gpu.uuid === selectedUuid ? 'border-blue-200 bg-blue-50 text-blue-700' : 'border-gray-200 bg-gray-50 text-gray-700'"
                  class="min-w-16 justify-center"
                  :selectable="true" :selected="gpu.uuid === selectedUuid" @toggle="toggleGpu(gpu.uuid)" />
              </div>
              <div class="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-gray-500">
                <template v-if="selectedGpu">
                  <span>{{ selectedGpu.name }}</span>
                  <details :key="selectedUuid">
                    <summary class="cursor-pointer text-gray-600 hover:text-blue-700">UUID</summary>
                    <code class="mt-1 block break-all font-mono">{{ selectedGpu.uuid }}</code>
                  </details>
                </template>
                <span v-else>全部 GPU</span>
              </div>
            </div>
            <TimeSeriesChart :key="trendKey" :label="trendLabel"
              :series="visibleSeries" :y-axis-max="trendAxisMax" :y-axis-interval="trendAxisMax ? trendAxisMax / 4 : undefined"
              :format-value="activeMetric.format" :reference-lines="temperatureReferenceLines"
              :range="rangeKey" :empty-text="emptyTrend" />
            <div v-if="temperatureTargets.length" class="mt-3 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-gray-600" role="group" aria-label="温控目标">
              <span class="font-medium text-gray-700">温控目标</span>
              <span v-for="target in temperatureTargets" :key="target.value" class="inline-flex items-center gap-1.5">
                <span class="w-3 border-t-2 border-dashed border-amber-700" aria-hidden="true" />
                <span class="tabular-nums">{{ formatGpuTemperature(target.value) }}</span>
                <span v-if="!selectedGpu">· GPU {{ target.indices.join('、') }}</span>
              </span>
            </div>
            <div v-if="series.length" class="mt-3 flex flex-wrap gap-1.5" role="group" aria-label="趋势曲线">
              <button v-for="item in series" :key="item.id" type="button"
                class="flex min-w-0 items-center gap-1.5 rounded border px-2 py-1 text-xs focus-visible:outline focus-visible:outline-2 focus-visible:outline-blue-500"
                :class="isSeriesHidden(item.id) ? 'border-transparent text-gray-500 line-through' : 'border-gray-200 text-gray-700 hover:bg-gray-50'"
                :aria-pressed="!isSeriesHidden(item.id)" @click="toggleSeries(item.id)">
                <span v-if="item.lineType === 'dashed'" class="w-3 shrink-0 border-t-2 border-dashed" :style="{ borderColor: item.color }" aria-hidden="true" />
                <span v-else class="h-2 w-2 shrink-0 rounded-full" :style="{ backgroundColor: item.color }" aria-hidden="true" />
                <span class="break-all font-medium">{{ item.name }}</span>
                <span class="shrink-0 tabular-nums">{{ item.latest }}</span>
              </button>
            </div>
          </div>
        </section>

        <section v-show="activeTab === 'monitor'" class="overflow-hidden rounded-lg border border-gray-200 bg-white">
          <div class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-200 px-4 py-3 sm:px-5">
            <div class="flex flex-wrap items-baseline gap-x-2 gap-y-1">
              <h3 class="text-sm font-semibold text-gray-900">显卡进程</h3>
              <span class="text-xs text-gray-500">{{ allProcesses.length }} 个</span>
            </div>
            <button type="button" class="h-8 rounded-md border border-gray-300 bg-white px-2.5 text-xs text-gray-700 hover:bg-gray-50 focus-visible:outline focus-visible:outline-2 focus-visible:outline-blue-500"
              :aria-expanded="!processCollapsed" aria-controls="gpu-process-list" @click="processCollapsed = !processCollapsed">{{ processCollapsed ? '展开' : '收起' }}</button>
          </div>
          <div v-show="!processCollapsed" id="gpu-process-list" class="p-4 sm:px-5">
            <div v-if="allProcesses.length" class="panel-table-frame process-table-frame">
              <table class="panel-table process-table w-full table-fixed border-collapse text-sm">
                <thead>
                  <tr class="border-b text-left text-xs text-gray-500">
                    <SortableHeader :sort="processTable" column="gpuIndex" label="GPU" class="process-gpu px-2 py-2 font-medium" />
                    <SortableHeader :sort="processTable" column="pid" label="PID" class="process-pid px-2 py-2 font-medium" />
                    <SortableHeader :sort="processTable" column="processName" label="进程" class="px-2 py-2 font-medium" />
                    <SortableHeader :sort="processTable" column="user" label="用户" class="process-user px-2 py-2 font-medium" />
                    <SortableHeader :sort="processTable" column="group" label="分组" class="process-group px-2 py-2 font-medium" />
                    <SortableHeader :sort="processTable" column="owner" label="归属" class="process-owner px-2 py-2 font-medium" />
                    <SortableHeader :sort="processTable" column="usedMemoryMiB" label="显存" numeric class="process-vram px-2 py-2 text-right font-medium" />
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="process in processTable.rows" :key="`${process.gpuUuid}-${process.pid}-${process.processName}`" class="border-b last:border-b-0">
                    <td class="whitespace-nowrap px-2 py-2 tabular-nums text-gray-700">GPU {{ process.gpuIndex }}</td>
                    <td class="whitespace-nowrap px-2 py-2 font-mono text-xs text-gray-700">{{ process.pid }}</td>
                    <td class="truncate px-2 py-2 text-gray-900" :title="process.processName">{{ process.processName }}</td>
                    <td class="truncate px-2 py-2 text-gray-700" :title="process.user || '-'">{{ process.user || '-' }}</td>
                    <td class="px-2 py-2">
                      <div class="flex flex-wrap gap-1">
                        <span
                          v-for="group in visibleGroups(process.groups)"
                          :key="group.id"
                          :class="groupBadgeClass(group)"
                          :title="group.name"
                        >
                          <span class="truncate">{{ group.name }}</span>
                        </span>
                        <span
                          v-if="hasMoreGroups(process.groups)"
                          class="inline-flex items-center rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-500 ring-1 ring-gray-200"
                          :title="process.groups.map((group) => group.name).join('、')"
                        >
                          ...
                        </span>
                        <span v-if="!process.groups?.length" class="text-xs text-gray-400">未分组</span>
                      </div>
                    </td>
                    <td class="px-2 py-2">
                      <span
                        :class="ownerBadgeClass(process)"
                        :title="ownerLabel(process)"
                      >
                        <span class="truncate">{{ ownerLabel(process) }}</span>
                      </span>
                    </td>
                    <td class="whitespace-nowrap px-2 py-2 text-right tabular-nums text-gray-900">{{ process.usedMemoryMiB }} MiB</td>
                  </tr>
                </tbody>
              </table>
            </div>
            <div v-else class="py-3 text-sm text-gray-600">
              当前没有进程占用 GPU。
            </div>
          </div>
        </section>
      </template>
    </main>
  </div>
</template>

<script setup>
import SortableHeader from '@/components/ui/SortableHeader.vue'
import { useTableSort } from '@/utils/tableSort'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { version as appVersion } from '../../package.json'
import api from '@/utils/api'
import Button from '@/components/ui/Button.vue'
import ContainerTag from '@/components/ui/ContainerTag.vue'
import AppHeader from '@/components/AppHeader.vue'
import GpuStatusTable from '@/components/monitor/GpuStatusTable.vue'
import GpuInterconnectPanel from '@/components/monitor/GpuInterconnectPanel.vue'
import TimeSeriesChart from '@/components/monitor/TimeSeriesChart.vue'
import { formatGpuCapacity as formatCapacity, formatGpuTemperature, readFanPercent, gpuTrendMetrics, gpuTrendLimit, gpuTrendColor, gpuTotalPoints } from '@/utils/gpuTrends'

const totalSeriesColor = '#111827'
const hostSeriesColor = '#6b7280'
const preview = import.meta.env.MODE === 'gpu-preview'
const route = useRoute()
const monitorTabs = [
  { id: 'monitor', label: '监控' },
  { id: 'interconnect', label: '互连' },
]
const activeTab = computed(() => route.query.tab === 'interconnect' ? 'interconnect' : 'monitor')
const tabLocation = tab => ({ query: { ...route.query, tab: tab === 'monitor' ? undefined : tab } })
const ownerChartColorVariants = {
  blue: ['#60a5fa', '#3b82f6', '#2563eb', '#1d4ed8'],
  emerald: ['#34d399', '#10b981', '#059669', '#047857'],
  amber: ['#fbbf24', '#f59e0b', '#d97706', '#b45309'],
  violet: ['#a78bfa', '#8b5cf6', '#7c3aed', '#6d28d9'],
  rose: ['#fb7185', '#f43f5e', '#e11d48', '#be123c'],
  cyan: ['#22d3ee', '#06b6d4', '#0891b2', '#0e7490'],
  lime: ['#a3e635', '#84cc16', '#65a30d', '#4d7c0f'],
  orange: ['#fb923c', '#f97316', '#ea580c', '#c2410c'],
  slate: ['#94a3b8', '#64748b', '#475569', '#334155'],
  indigo: ['#818cf8', '#6366f1', '#4f46e5', '#4338ca'],
}

function memoryChartSeries(gpu) {
  const groupColors = new Map()
  for (const process of gpu.processes || []) {
    if (process.ownerType === 'container' && process.containerName && !groupColors.has(process.containerName)) {
      groupColors.set(process.containerName, process.groups?.[0]?.color)
    }
  }
  return (gpu.memorySeries || []).map(({ ownerType, containerName, label, points }) => {
    let color = ownerType === 'total' ? totalSeriesColor : hostSeriesColor
    if (ownerType === 'container') {
      const variants = ownerChartColorVariants[groupColors.get(containerName)] || ownerChartColorVariants.blue
      color = variants[stableColorIndex(containerName || label, variants.length)]
    }
    const latest = points?.at(-1)?.usedMemoryMiB
    return {
      id: `${ownerType}:${containerName || label}`, name: ownerType === 'total' ? '进程合计' : label,
      color, points: points || [], valueKey: 'usedMemoryMiB',
      latest: Number.isFinite(latest) ? formatCapacity(latest) : '-',
    }
  })
}

const loading = ref(false)
const error = ref('')
const rangeKey = ref('1h')
const status = ref({
  available: false,
  message: '',
  updatedAt: '',
  gpus: [],
})
const loaded = ref(false)
const selectedUuid = ref('')
const metricKey = ref('memory')
const processCollapsed = ref(false)
const hiddenByView = ref(new Map())
const gpuList = computed(() => status.value.available ? status.value.gpus || [] : [])
const selectedGpu = computed(() => gpuList.value.find(gpu => gpu.uuid === selectedUuid.value))
const hasFanHistory = gpu => gpu.telemetry?.some(point => readFanPercent(point.fanPercent) !== null)
const showFanStatus = computed(() => gpuList.value.some(gpu => readFanPercent(gpu.fanPercent) !== null))
const availableTrendMetrics = computed(() => showFanStatus.value || gpuList.value.some(hasFanHistory)
  ? gpuTrendMetrics : gpuTrendMetrics.filter(metric => metric.id !== 'fan'))
watch(availableTrendMetrics, metrics => {
  if (!metrics.some(metric => metric.id === metricKey.value)) metricKey.value = 'memory'
})
watch(gpuList, gpus => {
  if (selectedUuid.value && !gpus.some(gpu => gpu.uuid === selectedUuid.value)) selectedUuid.value = ''
})
function toggleGpu(uuid) {
  selectedUuid.value = selectedUuid.value === uuid ? '' : uuid
}

const activeMetric = computed(() => gpuTrendMetrics.find(metric => metric.id === metricKey.value))
const scopeGpus = computed(() => selectedGpu.value ? [selectedGpu.value] : gpuList.value)
const useMemoryOwners = computed(() => metricKey.value === 'memory' &&
  Boolean(selectedGpu.value?.memorySeries?.some(({ points }) => points?.some(point => Number.isFinite(point.usedMemoryMiB)))))
const trendKey = computed(() => `${metricKey.value}:${selectedUuid.value}:${Boolean(useMemoryOwners.value)}`)
const trendLabel = computed(() => `${selectedGpu.value ? 'GPU ' + selectedGpu.value.index : '全部 GPU'} ${activeMetric.value.label}趋势`)
const series = computed(() => {
  const metric = activeMetric.value
  if (useMemoryOwners.value) return memoryChartSeries(selectedGpu.value)

  const isFan = metric.id === 'fan'
  const gpus = isFan ? scopeGpus.value.filter(hasFanHistory) : scopeGpus.value
  const devices = gpus.map(gpu => ({
    id: gpu.uuid, name: `GPU ${gpu.index}`, color: gpuTrendColor(gpu.index),
    points: isFan ? gpu.telemetry.map(point => ({ ...point, fanPercent: readFanPercent(point.fanPercent) })) : gpu.telemetry || [],
    valueKey: metric.valueKey,
  }))
  if (metric.sum && devices.length > 1) {
    devices.unshift({
      id: 'total', name: '合计', color: totalSeriesColor, lineType: 'dashed', valueKey: metric.valueKey,
      points: gpuTotalPoints(devices, metric.valueKey),
    })
  }
  return devices.map(item => {
    const value = item.points.at(-1)?.[metric.valueKey]
    return { ...item, latest: Number.isFinite(value) ? metric.format(value) : '-' }
  })
})
const trendAxisMax = computed(() => {
  const metric = activeMetric.value
  const limit = gpuTrendLimit(metric, scopeGpus.value)
  if (limit === undefined || metric.limit !== undefined) return limit

  let max = Math.max(limit, ...temperatureReferenceLines.value)
  for (const { points, valueKey } of series.value) {
    for (const point of points) {
      const value = point[valueKey]
      if (Number.isFinite(value) && value > max) max = value
    }
  }
  return max
})
const isSeriesHidden = id => hiddenByView.value.get(trendKey.value)?.has(id) ?? false
const visibleSeries = computed(() => series.value.filter(item => !isSeriesHidden(item.id)))
const temperatureTargets = computed(() => {
  if (metricKey.value !== 'temperature') return []
  const visibleIds = new Set(visibleSeries.value.filter(({ points }) =>
    points.some(point => Number.isFinite(point.temperature))).map(item => item.id))
  const targets = new Map()
  for (const gpu of scopeGpus.value) {
    const value = gpu.temperatureTargetCelsius
    if (!visibleIds.has(gpu.uuid) || !Number.isFinite(value) || value <= 0) continue
    if (!targets.has(value)) targets.set(value, { value, indices: [] })
    targets.get(value).indices.push(gpu.index)
  }
  return [...targets.values()].sort((a, b) => a.value - b.value)
})
const temperatureReferenceLines = computed(() => temperatureTargets.value.map(target => target.value))
const emptyTrend = computed(() => series.value.length && !visibleSeries.value.length
  ? '当前未选择曲线' : `暂无${activeMetric.value.label}历史采样`)

function toggleSeries(id) {
  const key = trendKey.value
  if (!hiddenByView.value.has(key)) hiddenByView.value.set(key, new Set())
  const hidden = hiddenByView.value.get(key)
  if (hidden.has(id)) hidden.delete(id)
  else hidden.add(id)
}
let requestId = 0
let refreshTimer = null

const rangeOptions = [
  { label: '1h', value: '1h' },
  { label: '24h', value: '24h' },
  { label: '7d', value: '7d' },
  { label: '30d', value: '30d' },
]

const allProcesses = computed(() =>
  gpuList.value
    .flatMap((gpu) => (gpu.processes || []).map((process) => ({
      ...process, gpuUuid: gpu.uuid, gpuIndex: gpu.index,
    })))
    .sort((a, b) => b.usedMemoryMiB - a.usedMemoryMiB)
)

const loadGpuStatus = async () => {
  const id = ++requestId
  loading.value = true
  error.value = ''
  try {
    const response = await api.get('/gpu/monitor', {
      params: { range: rangeKey.value },
    })
    if (id === requestId) {
      status.value = response.data || status.value
      loaded.value = true
    }
  } catch (err) {
    if (id === requestId) error.value = err.response?.data?.error || '加载显卡监控失败'
  } finally {
    if (id === requestId) loading.value = false
  }
}

const setRange = async (value) => {
  if (rangeKey.value === value) return
  rangeKey.value = value
  await loadGpuStatus()
}

const ownerLabel = (process) => {
  if (process.ownerType === 'container') {
    return process.containerName || '容器'
  }
  return '宿主机'
}

const groupColorClasses = {
  blue: 'bg-blue-50 text-blue-700 ring-blue-200',
  emerald: 'bg-emerald-50 text-emerald-700 ring-emerald-200',
  amber: 'bg-amber-50 text-amber-700 ring-amber-200',
  violet: 'bg-violet-50 text-violet-700 ring-violet-200',
  rose: 'bg-rose-50 text-rose-700 ring-rose-200',
  cyan: 'bg-cyan-50 text-cyan-700 ring-cyan-200',
  lime: 'bg-lime-50 text-lime-700 ring-lime-200',
  orange: 'bg-orange-50 text-orange-700 ring-orange-200',
  slate: 'bg-slate-100 text-slate-700 ring-slate-200',
  indigo: 'bg-indigo-50 text-indigo-700 ring-indigo-200',
}

const ownerColorClassVariants = {
  blue: [
    'bg-blue-50 text-blue-800 ring-blue-300',
    'bg-blue-100 text-blue-800 ring-blue-300',
    'bg-blue-100 text-blue-900 ring-blue-400',
    'bg-blue-200 text-blue-900 ring-blue-300',
  ],
  emerald: [
    'bg-emerald-50 text-emerald-800 ring-emerald-300',
    'bg-emerald-100 text-emerald-800 ring-emerald-300',
    'bg-emerald-100 text-emerald-900 ring-emerald-400',
    'bg-emerald-200 text-emerald-900 ring-emerald-300',
  ],
  amber: [
    'bg-amber-50 text-amber-800 ring-amber-300',
    'bg-amber-100 text-amber-800 ring-amber-300',
    'bg-amber-100 text-amber-900 ring-amber-400',
    'bg-amber-200 text-amber-900 ring-amber-300',
  ],
  violet: [
    'bg-violet-50 text-violet-800 ring-violet-300',
    'bg-violet-100 text-violet-800 ring-violet-300',
    'bg-violet-100 text-violet-900 ring-violet-400',
    'bg-violet-200 text-violet-900 ring-violet-300',
  ],
  rose: [
    'bg-rose-50 text-rose-800 ring-rose-300',
    'bg-rose-100 text-rose-800 ring-rose-300',
    'bg-rose-100 text-rose-900 ring-rose-400',
    'bg-rose-200 text-rose-900 ring-rose-300',
  ],
  cyan: [
    'bg-cyan-50 text-cyan-800 ring-cyan-300',
    'bg-cyan-100 text-cyan-800 ring-cyan-300',
    'bg-cyan-100 text-cyan-900 ring-cyan-400',
    'bg-cyan-200 text-cyan-900 ring-cyan-300',
  ],
  lime: [
    'bg-lime-50 text-lime-800 ring-lime-300',
    'bg-lime-100 text-lime-800 ring-lime-300',
    'bg-lime-100 text-lime-900 ring-lime-400',
    'bg-lime-200 text-lime-900 ring-lime-300',
  ],
  orange: [
    'bg-orange-50 text-orange-800 ring-orange-300',
    'bg-orange-100 text-orange-800 ring-orange-300',
    'bg-orange-100 text-orange-900 ring-orange-400',
    'bg-orange-200 text-orange-900 ring-orange-300',
  ],
  slate: [
    'bg-slate-100 text-slate-800 ring-slate-300',
    'bg-slate-200 text-slate-800 ring-slate-300',
    'bg-slate-200 text-slate-900 ring-slate-400',
    'bg-slate-300 text-slate-900 ring-slate-300',
  ],
  indigo: [
    'bg-indigo-50 text-indigo-800 ring-indigo-300',
    'bg-indigo-100 text-indigo-800 ring-indigo-300',
    'bg-indigo-100 text-indigo-900 ring-indigo-400',
    'bg-indigo-200 text-indigo-900 ring-indigo-300',
  ],
}

const visibleGroups = (items) => (Array.isArray(items) ? items.slice(0, 2) : [])

const hasMoreGroups = (items) => Array.isArray(items) && items.length > 2

const groupBadgeClass = (group) => [
  'inline-flex max-w-full items-center rounded-full px-2 py-0.5 text-xs font-medium ring-1',
  groupColorClasses[group?.color] || groupColorClasses.slate,
]

const stableColorIndex = (value, total) => {
  let hash = 0
  for (const char of String(value || '')) {
    hash = (hash * 31 + char.charCodeAt(0)) >>> 0
  }
  return total > 0 ? hash % total : 0
}

const ownerColorClass = (groupColor, containerName) => {
  const variants = ownerColorClassVariants[groupColor] || ownerColorClassVariants.blue
  return variants[stableColorIndex(containerName, variants.length)]
}

const ownerBadgeClass = (process) => {
  if (process.ownerType !== 'container') {
    return 'inline-flex max-w-full items-center truncate rounded px-2 py-1 text-xs font-medium ring-1 bg-gray-100 text-gray-700 ring-gray-200'
  }
  const groupColor = Array.isArray(process.groups) && process.groups.length ? process.groups[0]?.color : 'blue'
  return [
    'inline-flex max-w-full items-center truncate rounded px-2 py-1 text-xs font-medium ring-1',
    ownerColorClass(groupColor, process.containerName || process.user),
  ]
}

const formatTime = (value) => {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleString('zh-CN', { hour12: false })
}

onMounted(() => {
  loadGpuStatus()
  refreshTimer = window.setInterval(() => {
    if (!loading.value) loadGpuStatus()
  }, 5 * 1000)
})

onBeforeUnmount(() => {
  requestId++
  if (refreshTimer) {
    window.clearInterval(refreshTimer)
  }
})
const processTable = useTableSort(() => allProcesses.value, { group: row => row.groups?.[0]?.name, owner: row => ownerLabel(row) })
</script>
