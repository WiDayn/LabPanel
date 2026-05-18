<template>
  <div class="min-h-screen bg-gray-50">
    <AppHeader />

    <main class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      <div class="space-y-6">
        <div v-if="error" class="rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
          {{ error }}
        </div>

        <section class="overflow-hidden rounded-lg border border-gray-200 bg-white shadow-sm">
          <button
            type="button"
            class="flex w-full items-center justify-between gap-4 px-5 py-4 text-left"
            @click="toggleSection('summary')"
          >
            <div>
              <h2 class="text-base font-semibold text-gray-900">整机概览</h2>
              <div class="mt-1 text-xs text-gray-500">{{ overviewSubtitle }}</div>
            </div>
            <span class="inline-flex h-8 w-8 shrink-0 items-center justify-center rounded border border-gray-200 bg-white">
              <span
                class="h-0 w-0 border-x-[5px] border-t-[6px] border-x-transparent border-t-gray-500 transition-transform"
                :class="sectionArrowClass('summary')"
              ></span>
            </span>
          </button>
          <div v-show="!collapsedSections.summary" class="border-t border-gray-200 p-4">
            <div>
              <h3 class="text-sm font-semibold text-gray-900">主要配置</h3>
              <dl class="mt-3 divide-y divide-gray-200 rounded-md border border-gray-200 bg-gray-50">
                <div v-for="item in configItems" :key="item.label" class="grid gap-2 px-4 py-3 sm:grid-cols-[180px_1fr]">
                  <dt class="text-sm font-medium text-gray-500">{{ item.label }}</dt>
                  <dd class="min-w-0 text-sm font-medium text-gray-900">
                    <div class="relative min-w-0 pr-12">
                      <div class="min-w-0 break-words">{{ item.value }}</div>
                      <button
                        v-if="item.expandable && item.details.length"
                        type="button"
                        class="absolute right-0 top-1/2 h-7 -translate-y-1/2 rounded border border-gray-200 bg-white px-2 text-xs text-gray-600 hover:bg-gray-50"
                        @click="toggleMemoryDetails"
                      >
                        {{ memoryDetailsExpanded ? '收起' : '展开' }}
                      </button>
                    </div>
                    <dl
                      v-if="item.expandable && memoryDetailsExpanded && item.details.length"
                      class="mt-3 divide-y divide-gray-200 rounded-md border border-gray-200 bg-white text-xs"
                    >
                      <div
                        v-for="detail in item.details"
                        :key="detail.label"
                        class="grid gap-2 px-3 py-2 sm:grid-cols-[180px_1fr]"
                      >
                        <dt class="font-medium text-gray-500">{{ detail.label }}</dt>
                        <dd class="break-words text-gray-800">{{ detail.value }}</dd>
                      </div>
                    </dl>
                  </dd>
                </div>
              </dl>
            </div>

            <div class="mt-5 border-t border-gray-200 pt-4">
              <h3 class="text-sm font-semibold text-gray-900">实时指标</h3>
              <div class="mt-3 grid gap-4 md:grid-cols-2 xl:grid-cols-3">
                <div
                  v-for="item in summaryCards"
                  :key="item.label"
                  class="rounded-md border border-gray-200 bg-white p-4 shadow-sm"
                >
                  <div class="text-xs font-medium text-gray-500">{{ item.label }}</div>
                  <div class="mt-2 text-2xl font-semibold text-gray-900">{{ item.value }}</div>
                  <div v-if="item.detail" class="mt-1 text-xs text-gray-500">{{ item.detail }}</div>
                </div>
              </div>
            </div>
          </div>
        </section>

        <section class="overflow-hidden rounded-lg border border-gray-200 bg-white shadow-sm">
          <div class="flex flex-col gap-4 px-5 py-4 lg:flex-row lg:items-center lg:justify-between">
            <button type="button" class="text-left" @click="toggleSection('charts')">
              <div>
                <h2 class="text-base font-semibold text-gray-900">指标图表</h2>
                <div class="mt-1 text-xs text-gray-500">{{ points.length }} 个采样点 · {{ formatTime(metrics.updatedAt) }}</div>
              </div>
            </button>
            <div class="flex flex-col gap-2 sm:flex-row sm:items-center">
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
              <Button variant="outline" size="sm" @click="loadMetrics" :disabled="loading">
                {{ loading ? '刷新中...' : '刷新' }}
              </Button>
              <button
                type="button"
                class="inline-flex h-9 w-9 shrink-0 items-center justify-center rounded border border-gray-200 bg-white hover:bg-gray-50"
                @click="toggleSection('charts')"
                :aria-label="sectionToggleLabel('charts')"
                :title="sectionToggleLabel('charts')"
              >
                <span
                  class="h-0 w-0 border-x-[5px] border-t-[6px] border-x-transparent border-t-gray-500 transition-transform"
                  :class="sectionArrowClass('charts')"
                ></span>
              </button>
            </div>
          </div>
          <div v-show="!collapsedSections.charts" class="border-t border-gray-200 p-4">
            <div class="grid gap-4 xl:grid-cols-2">
              <MetricChart
                title="CPU 占用"
                color="#2563eb"
                :points="points"
                value-key="cpuPercent"
                :format-value="formatPercent"
              />
              <MetricChart
                title="CPU 主频"
                color="#16a34a"
                :points="points"
                value-key="cpuMhz"
                :format-value="formatFrequency"
              />
              <MetricChart
                title="温度"
                color="#ea580c"
                :points="points"
                value-key="temperatureC"
                :format-value="formatTemperature"
              />
              <MetricChart
                title="内存占用"
                color="#7c3aed"
                :points="points"
                value-key="memoryUsedBytes"
                :format-value="formatBytes"
              />
              <MetricChart
                title="网络下行"
                color="#0891b2"
                :points="points"
                value-key="networkRxBps"
                :format-value="formatRate"
              />
              <MetricChart
                title="网络上行"
                color="#dc2626"
                :points="points"
                value-key="networkTxBps"
                :format-value="formatRate"
              />
            </div>
          </div>
        </section>

        <section class="overflow-hidden rounded-lg border border-gray-200 bg-white shadow-sm">
          <button
            type="button"
            class="flex w-full items-center justify-between gap-4 px-5 py-4 text-left"
            @click="toggleSection('processes')"
          >
            <div>
              <h2 class="text-base font-semibold text-gray-900">进程监控</h2>
              <div class="mt-1 text-xs text-gray-500">{{ processes.length }} 个</div>
            </div>
            <span class="inline-flex h-8 w-8 shrink-0 items-center justify-center rounded border border-gray-200 bg-white">
              <span
                class="h-0 w-0 border-x-[5px] border-t-[6px] border-x-transparent border-t-gray-500 transition-transform"
                :class="sectionArrowClass('processes')"
              ></span>
            </span>
          </button>
          <div v-show="!collapsedSections.processes" class="border-t border-gray-200 p-4">
            <div v-if="processes.length" class="overflow-x-auto">
              <table class="w-full min-w-[980px] table-fixed border-collapse text-sm">
                <thead>
                  <tr class="border-b text-left text-xs text-gray-500">
                    <th class="w-20 px-2 py-2 font-medium">PID</th>
                    <th class="px-2 py-2 font-medium">进程</th>
                    <th class="w-28 px-2 py-2 font-medium">用户</th>
                    <th class="w-40 px-2 py-2 font-medium">分组</th>
                    <th class="w-36 px-2 py-2 font-medium">归属</th>
                    <th class="w-20 px-2 py-2 text-right font-medium">CPU</th>
                    <th class="w-32 px-2 py-2 text-right font-medium">内存</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="process in processes" :key="`${process.pid}-${process.command}`" class="border-b last:border-b-0">
                    <td class="px-2 py-2 font-mono text-xs text-gray-700">{{ process.pid }}</td>
                    <td class="truncate px-2 py-2 text-gray-900" :title="process.command || '-'">{{ process.command || '-' }}</td>
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
                    <td class="px-2 py-2 text-right text-gray-900">{{ formatPercent(process.cpuPercent) }}</td>
                    <td class="px-2 py-2 text-right text-gray-900">
                      {{ formatBytes(process.memoryBytes) }}
                      <span class="text-xs text-gray-500">({{ formatPercent(process.memoryPercent) }})</span>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
            <div v-else class="rounded-md border border-gray-200 bg-gray-50 p-4 text-sm text-gray-500">
              暂无进程数据。
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
import AppHeader from '@/components/AppHeader.vue'

const MetricChart = defineComponent({
  name: 'HostMetricChart',
  props: {
    title: { type: String, required: true },
    color: { type: String, required: true },
    points: { type: Array, required: true },
    valueKey: { type: String, required: true },
    formatValue: { type: Function, required: true },
  },
  setup(props) {
    const width = 640
    const height = 220
    const padding = { top: 20, right: 16, bottom: 28, left: 56 }

    const values = computed(() => props.points.map((point) => Number(point[props.valueKey] || 0)))
    const maxValue = computed(() => Math.max(...values.value, 0))
    const latestValue = computed(() => values.value.length ? values.value[values.value.length - 1] : 0)
    const pathData = computed(() => {
      if (!props.points.length) {
        return ''
      }
      const usableWidth = width - padding.left - padding.right
      const usableHeight = height - padding.top - padding.bottom
      const denominator = Math.max(maxValue.value, 1)
      return props.points.map((point, index) => {
        const x = padding.left + (props.points.length === 1 ? usableWidth : (index / (props.points.length - 1)) * usableWidth)
        const value = Number(point[props.valueKey] || 0)
        const y = padding.top + usableHeight - (value / denominator) * usableHeight
        return `${index === 0 ? 'M' : 'L'} ${x.toFixed(1)} ${y.toFixed(1)}`
      }).join(' ')
    })
    const xTicks = computed(() => buildTimeTicks(props.points, width, padding))

    return () => h('div', { class: 'rounded-md border border-gray-200 bg-white p-4 shadow-sm' }, [
      h('div', { class: 'flex items-start justify-between gap-3' }, [
        h('div', [
          h('h3', { class: 'text-sm font-semibold text-gray-900' }, props.title),
          h('div', { class: 'mt-1 text-xs text-gray-500' }, `${props.points.length} 个采样点`),
        ]),
        h('div', { class: 'text-right' }, [
          h('div', { class: 'text-sm font-semibold text-gray-900' }, props.formatValue(latestValue.value)),
          h('div', { class: 'text-xs text-gray-500' }, '当前'),
        ]),
      ]),
      h('div', { class: 'mt-3 h-56 w-full overflow-hidden rounded-md border border-gray-200 bg-white' }, [
        props.points.length
          ? h('svg', { viewBox: `0 0 ${width} ${height}`, class: 'h-full w-full' }, [
              h('line', { x1: padding.left, y1: height - padding.bottom, x2: width - padding.right, y2: height - padding.bottom, stroke: '#e5e7eb' }),
              h('line', { x1: padding.left, y1: padding.top, x2: padding.left, y2: height - padding.bottom, stroke: '#e5e7eb' }),
              h('text', { x: padding.left - 8, y: padding.top + 4, 'text-anchor': 'end', class: 'fill-gray-500 text-[10px]' }, props.formatValue(maxValue.value)),
              h('text', { x: padding.left - 8, y: height - padding.bottom, 'text-anchor': 'end', class: 'fill-gray-500 text-[10px]' }, props.formatValue(0)),
              ...xTicks.value.flatMap((tick) => [
                h('line', { x1: tick.x, y1: height - padding.bottom, x2: tick.x, y2: height - padding.bottom + 4, stroke: '#d1d5db' }),
                h('text', { x: tick.x, y: height - 10, 'text-anchor': 'middle', class: 'fill-gray-500 text-[10px]' }, tick.label),
              ]),
              h('path', { d: pathData.value, fill: 'none', stroke: props.color, 'stroke-width': 2.5, 'stroke-linejoin': 'round', 'stroke-linecap': 'round' }),
            ])
          : h('div', { class: 'flex h-full items-center justify-center text-sm text-gray-500' }, '等待采样数据'),
      ]),
    ])
  },
})

const rangeOptions = [
  { label: '1h', value: '1h' },
  { label: '24h', value: '24h' },
  { label: '7d', value: '7d' },
  { label: '30d', value: '30d' },
]

const loading = ref(false)
const error = ref('')
const rangeKey = ref('1h')
const metrics = ref({
  updatedAt: '',
  system: {},
  summary: {},
  points: [],
  processes: [],
  intervalSeconds: 60,
})
const collapsedSections = ref({
  summary: false,
  charts: false,
  processes: false,
})
const memoryDetailsExpanded = ref(false)
let refreshTimer = null

const systemInfo = computed(() => metrics.value.system || {})
const summary = computed(() => metrics.value.summary || {})
const points = computed(() => metrics.value.points || [])
const processes = computed(() => metrics.value.processes || [])
const memoryPercent = computed(() => {
  const total = Number(summary.value.memoryTotalBytes || 0)
  if (!total) return 0
  return (Number(summary.value.memoryUsedBytes || 0) / total) * 100
})

const summaryCards = computed(() => [
  {
    label: 'CPU 占用',
    value: formatPercent(summary.value.cpuPercent),
    detail: `主频 ${formatFrequency(summary.value.cpuMhz)}`,
  },
  {
    label: '温度',
    value: formatTemperature(summary.value.temperatureC),
    detail: '当前最高传感器读数',
  },
  {
    label: '内存占用',
    value: `${formatPercent(memoryPercent.value)} / ${formatBytes(summary.value.memoryTotalBytes)}`,
    detail: formatBytes(summary.value.memoryUsedBytes),
  },
  {
    label: '网络下行',
    value: formatRate(summary.value.networkRxBps),
    detail: `累计 ${formatBytes(summary.value.networkRxBytes)}`,
  },
  {
    label: '网络上行',
    value: formatRate(summary.value.networkTxBps),
    detail: `累计 ${formatBytes(summary.value.networkTxBytes)}`,
  },
  {
    label: '进程',
    value: `${processes.value.length}`,
    detail: '',
  },
])

const overviewSubtitle = computed(() => formatUptimeSummary(systemInfo.value.uptimeSeconds))

const configItems = computed(() => [
  { label: 'Hostname', value: systemInfo.value.hostname || '-', details: [] },
  { label: 'OS', value: systemInfo.value.os || '-', details: [] },
  ...formatCPUItems(systemInfo.value),
  ...formatGPUItems(systemInfo.value.gpus),
  {
    label: '内存',
    value: formatMemorySummary(systemInfo.value),
    details: formatMemoryModuleDetails(systemInfo.value.memoryModules),
    expandable: true,
  },
])

const loadMetrics = async () => {
  loading.value = true
  error.value = ''
  try {
    const response = await api.get('/host/metrics', {
      params: { range: rangeKey.value },
    })
    metrics.value = response.data || {
      updatedAt: '',
      system: {},
      summary: {},
      points: [],
      processes: [],
      intervalSeconds: 60,
    }
  } catch (err) {
    error.value = err.response?.data?.error || '加载宿主机监控失败'
  } finally {
    loading.value = false
  }
}

const setRange = async (value) => {
  rangeKey.value = value
  await loadMetrics()
}

const toggleSection = (section) => {
  collapsedSections.value[section] = !collapsedSections.value[section]
}

const toggleMemoryDetails = () => {
  memoryDetailsExpanded.value = !memoryDetailsExpanded.value
}

const sectionToggleLabel = (section) => collapsedSections.value[section] ? '展开' : '收起'
const sectionArrowClass = (section) => collapsedSections.value[section] ? '-rotate-90' : 'rotate-0'

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

const ownerLabel = (process) => {
  if (process.ownerType === 'container' && process.containerName) {
    return process.containerName
  }
  return '宿主机'
}

const buildTimeTicks = (chartPoints, width, padding) => {
  if (!chartPoints?.length) {
    return []
  }
  const maxTicks = 4
  const tickCount = Math.min(maxTicks, chartPoints.length)
  const indexes = new Set()
  if (tickCount === 1) {
    indexes.add(chartPoints.length - 1)
  } else {
    for (let i = 0; i < tickCount; i++) {
      indexes.add(Math.round((i * (chartPoints.length - 1)) / (tickCount - 1)))
    }
  }

  const usableWidth = width - padding.left - padding.right
  return [...indexes].sort((a, b) => a - b).map((index) => {
    const x = padding.left + (chartPoints.length === 1 ? usableWidth : (index / (chartPoints.length - 1)) * usableWidth)
    return {
      x,
      label: formatAxisTime(chartPoints[index]?.timestamp),
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

const formatBytes = (value) => {
  const bytes = Number(value || 0)
  if (bytes < 1024) return `${bytes.toFixed(0)} B`
  const units = ['KB', 'MB', 'GB', 'TB']
  let current = bytes / 1024
  let index = 0
  while (current >= 1024 && index < units.length - 1) {
    current /= 1024
    index++
  }
  return `${current >= 10 ? current.toFixed(1) : current.toFixed(2)} ${units[index]}`
}
const formatCapacityBytes = (value) => formatBytes(value).replace(/\.0 (?=[A-Z])/, ' ')

const formatRate = (value) => `${formatBytes(value)}/s`
const formatPercent = (value) => `${Number(value || 0).toFixed(1)}%`
const displayCPUModel = (model) => String(model || '')
  .replace(/\s+CPU\s*@\s*[\d.]+\s*GHz/ig, '')
  .replace(/\s+/g, ' ')
  .trim()
const nominalCPUFrequency = (model) => {
  const match = String(model || '').match(/@\s*([\d.]+)\s*GHz/i)
  if (!match) return ''
  return `${Number(match[1]).toFixed(2)} GHz`
}
const formatCPUTopology = (cores, threads) => {
  const coreCount = Number(cores || 0)
  const threadCount = Number(threads || 0)
  if (!coreCount || !threadCount) return ''
  return `${coreCount}C / ${threadCount}T`
}
const formatCPUValue = (cpu, system) => {
  const cores = Number(cpu?.cores || system.cpuCores || 0)
  const threads = Number(cpu?.threads || system.cpuThreads || 0)
  const topology = formatCPUTopology(cores, threads)
  const model = displayCPUModel(cpu?.model || system.cpuModel) || '-'
  const frequency = formatFrequency(cpu?.maxMhz || cpu?.currentMhz || system.cpuMhz)
  const suffix = topology ? ` (${topology})` : ''
  return `${model}${suffix} @ ${frequency}`
}
const formatCPUItems = (system) => {
  const cpus = Array.isArray(system.cpus) ? system.cpus : []
  if (cpus.length) {
    return cpus.map((cpu, index) => ({
      label: `CPU${cpu.index || index + 1}`,
      value: formatCPUValue(cpu, system),
      details: [],
    }))
  }
  return [{
    label: 'CPU',
    value: formatCPUValue(null, system).replace(/ @ -$/, nominalCPUFrequency(system.cpuModel) ? ` @ ${nominalCPUFrequency(system.cpuModel)}` : ' @ -'),
    details: [],
  }]
}
const formatGPUItems = (gpus) => {
  if (!Array.isArray(gpus) || !gpus.length) {
    return [{ label: 'GPU', value: '-', details: [] }]
  }
  return gpus.map((gpu, index) => ({
    label: `GPU${index + 1}`,
    value: gpu,
    details: [],
  }))
}
const isUsefulMemoryText = (value) => {
  if (!value) return false
  return !/unknown|not specified|not provided|no module/i.test(String(value))
}
const memoryModuleCollator = new Intl.Collator('zh-CN', { numeric: true, sensitivity: 'base' })
const formatMemoryModuleValue = (module) => {
  const brand = [module.manufacturer, module.partNumber].filter(isUsefulMemoryText).join(' ')
  const speed = isUsefulMemoryText(module.speed) ? module.speed : ''
  return [brand, formatCapacityBytes(module.sizeBytes), speed].filter(Boolean).join(' ')
}
const formatMemoryModuleLabel = (module, index) => {
  const bankLocator = isUsefulMemoryText(module.bankLocator) ? module.bankLocator : ''
  const locator = isUsefulMemoryText(module.locator) ? module.locator : ''
  if (bankLocator && locator && bankLocator !== locator) {
    return `${bankLocator} / ${locator}`
  }
  return bankLocator || locator || `DIMM${index + 1}`
}
const formatMemoryModuleDetail = (module, index) => {
  return {
    label: formatMemoryModuleLabel(module, index),
    value: formatMemoryModuleValue(module),
  }
}
const sortedMemoryModules = (modules) => {
  if (!Array.isArray(modules)) return []
  return modules
    .filter((module) => Number(module.sizeBytes || 0) > 0)
    .slice()
    .sort((left, right) => memoryModuleCollator.compare(
      `${left.bankLocator || ''} ${left.locator || ''}`,
      `${right.bankLocator || ''} ${right.locator || ''}`,
    ))
}
const formatMemoryModuleDetails = (modules) => sortedMemoryModules(modules).map(formatMemoryModuleDetail)
const formatMemorySummary = (system) => {
  const total = Number(system.memoryInstalledBytes || system.memoryTotalBytes || 0)
  if (!total) return '-'
  const gb = total / (1024 ** 3)
  return `${Number.isInteger(gb) ? gb.toFixed(0) : gb.toFixed(1)}G`
}
const formatDuration = (value) => {
  let seconds = Number(value || 0)
  if (!seconds) return '-'
  const days = Math.floor(seconds / 86400)
  seconds -= days * 86400
  const hours = Math.floor(seconds / 3600)
  seconds -= hours * 3600
  const minutes = Math.floor(seconds / 60)
  const parts = []
  if (days) parts.push(`${days} 天`)
  if (hours) parts.push(`${hours} 小时`)
  parts.push(`${minutes} 分钟`)
  return parts.join(' ')
}
const formatUptimeSummary = (value) => {
  const totalMinutes = Math.floor(Number(value || 0) / 60)
  const hours = Math.floor(totalMinutes / 60)
  const minutes = totalMinutes % 60
  return `运行时间: ${hours}小时${minutes}分钟`
}
const formatFrequency = (value) => {
  const mhz = Number(value || 0)
  if (!mhz) return '-'
  if (mhz >= 1000) return `${(mhz / 1000).toFixed(2)} GHz`
  return `${mhz.toFixed(0)} MHz`
}
const formatTemperature = (value) => {
  const temperature = Number(value || 0)
  if (!temperature) return '-'
  return `${temperature.toFixed(1)}°C`
}
const formatTime = (value) => {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleString('zh-CN', { hour12: false })
}

onMounted(() => {
  loadMetrics()
  refreshTimer = window.setInterval(loadMetrics, 60 * 1000)
})

onBeforeUnmount(() => {
  if (refreshTimer) {
    window.clearInterval(refreshTimer)
  }
})
</script>
