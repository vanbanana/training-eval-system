<script setup lang="ts">
import { cn } from '@/lib/utils'

defineProps<{ class?: string }>()
</script>

<template>
  <div :class="cn('py-6 text-center text-sm text-muted-foreground', $props.class)">
    <slot>未找到结果</slot>
  </div>
</template>
