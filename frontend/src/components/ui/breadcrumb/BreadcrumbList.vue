<script setup lang="ts">
import { cn } from '@/lib/utils'

defineProps<{ class?: string }>()
</script>

<template>
  <ol :class="cn('flex flex-wrap items-center gap-1.5 break-words text-xs text-muted-foreground sm:gap-2', $props.class)">
    <slot />
  </ol>
</template>
