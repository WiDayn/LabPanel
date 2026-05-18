<template>
  <header class="bg-white shadow-sm border-b">
    <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-4 flex justify-between items-center">
      <h1 class="text-xl font-semibold">{{ appTitle }}</h1>
      <Button variant="outline" @click="handleLogout">退出登录</Button>
    </div>
    <Navigation />
  </header>
</template>

<script setup>
import { useRouter } from 'vue-router'
import Button from '@/components/ui/Button.vue'
import Navigation from '@/components/Navigation.vue'
import { appTitle } from '@/utils/publicConfig'

const router = useRouter()

const handleLogout = () => {
  localStorage.removeItem('token')
  router.push('/login')
}
</script>
