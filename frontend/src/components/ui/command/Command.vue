<script setup lang="ts">
import { ListboxRoot, type ListboxRootEmits, type ListboxRootProps, useForwardPropsEmits } from 'reka-ui'
import { cn } from '@/lib/utils'

const props = defineProps<ListboxRootProps & { class?: string }>()
const emits = defineEmits<ListboxRootEmits>()
const forwarded = useForwardPropsEmits(props, emits)
</script>

<template>
  <ListboxRoot
    v-bind="forwarded"
    :class="cn('flex h-full w-full flex-col overflow-hidden rounded-md bg-popover text-popover-foreground', props.class)"
  >
    <slot />
  </ListboxRoot>
</template>
