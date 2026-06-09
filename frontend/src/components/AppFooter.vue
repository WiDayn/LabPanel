<template>
  <footer class="border-t bg-white">
    <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-3 flex flex-wrap items-center justify-end gap-x-3 gap-y-1 text-xs text-gray-500">
      <span>版本 {{ versionDisplay || '加载中' }}</span>
      <span class="text-gray-300">|</span>
      <a
        href="https://github.com/WiDayn/LabPanel"
        target="_blank"
        rel="noopener noreferrer"
        class="inline-flex items-center gap-1.5 font-medium text-gray-600 hover:text-gray-900"
        aria-label="GitHub 仓库 WiDayn/LabPanel"
      >
        <svg class="h-3.5 w-3.5" viewBox="0 0 16 16" aria-hidden="true" fill="currentColor">
          <path d="M8 0C3.58 0 0 3.67 0 8.2c0 3.62 2.29 6.69 5.47 7.77.4.08.55-.18.55-.4 0-.2-.01-.84-.01-1.52-2.01.38-2.53-.5-2.69-.96-.09-.24-.48-.96-.82-1.15-.28-.16-.68-.56-.01-.57.63-.01 1.08.59 1.23.84.72 1.24 1.87.89 2.33.68.07-.53.28-.89.51-1.1-1.78-.21-3.64-.91-3.64-4.03 0-.89.31-1.62.82-2.19-.08-.21-.36-1.04.08-2.16 0 0 .67-.22 2.2.84A7.4 7.4 0 0 1 8 3.98c.68 0 1.36.09 2 .27 1.53-1.06 2.2-.84 2.2-.84.44 1.12.16 1.95.08 2.16.51.57.82 1.3.82 2.19 0 3.13-1.87 3.82-3.65 4.03.29.25.54.75.54 1.52 0 1.1-.01 1.98-.01 2.25 0 .22.15.48.55.4A8.06 8.06 0 0 0 16 8.2C16 3.67 12.42 0 8 0Z" />
        </svg>
        <span>WiDayn/LabPanel</span>
      </a>
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
