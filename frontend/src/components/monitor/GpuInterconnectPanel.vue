<script setup>
import { computed, ref } from 'vue'
import GpuInterconnect from './GpuInterconnect.vue'
import { gpuInterconnects } from '@/utils/gpuInterconnects'

const props = defineProps({
  gpus: { type: Array, required: true },
  topology: Object,
})

const modes = [
  { id: 'path', label: '链路' },
  { id: 'p2pRead', label: 'P2P 读' },
  { id: 'p2pWrite', label: 'P2P 写' },
]
const p2pLabels = { supported: '支持', unsupported: '不支持', unknown: '未取得' }
const p2pReasons = { CNS: '芯片组不支持', GNS: 'GPU 不支持', TNS: '拓扑不支持', NS: '驱动报告不支持', U: '驱动未确定' }
const pathLabels = {
  PIX: '经过一个 PCIe 交换机', PXB: '经过多个 PCIe 交换机', PHB: '经过 PCIe 主桥',
  NODE: '跨 PCIe 主桥，同一 NUMA 节点', SYS: '跨 NUMA 节点', SOC: '经过片上互连',
}
const mode = ref('path')
const focusedKey = ref('')
const pairKey = (source, target) => `${source}:${target}`
const gpuLabel = gpu => `GPU ${gpu.index}`
const edges = computed(() => new Map((props.topology?.pairs || []).map(edge => [pairKey(edge.sourceUuid, edge.targetUuid), edge])))
const matrix = computed(() => props.gpus.map(source => ({
  source,
  cells: props.gpus.map(target => {
    if (source.uuid === target.uuid) return null
    const key = pairKey(source.uuid, target.uuid)
    const edge = edges.value.get(key)
    return {
      key, target, source, path: edge?.path, p2pRead: edge?.p2pRead, p2pWrite: edge?.p2pWrite,
      p2pReadCode: edge?.p2pReadCode, p2pWriteCode: edge?.p2pWriteCode,
    }
  }),
})))
const pairs = computed(() => matrix.value.flatMap(row => row.cells.filter(Boolean)))
const hasTopology = computed(() => pairs.value.some(pair => edges.value.has(pair.key)))
const focusedPair = computed(() => pairs.value.find(pair => pair.key === focusedKey.value)
  || pairs.value.find(pair => edges.value.has(pair.key)))
const visiblePaths = computed(() => [...new Set(pairs.value.map(pair => pair.path).filter(Boolean))])
const traffic = computed(() => props.gpus.map(gpu => ({ gpu, links: gpuInterconnects(gpu) })))
const hasTraffic = computed(() => traffic.value.some(row => row.links.length))

const pathDescription = code => /^NV\d+$/.test(code) ? `${code.slice(2)} 条 NVLink` : pathLabels[code] || ''
function cellLabel(pair) {
  const value = pair[mode.value]
  return mode.value === 'path' ? value || '未取得' : p2pLabels[value] || p2pLabels.unknown
}
function cellTone(pair) {
  const value = pair[mode.value]
  if (mode.value === 'path') return value?.startsWith('NV') ? 'text-blue-800' : value ? 'text-gray-800' : 'text-gray-500'
  return value === 'supported' ? 'text-emerald-800' : value === 'unsupported' ? 'text-gray-800' : 'text-gray-500'
}
</script>

<template>
  <div class="min-w-0 space-y-6">
    <section class="overflow-hidden rounded-lg border border-gray-200 bg-white">
      <header class="flex flex-wrap items-center justify-between gap-2 border-b border-gray-200 px-4 py-3 sm:px-5">
        <div class="flex items-baseline gap-2">
          <h3 class="text-sm font-semibold text-gray-900">GPU 拓扑</h3>
          <span v-if="gpus.length > 1" class="text-xs text-gray-600">宿主机</span>
        </div>
        <div v-if="hasTopology" class="inline-flex gap-0.5 rounded-md bg-gray-100 p-0.5" role="group" aria-label="拓扑视图">
          <button v-for="option in modes" :key="option.id" type="button" :aria-pressed="mode === option.id"
            class="h-7 whitespace-nowrap rounded px-2.5 text-xs font-medium focus-visible:outline focus-visible:outline-2 focus-visible:outline-blue-500"
            :class="mode === option.id ? 'bg-white text-blue-700 shadow-sm' : 'text-gray-700 hover:text-gray-900'"
            @click="mode = option.id">{{ option.label }}</button>
        </div>
      </header>
      <p v-if="gpus.length < 2" class="px-4 py-4 text-sm text-gray-600 sm:px-5">当前只有一张显卡，无卡间拓扑。</p>
      <p v-else-if="!hasTopology" class="px-4 py-4 text-sm text-gray-600 sm:px-5">未取得 GPU 拓扑和 P2P 数据。</p>
      <div v-else class="lg:grid lg:grid-cols-[minmax(0,1fr)_15rem]">
        <div class="min-w-0 p-4 sm:p-5">
          <div class="overflow-x-auto rounded-md border border-gray-200" role="region" aria-label="GPU 拓扑矩阵" tabindex="0">
            <table class="w-full table-fixed border-separate border-spacing-0 text-center text-xs" :style="{ minWidth: (1 + gpus.length) * 3.75 + 'rem' }">
              <thead class="bg-gray-50 text-gray-700">
                <tr>
                  <th scope="col" class="w-[3.75rem] border-b border-r border-gray-200 px-1 py-2 font-medium">源 / 目标</th>
                  <th v-for="gpu in gpus" :key="gpu.uuid" scope="col" class="border-b border-gray-200 px-1 py-2 font-medium">{{ gpuLabel(gpu) }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="row in matrix" :key="row.source.uuid" class="[&:not(:last-child)>*]:border-b">
                  <th scope="row" class="border-r border-gray-200 bg-gray-50 px-1 py-2 font-medium text-gray-700">{{ gpuLabel(row.source) }}</th>
                  <td v-for="(pair, i) in row.cells" :key="gpus[i].uuid" class="border-gray-100 p-0.5">
                    <button v-if="pair" type="button" class="h-8 w-full whitespace-nowrap rounded text-xs font-medium focus-visible:outline focus-visible:outline-2 focus-visible:outline-blue-500"
                      :class="[cellTone(pair), focusedPair?.key === pair.key ? 'bg-blue-50 ring-1 ring-inset ring-blue-200' : 'hover:bg-gray-100']"
                      :aria-label="`${gpuLabel(pair.source)} 到 ${gpuLabel(pair.target)}：${cellLabel(pair)}`"
                      :aria-pressed="focusedPair?.key === pair.key" @click="focusedKey = pair.key">{{ cellLabel(pair) }}</button>
                    <div v-else class="flex h-8 items-center justify-center rounded bg-gray-50 text-gray-500" aria-label="同一张显卡">—</div>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <div v-if="mode === 'path'" class="mt-3 flex flex-wrap gap-x-4 gap-y-1 text-xs text-gray-600">
            <span v-for="code in visiblePaths" :key="code">
              <span class="font-mono font-medium text-gray-700">{{ code }}</span>
              {{ pathDescription(code) }}
            </span>
          </div>
        </div>
        <aside v-if="focusedPair" class="min-w-0 border-t border-gray-200 bg-gray-50/60 p-4 sm:p-5 lg:border-l lg:border-t-0" aria-label="连接详情">
          <h4 class="text-xs font-semibold text-gray-600">连接详情</h4>
          <p class="mt-1.5 text-sm font-semibold tabular-nums text-gray-900">{{ gpuLabel(focusedPair.source) }} → {{ gpuLabel(focusedPair.target) }}</p>
          <dl class="mt-3 grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-2 text-xs">
            <dt class="text-gray-600">链路</dt>
            <dd class="text-gray-900">{{ focusedPair.path || '未取得' }}<span v-if="pathDescription(focusedPair.path)" class="ml-1 text-gray-600">{{ pathDescription(focusedPair.path) }}</span></dd>
            <dt class="text-gray-600">P2P 读</dt>
            <dd :class="focusedPair.p2pRead === 'supported' ? 'text-emerald-800' : 'text-gray-800'">
              {{ p2pLabels[focusedPair.p2pRead] || p2pLabels.unknown }}
              <span v-if="p2pReasons[focusedPair.p2pReadCode]" class="text-gray-600">· {{ p2pReasons[focusedPair.p2pReadCode] }}</span>
            </dd>
            <dt class="text-gray-600">P2P 写</dt>
            <dd :class="focusedPair.p2pWrite === 'supported' ? 'text-emerald-800' : 'text-gray-800'">
              {{ p2pLabels[focusedPair.p2pWrite] || p2pLabels.unknown }}
              <span v-if="p2pReasons[focusedPair.p2pWriteCode]" class="text-gray-600">· {{ p2pReasons[focusedPair.p2pWriteCode] }}</span>
            </dd>
          </dl>
        </aside>
      </div>
    </section>

    <section class="overflow-hidden rounded-lg border border-gray-200 bg-white">
      <header class="flex flex-wrap items-baseline gap-x-2 gap-y-1 border-b border-gray-200 px-4 py-3 sm:px-5">
        <h3 class="text-sm font-semibold text-gray-900">链路流量</h3>
        <span class="text-xs text-gray-600">按 GPU 统计</span>
      </header>
      <p v-if="!hasTraffic" class="px-4 py-4 text-sm text-gray-600 sm:px-5">未取得 PCIe 或 NVLink 读数。</p>
      <div v-else class="traffic-region" role="region" aria-label="GPU 链路流量">
        <table class="traffic-table w-full table-fixed text-left text-sm">
          <thead class="traffic-header bg-gray-50 text-xs text-gray-700">
            <tr>
              <th scope="col" class="w-[37%] px-4 py-2.5 sm:px-5">显卡</th>
              <th scope="col" class="px-4 py-2.5 sm:px-5">
                <div class="grid grid-cols-[var(--gpu-link-columns)] gap-x-2">
                  <span>协议</span><span>链路</span>
                  <abbr title="GPU 接收" class="text-right no-underline">RX</abbr>
                  <abbr title="GPU 发送" class="text-right no-underline">TX</abbr>
                </div>
              </th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100">
            <tr v-for="{ gpu, links } in traffic" :key="gpu.uuid">
              <th scope="row" class="px-4 py-2.5 text-left font-normal sm:px-5">
                <div class="flex flex-wrap items-baseline gap-x-2">
                  <span class="whitespace-nowrap text-xs font-semibold text-gray-600">{{ gpuLabel(gpu) }}</span>
                  <span class="break-words font-medium text-gray-900">{{ gpu.name }}</span>
                </div>
              </th>
              <td class="traffic-metrics px-4 py-2.5 sm:px-5"><GpuInterconnect :links="links" /></td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  </div>
</template>

<style scoped>
.traffic-region {
  container: gpu-traffic / inline-size;
  --gpu-link-columns: 4rem minmax(5.5rem, 1fr) repeat(2, minmax(5.75rem, 0.8fr));
}

@container gpu-traffic (width < 640px) {
  .traffic-table,
  .traffic-table tbody,
  .traffic-table tr,
  .traffic-table th,
  .traffic-table td {
    display: block;
  }

  .traffic-header {
    @apply sr-only;
  }

  .traffic-metrics {
    @apply pt-0;
  }
}
</style>
