<template>
  <div class="min-h-screen bg-gray-50">
    <AppHeader />

    <main class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      <div class="space-y-6">
        <div v-if="error" class="rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
          {{ error }}
        </div>

        <div class="grid items-start gap-6 lg:grid-cols-[minmax(0,320px)_minmax(0,1fr)]">
          <section class="min-w-0 rounded-lg border border-gray-200 bg-white shadow-sm">
            <div class="flex items-center justify-between gap-3 px-4 py-3">
              <div>
                <h2 class="text-base font-semibold">容器列表</h2>
                <p class="text-xs tabular-nums text-gray-600">{{ visibleContainers.length }} / {{ containers.length }} 个</p>
              </div>
              <details ref="filterMenu" class="relative" @keydown.esc.stop="closeFilters">
                <summary class="flex min-h-9 cursor-pointer list-none items-center gap-2 rounded-md border border-gray-300 px-3 text-sm font-medium hover:bg-gray-50 [&::-webkit-details-marker]:hidden">
                  筛选<span v-if="activeFilterCount" class="text-blue-700">{{ activeFilterCount }}</span>
                </summary>
                <div class="absolute right-0 z-20 mt-2 w-60 space-y-3 rounded-lg border border-gray-200 bg-white p-3 shadow-md">
                  <label class="block text-xs font-medium text-gray-700">类型
                    <select v-model="runtimeFilter" class="mt-1 w-full rounded-md border border-gray-300 bg-white px-2 py-1.5 text-sm">
                      <option value="">全部类型</option>
                      <option value="lxc">LXC</option>
                      <option value="docker">Docker</option>
                    </select>
                  </label>
                  <label class="block text-xs font-medium text-gray-700">状态
                    <select v-model="stateFilter" class="mt-1 w-full rounded-md border border-gray-300 bg-white px-2 py-1.5 text-sm">
                      <option value="">全部状态</option>
                      <option v-for="state in availableStates" :key="state" :value="state">{{ state }}</option>
                    </select>
                  </label>
                  <label class="block text-xs font-medium text-gray-700">分组
                    <select v-model="groupFilter" :disabled="!groupDataReady" class="mt-1 w-full rounded-md border border-gray-300 bg-white px-2 py-1.5 text-sm disabled:opacity-60">
                      <option value="">全部分组</option>
                      <option value="ungrouped">未分组</option>
                      <option v-for="group in groups" :key="group.id" :value="group.id">{{ group.name }}</option>
                    </select>
                  </label>
                  <Button size="sm" variant="outline" :disabled="!activeFilterCount" @click="resetFilters">清除筛选</Button>
                </div>
              </details>
            </div>
            <p v-if="groupError" class="border-t px-4 py-2 text-xs text-amber-800" role="status">{{ groupError }} <button type="button" class="font-medium underline" @click="loadGroups">重试</button></p>
            <div class="max-h-[44dvh] overflow-y-auto border-t border-gray-200 lg:max-h-[72dvh]">
              <section v-for="bucket in runtimeBuckets" :key="bucket.type" :aria-label="runtimeLabel(bucket.type)">
                <h3 class="sticky top-0 z-10 flex justify-between border-y border-gray-100 bg-gray-50 px-4 py-2 text-xs font-semibold text-gray-700">
                  {{ runtimeLabel(bucket.type) }} <span class="tabular-nums">{{ bucket.rows.length }}</span>
                </h3>
                <ul class="divide-y divide-gray-100">
                  <li v-for="container in bucket.rows" :key="containerKey(container)">
                    <button type="button" :aria-pressed="currentKey === containerKey(container)"
                      class="block w-full px-4 py-3 text-left focus-visible:outline focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-blue-500"
                      :class="currentKey === containerKey(container) ? 'bg-blue-50' : 'hover:bg-gray-50'"
                      @click="selectContainer(containerKey(container))"
                    >
                      <div class="flex flex-wrap items-center justify-between gap-x-2 gap-y-1">
                        <span class="min-w-0 break-all text-sm font-semibold" :class="currentKey === containerKey(container) ? 'text-blue-800' : 'text-gray-900'">{{ container.name }}</span>
                        <span class="inline-flex items-center gap-1.5 text-xs text-gray-600">
                          <span class="h-1.5 w-1.5 rounded-full" :class="containerState(container) === 'Running' ? 'bg-green-600' : 'bg-gray-400'" aria-hidden="true"></span>
                          {{ containerState(container) }}
                        </span>
                      </div>
                      <div class="mt-1.5 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs tabular-nums text-gray-600">
                        <span>CPU {{ formatPercent(container.cpuPercent) }}</span>
                        <span>内存 {{ formatBytes(container.memoryBytes) }}</span>
                        <span v-for="group in groupsFor(container).slice(0, 2)" :key="group.id" class="max-w-28 truncate rounded-full px-2 py-0.5 ring-1 ring-inset" :class="groupColors[group.color] || groupColors.slate" :title="group.name">{{ group.name }}</span>
                        <span v-if="groupsFor(container).length > 2" :title="groupsFor(container).map(group => group.name).join('、')">+{{ groupsFor(container).length - 2 }}</span>
                      </div>
                    </button>
                  </li>
                </ul>
              </section>
              <div v-if="!visibleContainers.length" class="px-4 py-5 text-sm text-gray-600" role="status">
                {{ loading ? '正在加载容器…' : activeFilterCount ? '没有符合筛选条件的容器' : '暂无容器监控数据' }}
              </div>
            </div>
          </section>

          <div class="space-y-6">
            <section class="overflow-hidden rounded-lg border border-gray-200 bg-white shadow-sm">
              <div class="flex flex-wrap items-center justify-between gap-3 px-5 py-4">
                <div class="min-w-0">
                  <h2 class="break-all text-base font-semibold">{{ currentSummary?.name || '未选择容器' }}</h2>
                  <p v-if="currentSummary" class="mt-1 text-xs text-gray-600">
                    {{ runtimeLabel(containerRuntime(currentSummary)) }} · {{ containerState(currentSummary) }} · {{ formatTime(currentSummary.updatedAt) }}
                  </p>
                </div>
                <div class="flex flex-wrap items-center gap-2">
                  <div v-if="currentSummary" class="flex rounded-md border border-gray-300 bg-white p-1">
                    <button
                      v-for="option in rangeOptions"
                      :key="option.value"
                      type="button"
                      class="h-7 min-w-10 rounded px-2 text-xs font-medium transition-colors"
                      :aria-pressed="rangeKey === option.value"
                      :class="rangeKey === option.value ? 'bg-blue-600 text-white' : 'text-gray-600 hover:bg-gray-100'"
                      @click="setRange(option.value)"
                    >
                      {{ option.label }}
                    </button>
                  </div>
                  <Button variant="outline" size="sm" @click="loadMetrics" :disabled="loading">
                    {{ loading ? '刷新中...' : '刷新' }}
                  </Button>
                </div>
              </div>
              <div v-if="currentSummary" class="border-t border-gray-200 px-5 py-4">
                <div class="grid grid-cols-2 gap-3 text-xs tabular-nums text-gray-600 sm:grid-cols-3 xl:grid-cols-6">
                  <div>
                    <div class="font-medium text-gray-900">{{ formatPercent(currentSummary.cpuPercent) }}</div>
                    <div>CPU</div>
                  </div>
                  <div>
                    <div class="font-medium text-gray-900">{{ formatBytes(currentSummary.memoryBytes) }}</div>
                    <div>内存</div>
                  </div>
                  <div>
                    <div class="font-medium text-gray-900">{{ formatBytes(currentSummary.diskUsageBytes) }}</div>
                    <div>磁盘</div>
                  </div>
                  <div>
                    <div class="font-medium text-gray-900">{{ currentSummary.processes || 0 }}</div>
                    <div>进程</div>
                  </div>
                  <div>
                    <div class="font-medium text-gray-900">{{ formatRate(currentSummary.networkRxBps) }}</div>
                    <div>下行</div>
                  </div>
                  <div>
                    <div class="font-medium text-gray-900">{{ formatRate(currentSummary.networkTxBps) }}</div>
                    <div>上行</div>
                  </div>
                </div>
              </div>

              <div v-if="currentSummary" class="border-t border-gray-200 p-4">
                <div>
                  <h2 class="text-base font-semibold text-gray-900">指标图表</h2>
                  <div class="mt-1 text-xs text-gray-500">{{ seriesPoints.length }} 个采样点</div>
                </div>
                <div class="mt-4 grid gap-4 xl:grid-cols-2">
                  <MetricChart
                    title="CPU 使用率"
                    color="#2563eb"
                    :points="seriesPoints"
                    value-key="cpuPercent"
                    :format-value="formatPercent"
                  />
                  <MetricChart
                    title="内存使用"
                    color="#16a34a"
                    :points="seriesPoints"
                    value-key="memoryBytes"
                    :format-value="formatBytes"
                  />
                  <MetricChart
                    title="磁盘占用"
                    color="#ca8a04"
                    :points="seriesPoints"
                    value-key="diskUsageBytes"
                    :format-value="formatBytes"
                  />
                  <MetricChart
                    title="网络下行"
                    color="#0891b2"
                    :points="seriesPoints"
                    value-key="networkRxBps"
                    :format-value="formatRate"
                  />
                  <MetricChart
                    title="网络上行"
                    color="#7c3aed"
                    :points="seriesPoints"
                    value-key="networkTxBps"
                    :format-value="formatRate"
                  />
                  <MetricChart
                    title="磁盘读取"
                    color="#ea580c"
                    :points="seriesPoints"
                    value-key="diskReadBps"
                    :format-value="formatRate"
                  />
                  <MetricChart
                    title="磁盘写入"
                    color="#dc2626"
                    :points="seriesPoints"
                    value-key="diskWriteBps"
                    :format-value="formatRate"
                  />
                </div>
              </div>
            </section>
          </div>
        </div>
      </div>
    </main>
  </div>
</template>

<script setup>
import { computed, defineComponent, h, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import api from '@/utils/api'
import Button from '@/components/ui/Button.vue'
import Card from '@/components/ui/Card.vue'
import AppHeader from '@/components/AppHeader.vue'

const MetricChart = defineComponent({
  name: 'MetricChart',
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
    const padding = { top: 20, right: 16, bottom: 28, left: 48 }

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

    return () => h(Card, { class: 'p-4' }, () => [
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
const selectedName = ref('')
const metrics = ref({ containers: [], selected: null, intervalSeconds: 60 })
const runtimeFilter = ref('')
const stateFilter = ref('')
const groupFilter = ref('')
const groups = ref([])
const groupAssignments = ref({})
const groupDataReady = ref(false)
const groupError = ref('')
const filterMenu = ref(null)
let refreshTimer = null
let metricsRequest

const rangeOptions = [
  { label: '1h', value: '1h' },
  { label: '24h', value: '24h' },
  { label: '7d', value: '7d' },
  { label: '30d', value: '30d' },
]

const containers = computed(() => metrics.value.containers || [])
const containerKey = container => container.key || container.name
const containerRuntime = container => container.runtimeType === 'docker' ? 'docker' : 'lxc'
const containerState = (container) => {
  const state = String(container.state || '').trim()
  if (!state) return 'Unknown'
  if (/^(stopped|exited|dead|created)$/i.test(state)) return 'Stopped'
  return state[0].toUpperCase() + state.slice(1).toLowerCase()
}
const availableStates = computed(() => [...new Set(['Running', 'Stopped', ...containers.value.map(containerState)])])
const groupsByID = computed(() => new Map(groups.value.map(group => [group.id, group])))
const groupsFor = container => containerRuntime(container) === 'lxc'
  ? (groupAssignments.value[container.name] || []).map(id => groupsByID.value.get(id)).filter(Boolean)
  : []
const groupColors = {
  blue: 'bg-blue-50 text-blue-700 ring-blue-200',
  emerald: 'bg-emerald-50 text-emerald-700 ring-emerald-200',
  amber: 'bg-amber-50 text-amber-800 ring-amber-200',
  violet: 'bg-violet-50 text-violet-700 ring-violet-200',
  rose: 'bg-rose-50 text-rose-700 ring-rose-200',
  cyan: 'bg-cyan-50 text-cyan-800 ring-cyan-200',
  lime: 'bg-lime-50 text-lime-800 ring-lime-200',
  orange: 'bg-orange-50 text-orange-800 ring-orange-200',
  indigo: 'bg-indigo-50 text-indigo-700 ring-indigo-200',
  slate: 'bg-slate-100 text-slate-700 ring-slate-200',
}

const activeFilterCount = computed(() => [runtimeFilter.value, stateFilter.value, groupFilter.value].filter(Boolean).length)
const visibleContainers = computed(() => containers.value.filter(container => {
  if (runtimeFilter.value && containerRuntime(container) !== runtimeFilter.value) return false
  if (stateFilter.value && containerState(container) !== stateFilter.value) return false
  if (!groupFilter.value) return true
  const assigned = groupsFor(container)
  if (groupFilter.value === 'ungrouped') return assigned.length === 0
  return assigned.some(group => group.id === groupFilter.value)
}))
const runtimeBuckets = computed(() => ['lxc', 'docker'].map(type => ({
  type, rows: visibleContainers.value.filter(container => containerRuntime(container) === type),
})).filter(bucket => bucket.rows.length))
const currentKey = computed(() => selectedName.value)
const currentSummary = computed(() => visibleContainers.value.find(container => containerKey(container) === currentKey.value))
const seriesPoints = computed(() => {
  const selected = metrics.value.selected
  return currentSummary.value && (selected?.key || selected?.name) === currentKey.value ? selected.points || [] : []
})

const resetFilters = () => {
  runtimeFilter.value = ''
  stateFilter.value = ''
  groupFilter.value = ''
}

const closeFilters = () => {
  if (!filterMenu.value?.open) return
  filterMenu.value.open = false
  filterMenu.value.querySelector('summary')?.focus()
}

const closeFiltersOutside = (event) => {
  const menu = filterMenu.value
  if (menu?.open && !menu.contains(event.target)) menu.open = false
}

const loadGroups = async () => {
  try {
    const { data } = await api.get('/lxc/groups', { timeout: 10000 })
    groups.value = data.groups || []
    groupAssignments.value = data.containerGroups || {}
    if (groupFilter.value && groupFilter.value !== 'ungrouped' && !groupsByID.value.has(groupFilter.value)) groupFilter.value = ''
    groupDataReady.value = true
    groupError.value = ''
  } catch {
    if (!groupDataReady.value) groupError.value = '暂时无法读取分组。'
  }
}

const loadMetrics = async () => {
  metricsRequest?.abort()
  const request = new AbortController()
  metricsRequest = request
  loading.value = true
  error.value = ''
  try {
    const response = await api.get('/lxc/metrics', {
      signal: request.signal,
      timeout: 45000,
      params: {
        range: rangeKey.value,
        name: selectedName.value,
      },
    })
    if (!request.signal.aborted) metrics.value = response.data || { containers: [], selected: null, intervalSeconds: 60 }
  } catch (err) {
    if (!request.signal.aborted) error.value = err.response?.data?.error || '加载容器监控失败'
  } finally {
    if (metricsRequest === request) loading.value = false
  }
}

const setRange = async (value) => {
  rangeKey.value = value
  await loadMetrics()
}

const selectContainer = async (name) => {
  if (selectedName.value === name) return
  selectedName.value = name
  const selected = metrics.value.selected
  if (name && (selected?.key || selected?.name) !== name) await loadMetrics()
}

watch(visibleContainers, (rows) => {
  if (rows.some(container => containerKey(container) === currentKey.value)) return
  const first = runtimeBuckets.value[0]?.rows[0]
  selectContainer(first ? containerKey(first) : '')
})

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

const formatRate = (value) => `${formatBytes(value)}/s`

const formatPercent = (value) => `${Number(value || 0).toFixed(1)}%`

const formatTime = (value) => {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleString('zh-CN', { hour12: false })
}

const runtimeLabel = (runtimeType) => {
  if (runtimeType === 'docker') return 'Docker'
  if (runtimeType === 'lxc') return 'LXC'
  return '容器'
}

onMounted(() => {
  document.addEventListener('pointerdown', closeFiltersOutside)
  const refresh = () => {
    loadMetrics()
    loadGroups()
  }
  refresh()
  refreshTimer = window.setInterval(refresh, 60 * 1000)
})

onBeforeUnmount(() => {
  document.removeEventListener('pointerdown', closeFiltersOutside)
  metricsRequest?.abort()
  if (refreshTimer) {
    window.clearInterval(refreshTimer)
  }
})
</script>
