<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { init, use } from 'echarts/core'
import { LineChart } from 'echarts/charts'
import { DataZoomComponent, GridComponent, MarkLineComponent, TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'

use([LineChart, DataZoomComponent, GridComponent, MarkLineComponent, TooltipComponent, CanvasRenderer])

const props = defineProps({
  label: { type: String, required: true },
  series: { type: Array, required: true },
  range: { type: String, required: true },
  formatValue: { type: Function, required: true },
  yAxisMax: Number,
  yAxisInterval: Number,
  referenceLines: { type: Array, default: () => [] },
  emptyText: { type: String, default: '等待采样数据' },
})

const plot = ref(null)
const zoomWindow = ref(null)
const reducedMotion = matchMedia('(prefers-reduced-motion: reduce)')
const dataSeries = computed(() => props.series.map(({ id, name, color, points, valueKey, lineType }) => {
  const samples = new Map()
  for (const point of points) {
    const time = point.timestamp ? new Date(point.timestamp).getTime() : NaN
    if (Number.isFinite(time)) {
      const value = point[valueKey]
      samples.set(time, Number.isFinite(value) ? value : null)
    }
  }
  const data = [...samples].sort(([a], [b]) => a - b)
  const count = data.filter(([, value]) => value !== null).length
  return { id, name, color, data, count, lineType }
}))
const hasData = computed(() => dataSeries.value.some(({ count }) => count > 0))
const timeExtent = computed(() => {
  const times = dataSeries.value.filter(({ data }) => data.length).map(({ data }) => [data[0][0], data.at(-1)[0]])
  return [Math.min(...times.map(([start]) => start)), Math.max(...times.map(([, end]) => end))]
})

const clock = new Intl.DateTimeFormat('zh-CN', { hour: '2-digit', minute: '2-digit', hour12: false })
const dateClock = new Intl.DateTimeFormat('zh-CN', { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit', hour12: false })
const fullDate = new Intl.DateTimeFormat('zh-CN', { dateStyle: 'medium', timeStyle: 'medium', hour12: false })
const formatAxisTime = time => (props.range === '1h' ? clock : dateClock).format(time)
const formatValue = value => Number.isFinite(value) ? props.formatValue(value) : '-'

let chart
let observer

function rememberZoom() {
  const [{ start, end, startValue, endValue }] = chart.getOption().dataZoom
  const zoomed = start > 0.001 || end < 99.999
  zoomWindow.value = zoomed && startValue < endValue ? [startValue, endValue] : null
}

function resetZoom() {
  chart?.dispatchAction({ type: 'hideTip' })
  chart?.dispatchAction({ type: 'dataZoom', start: 0, end: 100 })
}

function zoomOptions() {
  const [from, to] = timeExtent.value
  if (zoomWindow.value) {
    const start = Math.max(from, zoomWindow.value[0])
    const end = Math.min(to, zoomWindow.value[1])
    zoomWindow.value = start < end ? [start, end] : null
  }
  const range = zoomWindow.value
    ? { startValue: zoomWindow.value[0], endValue: zoomWindow.value[1], rangeMode: ['value', 'value'] }
    : { start: 0, end: 100, startValue: null, endValue: null, rangeMode: ['percent', 'percent'] }
  const common = { xAxisIndex: 0, filterMode: 'none', ...range }
  return [
    {
      ...common, id: 'slider', type: 'slider', show: from < to, height: 17, bottom: 7,
      showDetail: false, brushSelect: false, borderColor: '#e2e8f0',
      fillerColor: 'rgba(37, 99, 235, 0.13)', handleStyle: { borderColor: '#64748b' },
    },
    {
      ...common, id: 'inside', type: 'inside', disabled: from >= to,
      zoomOnMouseWheel: 'ctrl', moveOnMouseWheel: false,
    },
  ]
}

function updateChart() {
  if (!hasData.value) {
    chart?.clear()
    zoomWindow.value = null
    return
  }
  if (!chart) {
    if (!plot.value?.clientWidth) return
    chart = init(plot.value, null, { renderer: 'canvas' })
    chart.on('datazoom', rememberZoom)
  }
  const referenceSeries = props.referenceLines.length ? dataSeries.value.findIndex(series => series.count > 0) : -1
  const referenceMax = props.referenceLines.length ? Math.max(...props.referenceLines) : null
  chart.setOption({
    animationDuration: 0,
    animationDurationUpdate: reducedMotion.matches ? 0 : 200,
    animationEasingUpdate: 'cubicOut',
    grid: { left: 10, right: 12, top: 17, bottom: 43, outerBounds: { left: 6, right: 6, top: 2, bottom: 29 } },
    tooltip: {
      trigger: 'axis', confine: true, transitionDuration: 0,
      axisPointer: { type: 'line', snap: true }, valueFormatter: formatValue,
    },
    xAxis: {
      type: 'time', min: 'dataMin', max: 'dataMax', splitNumber: 4,
      axisLine: { lineStyle: { color: '#cbd5e1' } }, axisTick: { show: false },
      axisLabel: { color: '#475569', fontSize: 11, hideOverlap: true, formatter: formatAxisTime },
      axisPointer: { label: { formatter: ({ value }) => fullDate.format(value) } },
    },
    yAxis: {
      type: 'value', splitNumber: 4,
      interval: props.yAxisInterval,
      max: props.yAxisMax ?? (referenceMax === null ? null : ({ max }) => Math.max(max, referenceMax)),
      axisLabel: { color: '#475569', fontSize: 11, formatter: formatValue, hideOverlap: true, showMaxLabel: props.yAxisMax !== undefined || undefined },
      splitLine: { lineStyle: { color: '#e8edf4' } },
    },
    dataZoom: zoomOptions(),
    series: dataSeries.value.map(({ id, name, color, data, count, lineType }, index) => ({
      id, name, type: 'line', data, encode: { x: 0, y: 1, tooltip: [1] },
      connectNulls: false, showSymbol: count === 1, symbol: 'circle', symbolSize: 5,
      lineStyle: { width: 2, color, type: lineType || 'solid' }, itemStyle: { color },
      markLine: index === referenceSeries ? {
        silent: true, symbol: 'none', label: { show: false },
        lineStyle: { type: 'dashed', color: '#b45309', width: 1.5 },
        data: props.referenceLines.map(value => ({ yAxis: value })),
      } : { data: [] },
    })),
  }, { replaceMerge: ['series'] })
}

watch(() => props.range, () => { zoomWindow.value = null }, { flush: 'sync' })
watch([dataSeries, () => props.range, () => props.yAxisMax, () => props.yAxisInterval, () => props.referenceLines], updateChart, { flush: 'post' })

onMounted(() => {
  observer = new ResizeObserver(() => {
    if (!plot.value?.clientWidth) return
    if (chart) chart.resize()
    else updateChart()
  })
  observer.observe(plot.value)
  updateChart()
})

onBeforeUnmount(() => {
  observer?.disconnect()
  chart?.dispose()
})
</script>

<template>
  <div class="relative min-w-0 rounded-md border border-gray-200 bg-white">
    <div ref="plot" class="h-56 w-full" role="img" :aria-label="label" :aria-hidden="!hasData" />
    <div v-if="!hasData" class="absolute inset-0 flex items-center justify-center text-sm text-gray-500">
      {{ emptyText }}
    </div>
    <button
      v-if="zoomWindow"
      type="button"
      class="absolute right-2 top-2 z-10 rounded border border-gray-200 bg-white px-2 py-1 text-xs text-gray-700 shadow-sm hover:bg-gray-50 focus-visible:outline focus-visible:outline-2 focus-visible:outline-blue-500"
      @click="resetZoom"
    >重置缩放</button>
  </div>
</template>
