<script setup lang="ts">
import { TabsList, type TabsListProps } from 'reka-ui'
import { cn } from '@/lib/utils'

const props = defineProps<TabsListProps & { class?: string }>()
</script>

<template>
  <TabsList
    v-bind="props"
    :class="
      cn(
        'inline-flex h-10 items-center justify-start gap-1 rounded-md border-b border-border bg-card p-1',
        props.class,
      )
    "
  >
    <slot />
  </TabsList>
</template>
