<script setup>
import { computed } from 'vue'

const props = defineProps({
  sort: { type: Object, required: true },
  column: { type: String, required: true },
  label: { type: String, required: true },
  numeric: Boolean,
})
const direction = computed(() => props.sort.table.key === props.column ? props.sort.table.direction : undefined)
</script>

<template>
  <th scope="col" :aria-sort="direction" :class="{ 'text-right': numeric, 'text-blue-700': direction }">
    <button type="button" class="inline-flex min-h-7 max-w-full items-center gap-1.5 rounded-sm text-left font-medium focus-visible:outline focus-visible:outline-2 focus-visible:outline-blue-500 hover:text-blue-700" @click="sort.toggle(column)">
      <span>{{ label }}</span>
      <span aria-hidden="true" class="w-3 shrink-0 text-center" :class="direction ? 'text-blue-700' : 'text-gray-500'">{{ direction === 'ascending' ? '↑' : direction === 'descending' ? '↓' : '↕' }}</span>
    </button>
  </th>
</template>
