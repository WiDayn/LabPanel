<template>
  <div class="min-h-screen flex items-center justify-center bg-gray-50">
    <Card class="w-full max-w-md p-8">
      <h1 class="text-2xl font-bold text-center mb-6">LabPanel 管理面板</h1>
      <form @submit.prevent="handleLogin" class="space-y-4">
        <div>
          <label class="block text-sm font-medium mb-2">用户名</label>
          <Input
            v-model="username"
            placeholder="请输入用户名"
            required
          />
        </div>
        <div>
          <label class="block text-sm font-medium mb-2">密码</label>
          <Input
            v-model="password"
            type="password"
            placeholder="请输入密码"
            required
          />
        </div>
        <div v-if="error" class="text-red-500 text-sm">{{ error }}</div>
        <Button type="submit" :disabled="loading" class="w-full">
          {{ loading ? '登录中...' : '登录' }}
        </Button>
      </form>
    </Card>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import api from '@/utils/api'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'
import Card from '@/components/ui/Card.vue'

const router = useRouter()
const username = ref('')
const password = ref('')
const error = ref('')
const loading = ref(false)

const handleLogin = async () => {
  error.value = ''
  loading.value = true

  try {
    const response = await api.post('/login', {
      username: username.value,
      password: password.value,
    })

    localStorage.setItem('token', response.data.token)
    router.push('/dashboard')
  } catch (err) {
    error.value = err.response?.data?.error || '登录失败，请检查用户名和密码'
  } finally {
    loading.value = false
  }
}
</script>

