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
        <Card
          v-if="environmentCheck && !environmentCheck.lxc.ready"
          class="p-6 border-amber-200 bg-amber-50"
        >
          <div class="flex items-start justify-between gap-4">
            <div>
              <h2 class="text-lg font-semibold text-amber-900">LXC/LXD 尚未就绪</h2>
              <p class="mt-1 text-sm text-amber-800">{{ environmentCheck.lxc.message }}</p>
              <p
                v-if="environmentCheck.lxc.missingItems?.length"
                class="mt-2 text-sm text-amber-700"
              >
                缺少项目：{{ environmentCheck.lxc.missingItems.join('、') }}
              </p>
            </div>
            <Button variant="outline" @click="checkEnvironment" :disabled="checkingEnvironment">
              {{ checkingEnvironment ? '检查中...' : '重新检查' }}
            </Button>
          </div>

          <div
            v-for="guide in environmentCheck.lxc.guides || []"
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

        <!-- 容器列表卡片 -->
        <Card v-if="environmentCheck?.lxc.ready !== false" class="p-6">
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
                  <td class="p-2">
                    <div class="flex gap-2 flex-wrap">
                      <Button 
                        v-if="container.state !== 'Running'" 
                        variant="outline" 
                        size="sm" 
                        @click="startContainer(container.name)"
                      >
                        开启
                      </Button>
                      <Button 
                        v-if="container.state === 'Running'" 
                        variant="outline" 
                        size="sm" 
                        @click="stopContainer(container.name)"
                      >
                        关机
                      </Button>
                      <Button 
                        v-if="container.state === 'Running'" 
                        variant="outline" 
                        size="sm" 
                        @click="forceStopContainer(container.name)"
                      >
                        强制关机
                      </Button>
                      <Button variant="outline" size="sm" @click="showConfig(container.name)">配置</Button>
                      <Button variant="outline" size="sm" @click="changePassword(container.name)">修改密码</Button>
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

    <!-- 配置编辑对话框 -->
    <div
      v-if="showConfigDialog"
      class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50"
      @click.self="closeConfigDialog"
    >
      <Card class="w-full max-w-4xl p-6 m-4 max-h-[90vh] flex flex-col">
        <h3 class="text-lg font-semibold mb-4">容器配置 - {{ configContainer.name }}</h3>
        <div v-if="loadingConfig" class="flex-1 flex items-center justify-center py-8">
          <div class="text-center">
            <div class="inline-block animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600 mb-4"></div>
            <p class="text-gray-600">正在加载配置...</p>
          </div>
        </div>
        <div v-else class="flex-1 flex flex-col space-y-4 min-h-0">
          <div class="flex-1 min-h-0">
            <label class="block text-sm font-medium mb-2">配置文件 (YAML)</label>
            <textarea
              v-model="configContent"
              class="w-full h-full min-h-[400px] p-3 border border-gray-300 rounded-md font-mono text-sm"
              style="font-family: 'Courier New', monospace;"
              spellcheck="false"
            ></textarea>
          </div>
          <div v-if="configError" class="text-red-500 text-sm">{{ configError }}</div>
          <div class="flex gap-2 justify-end">
            <Button variant="outline" @click="closeConfigDialog" :disabled="savingConfig">取消</Button>
            <Button @click="saveConfig" :disabled="savingConfig">
              {{ savingConfig ? '保存中...' : '保存' }}
            </Button>
          </div>
        </div>
      </Card>
    </div>

    <!-- 修改密码对话框 -->
    <div
      v-if="showPasswordDialog"
      class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50"
      @click.self="closePasswordDialog"
    >
      <Card class="w-full max-w-md p-6 m-4">
        <h3 class="text-lg font-semibold mb-4">修改密码</h3>
        <div class="space-y-4">
          <div>
            <label class="block text-sm font-medium mb-2">容器名称</label>
            <Input :value="passwordContainer.name" disabled />
          </div>
          <div>
            <label class="block text-sm font-medium mb-2">新Root密码</label>
            <Input v-model="passwordContainer.password" type="password" placeholder="请输入新密码" />
          </div>
          <div class="flex gap-2 justify-end">
            <Button variant="outline" @click="closePasswordDialog" :disabled="changingPassword">取消</Button>
            <Button @click="confirmChangePassword" :disabled="changingPassword">
              {{ changingPassword ? '修改中...' : '确认修改' }}
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
const environmentCheck = ref(null)
const checkingEnvironment = ref(false)
const loading = ref(false)
const creating = ref(false)
const error = ref('')
const success = ref('')
const showCreateDialog = ref(false)
const showPasswordDialog = ref(false)
const changingPassword = ref(false)
const showConfigDialog = ref(false)
const configContainer = ref({ name: '' })
const configContent = ref('')
const loadingConfig = ref(false)
const savingConfig = ref(false)
const configError = ref('')
const newContainer = ref({
  name: '',
  password: '',
})
const passwordContainer = ref({
  name: '',
  password: '',
})

const loadContainers = async () => {
  if (environmentCheck.value && !environmentCheck.value.lxc.ready) {
    return
  }

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

const startContainer = async (name) => {
  error.value = ''
  success.value = ''

  try {
    await api.post('/lxc/start', { name })
    success.value = '容器启动成功'
    // 延迟刷新列表
    setTimeout(() => {
      loadContainers()
    }, 1000)
  } catch (err) {
    error.value = err.response?.data?.error || '启动容器失败'
  }
}

const stopContainer = async (name) => {
  error.value = ''
  success.value = ''

  try {
    await api.post('/lxc/stop', { name })
    success.value = '容器已关机'
    // 延迟刷新列表
    setTimeout(() => {
      loadContainers()
    }, 1000)
  } catch (err) {
    error.value = err.response?.data?.error || '关机失败'
  }
}

const forceStopContainer = async (name) => {
  error.value = ''
  success.value = ''

  if (!confirm('确定要强制关机吗？这可能会丢失未保存的数据。')) {
    return
  }

  try {
    await api.post('/lxc/force-stop', { name })
    success.value = '容器已强制关机'
    // 延迟刷新列表
    setTimeout(() => {
      loadContainers()
    }, 1000)
  } catch (err) {
    error.value = err.response?.data?.error || '强制关机失败'
  }
}

const changePassword = (name) => {
  passwordContainer.value = {
    name: name,
    password: '',
  }
  showPasswordDialog.value = true
  error.value = ''
  success.value = ''
}

const confirmChangePassword = async () => {
  if (!passwordContainer.value.password) {
    error.value = '请输入新密码'
    return
  }

  changingPassword.value = true
  error.value = ''
  success.value = ''

  try {
    await api.put('/lxc/password', {
      name: passwordContainer.value.name,
      password: passwordContainer.value.password,
    })
    success.value = '密码修改成功'
    closePasswordDialog()
  } catch (err) {
    error.value = err.response?.data?.error || '修改密码失败'
  } finally {
    changingPassword.value = false
  }
}

const closePasswordDialog = () => {
  showPasswordDialog.value = false
  passwordContainer.value = {
    name: '',
    password: '',
  }
  error.value = ''
}

const showConfig = async (name) => {
  configContainer.value = { name }
  configContent.value = ''
  configError.value = ''
  showConfigDialog.value = true
  loadingConfig.value = true

  try {
    const response = await api.get(`/lxc/config/${encodeURIComponent(name)}`)
    configContent.value = response.data.config || ''
  } catch (err) {
    configError.value = err.response?.data?.error || '加载配置失败'
  } finally {
    loadingConfig.value = false
  }
}

const saveConfig = async () => {
  if (!configContainer.value.name) {
    configError.value = '容器名称不能为空'
    return
  }

  savingConfig.value = true
  configError.value = ''

  try {
    await api.put('/lxc/config', {
      name: configContainer.value.name,
      config: configContent.value,
    })
    success.value = '配置保存成功'
    closeConfigDialog()
    // 延迟刷新列表
    setTimeout(() => {
      loadContainers()
    }, 1000)
  } catch (err) {
    configError.value = err.response?.data?.error || '保存配置失败'
  } finally {
    savingConfig.value = false
  }
}

const closeConfigDialog = () => {
  showConfigDialog.value = false
  configContainer.value = { name: '' }
  configContent.value = ''
  configError.value = ''
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

const checkEnvironment = async () => {
  checkingEnvironment.value = true
  error.value = ''

  try {
    const response = await api.get('/check')
    environmentCheck.value = response.data

    if (response.data?.lxc?.ready) {
      await loadContainers()
    } else {
      containers.value = []
    }
  } catch (err) {
    error.value = err.response?.data?.error || '环境检查失败'
  } finally {
    checkingEnvironment.value = false
  }
}

const handleLogout = () => {
  localStorage.removeItem('token')
  router.push('/login')
}

onMounted(() => {
  checkEnvironment()
})
</script>
