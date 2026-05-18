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
              <div class="mt-1 text-xs text-gray-500">状态: {{ serviceStatus.status }}</div>
            </div>
            <Button variant="outline" size="sm" @click="refreshStatus" :disabled="loading">
              {{ loading ? '刷新中...' : '刷新状态' }}
            </Button>
          </div>
          <div class="border-t border-gray-200 p-4">
            <div class="flex flex-wrap items-center gap-4">
              <div class="flex items-center gap-2">
                <div
                  :class="[
                    'h-3 w-3 rounded-full',
                    serviceStatus.active ? 'bg-green-500' : 'bg-red-500',
                  ]"
                ></div>
                <span class="text-sm font-medium text-gray-900">
                  {{ serviceStatus.active ? '运行中' : '已停止' }}
                </span>
              </div>
              <span class="text-sm text-gray-600">状态: {{ serviceStatus.status }}</span>
            </div>
            <div v-if="serviceStatus.statusDetail" class="mt-4">
              <h3 class="text-sm font-medium text-gray-900">详细状态</h3>
              <pre class="mt-2 max-h-64 overflow-auto rounded-md border border-gray-200 bg-gray-50 p-3 text-xs text-gray-800">{{ serviceStatus.statusDetail }}</pre>
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
            <div class="flex gap-2">
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
                    <th class="px-2 py-2 font-medium">本地IP</th>
                    <th class="px-2 py-2 font-medium">本地端口</th>
                    <th class="px-2 py-2 font-medium">映射地址</th>
                    <th class="px-2 py-2 font-medium">备注</th>
                    <th class="px-2 py-2 font-medium">操作</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="(proxy, index) in proxies" :key="index" class="border-b last:border-b-0">
                    <td class="px-2 py-2 font-medium text-gray-900">{{ proxy.name }}</td>
                    <td class="px-2 py-2 text-gray-700">{{ proxy.type }}</td>
                    <td class="px-2 py-2 text-gray-700">{{ proxy.localIP }}</td>
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

    <!-- 添加/编辑代理对话框 -->
    <div
      v-if="showAddDialog || editingIndex !== null"
      class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50"
      @click.self="closeDialog"
    >
      <Card class="w-full max-w-md p-6 m-4">
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
            <label class="block text-sm font-medium mb-2">本地IP</label>
            <Input v-model="currentProxy.localIP" placeholder="127.0.0.1" />
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
          <div class="flex gap-2 justify-end">
            <Button variant="outline" @click="closeDialog" :disabled="submitting">取消</Button>
            <Button @click="saveProxy" :disabled="submitting">
              {{ submitting ? '保存中...' : '保存' }}
            </Button>
          </div>
        </div>
      </Card>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import api from '@/utils/api'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'
import Card from '@/components/ui/Card.vue'
import AppHeader from '@/components/AppHeader.vue'

const proxies = ref([])
const frpConfig = ref({ serverAddr: '', serverPort: 0 })
const serviceStatus = ref({ active: false, status: 'unknown', statusDetail: '' })
const environmentCheck = ref(null)
const checkingEnvironment = ref(false)
const loading = ref(false)
const submitting = ref(false)
const error = ref('')
const success = ref('')
const showAddDialog = ref(false)
const editingIndex = ref(null)
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
        proxy: currentProxy.value,
      })
      success.value = '代理更新成功，已热重载'
      await loadProxies()
      closeDialog()
    } else {
      // 添加代理
      await api.post('/proxies', { proxy: currentProxy.value })
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
  showAddDialog.value = true
}

const deleteProxy = async (index) => {
  if (!confirm('确定要删除这个代理吗？')) {
    return
  }

  try {
    await api.delete('/proxies', { data: { index } })
    success.value = '代理删除成功，已热重载'
    await loadProxies()
  } catch (err) {
    error.value = err.response?.data?.error || '删除代理失败'
  }
}

const closeDialog = () => {
  showAddDialog.value = false
  editingIndex.value = null
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
      serviceStatus.value = { active: false, status: 'unavailable', statusDetail: '' }
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
