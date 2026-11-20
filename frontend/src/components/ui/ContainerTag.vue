<script setup>
const props = defineProps({
  label: { type: String, required: true },
  tone: { type: String, required: true },
  statusDot: String,
  selectable: Boolean,
  selected: Boolean,
})

const emit = defineEmits(['toggle'])
</script>

<template>
  <component
    :is="props.selectable ? 'button' : 'span'"
    :type="props.selectable ? 'button' : undefined"
    :aria-pressed="props.selectable ? props.selected : undefined"
    class="inline-flex max-w-full items-center gap-1 rounded-full border px-2 text-xs font-medium"
    :class="[
      props.tone,
      props.selectable ? 'min-h-7 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-500' : 'min-h-5',
      props.selected ? 'ring-1 ring-inset ring-current' : props.selectable ? 'hover:brightness-95' : '',
    ]"
    @click="props.selectable && emit('toggle')"
  >
    <span v-if="props.selected" aria-hidden="true">✓</span>
    <span v-else-if="props.statusDot" class="h-1.5 w-1.5 shrink-0 rounded-full" :class="props.statusDot" aria-hidden="true"></span>
    <span class="min-w-0 break-all">{{ props.label }}</span>
  </component>
</template>
