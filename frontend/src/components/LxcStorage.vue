<script setup>
import SortableHeader from '@/components/ui/SortableHeader.vue'
import { useTableSort } from '@/utils/tableSort'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import api from '@/utils/api'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'

const props = defineProps({
  containers: { type: Array, required: true },
  target: { type: Object, default: null },
})
const emit = defineEmits(['pools', 'refresh', 'close'])
const pools = ref([])
const mountpoints = ref([])
const mountError = ref('')
const moves = ref([])
const mounts = ref([])
const loading = ref(false)
const loadingMounts = ref(false)
const saving = ref(false)
const moving = ref(false)
const addPool = ref(false)
const error = ref('')
const moveError = ref('')
const mountCheckFailed = ref(false)
const pollError = ref('')
const poolName = ref('')
const mountpoint = ref('')
const destination = ref('')
const moveDialog = ref(null)
let pollTimer
let disposed = false

const destinations = computed(() => pools.value.filter(pool => pool.name !== props.target?.storagePool))
const counts = computed(() => {
  const result = new Map()
  for (const container of props.containers) {
    result.set(container.storagePool, (result.get(container.storagePool) || 0) + 1)
  }
  return result
})
const formatSize = (value) => {
  if (value == null) return '-'
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB', 'PiB']
  const exponent = Math.min(units.length - 1, Math.floor(Math.log(Math.max(1, value)) / Math.log(1024)))
  return `${(value / 1024 ** exponent).toLocaleString('zh-CN', { maximumFractionDigits: 1 })} ${units[exponent]}`
}

async function loadStorage() {
  loading.value = true
  error.value = ''
  try {
    const { data } = await api.get('/lxc/storage', { timeout: 30000 })
    pools.value = data.pools || []
    mountpoints.value = data.mounts || []
    mountError.value = data.mountError || ''
    emit('pools', pools.value)
  } catch (err) {
    error.value = err.response?.data?.error || '无法加载存储池'
  } finally {
    loading.value = false
  }
}

async function createPool() {
  saving.value = true
  error.value = ''
  try {
    await api.post('/lxc/storage/pools', { name: poolName.value.trim(), mount: mountpoint.value }, { timeout: 40000 })
    addPool.value = false
    poolName.value = ''
    mountpoint.value = ''
    await loadStorage()
  } catch (err) {
    error.value = err.response?.data?.error || '创建存储池失败，请刷新确认结果'
  } finally {
    saving.value = false
  }
}

async function loadMoves() {
  try {
    const { data } = await api.get('/lxc/storage/moves', { timeout: 5000 })
    const next = data.moves || []
    if (next.some(move => move.status !== 'running' && moves.value.some(previous => previous.name === move.name && previous.status === 'running'))) {
      emit('refresh')
      loadStorage()
    }
    moves.value = next
    pollError.value = ''
  } catch {
    if (moves.value.some(move => move.status === 'running')) pollError.value = '暂时无法读取迁移状态，正在重试'
  } finally {
    if (!disposed) pollTimer = window.setTimeout(loadMoves, 3000)
  }
}

async function submitMove() {
  if (!props.target || !destination.value || moving.value) return
  moving.value = true
  moveError.value = ''
  try {
    await api.post('/lxc/storage/moves', { name: props.target.name, pool: destination.value }, { timeout: 30000 })
    emit('close')
  } catch (err) {
    moveError.value = err.response?.data?.error || '无法确认迁移任务，请查看任务状态'
  } finally {
    moving.value = false
  }
}

watch(() => props.target, async (target, _, onCleanup) => {
  if (!target) {
    moveDialog.value?.close()
    return
  }
  let stale = false
  onCleanup(() => { stale = true })
  destination.value = ''
  mounts.value = []
  moveError.value = ''
  mountCheckFailed.value = false
  loadingMounts.value = true
  moveDialog.value?.showModal()
  try {
    const { data } = await api.get(`/lxc/storage/mounts/${encodeURIComponent(target.name)}`, { timeout: 15000 })
    if (!stale) mounts.value = data.mounts || []
  } catch (err) {
    if (!stale) {
      moveError.value = err.response?.data?.error || '无法核对当前挂载'
      mountCheckFailed.value = true
    }
  } finally {
    if (!stale) loadingMounts.value = false
  }
})

onMounted(() => {
  loadStorage()
  loadMoves()
})
onBeforeUnmount(() => {
  disposed = true
  window.clearTimeout(pollTimer)
})
const poolTable = useTableSort(() => pools.value, { location: row => row.source || row.driver, count: row => counts.value.get(row.name) || 0 })
</script>

<template>
  <section class="overflow-hidden rounded-lg border border-gray-200 bg-white shadow-sm">
    <div class="flex flex-wrap items-center justify-between gap-3 px-5 py-4">
      <h2 class="text-base font-semibold text-gray-900">容器存储池</h2>
      <div class="flex gap-2">
        <Button size="sm" :disabled="loading || !mountpoints.length" @click="addPool = !addPool">添加存储池</Button>
        <Button size="sm" variant="outline" :disabled="loading" @click="loadStorage">刷新</Button>
      </div>
    </div>
    <div class="border-t border-gray-200 px-5 py-4">
      <p v-if="error" role="alert" class="mb-3 text-sm text-red-700">{{ error }}</p>
      <p v-if="mountError" class="mb-3 text-sm text-gray-600">{{ mountError }}</p>
      <p v-else-if="!loading && !mountpoints.length" class="mb-3 text-sm text-gray-600">没有可用于新增存储池的本地挂载磁盘。</p>
      <form v-if="addPool" class="mb-4 flex flex-wrap items-end gap-3 border-b pb-4" @submit.prevent="createPool">
        <label class="min-w-0 flex-1 text-sm font-medium">
          名称
          <Input v-model="poolName" class="mt-1" maxlength="63" required />
        </label>
        <label class="min-w-0 flex-1 text-sm font-medium">
          挂载点
          <select v-model="mountpoint" required class="mt-1 block h-10 w-full rounded-md border border-gray-300 bg-white px-3">
            <option disabled value="">选择磁盘</option>
            <option v-for="path in mountpoints" :key="path" :value="path">{{ path }}</option>
          </select>
        </label>
        <Button type="submit" :disabled="saving">{{ saving ? '创建中…' : '创建' }}</Button>
      </form>
      <p v-if="!pools.length && loading" class="text-sm text-gray-600">正在读取存储池…</p>
      <p v-else-if="!pools.length && !error" class="text-sm text-gray-600">暂无存储池</p>
      <div v-if="pools.length" class="panel-table-frame">
        <table class="panel-table w-full min-w-[520px] text-left text-sm">
          <thead>
            <tr><SortableHeader :sort="poolTable" column="name" label="名称" class="py-2 pr-3" /><SortableHeader :sort="poolTable" column="location" label="位置" class="py-2 pr-3" /><SortableHeader :sort="poolTable" column="availableBytes" label="可用 / 总计" numeric class="py-2 pr-3" /><SortableHeader :sort="poolTable" column="count" label="容器" numeric class="py-2 text-right" /></tr>
          </thead>
          <tbody>
            <tr v-for="pool in poolTable.rows" :key="pool.name" class="border-t">
              <td class="break-all py-2 pr-3 font-medium">{{ pool.name }}</td>
              <td class="break-all py-2 pr-3 text-gray-600">{{ pool.source || pool.driver }}</td>
              <td class="whitespace-nowrap py-2 pr-3 text-right tabular-nums">{{ formatSize(pool.availableBytes) }} / {{ formatSize(pool.totalBytes) }}</td>
              <td class="py-2 text-right tabular-nums">{{ counts.get(pool.name) || 0 }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <div v-if="moves.length" class="mt-4 space-y-2 border-t pt-3" aria-live="polite">
        <p v-if="pollError" class="text-sm text-amber-800">{{ pollError }}</p>
        <div v-for="move in moves" :key="move.name" class="flex flex-wrap items-baseline gap-x-3 gap-y-1 text-sm">
          <span class="break-all font-medium">{{ move.name }}</span>
          <span class="break-all text-gray-600">{{ move.sourcePool }} → {{ move.targetPool }}</span>
          <span :class="move.status === 'failed' ? 'text-red-700' : move.status === 'completed' ? 'text-green-700' : 'text-blue-700'">{{ move.message }}</span>
        </div>
      </div>
    </div>
  </section>

  <Teleport to="body">
    <dialog ref="moveDialog" class="w-[calc(100%-2rem)] max-w-lg rounded-lg border p-0 shadow-lg backdrop:bg-black/50" aria-labelledby="lxc-move-title" @cancel.prevent="!moving && emit('close')">
      <form v-if="target" class="space-y-4 p-5" @submit.prevent="submitMove">
        <h2 id="lxc-move-title" class="text-base font-semibold">迁移容器</h2>
        <p class="break-all text-sm text-gray-700">{{ target.name }} · 当前存储池 {{ target.storagePool }}</p>
        <label class="block text-sm font-medium">
          目标存储池
          <select v-model="destination" required class="mt-1 block h-10 w-full rounded-md border border-gray-300 bg-white px-3" :disabled="moving">
            <option disabled value="">{{ destinations.length ? '选择存储池' : '暂无其他存储池' }}</option>
            <option v-for="pool in destinations" :key="pool.name" :value="pool.name">{{ pool.name }} · 可用 {{ formatSize(pool.availableBytes) }}</option>
          </select>
        </label>
        <p class="text-sm text-gray-600">容器文件及快照将迁往目标池。运行中的容器会暂停服务，验证通过后重新启动。</p>
        <p v-if="loadingMounts" class="text-sm text-gray-600">正在核对挂载…</p>
        <div v-if="mounts.length" class="space-y-1 border-t pt-3 text-sm">
          <h3 class="font-medium">保留在原位置的挂载</h3>
          <ul class="space-y-1 text-gray-600">
            <li v-for="mount in mounts" :key="mount.name" class="break-all">{{ mount.pool ? `${mount.pool} / ` : '' }}{{ mount.source }} → {{ mount.path }}</li>
          </ul>
        </div>
        <p v-if="moveError" role="alert" class="text-sm text-red-700">{{ moveError }}</p>
        <div class="flex justify-end gap-2">
          <Button type="button" variant="outline" :disabled="moving" @click="emit('close')">取消</Button>
          <Button type="submit" :disabled="!destination || loadingMounts || moving || mountCheckFailed">{{ moving ? '提交中…' : '开始迁移' }}</Button>
        </div>
      </form>
    </dialog>
  </Teleport>
</template>
