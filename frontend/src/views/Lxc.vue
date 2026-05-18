<template>
  <div class="min-h-screen bg-gray-50">
    <AppHeader />

    <main class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      <div class="space-y-6">
        <Card class="p-4">
          <div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
            <div>
              <h2 class="text-base font-semibold">宿主机 IP</h2>
              <p class="mt-1 text-sm text-gray-600">
                这里区分了“容器内访问宿主机”和“局域网其他设备访问宿主机”两种地址，避免混淆。
              </p>
              <p class="mt-1 text-sm text-gray-600">
                当前 LXC 容器里如果要访问宿主机，请优先使用
                <span class="font-medium text-gray-800">{{ hostInfo.recommendedContainerHostIP || '-' }}</span>
              </p>
            </div>
            <Button variant="outline" size="sm" @click="loadHostInfo" :disabled="loadingHostInfo">
              {{ loadingHostInfo ? '刷新中...' : '刷新 IP' }}
            </Button>
          </div>
          <div class="mt-3 grid gap-3 md:grid-cols-2">
            <div class="rounded-lg border border-gray-200 bg-gray-50 p-3">
              <div class="text-xs font-medium text-gray-500">容器内访问宿主机（LXD 网桥地址）</div>
              <div class="mt-1 break-all text-sm font-medium text-gray-900">
                {{ hostInfo.recommendedContainerHostIP || '未检测到' }}
              </div>
              <div class="mt-1 text-xs text-gray-500">
                适合在容器里连接宿主机上的 SSH、HTTP、数据库等服务。
              </div>
            </div>
            <div class="rounded-lg border border-gray-200 bg-gray-50 p-3">
              <div class="text-xs font-medium text-gray-500">局域网其他设备访问宿主机</div>
              <div class="mt-1 break-all text-sm font-medium text-gray-900">
                {{ hostInfo.lanIPs.length ? hostInfo.lanIPs.join(' / ') : '未检测到' }}
              </div>
              <div class="mt-1 text-xs text-gray-500">
                适合你的电脑、手机或同网段其他服务器访问这台宿主机。
              </div>
            </div>
          </div>
        </Card>

        <section class="overflow-hidden rounded-lg border border-gray-200 bg-white shadow-sm">
          <div class="flex flex-col gap-3 px-5 py-4 sm:flex-row sm:items-start sm:justify-between">
            <div>
              <h2 class="text-base font-semibold text-gray-900">容器默认设置</h2>
              <p class="mt-1 text-xs text-gray-500">设置新建容器默认镜像，以及容器备份导出的保存目录。</p>
            </div>
          </div>
          <div class="border-t border-gray-200 p-4">
            <div class="grid gap-4 lg:grid-cols-2">
              <div>
                <label class="mb-2 block text-sm font-medium text-gray-700">LXC 镜像</label>
                <Input
                  v-model="appConfig.lxcImage"
                  :disabled="savingAppConfig || loadingAppConfig"
                  placeholder="ubuntu:22.04"
                />
              </div>
              <div>
                <label class="mb-2 block text-sm font-medium text-gray-700">容器备份目录</label>
                <Input
                  v-model="appConfig.lxcBackupDir"
                  :disabled="savingAppConfig || loadingAppConfig"
                  placeholder="./backups"
                />
              </div>
            </div>
            <div class="mt-4 flex flex-col gap-3 border-t border-gray-200 pt-4 sm:flex-row sm:items-center">
              <div class="space-y-1 text-sm text-gray-500">
                <p>
                  当前新建容器默认使用：<span class="font-medium text-gray-700">{{ appConfig.lxcImage || 'ubuntu:22.04' }}</span>
                </p>
                <p>
                  当前容器备份将保存到：<span class="font-medium text-gray-700 break-all">{{ appConfig.lxcBackupDir || './backups' }}</span>
                </p>
              </div>
              <Button class="sm:ml-auto" @click="saveAppConfig" :disabled="savingAppConfig || loadingAppConfig">
                {{ savingAppConfig ? '保存中...' : '保存容器设置' }}
              </Button>
            </div>
          </div>
        </section>

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
        <section
          v-if="environmentCheck?.lxc.ready !== false"
          class="overflow-hidden rounded-lg border border-gray-200 bg-white shadow-sm"
        >
          <div class="flex flex-col gap-3 px-5 py-4 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <h2 class="text-base font-semibold text-gray-900">LXC 容器列表</h2>
              <div class="mt-1 text-xs text-gray-500">{{ containers.length }} 个容器</div>
            </div>
            <div class="flex gap-2">
              <Button size="sm" @click="openCreateDialog">新增容器</Button>
              <Button variant="outline" size="sm" @click="loadContainers" :disabled="loading">
                {{ loading ? '刷新中...' : '刷新' }}
              </Button>
            </div>
          </div>
          <div class="border-t border-gray-200 p-4">
            <div v-if="error" class="mb-4 rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">{{ error }}</div>
            <div v-if="success" class="mb-4 rounded-md border border-green-200 bg-green-50 px-3 py-2 text-sm text-green-700">{{ success }}</div>
            <div
              v-if="activeRestores.length > 0"
              class="mb-4 rounded-lg border border-amber-200 bg-amber-50 p-4"
            >
            <div class="text-sm font-medium text-amber-900">恢复任务状态</div>
            <div class="mt-1 text-xs text-amber-800">
              备份包导入会在后台运行，20GB 备份可能需要几分钟。任务运行时请不要重复创建同名容器。
            </div>
            <div class="mt-3 space-y-3">
              <div
                v-for="restore in activeRestores"
                :key="restore.taskId"
                class="rounded-md border border-amber-100 bg-white p-3"
              >
                <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                  <div>
                    <div class="text-sm font-medium text-gray-900">{{ restore.name }}</div>
                    <div class="mt-1 text-xs text-gray-600">{{ formatRestoreStatusSummary(restore) }}</div>
                    <div v-if="restore.backupFile" class="mt-1 text-xs text-gray-500 break-all">
                      备份包：{{ restore.backupFile }}
                    </div>
                  </div>
                  <div class="text-xs text-gray-500">
                    {{ formatBackupTime(restore.updatedAt) }}
                  </div>
                </div>
                <div class="mt-2 h-2 w-full overflow-hidden rounded-full bg-gray-100">
                  <div
                    class="h-full rounded-full transition-all"
                    :class="restore.status === 'failed' ? 'bg-red-500' : restore.status === 'completed' ? 'bg-green-500' : 'bg-amber-500'"
                    :style="{ width: `${Math.max(5, restore.progress || 0)}%` }"
                  />
                </div>
                <details
                  v-if="hasRestoreLog(restore)"
                  class="mt-3 rounded-md border border-gray-200 bg-gray-50"
                >
                  <summary class="cursor-pointer px-3 py-2 text-xs font-medium text-gray-700">
                    查看导入日志
                  </summary>
                  <pre class="max-h-72 overflow-auto whitespace-pre-wrap break-words border-t border-gray-200 px-3 py-2 text-xs text-gray-700">{{ restore.message }}</pre>
                </details>
              </div>
            </div>
            </div>
            <div
              v-if="activeBackups.length > 0"
              class="mb-4 rounded-lg border border-blue-200 bg-blue-50 p-4"
            >
            <div class="text-sm font-medium text-blue-900">备份任务状态</div>
            <div class="mt-3 space-y-3">
              <div
                v-for="backup in activeBackups"
                :key="backup.taskId"
                class="rounded-md border border-blue-100 bg-white p-3"
              >
                <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                  <div>
                    <div class="text-sm font-medium text-gray-900">{{ backup.name }}</div>
                    <div class="mt-1 text-xs text-gray-600">{{ formatBackupStatus(backup) }}</div>
                  </div>
                  <div class="text-xs text-gray-500">
                    {{ formatBackupTime(backup.updatedAt) }}
                  </div>
                </div>
                <div class="mt-2 h-2 w-full overflow-hidden rounded-full bg-gray-100">
                  <div
                    class="h-full rounded-full transition-all"
                    :class="backup.status === 'failed' ? 'bg-red-500' : backup.status === 'completed' ? 'bg-green-500' : 'bg-blue-500'"
                    :style="{ width: `${Math.max(5, backup.progress || 0)}%` }"
                  />
                </div>
                <div
                  v-if="backup.exportedFiles?.length"
                  class="mt-2 text-xs text-gray-600 break-all"
                >
                  文件：{{ backup.exportedFiles.join('，') }}
                </div>
              </div>
            </div>
            </div>
          
            <div v-if="loading && containers.length === 0" class="rounded-md border border-gray-200 bg-gray-50 p-4 text-sm text-gray-500">
              加载中...
            </div>
            <div v-else-if="containers.length === 0" class="rounded-md border border-gray-200 bg-gray-50 p-4 text-sm text-gray-500">
              暂无容器
            </div>
            <div v-else class="overflow-x-auto">
              <table class="w-full table-fixed border-collapse text-sm">
              <thead>
                <tr class="border-b text-left text-xs text-gray-500">
                  <th class="w-48 px-2 py-2 font-medium">名称</th>
                  <th class="w-24 px-2 py-2 font-medium">状态</th>
                  <th class="w-36 px-2 py-2 font-medium">IPv4</th>
                  <th class="w-48 px-2 py-2 font-medium">IPv6</th>
                  <th class="px-2 py-2 font-medium">操作</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(container, index) in containers" :key="index" class="border-b last:border-b-0">
                  <td class="truncate px-2 py-2 font-medium text-gray-900" :title="container.name">{{ container.name }}</td>
                  <td class="px-2 py-2">
                    <span
                      :class="[
                        'px-2 py-1 rounded text-xs',
                        container.state === 'Running' ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-800'
                      ]"
                    >
                      {{ container.state }}
                    </span>
                  </td>
                  <td class="truncate px-2 py-2 text-gray-700" :title="container.ipv4 || '-'">{{ container.ipv4 || '-' }}</td>
                  <td class="px-2 py-2 text-gray-700">
                    <div class="truncate" :title="container.ipv6 || '-'">
                      {{ container.ipv6 || '-' }}
                    </div>
                  </td>
                  <td class="px-2 py-2">
                    <div class="flex gap-2 flex-nowrap whitespace-nowrap">
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
                      <Button
                        variant="outline"
                        size="sm"
                        :disabled="isBackupRunning(container.name)"
                        @click="backupContainer(container.name)"
                      >
                        {{ isBackupRunning(container.name) ? '备份中...' : '备份' }}
                      </Button>
                      <Button variant="outline" size="sm" @click="changePassword(container.name)">修改密码</Button>
                      <Button variant="outline" size="sm" @click="restartContainer(container.name)">重启</Button>
                      <Button variant="destructive" size="sm" @click="deleteContainer(container.name)">删除</Button>
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
            <p class="text-gray-600 mb-2">
              {{ newContainer.sourceType === 'backup' ? '正在提交恢复任务...' : '正在创建容器，请稍候...' }}
            </p>
            <p class="text-sm text-gray-500">
              {{ newContainer.sourceType === 'backup' ? '提交后会在后台导入，可在容器列表上方查看状态' : '这可能需要10-20秒，请耐心等待' }}
            </p>
          </div>
        </div>
        <div v-else class="space-y-4">
          <div>
            <label class="block text-sm font-medium mb-2">容器名称</label>
            <Input v-model="newContainer.name" placeholder="请输入容器名称" />
          </div>
          <div>
            <label class="block text-sm font-medium mb-2">创建来源</label>
            <div class="grid gap-2 sm:grid-cols-3">
              <label class="flex cursor-pointer items-start gap-2 rounded-lg border p-3 text-sm" :class="newContainer.sourceType === 'default_image' ? 'border-blue-500 bg-blue-50' : 'border-gray-200'">
                <input v-model="newContainer.sourceType" type="radio" class="mt-1" value="default_image" />
                <span>
                  默认镜像
                </span>
              </label>
              <label class="flex cursor-pointer items-start gap-2 rounded-lg border p-3 text-sm" :class="newContainer.sourceType === 'custom_image' ? 'border-blue-500 bg-blue-50' : 'border-gray-200'">
                <input v-model="newContainer.sourceType" type="radio" class="mt-1" value="custom_image" />
                <span>
                  自定义镜像
                </span>
              </label>
              <label class="flex cursor-pointer items-start gap-2 rounded-lg border p-3 text-sm" :class="newContainer.sourceType === 'backup' ? 'border-blue-500 bg-blue-50' : 'border-gray-200'">
                <input v-model="newContainer.sourceType" type="radio" class="mt-1" value="backup" />
                <span>
                  备份恢复
                </span>
              </label>
            </div>
          </div>
          <div v-if="newContainer.sourceType === 'default_image'">
            <label class="block text-sm font-medium mb-2">默认镜像</label>
            <Input :value="appConfig.lxcImage || 'ubuntu:22.04'" disabled />
            <p class="mt-1 text-xs text-gray-500">使用上方“容器默认配置”中的默认镜像创建。</p>
          </div>
          <div v-if="newContainer.sourceType === 'custom_image'">
            <label class="block text-sm font-medium mb-2">自定义镜像</label>
            <Input v-model="newContainer.image" placeholder="例如 ubuntu:24.04 / images:debian/12" />
            <p class="mt-1 text-xs text-gray-500">这里填写要传给 `lxc launch` 的镜像名称。</p>
          </div>
          <div v-if="newContainer.sourceType === 'backup'">
            <div
              v-if="hasRunningRestores"
              class="mb-3 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-800"
            >
              当前已有备份恢复任务在后台运行。可以继续创建不同名称的容器，但不要重复提交同名恢复。
            </div>
            <div class="flex items-center justify-between gap-3">
              <label class="block text-sm font-medium">选择备份包</label>
              <Button variant="outline" size="sm" @click="loadBackupArchives({ silent: false })" :disabled="loadingBackupArchives">
                {{ loadingBackupArchives ? '刷新中...' : '刷新列表' }}
              </Button>
            </div>
            <select
              v-model="newContainer.backupFile"
              class="mt-2 w-full rounded-md border border-gray-300 px-3 py-2 text-sm"
            >
              <option value="">请选择备份文件</option>
              <option v-for="archive in backupArchives" :key="archive.path" :value="archive.name">
                {{ archive.displayLabel }}
              </option>
            </select>
            <div v-if="backupArchives.length" class="mt-3 max-h-40 overflow-auto rounded-lg border border-gray-200">
              <div
                v-for="archive in backupArchives"
                :key="archive.path"
                class="flex items-start justify-between gap-3 border-b border-gray-100 px-3 py-2 text-xs last:border-b-0"
              >
                <div class="min-w-0">
                  <div class="font-medium text-gray-800 break-all">{{ archive.name }}</div>
                  <div class="mt-1 text-gray-500">
                    {{ formatArchiveMeta(archive) }}
                  </div>
                </div>
                <Button
                  variant="destructive"
                  size="sm"
                  class="shrink-0"
                  @click="deleteBackupArchive(archive)"
                  :disabled="deletingBackupArchive === archive.name"
                >
                  {{ deletingBackupArchive === archive.name ? '删除中' : '删除' }}
                </Button>
              </div>
            </div>
            <p v-else class="mt-2 text-xs text-gray-500">
              {{ loadingBackupArchives ? '正在读取备份列表...' : '当前备份目录下没有 .tar.gz 文件' }}
            </p>
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
import { computed, onMounted, onUnmounted, ref } from 'vue'
import api from '@/utils/api'
import Button from '@/components/ui/Button.vue'
import Card from '@/components/ui/Card.vue'
import Input from '@/components/ui/Input.vue'
import AppHeader from '@/components/AppHeader.vue'

const containers = ref([])
const environmentCheck = ref(null)
const checkingEnvironment = ref(false)
const loading = ref(false)
const creating = ref(false)
const loadingAppConfig = ref(false)
const savingAppConfig = ref(false)
const loadingHostInfo = ref(false)
const loadingBackupArchives = ref(false)
const deletingBackupArchive = ref('')
const backupStatuses = ref({})
const restoreStatuses = ref({})
const backupArchives = ref([])
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
const appConfig = ref({
  lxcImage: 'ubuntu:22.04',
  lxcBackupDir: './backups',
})
const hostInfo = ref({
  recommendedContainerHostIP: '',
  lanIPs: [],
  bridgeIPs: [],
})
const newContainer = ref({
  name: '',
  password: '',
  sourceType: 'default_image',
  image: '',
  backupFile: '',
})
const passwordContainer = ref({
  name: '',
  password: '',
})
let backupPollingTimer = null
let restorePollingTimer = null

const activeBackups = computed(() =>
  Object.values(backupStatuses.value)
    .filter(Boolean)
    .sort((a, b) => new Date(b.updatedAt || 0) - new Date(a.updatedAt || 0))
)

const activeRestores = computed(() =>
  Object.values(restoreStatuses.value)
    .filter(Boolean)
    .sort((a, b) => new Date(b.updatedAt || 0) - new Date(a.updatedAt || 0))
)

const hasRunningRestores = computed(() =>
  Object.values(restoreStatuses.value).some(
    (restore) => restore?.status === 'queued' || restore?.status === 'running'
  )
)

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

const loadBackupStatuses = async ({ silent = true } = {}) => {
  try {
    const response = await api.get('/lxc/backup-status')
    const nextStatuses = {}
    for (const backup of response.data?.backups || []) {
      nextStatuses[backup.name] = backup
    }
    backupStatuses.value = nextStatuses
  } catch (err) {
    if (!silent) {
      error.value = err.response?.data?.error || '加载备份状态失败'
    }
  }
}

const loadRestoreStatuses = async ({ silent = true } = {}) => {
  try {
    const response = await api.get('/lxc/restore-status')
    const nextStatuses = {}
    for (const restore of response.data?.restores || []) {
      nextStatuses[restore.name] = restore
    }
    restoreStatuses.value = nextStatuses
  } catch (err) {
    if (!silent) {
      error.value = err.response?.data?.error || '加载恢复状态失败'
    }
  }
}

const loadAppConfig = async () => {
  loadingAppConfig.value = true

  try {
    const response = await api.get('/app-config')
    appConfig.value = {
      lxcImage: response.data.lxcImage || 'ubuntu:22.04',
      lxcBackupDir: response.data.lxcBackupDir || './backups',
    }
  } catch (err) {
    error.value = err.response?.data?.error || '加载容器配置失败'
  } finally {
    loadingAppConfig.value = false
  }
}

const loadHostInfo = async () => {
  loadingHostInfo.value = true

  try {
    const response = await api.get('/host-info')
    hostInfo.value = {
      recommendedContainerHostIP: response.data.recommendedContainerHostIP || '',
      lanIPs: response.data.lanIPs || [],
      bridgeIPs: response.data.bridgeIPs || [],
    }
  } catch (err) {
    error.value = err.response?.data?.error || '加载宿主机 IP 失败'
  } finally {
    loadingHostInfo.value = false
  }
}

const loadBackupArchives = async ({ silent = true } = {}) => {
  loadingBackupArchives.value = true

  try {
    const response = await api.get('/lxc/backup-archives')
    backupArchives.value = response.data?.archives || []
  } catch (err) {
    if (!silent) {
      error.value = err.response?.data?.error || '加载备份文件列表失败'
    }
  } finally {
    loadingBackupArchives.value = false
  }
}

const deleteBackupArchive = async (archive) => {
  if (!archive?.name) {
    return
  }
  if (!confirm(`确定要删除备份文件 "${archive.name}" 吗？此操作不可恢复！`)) {
    return
  }

  deletingBackupArchive.value = archive.name
  error.value = ''
  success.value = ''

  try {
    await api.delete('/lxc/backup-archives', { data: { name: archive.name } })
    if (newContainer.value.backupFile === archive.name) {
      newContainer.value.backupFile = ''
    }
    success.value = '备份文件删除成功'
    await loadBackupArchives({ silent: false })
  } catch (err) {
    error.value = err.response?.data?.error || '删除备份文件失败'
  } finally {
    deletingBackupArchive.value = ''
  }
}

const saveAppConfig = async () => {
  if (!appConfig.value.lxcImage) {
    error.value = '请填写默认容器镜像'
    return
  }

  if (!appConfig.value.lxcBackupDir) {
    error.value = '请填写容器备份目录'
    return
  }

  savingAppConfig.value = true
  error.value = ''
  success.value = ''

  try {
    await api.put('/app-config', {
      lxcImage: appConfig.value.lxcImage,
      lxcBackupDir: appConfig.value.lxcBackupDir,
    })
    success.value = '容器配置已更新'
    await loadAppConfig()
  } catch (err) {
    error.value = err.response?.data?.error || '保存容器配置失败'
  } finally {
    savingAppConfig.value = false
  }
}

const createContainer = async () => {
  if (!newContainer.value.name || !newContainer.value.password) {
    error.value = '请填写容器名称和密码'
    return
  }

  if (newContainer.value.sourceType === 'custom_image' && !newContainer.value.image) {
    error.value = '请填写自定义镜像'
    return
  }

  if (newContainer.value.sourceType === 'backup' && !newContainer.value.backupFile) {
    error.value = '请选择备份文件'
    return
  }

  creating.value = true
  error.value = ''
  success.value = ''

  try {
    const response = await api.post('/lxc/create', {
      name: newContainer.value.name,
      password: newContainer.value.password,
      sourceType: newContainer.value.sourceType,
      image: newContainer.value.image,
      backupFile: newContainer.value.backupFile,
    })
    if (newContainer.value.sourceType === 'backup' && response.data?.restore) {
      restoreStatuses.value = {
        ...restoreStatuses.value,
        [response.data.restore.name]: response.data.restore,
      }
      success.value = `已开始后台恢复容器 "${response.data.restore.name}"，可在恢复任务状态中查看进度`
      startRestorePolling()
    } else {
      success.value = '容器创建成功'
    }
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

const backupContainer = async (name) => {
  error.value = ''
  success.value = ''

  try {
    const response = await api.post('/lxc/backup', { name })
    if (response.data?.backup) {
      backupStatuses.value = {
        ...backupStatuses.value,
        [name]: response.data.backup,
      }
    }
    success.value = `已开始备份容器 "${name}"，备份文件会导出到 ${appConfig.value.lxcBackupDir || './backups'}`
    startBackupPolling()
  } catch (err) {
    error.value = err.response?.data?.error || '容器备份失败'
  }
}

const isBackupRunning = (name) => {
  const backup = backupStatuses.value[name]
  return backup?.status === 'queued' || backup?.status === 'running'
}

const formatBackupStatus = (backup) => {
  const percent = typeof backup.progress === 'number' ? `${backup.progress}%` : ''
  return [backup.message, percent].filter(Boolean).join(' · ')
}

const restoreLogPrefixes = ['输出:', '原始输出:', 'Importing instance:']

const formatRestoreStatusSummary = (restore) => {
  const percent = typeof restore.progress === 'number' ? `${restore.progress}%` : ''
  const message = restore.message || ''
  const logIndex = restoreLogPrefixes
    .map((prefix) => message.indexOf(prefix))
    .filter((index) => index >= 0)
    .sort((a, b) => a - b)[0]

  let summary = logIndex >= 0 ? message.slice(0, logIndex).trim() : message.trim()
  if (!summary || summary.length > 140) {
    summary = message.length > 140 ? `${message.slice(0, 140).trim()}...` : message
  }

  return [summary, percent].filter(Boolean).join(' · ')
}

const hasRestoreLog = (restore) => {
  const message = restore.message || ''
  return message.length > 180 || restoreLogPrefixes.some((prefix) => message.includes(prefix))
}

const formatBackupTime = (value) => {
  if (!value) {
    return ''
  }

  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return ''
  }

  return date.toLocaleString('zh-CN', { hour12: false })
}

const startBackupPolling = () => {
  if (backupPollingTimer) {
    return
  }

  backupPollingTimer = window.setInterval(async () => {
    await loadBackupStatuses()
    const hasRunningBackup = Object.values(backupStatuses.value).some(
      (backup) => backup?.status === 'queued' || backup?.status === 'running'
    )
    if (!hasRunningBackup) {
      stopBackupPolling()
    }
  }, 3000)
}

const stopBackupPolling = () => {
  if (backupPollingTimer) {
    window.clearInterval(backupPollingTimer)
    backupPollingTimer = null
  }
}

const startRestorePolling = () => {
  if (restorePollingTimer) {
    return
  }

  restorePollingTimer = window.setInterval(async () => {
    await loadRestoreStatuses()
    const hasRunningRestore = Object.values(restoreStatuses.value).some(
      (restore) => restore?.status === 'queued' || restore?.status === 'running'
    )
    if (!hasRunningRestore) {
      stopRestorePolling()
      loadContainers()
    }
  }, 3000)
}

const stopRestorePolling = () => {
  if (restorePollingTimer) {
    window.clearInterval(restorePollingTimer)
    restorePollingTimer = null
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
    sourceType: 'default_image',
    image: '',
    backupFile: '',
  }
  error.value = ''
}

const openCreateDialog = async () => {
  showCreateDialog.value = true
  error.value = ''
  success.value = ''
  newContainer.value = {
    name: '',
    password: '',
    sourceType: 'default_image',
    image: '',
    backupFile: '',
  }
  await loadBackupArchives()
}

const formatArchiveMeta = (archive) => {
  const size = typeof archive.sizeBytes === 'number'
    ? `${(archive.sizeBytes / 1024 / 1024 / 1024).toFixed(2)} GB`
    : ''
  const time = archive.modifiedAt ? formatBackupTime(archive.modifiedAt) : ''
  return [time, size, archive.path].filter(Boolean).join(' · ')
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

onMounted(() => {
  loadHostInfo()
  loadAppConfig()
  checkEnvironment()
  loadBackupStatuses().then(() => {
    const hasRunningBackup = Object.values(backupStatuses.value).some(
      (backup) => backup?.status === 'queued' || backup?.status === 'running'
    )
    if (hasRunningBackup) {
      startBackupPolling()
    }
  })
  loadRestoreStatuses().then(() => {
    if (hasRunningRestores.value) {
      startRestorePolling()
    }
  })
})

onUnmounted(() => {
  stopBackupPolling()
  stopRestorePolling()
})
</script>
