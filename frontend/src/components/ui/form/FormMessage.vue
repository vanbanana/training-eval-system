<script setup lang="ts">
import { cn } from '@/lib/utils'

defineProps<{ class?: string }>()
</script>

<template>
  <p v-if="$slots.default" :class="cn('text-xs font-medium text-danger', $props.class)">
    <slot />
  </p>
</template>
