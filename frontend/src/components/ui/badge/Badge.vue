<script setup lang="ts">
import { cn } from '@/lib/utils'
import { type BadgeVariants, badgeVariants } from './index'

defineProps<{
  variant?: BadgeVariants['variant']
  class?: string
}>()
</script>

<template>
  <span :class="cn(badgeVariants({ variant }), $props.class)">
    <slot />
  </span>
</template>
