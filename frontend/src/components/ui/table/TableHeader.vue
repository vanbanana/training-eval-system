<script setup lang="ts">
import { cn } from '@/lib/utils'

defineProps<{ class?: string }>()
</script>

<template>
  <thead :class="cn('bg-surface-2 [&_tr]:border-b border-border', $props.class)">
    <slot />
  </thead>
</template>
