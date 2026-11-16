<template>
  <div class="min-h-screen bg-gray-50">
    <AppHeader />

    <main class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 space-y-6">
      <div class="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <h1 class="text-2xl font-bold text-gray-900">端口转发</h1>
          <p class="mt-1 text-sm text-gray-600">将容器或任意可达地址的端口映射到 LabPanel 所在宿主机，无需 FRP 服务端。</p>
        </div>
        <div class="flex shrink-0 gap-2">
          <Button variant="outline" :disabled="loading || busy" @click="loadForwards">
            {{ loading ? '刷新中...' : '刷新' }}
          </Button>
          <Button :disabled="busy" @click="openEditor()">新增转发</Button>
        </div>
      </div>

      <div v-if="error" role="alert" class="rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
        {{ error }}
      </div>
      <div v-if="success" role="status" class="rounded-md border border-green-200 bg-green-50 px-4 py-3 text-sm text-green-700">
        {{ success }}
      </div>

      <section class="rounded-lg border border-blue-100 bg-blue-50 p-5">
        <h2 class="text-sm font-semibold text-blue-900">访问宿主机端口，即可连接目标服务</h2>
        <p class="mt-2 break-all font-mono text-sm text-blue-900">宿主机:18080 → 容器 10.0.3.20:8080</p>
        <p class="mt-2 text-sm text-blue-800">支持 TCP 和 UDP。配置自动保存，LabPanel 重启后恢复已启用的规则。目标地址须能从宿主机访问。</p>
      </section>

      <section class="overflow-hidden rounded-lg border border-gray-200 bg-white shadow-sm" :aria-busy="loading">
        <div class="border-b border-gray-200 px-5 py-4">
          <h2 class="text-base font-semibold text-gray-900">转发规则</h2>
          <p class="mt-1 text-xs text-gray-500">共 {{ forwards.length }} 条，{{ runningCount }} 条监听中。状态表示宿主机端口的监听状态。</p>
        </div>

        <div class="overflow-x-auto">
          <table class="w-full text-left text-sm">
            <thead class="bg-gray-50 text-xs font-medium text-gray-500">
              <tr>
                <th scope="col" class="min-w-36 px-5 py-3">名称</th>
                <th scope="col" class="px-4 py-3">协议</th>
                <th scope="col" class="min-w-44 px-4 py-3">宿主机监听</th>
                <th scope="col" class="min-w-44 px-4 py-3">目标地址</th>
                <th scope="col" class="min-w-28 px-4 py-3">状态</th>
                <th scope="col" class="px-5 py-3 text-right">操作</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100">
              <tr v-for="rule in forwards" :key="rule.id">
                <td class="max-w-56 px-5 py-4">
                  <div class="break-words font-medium text-gray-900">{{ rule.name }}</div>
                  <p v-if="rule.comment" class="mt-1 break-words text-xs text-gray-500">{{ rule.comment }}</p>
                </td>
                <td class="px-4 py-4 font-mono text-xs text-gray-600">{{ rule.protocol.toUpperCase() }}</td>
                <td class="px-4 py-4 font-mono text-xs text-gray-700">{{ endpoint(rule.listenAddress, rule.listenPort) }}</td>
                <td class="max-w-64 break-all px-4 py-4 font-mono text-xs text-gray-700">{{ endpoint(rule.targetAddress, rule.targetPort) }}</td>
                <td class="px-4 py-4">
                  <span class="inline-block whitespace-nowrap rounded px-2 py-1 text-xs" :class="statusClass(rule)">{{ statusLabel(rule) }}</span>
                  <p v-if="rule.error" class="mt-2 max-w-64 break-words text-xs text-red-600">{{ rule.error }}</p>
                </td>
                <td class="px-5 py-4">
                  <div class="flex justify-end gap-2 whitespace-nowrap">
                    <Button variant="outline" size="sm" :disabled="busy || loading" @click="toggleForward(rule)">
                      {{ busyId === rule.id ? '处理中...' : rule.enabled && rule.status !== 'error' ? '停用' : rule.enabled ? '重试' : '启用' }}
                    </Button>
                    <Button variant="outline" size="sm" :disabled="busy || loading" @click="openEditor(rule)">编辑</Button>
                    <Button variant="destructive" size="sm" :disabled="busy || loading" @click="deleteForward(rule)">删除</Button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-if="loading && !forwards.length" class="p-10 text-center text-sm text-gray-500">正在加载转发规则...</div>
        <div v-else-if="!forwards.length" class="space-y-2 p-10 text-center">
          <p class="text-sm font-medium text-gray-700">{{ error ? '未能加载转发规则' : '暂无转发规则' }}</p>
          <p class="text-sm text-gray-500">{{ error ? '请刷新重试。' : '新增规则，填写宿主机监听端口和容器或其他服务的目标地址。' }}</p>
        </div>
      </section>
    </main>

    <dialog ref="editor" aria-labelledby="forward-editor-title" class="m-auto w-[calc(100%-2rem)] max-w-2xl rounded-lg p-0 shadow-xl backdrop:bg-black/40" @cancel="cancelEditor">
      <form class="p-6" @submit.prevent="saveForward">
        <h2 id="forward-editor-title" class="text-lg font-semibold text-gray-900">{{ editingId ? '编辑转发规则' : '新增转发规则' }}</h2>
        <p class="mt-1 text-sm text-gray-500">监听端口位于本机，目标端口位于容器或指定地址。</p>

        <fieldset :disabled="saving" class="mt-5 space-y-4">
          <div class="grid gap-4 sm:grid-cols-2">
            <div>
              <label for="forward-name" class="mb-2 block text-sm font-medium text-gray-700">名称</label>
              <input id="forward-name" v-model="form.name" autofocus required maxlength="100" placeholder="例如：容器 Web 服务" class="form-input" />
            </div>
            <div>
              <label for="forward-protocol" class="mb-2 block text-sm font-medium text-gray-700">协议</label>
              <select id="forward-protocol" v-model="form.protocol" class="form-input">
                <option value="tcp">TCP</option>
                <option value="udp">UDP</option>
              </select>
            </div>
          </div>

          <div class="grid gap-4 sm:grid-cols-[1fr_10rem]">
            <div>
              <label for="forward-listen-address" class="mb-2 block text-sm font-medium text-gray-700">宿主机监听地址</label>
              <input id="forward-listen-address" v-model="form.listenAddress" required placeholder="0.0.0.0" class="form-input" aria-describedby="listen-address-help" />
            </div>
            <div>
              <label for="forward-listen-port" class="mb-2 block text-sm font-medium text-gray-700">宿主机端口</label>
              <input id="forward-listen-port" v-model="form.listenPort" type="number" min="1" max="65535" step="1" required placeholder="18080" class="form-input" />
            </div>
          </div>
          <p id="listen-address-help" class="text-xs text-gray-500">0.0.0.0 监听所有 IPv4 网卡；127.0.0.1 仅允许本机访问；IPv6 可填写 :: 或本机 IPv6 地址。</p>

          <div class="grid gap-4 sm:grid-cols-[1fr_10rem]">
            <div>
              <label for="forward-target-address" class="mb-2 block text-sm font-medium text-gray-700">目标地址</label>
              <input id="forward-target-address" v-model="form.targetAddress" required placeholder="10.0.3.20 或 service.example.com" class="form-input" aria-describedby="target-address-help" />
            </div>
            <div>
              <label for="forward-target-port" class="mb-2 block text-sm font-medium text-gray-700">目标端口</label>
              <input id="forward-target-port" v-model="form.targetPort" type="number" min="1" max="65535" step="1" required placeholder="8080" class="form-input" />
            </div>
          </div>
          <p id="target-address-help" class="text-xs text-gray-500">填写宿主机可达的容器 IP、其他 IP 或域名，不含 http:// 等协议和端口。</p>

          <div>
            <label for="forward-comment" class="mb-2 block text-sm font-medium text-gray-700">备注（可选）</label>
            <textarea id="forward-comment" v-model="form.comment" rows="2" maxlength="500" class="form-input h-auto py-2" placeholder="说明用途或对应的容器" />
          </div>
          <label class="flex cursor-pointer items-center gap-2 text-sm text-gray-700">
            <input v-model="form.enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 accent-blue-600" />
            启用此转发规则
          </label>
        </fieldset>

        <div v-if="formError" role="alert" class="mt-4 rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">{{ formError }}</div>

        <div class="mt-6 flex justify-end gap-2">
          <Button variant="outline" :disabled="saving" @click="closeEditor">取消</Button>
          <Button type="submit" :disabled="saving">{{ saving ? '保存中...' : '保存规则' }}</Button>
        </div>
      </form>
    </dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import api from '@/utils/api'
import AppHeader from '@/components/AppHeader.vue'
import Button from '@/components/ui/Button.vue'

const forwards = ref([])
const loading = ref(false)
const saving = ref(false)
const busyId = ref('')
const error = ref('')
const success = ref('')
const editor = ref(null)
const editingId = ref('')
const formError = ref('')
const emptyForm = () => ({
  name: '', protocol: 'tcp', listenAddress: '0.0.0.0', listenPort: '',
  targetAddress: '', targetPort: '', enabled: true, comment: '',
})
const form = ref(emptyForm())
const busy = computed(() => saving.value || Boolean(busyId.value))
const runningCount = computed(() => forwards.value.filter(rule => rule.status === 'running').length)

const endpoint = (address, port) => `${address.includes(':') ? `[${address}]` : address}:${port}`
const statusLabel = rule => rule.status === 'running' ? '监听中' : rule.status === 'error' ? '启动失败' : '已停用'
const statusClass = rule => rule.status === 'running'
  ? 'bg-green-100 text-green-700'
  : rule.status === 'error' ? 'bg-red-100 text-red-700' : 'bg-gray-100 text-gray-600'

const loadForwards = async () => {
  loading.value = true
  error.value = ''
  try {
    const response = await api.get('/port-forwards')
    forwards.value = response.data.forwards || []
  } catch (err) {
    error.value = err.response?.data?.error || '加载转发规则失败'
  } finally {
    loading.value = false
  }
}

const rulePayload = rule => ({
  name: rule.name.trim(),
  protocol: rule.protocol,
  listenAddress: rule.listenAddress.trim(),
  listenPort: Number(rule.listenPort),
  targetAddress: rule.targetAddress.trim(),
  targetPort: Number(rule.targetPort),
  enabled: Boolean(rule.enabled),
  comment: (rule.comment || '').trim(),
})

const openEditor = (rule) => {
  editingId.value = rule?.id || ''
  form.value = rule ? { ...rulePayload(rule), listenPort: String(rule.listenPort), targetPort: String(rule.targetPort) } : emptyForm()
  formError.value = ''
  editor.value.showModal()
}
const closeEditor = () => {
  if (!saving.value) editor.value.close()
}
const cancelEditor = event => {
  if (saving.value) event.preventDefault()
}

const saveForward = async () => {
  if (saving.value) return
  formError.value = ''
  const payload = rulePayload(form.value)
  if (!payload.name || !payload.listenAddress || !payload.targetAddress) {
    formError.value = '请填写名称、宿主机监听地址和目标地址'
    return
  }
  if (![payload.listenPort, payload.targetPort].every(port => Number.isInteger(port) && port >= 1 && port <= 65535)) {
    formError.value = '端口必须是 1 到 65535 之间的整数'
    return
  }
  if (/[\s/?#@]/.test(payload.targetAddress) || payload.targetAddress.split(':').length === 2) {
    formError.value = '目标地址仅填写 IP 或域名，协议和端口请分别填写'
    return
  }

  saving.value = true
  error.value = ''
  success.value = ''
  try {
    if (editingId.value) {
      await api.put(`/port-forwards/${encodeURIComponent(editingId.value)}`, payload)
    } else {
      await api.post('/port-forwards', payload)
    }
    success.value = editingId.value ? '转发规则已更新' : '转发规则已添加'
    editor.value.close()
    await loadForwards()
  } catch (err) {
    formError.value = err.response?.data?.error || '保存转发规则失败'
  } finally {
    saving.value = false
  }
}

const toggleForward = async (rule) => {
  if (busy.value) return
  busyId.value = rule.id
  error.value = ''
  success.value = ''
  const enabled = rule.status === 'error' ? true : !rule.enabled
  try {
    await api.put(`/port-forwards/${encodeURIComponent(rule.id)}`, { ...rulePayload(rule), enabled })
    success.value = enabled ? '转发规则已启用' : '转发规则已停用'
    await loadForwards()
  } catch (err) {
    error.value = err.response?.data?.error || '更新转发状态失败'
  } finally {
    busyId.value = ''
  }
}

const deleteForward = async (rule) => {
  if (busy.value || !window.confirm(`确定删除转发规则“${rule.name}”？删除后将停止此端口的转发。`)) return
  busyId.value = rule.id
  error.value = ''
  success.value = ''
  try {
    await api.delete(`/port-forwards/${encodeURIComponent(rule.id)}`)
    success.value = '转发规则已删除'
    await loadForwards()
  } catch (err) {
    error.value = err.response?.data?.error || '删除转发规则失败'
  } finally {
    busyId.value = ''
  }
}

onMounted(loadForwards)
</script>

<style scoped>
.form-input {
  @apply flex h-10 w-full rounded-md border border-gray-300 bg-white px-3 text-sm text-gray-900 placeholder:text-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:opacity-50;
}
textarea.form-input {
  height: auto;
}
</style>
