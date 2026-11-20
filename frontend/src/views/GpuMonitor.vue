<template>
  <div class="min-h-screen bg-gray-50">
    <AppHeader />

    <main class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      <div class="space-y-6">
        <div v-if="error" class="rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
          {{ error }}
        </div>

        <Card v-if="!status.available" class="p-6 border-amber-200 bg-amber-50">
          <h2 class="text-base font-semibold text-amber-900">nvidia-smi 无显卡</h2>
          <p class="mt-1 text-sm text-amber-800">{{ status.message || '当前宿主机未检测到可用 NVIDIA 显卡。' }}</p>
        </Card>

        <section v-else class="overflow-hidden rounded-lg border border-gray-200 bg-white shadow-sm">
          <div class="flex flex-col gap-3 px-5 py-4 sm:flex-row sm:items-start sm:justify-between">
            <div>
              <h2 class="text-base font-semibold text-gray-900">nvidia-smi</h2>
              <div class="mt-1 text-sm text-gray-600">刷新时间: {{ formatTime(status.updatedAt) }}</div>
            </div>
            <div class="flex flex-col items-start gap-2 sm:flex-row sm:items-center sm:justify-end">
              <div class="flex rounded-md border border-gray-300 bg-white p-1">
                <button
                  v-for="option in rangeOptions"
                  :key="option.value"
                  type="button"
                  class="h-7 min-w-12 rounded px-3 text-xs font-medium transition-colors"
                  :class="rangeKey === option.value ? 'bg-blue-600 text-white' : 'text-gray-600 hover:bg-gray-100'"
                  @click="setRange(option.value)"
                >
                  {{ option.label }}
                </button>
              </div>
              <Button variant="outline" size="sm" @click="loadGpuStatus" :disabled="loading">
                {{ loading ? '刷新中...' : '刷新' }}
              </Button>
            </div>
          </div>

          <div class="border-t border-gray-200 p-4">
            <div class="grid gap-4 xl:grid-cols-2">
              <div
                v-for="{ gpu, series } in gpuCharts"
                :key="gpu.uuid"
                class="min-w-0 rounded-md border border-gray-200 bg-white p-4 shadow-sm"
              >
                <div class="flex items-start justify-between gap-3">
                  <div class="min-w-0">
                    <div class="text-sm font-semibold text-gray-900">GPU {{ gpu.index }}</div>
                    <div class="mt-1 truncate text-sm text-gray-700" :title="gpu.name">{{ gpu.name }}</div>
                    <div class="mt-1 truncate text-xs text-gray-500" :title="gpu.uuid">{{ gpu.uuid }}</div>
                  </div>
                  <span class="rounded bg-green-100 px-2 py-1 text-xs text-green-700">
                    {{ gpu.processes?.length || 0 }} 进程
                  </span>
                </div>

                <div class="mt-4 space-y-3">
                  <div>
                    <div class="flex justify-between text-xs text-gray-600">
                      <span>显存</span>
                      <span>{{ gpu.memoryUsedMiB }} / {{ gpu.memoryTotalMiB }} MiB</span>
                    </div>
                    <div class="mt-1 h-2 overflow-hidden rounded-full bg-gray-100">
                      <div class="h-full rounded-full bg-blue-600" :style="{ width: `${memoryPercent(gpu)}%` }" />
                    </div>
                  </div>
                  <div class="grid grid-cols-2 gap-3 text-xs text-gray-600">
                    <div>
                      <div class="font-medium text-gray-900">{{ gpu.utilization }}%</div>
                      <div>GPU 利用率</div>
                    </div>
                    <div>
                      <div class="font-medium text-gray-900">{{ gpu.temperature }}°C</div>
                      <div>温度</div>
                    </div>
                  </div>
                  <div>
                    <div class="flex items-center justify-between gap-3 text-xs">
                      <div class="font-medium text-gray-700">容器显存占用</div>
                      <div class="text-gray-500">{{ series.length }} 条曲线</div>
                    </div>
                    <TimeSeriesChart
                      class="mt-2"
                      :label="`GPU ${gpu.index} 容器显存占用`"
                      :series="series"
                      :minimum-max="gpu.memoryTotalMiB"
                      :format-value="formatMemory"
                      :range="rangeKey"
                      empty-text="等待显存采样数据"
                    />
                    <div v-if="series.length" class="mt-3 flex flex-wrap gap-3">
                      <div v-for="item in series" :key="item.id" class="flex min-w-0 items-center gap-2 text-xs text-gray-600">
                        <span class="h-2.5 w-2.5 shrink-0 rounded-full" :style="{ backgroundColor: item.color }" />
                        <span class="min-w-0 break-all font-medium text-gray-800">{{ item.name }}</span>
                        <span class="shrink-0 tabular-nums">{{ item.latest }}</span>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <div class="border-t border-gray-200 p-4">
            <div class="flex items-center justify-between">
              <h2 class="text-base font-semibold text-gray-900">显卡进程</h2>
              <span class="text-xs text-gray-500">{{ allProcesses.length }} 个</span>
            </div>

            <div v-if="allProcesses.length" class="panel-table-frame mt-4">
              <table class="panel-table w-full min-w-[920px] table-fixed border-collapse text-sm">
                <thead>
                  <tr class="border-b text-left text-xs text-gray-500">
                    <SortableHeader :sort="processTable" column="gpu" label="GPU" class="w-20 px-2 py-2 font-medium" />
                    <SortableHeader :sort="processTable" column="pid" label="PID" class="w-20 px-2 py-2 font-medium" />
                    <SortableHeader :sort="processTable" column="processName" label="进程" class="px-2 py-2 font-medium" />
                    <SortableHeader :sort="processTable" column="user" label="用户" class="w-28 px-2 py-2 font-medium" />
                    <SortableHeader :sort="processTable" column="group" label="分组" class="w-40 px-2 py-2 font-medium" />
                    <SortableHeader :sort="processTable" column="owner" label="归属" class="w-36 px-2 py-2 font-medium" />
                    <SortableHeader :sort="processTable" column="usedMemoryMiB" label="显存" numeric class="w-28 px-2 py-2 text-right font-medium" />
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="process in processTable.rows" :key="`${process.gpuUuid}-${process.pid}-${process.processName}`" class="border-b last:border-b-0">
                    <td class="px-2 py-2 text-gray-700">GPU {{ gpuIndexByUuid[process.gpuUuid] ?? '-' }}</td>
                    <td class="px-2 py-2 font-mono text-xs text-gray-700">{{ process.pid }}</td>
                    <td class="truncate px-2 py-2 text-gray-900" :title="process.processName">{{ process.processName }}</td>
                    <td class="truncate px-2 py-2 text-gray-700" :title="process.user || '-'">{{ process.user || '-' }}</td>
                    <td class="w-40 px-2 py-2">
                      <div class="flex flex-wrap gap-1">
                        <span
                          v-for="group in visibleGroups(process.groups)"
                          :key="group.id"
                          :class="groupBadgeClass(group)"
                        >
                          {{ group.name }}
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
                    <td class="w-36 px-2 py-2">
                      <span
                        :class="ownerBadgeClass(process)"
                        :title="ownerLabel(process)"
                      >
                        {{ ownerLabel(process) }}
                      </span>
                    </td>
                    <td class="px-2 py-2 text-right tabular-nums text-gray-900">{{ process.usedMemoryMiB }} MiB</td>
                  </tr>
                </tbody>
              </table>
            </div>
            <div v-else class="mt-4 rounded-md border border-gray-200 bg-gray-50 p-4 text-sm text-gray-500">
              当前没有进程占用 NVIDIA 显卡。
            </div>
          </div>
        </section>
      </div>
    </main>
  </div>
</template>

<script setup>
import SortableHeader from '@/components/ui/SortableHeader.vue'
import { useTableSort } from '@/utils/tableSort'
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import api from '@/utils/api'
import Button from '@/components/ui/Button.vue'
import Card from '@/components/ui/Card.vue'
import AppHeader from '@/components/AppHeader.vue'
import TimeSeriesChart from '@/components/monitor/TimeSeriesChart.vue'

const totalSeriesColor = '#111827'
const hostSeriesColor = '#6b7280'
const formatMemory = value => `${value.toFixed(0)} MiB`
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
      id: `${ownerType}:${containerName || label}`, name: label, color, points: points || [], valueKey: 'usedMemoryMiB',
      latest: Number.isFinite(latest) ? formatMemory(latest) : '-',
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
const gpuCharts = computed(() => (status.value.gpus || []).map(gpu => ({ gpu, series: memoryChartSeries(gpu) })))
let refreshTimer = null

const rangeOptions = [
  { label: '1h', value: '1h' },
  { label: '24h', value: '24h' },
  { label: '7d', value: '7d' },
  { label: '30d', value: '30d' },
]

const allProcesses = computed(() =>
  (status.value.gpus || [])
    .flatMap((gpu) => (gpu.processes || []).map((process) => ({ ...process, gpuUuid: process.gpuUuid || gpu.uuid })))
    .sort((a, b) => b.usedMemoryMiB - a.usedMemoryMiB)
)

const gpuIndexByUuid = computed(() => {
  const mapping = {}
  for (const gpu of status.value.gpus || []) {
    mapping[gpu.uuid] = gpu.index
  }
  return mapping
})

const loadGpuStatus = async () => {
  loading.value = true
  error.value = ''
  try {
    const response = await api.get('/gpu/monitor', {
      params: { range: rangeKey.value },
    })
    status.value = response.data || status.value
  } catch (err) {
    error.value = err.response?.data?.error || '加载显卡监控失败'
  } finally {
    loading.value = false
  }
}

const setRange = async (value) => {
  rangeKey.value = value
  await loadGpuStatus()
}

const memoryPercent = (gpu) => {
  if (!gpu.memoryTotalMiB) {
    return 0
  }
  return Math.min(100, Math.max(0, (gpu.memoryUsedMiB / gpu.memoryTotalMiB) * 100))
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
  refreshTimer = window.setInterval(loadGpuStatus, 15 * 1000)
})

onBeforeUnmount(() => {
  if (refreshTimer) {
    window.clearInterval(refreshTimer)
  }
})
const processTable = useTableSort(() => allProcesses.value, { gpu: row => gpuIndexByUuid.value[row.gpuUuid], group: row => row.groups?.[0]?.name, owner: row => ownerLabel(row) })
</script>
