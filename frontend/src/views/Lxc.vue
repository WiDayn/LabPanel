<template>
  <div class="min-h-screen bg-gray-50">
    <header class="bg-white shadow-sm border-b">
      <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-4 flex justify-between items-center">
        <h1 class="text-xl font-semibold">LXC 容器管理</h1>
        <Button variant="outline" @click="handleLogout">退出登录</Button>
      </div>
      <Navigation />
    </header>

    <main class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      <div class="space-y-6">
        <!-- 容器列表卡片 -->
        <Card class="p-6">
          <div class="flex items-center justify-between mb-4">
            <h2 class="text-lg font-semibold">容器列表</h2>
            <div class="flex gap-2">
              <Button @click="showCreateDialog = true">新增容器</Button>
              <Button @click="loadContainers" :disabled="loading">刷新</Button>
            </div>
          </div>
          <div v-if="error" class="mb-4 text-red-500 text-sm">{{ error }}</div>
          <div v-if="success" class="mb-4 text-green-500 text-sm">{{ success }}</div>
          
          <div v-if="loading && containers.length === 0" class="text-center py-8 text-gray-500">
            加载中...
          </div>
          <div v-else-if="containers.length === 0" class="text-center py-8 text-gray-500">
            暂无容器
          </div>
          <div v-else class="overflow-x-auto">
            <table class="w-full border-collapse">
              <thead>
                <tr class="border-b">
                  <th class="text-left p-2">名称</th>
                  <th class="text-left p-2">状态</th>
                  <th class="text-left p-2">类型</th>
                  <th class="text-left p-2">架构</th>
                  <th class="text-left p-2">IPv4</th>
                  <th class="text-left p-2">IPv6</th>
                  <th class="text-left p-2">配置集</th>
                  <th class="text-left p-2">操作</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(container, index) in containers" :key="index" class="border-b">
                  <td class="p-2 font-medium">{{ container.name }}</td>
                  <td class="p-2">
                    <span
                      :class="[
                        'px-2 py-1 rounded text-xs',
                        container.state === 'Running' ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-800'
                      ]"
                    >
                      {{ container.state }}
                    </span>
                  </td>
                  <td class="p-2">{{ container.type }}</td>
                  <td class="p-2">{{ container.arch }}</td>
                  <td class="p-2">{{ container.ipv4 || '-' }}</td>
                  <td class="p-2">{{ container.ipv6 || '-' }}</td>
                  <td class="p-2 text-sm text-gray-600">{{ container.profiles || '-' }}</td>
                  <td class="p-2">
                    <div class="flex gap-2">
                      <Button variant="outline" size="sm" @click="restartContainer(container.name)">重启</Button>
                      <Button variant="destructive" size="sm" @click="deleteContainer(container.name)">删除</Button>
                    </div>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </Card>
      </div>
    </main>

    <!-- 新增容器对话框 -->
    <div
      v-if="showCreateDialog"
      class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50"
      @click.self="closeCreateDialog"
    >
      <Card class="w-full max-w-md p-6 m-4">
        <h3 class="text-lg font-semibold mb-4">新增容器</h3>
        <div v-if="creating" class="space-y-4">
          <div class="text-center py-8">
            <div class="inline-block animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600 mb-4"></div>
            <p class="text-gray-600 mb-2">正在创建容器，请稍候...</p>
            <p class="text-sm text-gray-500">这可能需要10-20秒，请耐心等待</p>
          </div>
        </div>
        <div v-else class="space-y-4">
          <div>
            <label class="block text-sm font-medium mb-2">容器名称</label>
            <Input v-model="newContainer.name" placeholder="请输入容器名称" />
          </div>
          <div>
            <label class="block text-sm font-medium mb-2">Root密码</label>
            <Input v-model="newContainer.password" type="password" placeholder="请输入root密码" />
          </div>
          <div class="flex gap-2 justify-end">
            <Button variant="outline" @click="closeCreateDialog" :disabled="creating">取消</Button>
            <Button @click="createContainer" :disabled="creating">
              {{ creating ? '创建中...' : '创建' }}
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
import Card from '@/components/ui/Card.vue'
import Input from '@/components/ui/Input.vue'
import Navigation from '@/components/Navigation.vue'

const router = useRouter()
const containers = ref([])
const loading = ref(false)
const creating = ref(false)
const error = ref('')
const success = ref('')
const showCreateDialog = ref(false)
const newContainer = ref({
  name: '',
  password: '',
})

const loadContainers = async () => {
  loading.value = true
  error.value = ''
  success.value = ''

  try {
    const response = await api.get('/lxc/list')
    containers.value = response.data.containers || []
  } catch (err) {
    error.value = err.response?.data?.error || '加载容器列表失败'
  } finally {
    loading.value = false
  }
}

const createContainer = async () => {
  if (!newContainer.value.name || !newContainer.value.password) {
    error.value = '请填写容器名称和密码'
    return
  }

  creating.value = true
  error.value = ''
  success.value = ''

  try {
    await api.post('/lxc/create', {
      name: newContainer.value.name,
      password: newContainer.value.password,
    })
    success.value = '容器创建成功'
    creating.value = false
    closeCreateDialog()
    // 延迟刷新列表，等待容器创建完成
    setTimeout(() => {
      loadContainers()
    }, 2000)
  } catch (err) {
    error.value = err.response?.data?.error || '创建容器失败'
    creating.value = false // 出错时允许关闭对话框
  }
}

const deleteContainer = async (name) => {
  if (!confirm(`确定要删除容器 "${name}" 吗？此操作不可恢复！`)) {
    return
  }

  error.value = ''
  success.value = ''

  try {
    await api.delete('/lxc/delete', { data: { name } })
    success.value = '容器删除成功'
    await loadContainers()
  } catch (err) {
    error.value = err.response?.data?.error || '删除容器失败'
  }
}

const restartContainer = async (name) => {
  error.value = ''
  success.value = ''

  try {
    await api.post('/lxc/restart', { name })
    success.value = '容器重启成功'
    // 延迟刷新列表
    setTimeout(() => {
      loadContainers()
    }, 1000)
  } catch (err) {
    error.value = err.response?.data?.error || '重启容器失败'
  }
}

const closeCreateDialog = () => {
  showCreateDialog.value = false
  creating.value = false
  newContainer.value = {
    name: '',
    password: '',
  }
  error.value = ''
}

const handleLogout = () => {
  localStorage.removeItem('token')
  router.push('/login')
}

onMounted(() => {
  loadContainers()
})
</script>

