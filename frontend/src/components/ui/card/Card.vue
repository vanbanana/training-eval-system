<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { cn } from '@/lib/utils'

defineProps<{ class?: HTMLAttributes['class'] }>()
</script>

<template>
  <div
    :class="cn('min-w-0 h-auto rounded-lg border border-border bg-card text-card-foreground transition-shadow duration-200 ease-out [box-shadow:var(--shadow-sm)] hover:[box-shadow:var(--shadow-md)]', $props.class)"
  >
    <slot />
  </div>
</template>
