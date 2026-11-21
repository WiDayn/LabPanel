<script setup>
import { computed } from 'vue'

const props = defineProps({
  label: { type: String, required: true },
  text: { type: String, required: true },
  value: Number,
  limit: Number,
  caption: Boolean,
  tone: { type: String, default: 'normal' },
})

const tones = {
  normal: { value: '', fill: 'bg-blue-600' },
  high: { value: 'font-medium text-blue-900', fill: 'bg-blue-800' },
  warning: { value: 'font-semibold text-amber-800', fill: 'bg-amber-600' },
  critical: { value: 'font-semibold text-red-800', fill: 'bg-red-600' },
}
const colors = computed(() => tones[props.tone] ?? tones.normal)

const percent = computed(() => {
  if (!Number.isFinite(props.value) || props.value < 0 || !Number.isFinite(props.limit) || props.limit <= 0) return null
  return Math.min(100, 100 * props.value / props.limit)
})
</script>

<template>
  <div class="text-xs tabular-nums text-gray-800">
    <div v-if="caption" class="mb-0.5 text-gray-600">{{ label }}</div>
    <div class="whitespace-nowrap" :class="colors.value">{{ text }}</div>
    <div class="reading-track mt-1.5 h-1.5 w-full overflow-hidden rounded-full bg-gray-200"
      :class="{ invisible: percent === null }" :role="percent === null ? undefined : 'meter'"
      :aria-label="percent === null ? undefined : label"
      :aria-valuenow="percent === null ? undefined : Math.round(percent)">
      <div v-if="percent !== null" class="h-full rounded-full" :class="colors.fill"
        :style="{ width: `${percent}%` }" />
    </div>
  </div>
</template>

<style scoped>
.reading-track {
  max-width: var(--gpu-meter-max, 100%);
}
</style>
