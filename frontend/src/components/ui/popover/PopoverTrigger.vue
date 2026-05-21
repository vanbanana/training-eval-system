<script setup lang="ts">
import { PopoverTrigger, type PopoverTriggerProps } from 'reka-ui'

const props = defineProps<PopoverTriggerProps & { class?: string }>()
</script>

<template>
  <PopoverTrigger v-bind="props">
    <slot />
  </PopoverTrigger>
</template>
