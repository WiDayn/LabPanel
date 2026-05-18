<template>
  <div class="min-h-screen bg-gray-50">
    <AppHeader />

    <main class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      <div class="space-y-6">
        <Card class="p-4">
          <div class="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-end">
            <div class="flex flex-col gap-2 sm:flex-row sm:items-center">
              <select
                v-model="selectedName"
                class="h-9 rounded-md border border-gray-300 bg-white px-3 text-sm"
                @change="loadMetrics"
              >
                <option value="">自动选择容器</option>
                <option v-for="container in containers" :key="container.name" :value="container.name">
                  {{ container.name }}
                </option>
              </select>
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
            </div>
          </div>
        </Card>

        <div v-if="error" class="rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
          {{ error }}
        </div>

        <div class="grid gap-6 lg:grid-cols-[320px_1fr]">
          <Card class="p-4">
            <div class="flex items-center justify-between">
              <h2 class="text-base font-semibold">容器列表</h2>
              <span class="text-xs text-gray-500">{{ containers.length }} 个</span>
            </div>
            <div class="mt-4 space-y-2">
              <button
                v-for="container in containers"
                :key="container.name"
                type="button"
                class="w-full rounded-md border p-3 text-left transition-colors"
                :class="currentName === container.name ? 'border-blue-300 bg-blue-50' : 'border-gray-200 bg-white hover:bg-gray-50'"
                @click="selectContainer(container.name)"
              >
                <div class="flex items-center justify-between gap-3">
                  <div class="min-w-0">
                    <div class="truncate text-sm font-medium text-gray-900">{{ container.name }}</div>
                    <div class="mt-1 text-xs text-gray-500">{{ formatTime(container.updatedAt) }}</div>
                  </div>
                  <span
                    class="shrink-0 rounded px-2 py-1 text-xs"
                    :class="container.state === 'Running' ? 'bg-green-100 text-green-700' : 'bg-gray-100 text-gray-600'"
                  >
                    {{ container.state || 'Unknown' }}
                  </span>
                </div>
                <div class="mt-3 grid grid-cols-2 gap-2 text-xs text-gray-600">
                  <div>CPU {{ formatPercent(container.cpuPercent) }}</div>
                  <div>内存 {{ formatBytes(container.memoryBytes) }}</div>
                  <div>磁盘 {{ formatBytes(container.diskUsageBytes) }}</div>
                  <div>进程 {{ container.processes || 0 }}</div>
                  <div>下行 {{ formatRate(container.networkRxBps) }}</div>
                  <div>上行 {{ formatRate(container.networkTxBps) }}</div>
                </div>
              </button>
              <div v-if="!containers.length && !loading" class="rounded-md border border-gray-200 bg-gray-50 p-4 text-sm text-gray-500">
                暂无容器监控数据
              </div>
            </div>
          </Card>

          <div class="space-y-6">
            <Card class="p-4">
              <div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
                <div>
                  <h2 class="text-base font-semibold">{{ currentName || '未选择容器' }}</h2>
                  <p class="mt-1 text-sm text-gray-600">{{ currentRangeLabel }} · {{ metrics.intervalSeconds || 60 }}s</p>
                </div>
                <div v-if="currentSummary" class="grid grid-cols-2 gap-3 text-right text-xs text-gray-600 sm:grid-cols-5">
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
                    <div class="font-medium text-gray-900">{{ formatTime(currentSummary.updatedAt) }}</div>
                    <div>更新时间</div>
                  </div>
                </div>
              </div>
            </Card>

            <div class="grid gap-6 xl:grid-cols-2">
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
        </div>
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
let refreshTimer = null

const rangeOptions = [
  { label: '1h', value: '1h' },
  { label: '24h', value: '24h' },
  { label: '7d', value: '7d' },
  { label: '30d', value: '30d' },
]

const containers = computed(() => metrics.value.containers || [])
const currentName = computed(() => metrics.value.selected?.name || selectedName.value || containers.value[0]?.name || '')
const seriesPoints = computed(() => metrics.value.selected?.points || [])
const currentSummary = computed(() => containers.value.find((container) => container.name === currentName.value))
const currentRangeLabel = computed(() => rangeOptions.find((option) => option.value === rangeKey.value)?.label || rangeKey.value)

const loadMetrics = async () => {
  loading.value = true
  error.value = ''
  try {
    const response = await api.get('/lxc/metrics', {
      params: {
        range: rangeKey.value,
        name: selectedName.value,
      },
    })
    metrics.value = response.data || { containers: [], selected: null, intervalSeconds: 60 }
    if (!selectedName.value && metrics.value.selected?.name) {
      selectedName.value = metrics.value.selected.name
    }
  } catch (err) {
    error.value = err.response?.data?.error || '加载容器监控失败'
  } finally {
    loading.value = false
  }
}

const setRange = async (value) => {
  rangeKey.value = value
  await loadMetrics()
}

const selectContainer = async (name) => {
  selectedName.value = name
  await loadMetrics()
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

const formatRate = (value) => `${formatBytes(value)}/s`

const formatPercent = (value) => `${Number(value || 0).toFixed(1)}%`

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
