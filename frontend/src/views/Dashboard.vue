<template>
  <div class="min-h-screen bg-gray-50">
    <AppHeader />

    <main class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      <div class="space-y-6">
        <Card
          v-if="environmentCheck && !environmentCheck.frp.ready"
          class="p-6 border-amber-200 bg-amber-50"
        >
          <div class="flex items-start justify-between gap-4">
            <div>
              <h2 class="text-lg font-semibold text-amber-900">FRP 尚未就绪</h2>
              <p class="mt-1 text-sm text-amber-800">{{ environmentCheck.frp.message }}</p>
              <p
                v-if="environmentCheck.frp.missingItems?.length"
                class="mt-2 text-sm text-amber-700"
              >
                缺少项目：{{ environmentCheck.frp.missingItems.join('、') }}
              </p>
            </div>
            <Button variant="outline" @click="checkEnvironment" :disabled="checkingEnvironment">
              {{ checkingEnvironment ? '检查中...' : '重新检查' }}
            </Button>
          </div>

          <div
            v-for="guide in environmentCheck.frp.guides || []"
            :key="guide.title"
            class="mt-4 rounded-lg border border-amber-200 bg-white p-4"
          >
            <h3 class="font-medium text-gray-900">{{ guide.title }}</h3>
            <p class="mt-1 text-sm text-gray-600">{{ guide.description }}</p>
            <div
              v-for="command in guide.commands || []"
              :key="`${guide.title}-${command.label}`"
              class="mt-3"
            >
              <div class="text-sm font-medium text-gray-800">{{ command.label }}</div>
              <pre class="mt-1 overflow-auto rounded bg-gray-100 p-3 text-xs text-gray-800">{{ command.command }}</pre>
            </div>
          </div>
        </Card>

        <!-- 服务状态卡片 -->
        <section
          v-if="environmentCheck?.frp.ready !== false"
          class="overflow-hidden rounded-lg border border-gray-200 bg-white shadow-sm"
        >
          <div class="flex flex-col gap-3 px-5 py-4 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <h2 class="text-base font-semibold text-gray-900">服务状态</h2>
              <div class="mt-1 flex flex-wrap items-center gap-2 text-xs text-gray-500">
                <span
                  :class="[
                    'h-2.5 w-2.5 rounded-full',
                    serviceStatus.active ? 'bg-green-500' : 'bg-red-500',
                  ]"
                ></span>
                <span class="font-medium text-gray-700">
                  {{ serviceStatus.active ? '运行中' : '已停止' }}
                </span>
                <span class="text-gray-300">·</span>
                <span>{{ serviceStatus.status }}</span>
              </div>
            </div>
            <div class="flex items-center gap-2">
              <Button variant="outline" size="sm" @click="refreshStatus" :disabled="loading">
                {{ loading ? '刷新中...' : '刷新' }}
              </Button>
              <button
                type="button"
                class="inline-flex h-8 w-8 shrink-0 items-center justify-center rounded border border-gray-200 bg-white hover:bg-gray-50"
                :aria-label="showStatusDetail ? '收起详细状态' : '展开详细状态'"
                :title="showStatusDetail ? '收起详细状态' : '展开详细状态'"
                @click="showStatusDetail = !showStatusDetail"
              >
                <span
                  class="h-0 w-0 border-x-[5px] border-t-[6px] border-x-transparent border-t-gray-500 transition-transform"
                  :class="showStatusDetail ? 'rotate-0' : '-rotate-90'"
                ></span>
              </button>
            </div>
          </div>
          <div v-show="showStatusDetail" class="border-t border-gray-200 p-4">
            <div class="mb-3">
              <h3 class="text-sm font-medium text-gray-900">详细状态</h3>
              <div class="mt-1 text-xs text-gray-500">
                {{ serviceStatus.detailCommand || frpStatusCommand }}
              </div>
            </div>
              <pre
                v-if="serviceStatus.statusDetail"
                class="max-h-64 overflow-auto rounded-md border border-gray-200 bg-gray-50 p-3 text-xs text-gray-800"
              >{{ serviceStatus.statusDetail }}</pre>
              <div v-else class="rounded-md border border-gray-200 bg-gray-50 p-4 text-sm text-gray-500">
                暂无详细状态。
              </div>
          </div>
        </section>

        <!-- 代理列表卡片 -->
        <section
          v-if="environmentCheck?.frp.ready !== false"
          class="overflow-hidden rounded-lg border border-gray-200 bg-white shadow-sm"
        >
          <div class="flex flex-col gap-3 px-5 py-4 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <h2 class="text-base font-semibold text-gray-900">代理映射</h2>
              <div class="mt-1 text-xs text-gray-500">{{ proxies.length }} 个代理</div>
            </div>
            <div class="flex flex-wrap gap-2">
              <Button v-if="pendingProxies.length" variant="outline" size="sm" :disabled="loading" @click="openBindingReview(pendingProxies)">
                批量确认（{{ pendingProxies.length }}）
              </Button>
              <Button variant="outline" size="sm" @click="loadProxies" :disabled="loading">
                {{ loading ? '重载中...' : '重载配置' }}
              </Button>
              <Button size="sm" @click="showAddDialog = true">新增代理</Button>
            </div>
          </div>
          <div class="border-t border-gray-200 p-4">
            <div v-if="error" class="mb-4 rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">{{ error }}</div>
            <div v-if="success" class="mb-4 rounded-md border border-green-200 bg-green-50 px-3 py-2 text-sm text-green-700">{{ success }}</div>

            <div v-if="proxies.length === 0" class="rounded-md border border-gray-200 bg-gray-50 p-4 text-sm text-gray-500">
              暂无代理配置
            </div>
            <div v-else class="overflow-x-auto">
              <table class="w-full border-collapse text-sm">
                <thead>
                  <tr class="border-b text-left text-xs text-gray-500">
                    <th class="px-2 py-2 font-medium">名称</th>
                    <th class="px-2 py-2 font-medium">类型</th>
                    <th class="px-2 py-2 font-medium">目标</th>
                    <th class="px-2 py-2 font-medium">本地端口</th>
                    <th class="px-2 py-2 font-medium">映射地址</th>
                    <th class="px-2 py-2 font-medium">备注</th>
                    <th class="px-2 py-2 font-medium">操作</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="(proxy, index) in proxies" :key="proxy.name" class="border-b last:border-b-0">
                    <td class="px-2 py-2 font-medium text-gray-900">{{ proxy.name }}</td>
                    <td class="px-2 py-2 text-gray-700">{{ proxy.type }}</td>
                    <td class="px-2 py-2 text-gray-700">
                      <div class="break-all">{{ proxy.binding?.container || proxy.localIP }}</div>
                      <div v-if="proxy.binding" class="mt-1 break-all text-xs" :class="proxy.paused ? 'text-amber-800' : 'text-gray-600'">
                        {{ proxy.paused ? '已暂停 · 地址归属未通过核验' : `${proxy.localIP} · ${proxy.binding.device}` }}
                      </div>
                      <div v-else class="mt-1 break-all text-xs text-gray-600">
                        <template v-if="canConfirmBinding(proxy)">
                          待确认 · {{ proxy.candidate }}
                          <button type="button" class="ml-1 min-h-6 font-medium text-blue-700 hover:underline" @click="openBindingReview([proxy])">确认绑定</button>
                        </template>
                        <template v-else>手动目标</template>
                      </div>
                    </td>
                    <td class="px-2 py-2 text-gray-700">{{ proxy.localPort }}</td>
                    <td class="px-2 py-2 text-gray-900">
                      <span v-if="frpConfig.serverAddr">
                        {{ frpConfig.serverAddr }}:{{ proxy.remotePort }}
                      </span>
                      <span v-else>{{ proxy.remotePort }}</span>
                    </td>
                    <td class="px-2 py-2 text-gray-600">{{ proxy.comment || '-' }}</td>
                    <td class="px-2 py-2">
                      <div class="flex gap-2">
                        <Button variant="outline" size="sm" @click="editProxy(index)">编辑</Button>
                        <Button variant="destructive" size="sm" @click="deleteProxy(index)">删除</Button>
                      </div>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </section>
      </div>
    </main>

    <dialog ref="bindingReview" aria-labelledby="frp-review-title" class="w-[calc(100%-2rem)] max-w-xl rounded-lg border p-0 shadow-lg backdrop:bg-black/50" @cancel.prevent="closeBindingReview">
      <form class="space-y-4 p-5" @submit.prevent="confirmBindings">
        <h2 id="frp-review-title" class="text-base font-semibold">确认容器绑定</h2>
        <p v-if="reviewLoading" role="status" class="text-sm text-gray-600">正在核对容器网卡…</p>
        <p v-if="reviewError" role="alert" class="text-sm text-red-700">{{ reviewError }}</p>
        <p v-if="reviewResult" role="status" class="text-sm text-gray-700">{{ reviewResult }}</p>
        <template v-if="reviewRows.length">
          <label v-if="reviewRows.length > 1" class="flex items-center gap-2 text-sm font-medium">
            <input type="checkbox" :checked="reviewAllSelected" :indeterminate.prop="!reviewAllSelected && selectedRules.length > 0" :disabled="confirmingBindings || !reviewTargets.length" @change="selectAllBindings($event.target.checked)" />
            全选可确认规则
          </label>
          <ul class="max-h-[45dvh] divide-y overflow-y-auto rounded-md border">
            <li v-for="row in reviewRows" :key="row.name" class="px-3 py-2">
              <label class="flex items-start gap-3 text-sm">
                <input v-model="selectedRules" class="mt-1" type="checkbox" :value="row.name" :disabled="confirmingBindings || !row.target || row.done" />
                <span class="min-w-0 break-all">
                  <span class="block font-medium">{{ row.name }} · 远端端口 {{ row.remotePort }}</span>
                  <span class="block text-gray-600">{{ row.localIP }}:{{ row.localPort }} → {{ row.candidate }}<template v-if="row.target"> · {{ row.target.device }}</template></span>
                  <span v-if="row.error" class="block text-red-700">{{ row.error }}</span>
                  <span v-else-if="row.done" class="block text-green-700">已绑定</span>
                </span>
              </label>
            </li>
          </ul>
        </template>
        <div class="flex flex-wrap justify-end gap-2">
          <Button variant="outline" :disabled="confirmingBindings" @click="closeBindingReview">关闭</Button>
          <Button type="submit" :disabled="reviewLoading || confirmingBindings || !selectedRules.length">
            {{ confirmingBindings ? '绑定中…' : `确认绑定（${selectedRules.length}）` }}
          </Button>
        </div>
      </form>
    </dialog>

    <!-- 添加/编辑代理对话框 -->
    <div
      v-if="showAddDialog || editingIndex !== null"
      class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50"
      @click.self="closeDialog"
    >
      <Card class="max-h-[calc(100dvh-2rem)] w-full max-w-md overflow-y-auto p-6 m-4">
        <h3 class="text-lg font-semibold mb-4">
          {{ editingIndex !== null ? '编辑代理' : '新增代理' }}
        </h3>
        <div class="space-y-4">
          <div>
            <label class="block text-sm font-medium mb-2">名称</label>
            <Input v-model="currentProxy.name" placeholder="name" />
          </div>
          <div>
            <label class="block text-sm font-medium mb-2">类型</label>
            <Input v-model="currentProxy.type" placeholder="tcp" />
          </div>
          <div>
            <label for="frp-target-mode" class="block text-sm font-medium mb-2">目标模式</label>
            <select id="frp-target-mode" v-model="targetMode" class="h-10 w-full rounded-md border border-gray-300 bg-white px-3 text-sm" @change="loadTargets">
              <option value="manual">手动目标</option>
              <option value="container">绑定容器</option>
            </select>
          </div>
          <div v-if="targetMode === 'manual'">
            <label for="frp-target-address" class="block text-sm font-medium mb-2">目标地址</label>
            <Input id="frp-target-address" v-model="currentProxy.localIP" placeholder="127.0.0.1" />
          </div>
          <div v-else>
            <label for="frp-target-container" class="block text-sm font-medium mb-2">容器网卡</label>
            <select id="frp-target-container" v-model="selectedTarget" class="h-10 w-full rounded-md border border-gray-300 bg-white px-3 text-sm" :disabled="loadingTargets">
              <option :value="null" disabled>{{ loadingTargets ? '正在读取…' : '请选择容器网卡' }}</option>
              <option v-for="target in containerTargets" :key="`${target.instanceId}/${target.device}/${target.address}`" :value="target">
                {{ target.container }} · {{ target.device }} · {{ target.address }}
              </option>
            </select>
            <p v-if="targetError" class="mt-2 text-sm text-red-700" role="alert">{{ targetError }}</p>
            <p v-else-if="!loadingTargets && !containerTargets.length" class="mt-2 text-sm text-gray-600">没有运行中的 LXD 容器网卡可供绑定。</p>
          </div>
          <div>
            <label class="block text-sm font-medium mb-2">本地端口</label>
            <Input v-model.number="currentProxy.localPort" type="number" placeholder="22" />
          </div>
          <div>
            <label class="block text-sm font-medium mb-2">远程端口</label>
            <Input v-model.number="currentProxy.remotePort" type="number" placeholder="20022" />
          </div>
          <div>
            <label class="block text-sm font-medium mb-2">备注</label>
            <Input v-model="currentProxy.comment" placeholder="可选，用于记录此映射的说明" />
          </div>
          <p v-if="error" role="alert" class="break-words text-sm text-red-700">{{ error }}</p>
          <div class="flex gap-2 justify-end">
            <Button variant="outline" @click="closeDialog" :disabled="submitting">取消</Button>
            <Button @click="saveProxy" :disabled="submitting || (targetMode === 'container' && (!selectedTarget || loadingTargets))">
              {{ submitting ? '保存中...' : '保存' }}
            </Button>
          </div>
        </div>
      </Card>
    </div>
  </div>
</template>

<script setup>
import { computed, ref, onMounted } from 'vue'
import api from '@/utils/api'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'
import Card from '@/components/ui/Card.vue'
import AppHeader from '@/components/AppHeader.vue'

const proxies = ref([])
const frpConfig = ref({ serverAddr: '', serverPort: 0 })
const frpStatusCommand = 'systemctl status frpc'
const serviceStatus = ref({ active: false, status: 'unknown', statusDetail: '', detailCommand: '' })
const environmentCheck = ref(null)
const checkingEnvironment = ref(false)
const loading = ref(false)
const submitting = ref(false)
const showStatusDetail = ref(false)
const error = ref('')
const success = ref('')
const showAddDialog = ref(false)
const editingIndex = ref(null)
const originalName = ref('')
const targetMode = ref('manual')
const selectedTarget = ref(null)
const containerTargets = ref([])
const loadingTargets = ref(false)
const targetError = ref('')
const bindingReview = ref(null)
const reviewRows = ref([])
const selectedRules = ref([])
const reviewLoading = ref(false)
const confirmingBindings = ref(false)
const reviewError = ref('')
const reviewResult = ref('')
let reviewRequest = 0

const canConfirmBinding = (proxy) => !proxy.binding && !!proxy.candidate && ['tcp', 'udp'].includes(proxy.type)
const pendingProxies = computed(() => proxies.value.filter(canConfirmBinding))
const reviewTargets = computed(() => reviewRows.value.filter(row => row.target && !row.done))
const reviewAllSelected = computed(() => reviewTargets.value.length > 0 && reviewTargets.value.every(row => selectedRules.value.includes(row.name)))

const selectAllBindings = (checked) => {
  selectedRules.value = checked ? reviewTargets.value.map(row => row.name) : []
}

const openBindingReview = async (rules) => {
  const request = ++reviewRequest
  reviewRows.value = []
  selectedRules.value = []
  reviewError.value = ''
  reviewResult.value = ''
  reviewLoading.value = true
  bindingReview.value.showModal()
  try {
    const { data } = await api.get('/proxies/container-targets', { timeout: 15000 })
    if (request !== reviewRequest) return
    reviewRows.value = rules.map(rule => {
      const matches = (data.targets || []).filter(target => target.address === rule.localIP)
      const target = matches.length === 1 && matches[0].container === rule.candidate ? matches[0] : null
      return { ...rule, target, done: false, error: target ? '' : '地址归属已变化，请刷新列表后核对' }
    })
    if (rules.length === 1) selectAllBindings(true)
  } catch (err) {
    if (request === reviewRequest) reviewError.value = err.response?.data?.error || '无法读取容器网卡'
  } finally {
    if (request === reviewRequest) reviewLoading.value = false
  }
}

const closeBindingReview = () => {
  if (confirmingBindings.value) return
  reviewRequest++
  bindingReview.value.close()
}

const confirmBindings = async () => {
  if (confirmingBindings.value) return
  const rows = reviewTargets.value.filter(row => selectedRules.value.includes(row.name))
  if (!rows.length) return
  confirmingBindings.value = true
  let succeeded = 0
  try {
    for (const row of rows) {
      row.error = ''
      try {
        await api.post('/proxies/confirm-binding', { name: row.name, binding: row.target }, { timeout: 60000 })
        row.done = true
        succeeded++
        selectedRules.value = selectedRules.value.filter(name => name !== row.name)
      } catch (err) {
        row.error = err.response?.data?.error || '无法确认绑定结果，请刷新列表核对'
      }
    }
    reviewResult.value = `已绑定 ${succeeded} 条${succeeded < rows.length ? `，${rows.length - succeeded} 条需要处理` : ''}`
    await loadProxies()
  } finally {
    confirmingBindings.value = false
  }
}
const currentProxy = ref({
  name: '',
  type: 'tcp',
  localIP: '127.0.0.1',
  localPort: 22,
  remotePort: 20022,
  comment: '',
})

const loadProxies = async () => {
  if (environmentCheck.value && !environmentCheck.value.frp.ready) {
    return
  }

  loading.value = true
  error.value = ''
  success.value = ''

  try {
    const response = await api.get('/proxies')
    // 保存配置信息（包含ServerAddr）
    if (response.data.config) {
      frpConfig.value = {
        serverAddr: response.data.config.serverAddr || '',
        serverPort: response.data.config.serverPort || 0,
      }
    }
    // 只使用代理列表，基础配置由后端自动保持
    proxies.value = response.data.proxies || []
  } catch (err) {
    error.value = err.response?.data?.error || '加载配置失败'
  } finally {
    loading.value = false
  }
}

const loadTargets = async () => {
  if (targetMode.value !== 'container') return
  loadingTargets.value = true
  selectedTarget.value = null
  targetError.value = ''
  try {
    const response = await api.get('/proxies/container-targets', { timeout: 15000 })
    containerTargets.value = response.data.targets || []
    const binding = currentProxy.value.binding
    if (binding) {
      selectedTarget.value = containerTargets.value.find(target =>
        target.instanceId === binding.instanceId && target.device === binding.device && target.address === binding.address
      ) || null
    }
  } catch (err) {
    containerTargets.value = []
    targetError.value = err.response?.data?.error || '无法读取容器网卡'
  } finally {
    loadingTargets.value = false
  }
}

const refreshStatus = async () => {
  if (environmentCheck.value && !environmentCheck.value.frp.ready) {
    return
  }

  loading.value = true
  try {
    const response = await api.get('/status')
    serviceStatus.value = response.data
  } catch (err) {
    error.value = err.response?.data?.error || '获取状态失败'
  } finally {
    loading.value = false
  }
}

const saveProxy = async () => {
  // 防止重复提交
  if (submitting.value) {
    return
  }

  submitting.value = true
  error.value = ''
  success.value = ''

  try {
    const proxy = { ...currentProxy.value, binding: targetMode.value === 'container' ? selectedTarget.value : null }
    if (editingIndex.value !== null && editingIndex.value !== undefined) {
      // 更新代理
      const indexValue = Number(editingIndex.value)
      if (isNaN(indexValue)) {
        error.value = '索引值无效'
        submitting.value = false
        return
      }
      await api.put('/proxies', {
        index: indexValue,
        originalName: originalName.value,
        proxy,
      })
      success.value = '代理更新成功，已热重载'
      await loadProxies()
      closeDialog()
    } else {
      // 添加代理
      await api.post('/proxies', { proxy })
      success.value = '代理添加成功，已热重载'
      await loadProxies()
      closeDialog()
    }
  } catch (err) {
    error.value = err.response?.data?.error || (editingIndex.value !== null ? '更新代理失败' : '添加代理失败')
  } finally {
    submitting.value = false
  }
}

const editProxy = (index) => {
  editingIndex.value = index
  currentProxy.value = { ...proxies.value[index] }
  originalName.value = currentProxy.value.name
  targetMode.value = currentProxy.value.binding ? 'container' : 'manual'
  showAddDialog.value = true
  loadTargets()
}

const deleteProxy = async (index) => {
  if (!confirm('确定要删除这个代理吗？')) {
    return
  }

  try {
    await api.delete('/proxies', { data: { index, name: proxies.value[index].name } })
    success.value = '代理删除成功，已热重载'
    await loadProxies()
  } catch (err) {
    error.value = err.response?.data?.error || '删除代理失败'
  }
}

const closeDialog = () => {
  showAddDialog.value = false
  editingIndex.value = null
  originalName.value = ''
  targetMode.value = 'manual'
  selectedTarget.value = null
  submitting.value = false
  error.value = ''
  currentProxy.value = {
    name: '',
    type: 'tcp',
    localIP: '127.0.0.1',
    localPort: 22,
    remotePort: 20022,
    comment: '',
  }
}

const checkEnvironment = async () => {
  checkingEnvironment.value = true
  error.value = ''

  try {
    const response = await api.get('/check')
    environmentCheck.value = response.data

    if (response.data?.frp?.ready) {
      await Promise.all([loadProxies(), refreshStatus()])
    } else {
      proxies.value = []
      serviceStatus.value = { active: false, status: 'unavailable', statusDetail: '', detailCommand: '' }
    }
  } catch (err) {
    error.value = err.response?.data?.error || '环境检查失败'
  } finally {
    checkingEnvironment.value = false
  }
}

onMounted(() => {
  checkEnvironment()
})
</script>
