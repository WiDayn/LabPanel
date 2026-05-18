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
                v-for="gpu in status.gpus"
                :key="gpu.uuid"
                class="rounded-md border border-gray-200 bg-white p-4 shadow-sm"
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
                  <GpuMemoryChart
                    :series="gpu.memorySeries || []"
                    :total-memory="gpu.memoryTotalMiB"
                  />
                </div>
              </div>
            </div>
          </div>

          <div class="border-t border-gray-200 p-4">
            <div class="flex items-center justify-between">
              <h2 class="text-base font-semibold text-gray-900">显卡进程</h2>
              <span class="text-xs text-gray-500">{{ allProcesses.length }} 个</span>
            </div>

            <div v-if="allProcesses.length" class="mt-4 overflow-x-auto">
              <table class="w-full min-w-[920px] table-fixed border-collapse text-sm">
                <thead>
                  <tr class="border-b text-left text-xs text-gray-500">
                    <th class="w-20 px-2 py-2 font-medium">GPU</th>
                    <th class="w-20 px-2 py-2 font-medium">PID</th>
                    <th class="px-2 py-2 font-medium">进程</th>
                    <th class="w-28 px-2 py-2 font-medium">用户</th>
                    <th class="w-40 px-2 py-2 font-medium">分组</th>
                    <th class="w-36 px-2 py-2 font-medium">归属</th>
                    <th class="w-28 px-2 py-2 text-right font-medium">显存</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="process in allProcesses" :key="`${process.gpuUuid}-${process.pid}-${process.processName}`" class="border-b last:border-b-0">
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
                    <td class="px-2 py-2 text-right text-gray-900">{{ process.usedMemoryMiB }} MiB</td>
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
import { computed, defineComponent, h, onBeforeUnmount, onMounted, ref } from 'vue'
import api from '@/utils/api'
import Button from '@/components/ui/Button.vue'
import Card from '@/components/ui/Card.vue'
import AppHeader from '@/components/AppHeader.vue'

const chartColors = ['#2563eb', '#16a34a', '#ea580c', '#7c3aed', '#0891b2', '#dc2626', '#4f46e5', '#ca8a04']

const GpuMemoryChart = defineComponent({
  name: 'GpuMemoryChart',
  props: {
    series: { type: Array, required: true },
    totalMemory: { type: Number, required: true },
  },
  setup(props) {
    const width = 680
    const height = 220
    const padding = { top: 18, right: 18, bottom: 28, left: 56 }

    const maxValue = computed(() => {
      const values = props.series.flatMap((item) => (item.points || []).map((point) => Number(point.usedMemoryMiB || 0)))
      return Math.max(...values, props.totalMemory || 0, 1)
    })

    const linePath = (points) => {
      if (!points?.length) return ''
      const usableWidth = width - padding.left - padding.right
      const usableHeight = height - padding.top - padding.bottom
      return points.map((point, index) => {
        const x = padding.left + (points.length === 1 ? usableWidth : (index / (points.length - 1)) * usableWidth)
        const y = padding.top + usableHeight - (Number(point.usedMemoryMiB || 0) / maxValue.value) * usableHeight
        return `${index === 0 ? 'M' : 'L'} ${x.toFixed(1)} ${y.toFixed(1)}`
      }).join(' ')
    }

    const latestValue = (item) => {
      const points = item.points || []
      if (!points.length) return 0
      return Number(points[points.length - 1].usedMemoryMiB || 0)
    }
    const xTicks = computed(() => {
      const longestSeries = props.series.reduce((longest, item) => {
        const points = item.points || []
        return points.length > longest.length ? points : longest
      }, [])
      return buildTimeTicks(longestSeries, width, padding)
    })

    return () => h('div', { class: 'mt-4' }, [
      h('div', { class: 'flex items-center justify-between gap-3' }, [
        h('div', { class: 'text-xs font-medium text-gray-700' }, '容器显存占用'),
        h('div', { class: 'text-xs text-gray-500' }, `${props.series.length} 条曲线`),
      ]),
      h('div', { class: 'mt-2 h-56 overflow-hidden rounded-md border border-gray-200 bg-white' }, [
        props.series.length
          ? h('svg', { viewBox: `0 0 ${width} ${height}`, class: 'h-full w-full' }, [
              h('line', { x1: padding.left, y1: height - padding.bottom, x2: width - padding.right, y2: height - padding.bottom, stroke: '#e5e7eb' }),
              h('line', { x1: padding.left, y1: padding.top, x2: padding.left, y2: height - padding.bottom, stroke: '#e5e7eb' }),
              h('text', { x: padding.left - 8, y: padding.top + 4, 'text-anchor': 'end', class: 'fill-gray-500 text-[10px]' }, `${maxValue.value.toFixed(0)} MiB`),
              h('text', { x: padding.left - 8, y: height - padding.bottom, 'text-anchor': 'end', class: 'fill-gray-500 text-[10px]' }, '0 MiB'),
              ...xTicks.value.flatMap((tick) => [
                h('line', { x1: tick.x, y1: height - padding.bottom, x2: tick.x, y2: height - padding.bottom + 4, stroke: '#d1d5db' }),
                h('text', { x: tick.x, y: height - 10, 'text-anchor': 'middle', class: 'fill-gray-500 text-[10px]' }, tick.label),
              ]),
              ...props.series.map((item, index) => h('path', {
                d: linePath(item.points || []),
                fill: 'none',
                stroke: chartColors[index % chartColors.length],
                'stroke-width': 2.5,
                'stroke-linejoin': 'round',
                'stroke-linecap': 'round',
              })),
            ])
          : h('div', { class: 'flex h-full items-center justify-center text-sm text-gray-500' }, '等待显存采样数据'),
      ]),
      props.series.length
        ? h('div', { class: 'mt-3 flex flex-wrap gap-3' }, props.series.map((item, index) =>
            h('div', { class: 'flex items-center gap-2 text-xs text-gray-600' }, [
              h('span', { class: 'h-2.5 w-2.5 rounded-full', style: { backgroundColor: chartColors[index % chartColors.length] } }),
              h('span', { class: 'font-medium text-gray-800' }, item.label),
              h('span', `${latestValue(item).toFixed(0)} MiB`),
            ])
          ))
        : null,
    ])
  },
})

const buildTimeTicks = (points, width, padding) => {
  if (!points?.length) {
    return []
  }
  const maxTicks = 4
  const tickCount = Math.min(maxTicks, points.length)
  const indexes = new Set()
  if (tickCount === 1) {
    indexes.add(points.length - 1)
  } else {
    for (let i = 0; i < tickCount; i++) {
      indexes.add(Math.round((i * (points.length - 1)) / (tickCount - 1)))
    }
  }

  const usableWidth = width - padding.left - padding.right
  return [...indexes].sort((a, b) => a - b).map((index) => {
    const x = padding.left + (points.length === 1 ? usableWidth : (index / (points.length - 1)) * usableWidth)
    return {
      x,
      label: formatAxisTime(points[index]?.timestamp),
    }
  }).filter((tick) => tick.label)
}

const formatAxisTime = (value) => {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  const now = new Date()
  const sameDay = date.toDateString() === now.toDateString()
  if (sameDay) {
    return date.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', hour12: false })
  }
  return `${date.getMonth() + 1}/${date.getDate()} ${date.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', hour12: false })}`
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
    'bg-blue-100 text-blue-800 ring-blue-300',
    'bg-sky-100 text-sky-800 ring-sky-300',
    'bg-cyan-100 text-cyan-800 ring-cyan-300',
    'bg-indigo-100 text-indigo-800 ring-indigo-300',
  ],
  emerald: [
    'bg-emerald-100 text-emerald-800 ring-emerald-300',
    'bg-teal-100 text-teal-800 ring-teal-300',
    'bg-green-100 text-green-800 ring-green-300',
    'bg-cyan-100 text-cyan-800 ring-cyan-300',
  ],
  amber: [
    'bg-amber-100 text-amber-800 ring-amber-300',
    'bg-yellow-100 text-yellow-800 ring-yellow-300',
    'bg-orange-100 text-orange-800 ring-orange-300',
    'bg-lime-100 text-lime-800 ring-lime-300',
  ],
  violet: [
    'bg-violet-100 text-violet-800 ring-violet-300',
    'bg-purple-100 text-purple-800 ring-purple-300',
    'bg-fuchsia-100 text-fuchsia-800 ring-fuchsia-300',
    'bg-indigo-100 text-indigo-800 ring-indigo-300',
  ],
  rose: [
    'bg-rose-100 text-rose-800 ring-rose-300',
    'bg-pink-100 text-pink-800 ring-pink-300',
    'bg-red-100 text-red-800 ring-red-300',
    'bg-orange-100 text-orange-800 ring-orange-300',
  ],
  cyan: [
    'bg-cyan-100 text-cyan-800 ring-cyan-300',
    'bg-sky-100 text-sky-800 ring-sky-300',
    'bg-blue-100 text-blue-800 ring-blue-300',
    'bg-teal-100 text-teal-800 ring-teal-300',
  ],
  lime: [
    'bg-lime-100 text-lime-800 ring-lime-300',
    'bg-green-100 text-green-800 ring-green-300',
    'bg-emerald-100 text-emerald-800 ring-emerald-300',
    'bg-yellow-100 text-yellow-800 ring-yellow-300',
  ],
  orange: [
    'bg-orange-100 text-orange-800 ring-orange-300',
    'bg-amber-100 text-amber-800 ring-amber-300',
    'bg-red-100 text-red-800 ring-red-300',
    'bg-yellow-100 text-yellow-800 ring-yellow-300',
  ],
  slate: [
    'bg-slate-200 text-slate-800 ring-slate-300',
    'bg-zinc-200 text-zinc-800 ring-zinc-300',
    'bg-neutral-200 text-neutral-800 ring-neutral-300',
    'bg-stone-200 text-stone-800 ring-stone-300',
  ],
  indigo: [
    'bg-indigo-100 text-indigo-800 ring-indigo-300',
    'bg-blue-100 text-blue-800 ring-blue-300',
    'bg-violet-100 text-violet-800 ring-violet-300',
    'bg-sky-100 text-sky-800 ring-sky-300',
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
</script>
