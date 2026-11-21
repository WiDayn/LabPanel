<script setup>
import { computed, ref, watch } from 'vue'
import GpuMetricReading from './GpuMetricReading.vue'
import { formatGpuCapacity, formatGpuPercent, formatGpuTemperature, readFanPercent } from '@/utils/gpuTrends'

const props = defineProps({
  gpus: { type: Array, required: true },
  selectedUuid: { type: String, default: '' },
})
const emit = defineEmits(['select'])
const viewport = ref(null)
const width = ref(0)
const expanded = ref(new Set())

watch(viewport, (element, _, onCleanup) => {
  if (!element) return
  width.value = element.clientWidth
  const observer = new ResizeObserver(() => { width.value = element.clientWidth })
  observer.observe(element)
  onCleanup(() => observer.disconnect())
}, { flush: 'post' })

const stacked = computed(() => width.value < 510)
const showSecondary = computed(() => width.value >= 690)
const wide = computed(() => width.value >= 1080)
const hasFan = computed(() => props.gpus.some(gpu => readFanPercent(gpu.fanPercent) !== null))
const hasPowerLimit = gpu => Number.isFinite(gpu.maxPowerWatts) && gpu.maxPowerWatts > 0
const showPowerLimit = computed(() => wide.value && props.gpus.some(hasPowerLimit))
const columnCount = computed(() => 4 + (showSecondary.value ? 2 + Number(hasFan.value) : 0))

function canExpand(gpu) {
  return !showSecondary.value || (!showPowerLimit.value && hasPowerLimit(gpu))
}
function toggleDetails(uuid) {
  if (expanded.value.has(uuid)) expanded.value.delete(uuid)
  else expanded.value.add(uuid)
}

const detailsId = gpu => `gpu-status-details-${gpu.uuid}`
const formatReading = (value, formatter) => Number.isFinite(value) ? formatter(value) : 'N/A'
const formatWatts = value => `${Math.round(value)} W`
function memoryText(gpu) {
  const { memoryUsedMiB: used, memoryTotalMiB: total } = gpu
  if (used >= 1024 && total >= 1024) return `${(used / 1024).toFixed(1)} / ${formatGpuCapacity(total)}`
  return `${formatReading(used, formatGpuCapacity)} / ${formatReading(total, formatGpuCapacity)}`
}
const fanText = gpu => formatReading(readFanPercent(gpu.fanPercent), formatGpuPercent)
const atLeast90Percent = (value, limit) => Number.isFinite(value) && Number.isFinite(limit) && limit > 0 && value >= 0.9 * limit
const usageTone = (value, limit) => atLeast90Percent(value, limit) ? 'high' : 'normal'
const memoryTone = gpu => atLeast90Percent(gpu.memoryUsedMiB, gpu.memoryTotalMiB) ? 'warning' : 'normal'
function temperatureTone({ temperature, temperatureTargetCelsius, maxTemperatureCelsius }) {
  if (!Number.isFinite(temperature)) return 'normal'
  if (Number.isFinite(maxTemperatureCelsius) && maxTemperatureCelsius > 0 && temperature >= maxTemperatureCelsius) return 'critical'
  if (Number.isFinite(temperatureTargetCelsius) && temperatureTargetCelsius > 0 && temperature >= temperatureTargetCelsius - 5) return 'warning'
  return 'normal'
}
function powerText(gpu) {
  const current = formatReading(gpu.powerWatts, formatWatts)
  if (!showPowerLimit.value || !hasPowerLimit(gpu)) return current
  const limit = formatWatts(gpu.maxPowerWatts)
  return Number.isFinite(gpu.powerWatts) ? `${Math.round(gpu.powerWatts)} / ${limit}` : `N/A / ${limit}`
}
</script>

<template>
  <section class="overflow-hidden rounded-lg border border-gray-200 bg-white">
    <header class="flex items-center justify-between gap-3 border-b border-gray-200 px-4 py-3 sm:px-5">
      <h3 class="text-sm font-semibold text-gray-900">显卡状态</h3>
      <span class="text-xs text-gray-500">{{ gpus.length }} 张</span>
    </header>
    <div ref="viewport" role="region" aria-label="显卡状态列表" class="status-region overflow-x-auto" :class="{ stacked, wide }">
      <table class="status-table w-full table-fixed text-left text-sm">
        <colgroup>
          <col />
          <col :span="showSecondary ? 4 : 3" class="metric-column" />
          <col v-if="showSecondary" :span="hasFan ? 2 : 1" class="counter-column" />
        </colgroup>
        <thead class="bg-gray-50 text-xs text-gray-600" :class="{ 'sr-only': stacked }">
          <tr>
            <th scope="col" class="px-4 py-2.5">显卡</th>
            <th scope="col" class="status-metric">负载</th>
            <th scope="col" class="status-metric">温度</th>
            <th scope="col" class="status-metric">功率<span v-if="showPowerLimit"> / 上限</span></th>
            <th v-if="showSecondary" scope="col" class="status-metric">显存</th>
            <th v-if="showSecondary && hasFan" scope="col" class="status-stat">风扇</th>
            <th v-if="showSecondary" scope="col" class="status-stat">进程</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-100">
          <template v-for="gpu in gpus" :key="gpu.uuid">
            <tr class="status-summary" :class="gpu.uuid === selectedUuid ? 'bg-blue-50/80' : ''">
              <th scope="row" class="status-device px-4 py-2.5 font-normal">
                <div class="flex items-start gap-1.5">
                  <button v-if="canExpand(gpu)" type="button" class="inline-grid h-7 w-7 shrink-0 place-items-center rounded text-gray-600 hover:bg-gray-100 focus-visible:outline focus-visible:outline-2 focus-visible:outline-blue-500"
                    :aria-label="`${expanded.has(gpu.uuid) ? '收起' : '展开'} GPU ${gpu.index} 详情`"
                    :aria-expanded="expanded.has(gpu.uuid)"
                    :aria-controls="expanded.has(gpu.uuid) ? detailsId(gpu) : undefined"
                    :title="expanded.has(gpu.uuid) ? '收起详情' : '展开详情'" @click="toggleDetails(gpu.uuid)">
                    <span aria-hidden="true" class="h-1.5 w-1.5 border-b border-r border-current"
                      :class="expanded.has(gpu.uuid) ? 'rotate-45' : '-rotate-45'" />
                  </button>
                  <button type="button" class="inline-flex min-h-7 min-w-0 flex-wrap items-center gap-x-2 rounded text-left leading-snug hover:text-blue-700 focus-visible:outline focus-visible:outline-2 focus-visible:outline-blue-500"
                    :aria-pressed="gpu.uuid === selectedUuid" @click="emit('select', gpu.uuid)">
                    <span class="whitespace-nowrap text-xs font-semibold" :class="gpu.uuid === selectedUuid ? 'text-blue-700' : 'text-gray-600'">GPU {{ gpu.index }}</span>
                    <span class="break-words font-medium">{{ gpu.name }}</span>
                  </button>
                </div>
              </th>
              <td class="status-metric">
                <GpuMetricReading label="负载" :text="formatReading(gpu.utilization, formatGpuPercent)"
                  :value="gpu.utilization" :limit="100" :caption="stacked" :tone="usageTone(gpu.utilization, 100)" />
              </td>
              <td class="status-metric">
                <GpuMetricReading label="温度" :text="formatReading(gpu.temperature, formatGpuTemperature)"
                  :value="gpu.temperature" :limit="gpu.maxTemperatureCelsius" :caption="stacked" :tone="temperatureTone(gpu)" />
              </td>
              <td class="status-metric">
                <GpuMetricReading label="功率" :text="powerText(gpu)"
                  :value="gpu.powerWatts" :limit="gpu.maxPowerWatts" :caption="stacked"
                  :tone="usageTone(gpu.powerWatts, gpu.maxPowerWatts)" />
              </td>
              <td v-if="showSecondary" class="status-metric">
                <GpuMetricReading label="显存" :text="memoryText(gpu)"
                  :value="gpu.memoryUsedMiB" :limit="gpu.memoryTotalMiB" :tone="memoryTone(gpu)" />
              </td>
              <td v-if="showSecondary && hasFan" class="status-stat tabular-nums text-gray-700">{{ fanText(gpu) }}</td>
              <td v-if="showSecondary" class="status-stat tabular-nums text-gray-700">{{ gpu.processes?.length || 0 }}</td>
            </tr>
            <tr v-if="canExpand(gpu) && expanded.has(gpu.uuid)" :id="detailsId(gpu)" class="status-details bg-gray-50">
              <td :colspan="columnCount" class="py-2.5 pl-6 pr-4 sm:pl-10">
                <div class="space-y-3 border-l-2 border-gray-300 pl-3">
                  <div>
                    <h4 class="mb-1.5 text-xs font-semibold text-gray-800">运行指标</h4>
                    <dl class="flex flex-wrap gap-x-5 gap-y-2 text-xs">
                      <div v-if="!showSecondary">
                        <dt class="mb-0.5 text-gray-500">显存</dt>
                        <dd class="detail-meter">
                          <GpuMetricReading label="显存" :text="memoryText(gpu)"
                            :value="gpu.memoryUsedMiB" :limit="gpu.memoryTotalMiB" :tone="memoryTone(gpu)" />
                        </dd>
                      </div>
                      <div v-if="!showSecondary && hasFan"><dt class="text-gray-500">风扇</dt><dd class="tabular-nums">{{ fanText(gpu) }}</dd></div>
                      <div v-if="!showSecondary"><dt class="text-gray-500">进程</dt><dd class="tabular-nums">{{ gpu.processes?.length || 0 }}</dd></div>
                      <div v-if="!showPowerLimit && hasPowerLimit(gpu)"><dt class="text-gray-500">功率上限</dt><dd class="tabular-nums">{{ formatWatts(gpu.maxPowerWatts) }}</dd></div>
                    </dl>
                  </div>
                </div>
              </td>
            </tr>
          </template>
        </tbody>
      </table>
    </div>
  </section>
</template>

<style scoped>
.status-region {
  container-type: inline-size;
  --gpu-stat-width: 7.25rem;
}

.status-region.wide {
  --gpu-stat-width: 7.75rem;
}

.metric-column { width: var(--gpu-stat-width); }
.counter-column { width: 3.25rem; }

.status-stat {
  @apply whitespace-nowrap px-3 py-2.5 text-right;
}

.status-metric {
  @apply whitespace-nowrap px-2 py-2.5 text-left;
}

.detail-meter {
  --gpu-meter-max: calc(var(--gpu-stat-width) - 1rem);
  min-width: var(--gpu-meter-max);
}

.stacked .detail-meter {
  --gpu-meter-max: calc((100cqw - 3rem) / 3);
}

.stacked colgroup {
  display: none;
}

.stacked .status-table,
.stacked .status-table tbody,
.stacked .status-details,
.stacked .status-details > td {
  display: block;
}

.stacked .status-summary {
  @apply grid grid-cols-3 gap-x-2 gap-y-1 px-4 py-2.5;
}

.stacked .status-device {
  @apply col-span-full p-0;
}

.stacked .status-summary .status-metric {
  @apply px-0 py-0 text-left text-xs;
}
</style>
