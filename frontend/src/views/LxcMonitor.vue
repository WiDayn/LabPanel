<template>
  <div class="min-h-screen bg-gray-50">
    <AppHeader />

    <main class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      <div class="space-y-6">
        <div v-if="error" class="rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
          {{ error }}
        </div>

        <div class="grid items-start gap-6 lg:grid-cols-[minmax(0,320px)_minmax(0,1fr)] lg:items-stretch">
          <div class="relative min-w-0">
            <section class="flex min-w-0 flex-col overflow-hidden rounded-lg border border-gray-200 bg-white shadow-sm"
              :class="{ 'lg:absolute lg:inset-x-0 lg:top-0 lg:max-h-full': currentSummary }">
              <div class="flex shrink-0 items-center justify-between gap-3 px-4 py-3">
                <div>
                  <h2 class="text-base font-semibold">容器列表</h2>
                  <p class="text-xs tabular-nums text-gray-600">{{ visibleContainers.length }} / {{ containers.length }} 个</p>
                </div>
                <button v-if="selectedTags.size" type="button" class="min-h-7 rounded-sm text-xs font-medium text-blue-700 hover:underline focus-visible:outline focus-visible:outline-blue-500" @click="selectedTags.clear()">清除筛选</button>
              </div>
              <div class="shrink-0 border-t border-gray-100 px-4 py-3">
                <div class="flex max-h-40 flex-wrap items-start gap-1.5 overflow-y-auto p-1 -m-1" role="group" aria-label="容器标签筛选">
                  <ContainerTag
                    v-for="tag in filterTags"
                    :key="tag.id"
                    :label="tag.label"
                    :tone="tag.tone"
                    :status-dot="tag.statusDot"
                    :selected="selectedTags.has(tag.id)"
                    selectable
                    @toggle="toggleTag(tag.id)"
                  />
                </div>
              </div>
              <p v-if="groupError" class="shrink-0 border-t px-4 py-2 text-xs text-amber-800" role="status">{{ groupError }} <button type="button" class="font-medium underline" @click="loadGroups">重试</button></p>
              <div class="min-h-0 max-h-[44dvh] overflow-y-auto border-t border-gray-200" :class="{ 'lg:max-h-none': currentSummary }">
                <ul v-if="visibleContainers.length" class="divide-y divide-gray-100" aria-label="容器列表">
                  <li v-for="container in visibleContainers" :key="containerKey(container)">
                    <button type="button" :aria-pressed="currentKey === containerKey(container)"
                      class="block w-full px-4 py-3 text-left focus-visible:outline focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-blue-500"
                      :class="currentKey === containerKey(container) ? 'bg-blue-50' : 'hover:bg-gray-50'"
                      @click="selectContainer(containerKey(container))"
                    >
                      <div class="min-w-0 break-all text-sm font-semibold" :class="currentKey === containerKey(container) ? 'text-blue-800' : 'text-gray-900'">{{ container.name }}</div>
                      <div class="mt-1.5 flex flex-wrap gap-1.5">
                        <ContainerTag
                          v-for="tag in tagsByContainer.get(containerKey(container))"
                          :key="tag.id"
                          :label="tag.label"
                          :tone="tag.tone"
                          :status-dot="tag.statusDot"
                        />
                      </div>
                      <div class="mt-1.5 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs tabular-nums text-gray-600">
                        <span>CPU {{ formatPercent(container.cpuPercent) }}</span>
                        <span>内存 {{ formatBytes(container.memoryBytes) }}</span>
                      </div>
                    </button>
                  </li>
                </ul>
                <div v-else class="px-4 py-5 text-sm text-gray-600" role="status">
                  {{ loading && !containers.length ? '正在加载容器…' : selectedTags.size ? '没有同时匹配这些标签的容器' : '暂无容器监控数据' }}
                </div>
              </div>
            </section>
          </div>

          <div class="min-w-0">
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
                <div :key="currentKey" class="mt-4 grid gap-4 xl:grid-cols-2">
                  <MetricChart
                    title="CPU 使用率"
                    color="#2563eb"
                    :points="seriesPoints"
                    :range="rangeKey"
                    value-key="cpuPercent"
                    :format-value="formatPercent"
                  />
                  <MetricChart
                    title="内存使用"
                    color="#16a34a"
                    :points="seriesPoints"
                    :range="rangeKey"
                    value-key="memoryBytes"
                    :format-value="formatBytes"
                  />
                  <MetricChart
                    title="磁盘占用"
                    color="#ca8a04"
                    :points="seriesPoints"
                    :range="rangeKey"
                    value-key="diskUsageBytes"
                    :format-value="formatBytes"
                  />
                  <MetricChart
                    title="网络下行"
                    color="#0891b2"
                    :points="seriesPoints"
                    :range="rangeKey"
                    value-key="networkRxBps"
                    :format-value="formatRate"
                  />
                  <MetricChart
                    title="网络上行"
                    color="#7c3aed"
                    :points="seriesPoints"
                    :range="rangeKey"
                    value-key="networkTxBps"
                    :format-value="formatRate"
                  />
                  <MetricChart
                    title="磁盘读取"
                    color="#ea580c"
                    :points="seriesPoints"
                    :range="rangeKey"
                    value-key="diskReadBps"
                    :format-value="formatRate"
                  />
                  <MetricChart
                    title="磁盘写入"
                    color="#dc2626"
                    :points="seriesPoints"
                    :range="rangeKey"
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
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import api from '@/utils/api'
import Button from '@/components/ui/Button.vue'
import AppHeader from '@/components/AppHeader.vue'
import ContainerTag from '@/components/ui/ContainerTag.vue'
import MetricChart from '@/components/monitor/MetricChart.vue'

const loading = ref(false)
const error = ref('')
const rangeKey = ref('1h')
const selectedName = ref('')
const metrics = ref({ containers: [], selected: null, intervalSeconds: 60 })
const selectedTags = ref(new Set(['runtime:lxc']))
const groups = ref([])
const groupAssignments = ref({})
const groupDataReady = ref(false)
const groupError = ref('')
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
const groupsByID = computed(() => new Map(groups.value.map(group => [group.id, group])))
const groupsFor = container => containerRuntime(container) === 'lxc'
  ? (groupAssignments.value[container.name] || []).map(id => groupsByID.value.get(id)).filter(Boolean)
  : []
const groupColors = {
  blue: 'bg-blue-50 text-blue-700 border-blue-200',
  emerald: 'bg-emerald-50 text-emerald-700 border-emerald-200',
  amber: 'bg-amber-50 text-amber-800 border-amber-200',
  violet: 'bg-violet-50 text-violet-700 border-violet-200',
  rose: 'bg-rose-50 text-rose-700 border-rose-200',
  cyan: 'bg-cyan-50 text-cyan-800 border-cyan-200',
  lime: 'bg-lime-50 text-lime-800 border-lime-200',
  orange: 'bg-orange-50 text-orange-800 border-orange-200',
  indigo: 'bg-indigo-50 text-indigo-700 border-indigo-200',
  slate: 'bg-slate-100 text-slate-700 border-slate-200',
}

const runtimeTags = {
  lxc: { id: 'runtime:lxc', label: 'LXC', tone: 'border-blue-700 bg-blue-700 text-white' },
  docker: { id: 'runtime:docker', label: 'Docker', tone: 'border-violet-700 bg-violet-700 text-white' },
}
const stateStyles = {
  Running: { tone: 'border-green-500 bg-white text-green-800', statusDot: 'bg-green-600' },
  Stopped: { tone: 'border-amber-500 bg-white text-amber-900', statusDot: 'bg-amber-600' },
}
const otherStateStyle = { tone: 'border-gray-400 bg-white text-gray-700', statusDot: 'bg-gray-500' }
const stateTag = state => ({
  id: `state:${state}`,
  label: state,
  ...(stateStyles[state] || otherStateStyle),
})
const groupTag = group => ({
  id: `group:${group.id}`,
  label: group.name,
  tone: groupColors[group.color] || groupColors.slate,
})

const tagsByContainer = computed(() => new Map(containers.value.map(container => {
  return [containerKey(container), [
    runtimeTags[containerRuntime(container)],
    stateTag(containerState(container)),
    ...groupsFor(container).map(groupTag),
  ]]
})))

const filterTags = computed(() => {
  const activeStates = [...selectedTags.value].filter(id => id.startsWith('state:')).map(id => id.slice(6))
  const states = new Set(['Running', 'Stopped', ...containers.value.map(containerState), ...activeStates])
  return [...Object.values(runtimeTags), ...[...states].map(stateTag), ...groups.value.map(groupTag)]
})

const visibleContainers = computed(() => containers.value.filter(container => {
  const tags = tagsByContainer.value.get(containerKey(container))
  return [...selectedTags.value].every(id => tags.some(tag => tag.id === id))
}))
const currentKey = computed(() => selectedName.value)
const currentSummary = computed(() => visibleContainers.value.find(container => containerKey(container) === currentKey.value))
const seriesPoints = computed(() => {
  const selected = metrics.value.selected
  return currentSummary.value && (selected?.key || selected?.name) === currentKey.value ? selected.points || [] : []
})

const toggleTag = (id) => {
  if (selectedTags.value.has(id)) selectedTags.value.delete(id)
  else selectedTags.value.add(id)
}

const loadGroups = async () => {
  try {
    const { data } = await api.get('/lxc/groups', { timeout: 10000 })
    groups.value = data.groups || []
    groupAssignments.value = data.containerGroups || {}
    for (const id of selectedTags.value) {
      if (id.startsWith('group:') && !groupsByID.value.has(id.slice(6))) selectedTags.value.delete(id)
    }
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
  const first = rows[0]
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
  const refresh = () => {
    loadMetrics()
    loadGroups()
  }
  refresh()
  refreshTimer = window.setInterval(refresh, 60 * 1000)
})

onBeforeUnmount(() => {
  metricsRequest?.abort()
  if (refreshTimer) {
    window.clearInterval(refreshTimer)
  }
})
</script>
