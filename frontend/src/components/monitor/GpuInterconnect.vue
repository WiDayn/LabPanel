<script setup>
import { formatGpuLinkRate } from '@/utils/gpuInterconnects'

defineProps({
  links: { type: Array, required: true },
})
</script>

<template>
  <div v-if="links.length" class="space-y-1.5 text-xs tabular-nums">
    <div v-for="link in links" :key="link.protocol" class="link-row grid grid-cols-[var(--gpu-link-columns)] items-baseline gap-x-2">
      <span class="whitespace-nowrap font-semibold text-gray-800">{{ link.protocol }}</span>
      <div class="link-spec text-gray-600">
        <template v-if="link.protocol === 'PCIe'">
          <div class="whitespace-nowrap"><span class="mr-1 text-gray-500">当前</span>{{ link.spec || 'N/A' }}</div>
          <div v-if="link.maxSpec" class="mt-0.5 whitespace-nowrap"><span class="mr-1 text-gray-500">上限</span>{{ link.maxSpec }}</div>
        </template>
        <template v-else>{{ link.spec || '—' }}</template>
      </div>
      <span class="link-rate whitespace-nowrap text-right text-gray-700">
        <abbr title="GPU 接收" class="link-direction sr-only no-underline">RX</abbr>
        {{ formatGpuLinkRate(link.rxBytesPerSecond) }}
      </span>
      <span class="link-rate whitespace-nowrap text-right text-gray-700">
        <abbr title="GPU 发送" class="link-direction sr-only no-underline">TX</abbr>
        {{ formatGpuLinkRate(link.txBytesPerSecond) }}
      </span>
    </div>
  </div>
  <span v-else class="text-xs text-gray-500">N/A</span>
</template>

<style scoped>
@container gpu-traffic (width < 640px) {
  .link-row {
    @apply grid-cols-2 gap-y-1;
  }

  .link-spec {
    @apply text-right;
  }

  .link-rate {
    @apply flex items-baseline justify-between gap-2;
  }

  .link-direction {
    @apply not-sr-only text-gray-500;
  }
}
</style>
