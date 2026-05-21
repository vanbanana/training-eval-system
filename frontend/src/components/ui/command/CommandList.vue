<script setup lang="ts">
import { ListboxContent, type ListboxContentProps } from 'reka-ui'
import { cn } from '@/lib/utils'

const props = defineProps<ListboxContentProps & { class?: string }>()
</script>

<template>
  <ListboxContent
    v-bind="props"
    :class="cn('max-h-[300px] overflow-y-auto overflow-x-hidden p-1', props.class)"
  >
    <slot />
  </ListboxContent>
</template>
