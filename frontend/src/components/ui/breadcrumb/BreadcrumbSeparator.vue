<script setup lang="ts">
import { ChevronRight } from 'lucide-vue-next'
import { cn } from '@/lib/utils'

defineProps<{ class?: string }>()
</script>

<template>
  <li role="presentation" aria-hidden="true" :class="cn('text-subtle-foreground', $props.class)">
    <slot>
      <ChevronRight class="h-3 w-3" />
    </slot>
  </li>
</template>
