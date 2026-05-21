<script setup lang="ts">
import { TabsTrigger, type TabsTriggerProps } from 'reka-ui'
import { cn } from '@/lib/utils'

const props = defineProps<TabsTriggerProps & { class?: string }>()
</script>

<template>
  <TabsTrigger
    v-bind="props"
    :class="
      cn(
        'inline-flex items-center justify-center whitespace-nowrap rounded-sm px-3 py-1.5 text-sm font-medium text-muted-foreground transition-all duration-150 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50',
        'data-[state=active]:bg-primary data-[state=active]:text-primary-foreground data-[state=active]:shadow-sm',
        'hover:text-ink',
        props.class,
      )
    "
  >
    <slot />
  </TabsTrigger>
</template>
