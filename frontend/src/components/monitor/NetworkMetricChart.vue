<script setup>
import { computed } from 'vue'
import TimeSeriesChart from './TimeSeriesChart.vue'

const props = defineProps({
  title: { type: String, required: true },
  points: { type: Array, required: true },
  summary: { type: Object, required: true },
  range: { type: String, required: true },
  direction: { type: String, required: true },
  formatValue: { type: Function, required: true },
})

const palette = ['#2563eb', '#047857', '#b45309', '#7c3aed', '#be123c', '#0e7490', '#4d7c0f', '#475569']
const connected = computed(() => (props.summary.networkPorts || []).flatMap((port, index) => {
  if (!port.linkUp) return []
  return [{
    name: port.name,
    color: palette[index] || `hsl(${(index * 137.5) % 360} 61% 35%)`,
    latest: port[`${props.direction}Bps`],
  }]
}))
const series = computed(() => {
  const samples = props.points.map(point => ({
    timestamp: point.timestamp,
    ports: new Map((point.networkPorts || []).map(port => [port.name, port])),
  }))
  return connected.value.map(({ name, color }) => ({
    id: name, name, color, valueKey: 'rate',
    points: samples.map(({ timestamp, ports }) => {
      const port = ports.get(name)
      return { timestamp, rate: port?.linkUp ? port[`${props.direction}Bps`] : null }
    }),
  }))
})
const total = computed(() => props.summary[props.direction === 'rx' ? 'networkRxBps' : 'networkTxBps'])
const emptyText = computed(() => {
  if (connected.value.length || !Array.isArray(props.summary.networkPorts)) return '等待网口采样数据'
  return '暂无链路已连接的物理网口'
})
</script>

<template>
  <div class="min-w-0 rounded-md border border-gray-200 bg-white p-4 shadow-sm">
    <div class="flex items-start justify-between gap-3">
      <div>
        <h3 class="text-sm font-semibold text-gray-900">{{ title }}</h3>
        <div class="mt-1 text-xs text-gray-500">{{ connected.length }} 个已连接网口</div>
      </div>
      <div class="shrink-0 text-right">
        <div class="text-sm font-semibold tabular-nums text-gray-900">{{ Number.isFinite(total) ? formatValue(total) : '-' }}</div>
        <div class="text-xs text-gray-500">物理网口合计</div>
      </div>
    </div>
    <TimeSeriesChart
      class="mt-3" :label="title" :series="series" :range="range"
      :format-value="formatValue" :empty-text="emptyText"
    />
    <ul v-if="connected.length" class="mt-3 flex flex-wrap gap-x-4 gap-y-2" aria-label="网口图例">
      <li v-for="port in connected" :key="port.name" class="flex min-w-0 items-center gap-1.5 text-xs">
        <span class="h-2 w-2 shrink-0 rounded-full" :style="{ backgroundColor: port.color }" aria-hidden="true" />
        <span class="break-all font-medium text-gray-800">{{ port.name }}</span>
        <span class="shrink-0 tabular-nums text-gray-600">{{ Number.isFinite(port.latest) ? formatValue(port.latest) : '-' }}</span>
      </li>
    </ul>
  </div>
</template>
