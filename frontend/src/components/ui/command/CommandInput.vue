<script setup lang="ts">
import { ListboxFilter } from 'reka-ui'
import { Search } from 'lucide-vue-next'
import { cn } from '@/lib/utils'
import { useVModel } from '@vueuse/core'

interface Props {
  modelValue?: string
  placeholder?: string
  class?: string
}

const props = defineProps<Props>()
const emits = defineEmits<{ (e: 'update:modelValue', payload: string): void }>()

const value = useVModel(props, 'modelValue', emits, { passive: true, defaultValue: '' })
</script>

<template>
  <div class="flex items-center border-b border-border px-3" cmdk-input-wrapper>
    <Search class="mr-2 h-4 w-4 shrink-0 text-subtle-foreground" />
    <ListboxFilter
      v-model="value"
      :placeholder="placeholder"
      :class="
        cn(
          'flex h-11 w-full bg-transparent py-3 text-sm outline-none placeholder:text-subtle-foreground disabled:cursor-not-allowed disabled:opacity-50',
          props.class,
        )
      "
    />
  </div>
</template>
