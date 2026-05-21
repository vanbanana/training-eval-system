<script setup lang="ts">
import { ToastRoot, type ToastRootEmits, type ToastRootProps, useForwardPropsEmits } from 'reka-ui'
import { cn } from '@/lib/utils'
import { type ToastVariants, toastVariants } from './variants'

const props = defineProps<ToastRootProps & { class?: string; variant?: ToastVariants['variant'] }>()
const emits = defineEmits<ToastRootEmits>()
const forwarded = useForwardPropsEmits(props, emits)
</script>

<template>
  <ToastRoot v-bind="forwarded" :class="cn(toastVariants({ variant }), props.class)">
    <slot />
  </ToastRoot>
</template>
