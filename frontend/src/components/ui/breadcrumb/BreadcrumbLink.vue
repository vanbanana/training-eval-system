<script setup lang="ts">
import { Primitive, type PrimitiveProps } from 'reka-ui'
import { cn } from '@/lib/utils'

const props = withDefaults(defineProps<PrimitiveProps & { class?: string }>(), {
  as: 'a',
})
</script>

<template>
  <Primitive :as="as" :as-child="asChild" :class="cn('transition-colors hover:text-ink', props.class)">
    <slot />
  </Primitive>
</template>
