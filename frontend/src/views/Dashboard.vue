<template>
  <div class="min-h-screen bg-gray-50">
    <header class="bg-white shadow-sm border-b">
      <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-4 flex justify-between items-center">
        <h1 class="text-xl font-semibold">LabPanel 配置管理</h1>
        <Button variant="outline" @click="handleLogout">退出登录</Button>
      </div>
      <Navigation />
    </header>

    <main class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      <div class="space-y-6">
        <!-- 服务状态卡片 -->
        <Card class="p-6">
          <div class="flex items-center justify-between mb-4">
            <h2 class="text-lg font-semibold">服务状态</h2>
            <Button @click="refreshStatus" :disabled="loading">刷新状态</Button>
          </div>
          <div class="flex items-center gap-4 mb-4">
            <div class="flex items-center gap-2">
              <div
                :class="[
                  'w-3 h-3 rounded-full',
                  serviceStatus.active ? 'bg-green-500' : 'bg-red-500',
                ]"
              ></div>
              <span>{{ serviceStatus.active ? '运行中' : '已停止' }}</span>
            </div>
            <span class="text-sm text-gray-600">状态: {{ serviceStatus.status }}</span>
          </div>
          <div v-if="serviceStatus.statusDetail" class="mt-4">
            <h3 class="text-sm font-medium mb-2">详细状态:</h3>
            <pre class="bg-gray-100 p-4 rounded text-xs overflow-auto max-h-64">{{ serviceStatus.statusDetail }}</pre>
          </div>
        </Card>

        <!-- 代理列表卡片 -->
        <Card class="p-6">
          <div class="flex items-center justify-between mb-4">
            <h2 class="text-lg font-semibold">代理映射</h2>
            <Button @click="showAddDialog = true">新增代理</Button>
          </div>
          <div v-if="error" class="mb-4 text-red-500 text-sm">{{ error }}</div>
          <div v-if="success" class="mb-4 text-green-500 text-sm">{{ success }}</div>
          
          <div v-if="proxies.length === 0" class="text-center py-8 text-gray-500">
            暂无代理配置
          </div>
          <div v-else class="overflow-x-auto">
            <table class="w-full border-collapse">
              <thead>
                <tr class="border-b">
                  <th class="text-left p-2">名称</th>
                  <th class="text-left p-2">类型</th>
                  <th class="text-left p-2">本地IP</th>
                  <th class="text-left p-2">本地端口</th>
                  <th class="text-left p-2">远程端口</th>
                  <th class="text-left p-2">操作</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(proxy, index) in proxies" :key="index" class="border-b">
                  <td class="p-2">{{ proxy.name }}</td>
                  <td class="p-2">{{ proxy.type }}</td>
                  <td class="p-2">{{ proxy.localIP }}</td>
                  <td class="p-2">{{ proxy.localPort }}</td>
                  <td class="p-2">{{ proxy.remotePort }}</td>
                  <td class="p-2">
                    <div class="flex gap-2">
                      <Button variant="outline" size="sm" @click="editProxy(index)">编辑</Button>
                      <Button variant="destructive" size="sm" @click="deleteProxy(index)">删除</Button>
                    </div>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </Card>

        <!-- 操作按钮 -->
        <Card class="p-6">
          <div class="flex gap-2">
            <Button variant="outline" @click="loadProxies" :disabled="loading">
              重新加载
            </Button>
          </div>
          <p class="text-sm text-gray-500 mt-2">提示：配置修改后会自动验证并热重载，无需手动重启</p>
        </Card>
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
import { useRouter } from 'vue-router'
import api from '@/utils/api'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'
import Card from '@/components/ui/Card.vue'
import Navigation from '@/components/Navigation.vue'

const router = useRouter()
const proxies = ref([])
const serviceStatus = ref({ active: false, status: 'unknown', statusDetail: '' })
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
})

const loadProxies = async () => {
  loading.value = true
  error.value = ''
  success.value = ''

  try {
    const response = await api.get('/proxies')
    // 只使用代理列表，基础配置由后端自动保持
    proxies.value = response.data.proxies || []
  } catch (err) {
    error.value = err.response?.data?.error || '加载配置失败'
  } finally {
    loading.value = false
  }
}

const refreshStatus = async () => {
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
    if (editingIndex.value !== null) {
      // 更新代理
      await api.put('/proxies', {
        index: editingIndex.value,
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
  }
}


const handleLogout = () => {
  localStorage.removeItem('token')
  router.push('/login')
}

onMounted(() => {
  loadProxies()
  refreshStatus()
})
</script>
