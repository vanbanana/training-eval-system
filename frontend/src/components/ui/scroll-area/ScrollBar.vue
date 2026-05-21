<script setup lang="ts">
import {
  ScrollAreaScrollbar,
  type ScrollAreaScrollbarProps,
  ScrollAreaThumb,
} from 'reka-ui'
import { cn } from '@/lib/utils'

const props = withDefaults(defineProps<ScrollAreaScrollbarProps & { class?: string }>(), {
  orientation: 'vertical',
})
</script>

<template>
  <ScrollAreaScrollbar
    v-bind="props"
    :class="
      cn(
        'flex touch-none select-none transition-colors',
        props.orientation === 'vertical' && 'h-full w-2 border-l border-l-transparent p-px',
        props.orientation === 'horizontal' && 'h-2 flex-col border-t border-t-transparent p-px',
        props.class,
      )
    "
  >
    <ScrollAreaThumb class="relative flex-1 rounded-full bg-border-strong" />
  </ScrollAreaScrollbar>
</template>
