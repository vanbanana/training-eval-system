<script setup lang="ts">
import { cn } from '@/lib/utils'

defineProps<{ class?: string }>()
</script>

<template>
  <span aria-current="page" role="link" aria-disabled="true" :class="cn('font-semibold text-ink', $props.class)">
    <slot />
  </span>
</template>
