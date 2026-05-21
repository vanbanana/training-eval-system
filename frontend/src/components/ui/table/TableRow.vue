<script setup lang="ts">
import { cn } from '@/lib/utils'

defineProps<{ class?: string }>()
</script>

<template>
  <tr :class="cn('border-b border-border transition-colors duration-150 hover:bg-surface-2 data-[state=selected]:bg-muted', $props.class)">
    <slot />
  </tr>
</template>
