<script setup lang="ts">
import { Separator, type SeparatorProps } from 'reka-ui'
import { cn } from '@/lib/utils'

const props = withDefaults(defineProps<SeparatorProps & { class?: string }>(), {
  decorative: true,
  orientation: 'horizontal',
})
</script>

<template>
  <Separator
    v-bind="props"
    :class="
      cn(
        'shrink-0 bg-border',
        props.orientation === 'vertical' ? 'h-full w-px' : 'h-px w-full',
        props.class,
      )
    "
  />
</template>
