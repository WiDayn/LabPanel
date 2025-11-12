<template>
  <div :class="cardClass">
    <slot />
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { cn } from '@/utils/cn'

const props = defineProps({
  class: {
    type: String,
    default: '',
  },
})

const cardClass = computed(() =>
  cn('rounded-lg border bg-card text-card-foreground shadow-sm', props.class)
)
</script>

