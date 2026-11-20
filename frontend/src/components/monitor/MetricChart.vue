<script setup>
import { computed } from 'vue'
import TimeSeriesChart from './TimeSeriesChart.vue'

const props = defineProps({
  title: { type: String, required: true },
  color: { type: String, required: true },
  points: { type: Array, required: true },
  valueKey: { type: String, required: true },
  formatValue: { type: Function, required: true },
  range: { type: String, required: true },
})

const latestValue = computed(() => props.points.at(-1)?.[props.valueKey])
const series = computed(() => [{
  id: props.valueKey, name: props.title, color: props.color,
  points: props.points, valueKey: props.valueKey,
}])
</script>

<template>
  <div class="min-w-0 rounded-md border border-gray-200 bg-white p-4 shadow-sm">
    <div class="flex items-start justify-between gap-3">
      <div>
        <h3 class="text-sm font-semibold text-gray-900">{{ title }}</h3>
        <div class="mt-1 text-xs text-gray-500">{{ points.length }} 个采样点</div>
      </div>
      <div class="text-right">
        <div class="text-sm font-semibold tabular-nums text-gray-900">{{ Number.isFinite(latestValue) ? formatValue(latestValue) : '-' }}</div>
        <div class="text-xs text-gray-500">当前</div>
      </div>
    </div>
    <TimeSeriesChart class="mt-3" :label="title" :series="series" :range="range" :format-value="formatValue" />
  </div>
</template>
