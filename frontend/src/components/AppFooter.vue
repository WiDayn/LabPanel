<template>
  <footer class="border-t bg-white">
    <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-3 flex items-center justify-end">
      <div class="text-xs text-gray-500">
        版本 {{ versionDisplay || '加载中' }}
      </div>
    </div>
  </footer>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import api from '@/utils/api'

const versionDisplay = ref('')

onMounted(async () => {
  try {
    const response = await api.get('/version')
    versionDisplay.value = response.data.display || ''
  } catch (err) {
    versionDisplay.value = ''
  }
})
</script>
