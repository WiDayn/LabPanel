<template>
  <div class="min-h-screen bg-gray-50">
    <AppHeader />

    <main class="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8 py-6 space-y-6">
      <Card class="p-6">
        <div class="mb-5">
          <h2 class="text-lg font-semibold text-gray-900">系统标题</h2>
          <p class="mt-1 text-sm text-gray-500">用于浏览器标题和页面顶部标题。</p>
        </div>

        <form class="space-y-4" @submit.prevent="saveTitle">
          <div>
            <label class="block text-sm font-medium mb-2">标题</label>
            <Input v-model="titleForm.title" placeholder="请输入系统标题" />
          </div>

          <div v-if="titleMessage" :class="titleMessageClass">{{ titleMessage }}</div>

          <div class="flex justify-end">
            <Button type="submit" :disabled="savingTitle">
              {{ savingTitle ? '保存中...' : '保存标题' }}
            </Button>
          </div>
        </form>
      </Card>

      <Card class="p-6">
        <div class="mb-5">
          <h2 class="text-lg font-semibold text-gray-900">管理员账号</h2>
          <p class="mt-1 text-sm text-gray-500">修改登录账号，或填写新密码后一起更新密码。</p>
        </div>

        <form class="space-y-4" @submit.prevent="saveAccount">
          <div>
            <label class="block text-sm font-medium mb-2">账号</label>
            <Input v-model="accountForm.username" placeholder="请输入管理员账号" />
          </div>

          <div>
            <label class="block text-sm font-medium mb-2">当前密码</label>
            <Input v-model="accountForm.currentPassword" type="password" placeholder="请输入当前密码" />
          </div>

          <div class="grid gap-4 sm:grid-cols-2">
            <div>
              <label class="block text-sm font-medium mb-2">新密码</label>
              <Input v-model="accountForm.newPassword" type="password" placeholder="不修改可留空" />
            </div>
            <div>
              <label class="block text-sm font-medium mb-2">确认新密码</label>
              <Input v-model="accountForm.confirmPassword" type="password" placeholder="再次输入新密码" />
            </div>
          </div>

          <div v-if="accountMessage" :class="accountMessageClass">{{ accountMessage }}</div>

          <div class="flex justify-end">
            <Button type="submit" :disabled="savingAccount">
              {{ savingAccount ? '保存中...' : '保存账号' }}
            </Button>
          </div>
        </form>
      </Card>
    </main>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import api from '@/utils/api'
import AppHeader from '@/components/AppHeader.vue'
import Button from '@/components/ui/Button.vue'
import Card from '@/components/ui/Card.vue'
import Input from '@/components/ui/Input.vue'
import { setAppTitle } from '@/utils/publicConfig'

const titleForm = ref({ title: '' })
const accountForm = ref({
  username: '',
  currentPassword: '',
  newPassword: '',
  confirmPassword: '',
})

const savingTitle = ref(false)
const savingAccount = ref(false)
const titleMessage = ref('')
const titleMessageType = ref('success')
const accountMessage = ref('')
const accountMessageType = ref('success')

const titleMessageClass = computed(() =>
  titleMessageType.value === 'success' ? 'text-sm text-green-600' : 'text-sm text-red-500'
)
const accountMessageClass = computed(() =>
  accountMessageType.value === 'success' ? 'text-sm text-green-600' : 'text-sm text-red-500'
)

const loadSettings = async () => {
  const response = await api.get('/system-settings')
  titleForm.value.title = response.data.title || ''
  accountForm.value.username = response.data.username || ''
}

const saveTitle = async () => {
  titleMessage.value = ''
  const title = titleForm.value.title.trim()
  if (!title) {
    titleMessageType.value = 'error'
    titleMessage.value = '系统标题不能为空'
    return
  }

  savingTitle.value = true
  try {
    const response = await api.put('/system-settings/title', { title })
    setAppTitle(response.data.title || title)
    titleForm.value.title = response.data.title || title
    titleMessageType.value = 'success'
    titleMessage.value = response.data.message || '系统标题已更新'
  } catch (err) {
    titleMessageType.value = 'error'
    titleMessage.value = err.response?.data?.error || '保存系统标题失败'
  } finally {
    savingTitle.value = false
  }
}

const saveAccount = async () => {
  accountMessage.value = ''
  const username = accountForm.value.username.trim()
  const newPassword = accountForm.value.newPassword.trim()
  const confirmPassword = accountForm.value.confirmPassword.trim()

  if (!username || !accountForm.value.currentPassword) {
    accountMessageType.value = 'error'
    accountMessage.value = '账号和当前密码不能为空'
    return
  }
  if (newPassword !== confirmPassword) {
    accountMessageType.value = 'error'
    accountMessage.value = '两次输入的新密码不一致'
    return
  }

  savingAccount.value = true
  try {
    const response = await api.put('/system-settings/account', {
      username,
      currentPassword: accountForm.value.currentPassword,
      newPassword,
    })
    accountForm.value.username = response.data.username || username
    accountForm.value.currentPassword = ''
    accountForm.value.newPassword = ''
    accountForm.value.confirmPassword = ''
    accountMessageType.value = 'success'
    accountMessage.value = response.data.message || '管理员账号已更新'
  } catch (err) {
    accountMessageType.value = 'error'
    accountMessage.value = err.response?.data?.error || '保存管理员账号失败'
  } finally {
    savingAccount.value = false
  }
}

onMounted(() => {
  loadSettings().catch((err) => {
    accountMessageType.value = 'error'
    accountMessage.value = err.response?.data?.error || '加载系统设置失败'
  })
})
</script>
