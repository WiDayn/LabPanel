<template>
  <div class="min-h-screen bg-gray-50">
    <AppHeader />

    <main class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      <div class="space-y-6">
        <div class="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
          <div>
            <h1 class="text-2xl font-bold text-gray-900">Docker 管理</h1>
          </div>
          <Button variant="outline" @click="loadDocker" :disabled="loading">
            {{ loading ? '刷新中...' : '刷新' }}
          </Button>
        </div>

        <div v-if="error" class="rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
          {{ error }}
        </div>
        <div v-if="success" class="rounded-md border border-green-200 bg-green-50 px-4 py-3 text-sm text-green-700">
          {{ success }}
        </div>

        <section v-if="!dockerReady" class="min-w-0 rounded-lg border border-gray-200 bg-white shadow-sm">
          <div class="px-5 py-4">
            <h2 class="flex items-center gap-2 text-base font-semibold text-gray-900">
              <span class="h-2 w-2 rounded-full bg-amber-500" aria-hidden="true"></span>
              {{ loading && !dockerCheck.message ? '正在检查 Docker…' : 'Docker 尚未就绪' }}
            </h2>
            <p v-if="dockerCheck.message" class="mt-1 text-sm text-gray-700">{{ dockerCheck.message }}</p>
            <p v-if="dockerCheck.missingItems?.length" class="mt-1 text-xs text-gray-600">缺少 {{ dockerCheck.missingItems.join('、') }}</p>
          </div>
          <details v-if="dockerCheck.guides?.length" class="border-t border-gray-200">
            <summary class="cursor-pointer px-5 py-3 text-sm font-medium text-gray-800 hover:bg-gray-50">安装与配置</summary>
            <div class="divide-y px-5">
              <section v-for="guide in dockerCheck.guides" :key="guide.title" class="min-w-0 py-4">
                <h3 class="text-sm font-semibold">{{ guide.title }}</h3>
                <p v-if="guide.description" class="mt-1 text-sm text-gray-600">{{ guide.description }}</p>
                <dl class="mt-3 space-y-3">
                  <div v-for="command in guide.commands" :key="command.label" class="min-w-0">
                    <dt class="text-xs font-medium text-gray-700">{{ command.label }}</dt>
                    <dd class="mt-1 overflow-x-auto rounded-md border border-gray-200 bg-gray-50 px-3 py-2"><code class="whitespace-pre text-xs text-gray-900">{{ command.command }}</code></dd>
                  </div>
                </dl>
              </section>
            </div>
          </details>
        </section>

        <template v-else>
          <div class="flex rounded-md border border-gray-300 bg-white p-1 w-fit">
            <button
              v-for="tab in tabs"
              :key="tab.value"
              type="button"
              class="h-8 min-w-24 rounded px-3 text-sm font-medium transition-colors"
              :class="activeTab === tab.value ? 'bg-blue-600 text-white' : 'text-gray-600 hover:bg-gray-100'"
              @click="activeTab = tab.value"
            >
              {{ tab.label }}
            </button>
          </div>

          <section v-if="activeTab === 'containers'" class="overflow-hidden rounded-lg border border-gray-200 bg-white shadow-sm">
            <div class="flex items-center justify-between border-b border-gray-200 px-5 py-4">
              <div>
                <h2 class="text-base font-semibold text-gray-900">Docker 容器管理</h2>
                <p class="mt-1 text-xs text-gray-500">{{ containers.length }} 个容器</p>
              </div>
            </div>

            <div class="panel-table-frame m-4">
              <table class="panel-table min-w-[960px] w-full table-fixed text-sm">
                <thead>
                  <tr>
                    <SortableHeader :sort="containerTable" column="name" label="名称" class="w-44 px-4 py-3" />
                    <SortableHeader :sort="containerTable" column="image" label="镜像" class="w-48 px-4 py-3" />
                    <SortableHeader :sort="containerTable" column="state" label="状态" class="w-28 px-4 py-3" />
                    <SortableHeader :sort="containerTable" column="ports" label="端口" class="w-56 px-4 py-3" />
                    <SortableHeader :sort="containerTable" column="status" label="运行信息" class="w-56 px-4 py-3" />
                    <th scope="col" class="w-56 px-4 py-3 text-right">操作</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-100">
                  <tr v-for="container in containerTable.rows" :key="container.id || container.name">
                    <td class="truncate px-4 py-3 font-medium text-gray-900" :title="container.name">{{ container.name }}</td>
                    <td class="truncate px-4 py-3 text-gray-700" :title="container.image">{{ container.image }}</td>
                    <td class="px-4 py-3">
                      <span
                        class="rounded px-2 py-1 text-xs"
                        :class="container.state === 'Running' ? 'bg-green-100 text-green-700' : 'bg-gray-100 text-gray-600'"
                      >
                        {{ container.state || 'Unknown' }}
                      </span>
                    </td>
                    <td class="truncate px-4 py-3 text-gray-600" :title="container.ports || '-'">{{ container.ports || '-' }}</td>
                    <td class="truncate px-4 py-3 text-gray-600" :title="container.status || '-'">{{ container.status || '-' }}</td>
                    <td class="px-4 py-3">
                      <div class="flex justify-end gap-2">
                        <Button
                          v-if="container.state !== 'Running'"
                          variant="outline"
                          size="sm"
                          @click="startContainer(container.name)"
                        >
                          启动
                        </Button>
                        <Button
                          v-if="container.state === 'Running'"
                          variant="outline"
                          size="sm"
                          @click="stopContainer(container.name)"
                        >
                          停止
                        </Button>
                        <Button variant="outline" size="sm" @click="restartContainer(container.name)">重启</Button>
                        <Button variant="destructive" size="sm" @click="deleteContainer(container.name)">删除</Button>
                      </div>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
            <div v-if="!containers.length && !loading" class="border-t border-gray-200 bg-gray-50 p-5 text-sm text-gray-500">
              暂无 Docker 容器
            </div>
          </section>

          <section v-else class="overflow-hidden rounded-lg border border-gray-200 bg-white shadow-sm">
            <div class="flex items-center justify-between border-b border-gray-200 px-5 py-4">
              <div>
                <h2 class="text-base font-semibold text-gray-900">Docker 镜像管理</h2>
                <p class="mt-1 text-xs text-gray-500">{{ images.length }} 个镜像</p>
              </div>
            </div>

            <div class="panel-table-frame m-4">
              <table class="panel-table min-w-[800px] w-full table-fixed text-sm">
                <thead>
                  <tr>
                    <SortableHeader :sort="imageTable" column="repository" label="仓库" class="w-56 px-4 py-3" />
                    <SortableHeader :sort="imageTable" column="tag" label="标签" class="w-32 px-4 py-3" />
                    <SortableHeader :sort="imageTable" column="id" label="镜像 ID" class="w-44 px-4 py-3" />
                    <SortableHeader :sort="imageTable" column="size" label="大小" numeric class="w-28 px-4 py-3" />
                    <SortableHeader :sort="imageTable" column="createdAt" label="创建时间" class="w-44 px-4 py-3" />
                    <th scope="col" class="w-28 px-4 py-3 text-right">操作</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-100">
                  <tr v-for="image in imageTable.rows" :key="`${image.id}-${image.repository}-${image.tag}`">
                    <td class="truncate px-4 py-3 font-medium text-gray-900" :title="image.repository">{{ image.repository }}</td>
                    <td class="truncate px-4 py-3 text-gray-700" :title="image.tag">{{ image.tag }}</td>
                    <td class="truncate px-4 py-3 text-gray-600" :title="image.id">{{ image.id }}</td>
                    <td class="truncate px-4 py-3 text-right tabular-nums text-gray-600">{{ image.size }}</td>
                    <td class="truncate px-4 py-3 text-gray-600" :title="image.createdAt">{{ image.createdAt }}</td>
                    <td class="px-4 py-3 text-right">
                      <Button variant="destructive" size="sm" @click="deleteImage(image.id)">删除</Button>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
            <div v-if="!images.length && !loading" class="border-t border-gray-200 bg-gray-50 p-5 text-sm text-gray-500">
              暂无 Docker 镜像
            </div>
          </section>
        </template>
      </div>
    </main>
  </div>
</template>

<script setup>
import SortableHeader from '@/components/ui/SortableHeader.vue'
import { useTableSort, sortSize } from '@/utils/tableSort'
import { computed, onMounted, ref } from 'vue'
import api from '@/utils/api'
import AppHeader from '@/components/AppHeader.vue'
import Button from '@/components/ui/Button.vue'

const loading = ref(false)
const error = ref('')
const success = ref('')
const activeTab = ref('containers')
const dockerData = ref({ ready: false, check: {}, containers: [], images: [] })

const tabs = [
  { label: '容器管理', value: 'containers' },
  { label: '镜像管理', value: 'images' },
]

const dockerReady = computed(() => dockerData.value.ready)
const dockerCheck = computed(() => dockerData.value.check || {})
const containers = computed(() => dockerData.value.containers || [])
const images = computed(() => dockerData.value.images || [])

const loadDocker = async () => {
  loading.value = true
  error.value = ''
  try {
    const response = await api.get('/docker/list')
    dockerData.value = response.data || { ready: false, check: {}, containers: [], images: [] }
  } catch (err) {
    error.value = err.response?.data?.error || '加载 Docker 信息失败'
  } finally {
    loading.value = false
  }
}

const runAction = async (action, message) => {
  error.value = ''
  success.value = ''
  try {
    await action()
    success.value = message
    await loadDocker()
  } catch (err) {
    error.value = err.response?.data?.error || '操作失败'
  }
}

const startContainer = (name) => runAction(
  () => api.post('/docker/start', { name }),
  `Docker 容器 "${name}" 已启动`
)

const stopContainer = (name) => runAction(
  () => api.post('/docker/stop', { name }),
  `Docker 容器 "${name}" 已停止`
)

const restartContainer = (name) => runAction(
  () => api.post('/docker/restart', { name }),
  `Docker 容器 "${name}" 已重启`
)

const deleteContainer = (name) => {
  if (!confirm(`确定要删除 Docker 容器 "${name}" 吗？此操作不可恢复！`)) {
    return
  }
  runAction(
    () => api.delete('/docker/delete', { data: { name } }),
    `Docker 容器 "${name}" 已删除`
  )
}

const deleteImage = (id) => {
  if (!confirm(`确定要删除 Docker 镜像 "${id}" 吗？`)) {
    return
  }
  runAction(
    () => api.delete('/docker/images', { data: { id } }),
    `Docker 镜像 "${id}" 已删除`
  )
}

onMounted(loadDocker)
const containerTable = useTableSort(() => containers.value, {})
const imageTable = useTableSort(() => images.value, { size: row => sortSize(row.size), createdAt: row => Date.parse(row.createdAt?.replace(/ [A-Z]{2,5}$/, '') || '') })
</script>
