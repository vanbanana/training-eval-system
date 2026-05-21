<script setup lang="ts">
import { ListboxItem, type ListboxItemProps } from 'reka-ui'
import { cn } from '@/lib/utils'

const props = defineProps<ListboxItemProps & { class?: string }>()
</script>

<template>
  <ListboxItem
    v-bind="props"
    :class="
      cn(
        'relative flex cursor-default select-none items-center gap-2 rounded-sm px-2 py-1.5 text-sm outline-none data-[highlighted]:bg-accent-soft data-[highlighted]:text-ink data-[disabled=true]:pointer-events-none data-[disabled=true]:opacity-50 [&_svg]:size-4 [&_svg]:shrink-0',
        props.class,
      )
    "
  >
    <slot />
  </ListboxItem>
</template>
