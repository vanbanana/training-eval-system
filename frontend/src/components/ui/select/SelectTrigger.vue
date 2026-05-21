<script setup lang="ts">
import { SelectIcon, SelectTrigger, type SelectTriggerProps } from 'reka-ui'
import { ChevronDown } from 'lucide-vue-next'
import { cn } from '@/lib/utils'

const props = defineProps<SelectTriggerProps & { class?: string }>()
</script>

<template>
  <SelectTrigger
    v-bind="props"
    :class="
      cn(
        'flex h-9 w-full items-center justify-between rounded-md border border-border-strong bg-surface px-3 py-2 text-sm shadow-sm placeholder:text-subtle-foreground focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-1 disabled:cursor-not-allowed disabled:opacity-50 [&>span]:line-clamp-1',
        props.class,
      )
    "
  >
    <slot />
    <SelectIcon as-child>
      <ChevronDown class="w-4 h-4 opacity-60" />
    </SelectIcon>
  </SelectTrigger>
</template>
