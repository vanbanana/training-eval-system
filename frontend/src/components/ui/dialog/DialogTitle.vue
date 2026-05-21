<script setup lang="ts">
import { DialogTitle, type DialogTitleProps } from 'reka-ui'
import { cn } from '@/lib/utils'

const props = defineProps<DialogTitleProps & { class?: string }>()
</script>

<template>
  <DialogTitle
    v-bind="props"
    :class="cn('text-base font-semibold leading-none tracking-tight text-ink', props.class)"
  >
    <slot />
  </DialogTitle>
</template>
