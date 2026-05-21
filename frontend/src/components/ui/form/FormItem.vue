<script setup lang="ts">
import { type Ref, provide, ref } from 'vue'
import { cn } from '@/lib/utils'
import { FORM_ITEM_ID_KEY } from './use-form-item-id'

const id = ref(`form-item-${Math.random().toString(36).slice(2, 11)}`)
provide<Ref<string>>(FORM_ITEM_ID_KEY, id)

defineProps<{ class?: string }>()
</script>

<template>
  <div :class="cn('space-y-2', $props.class)">
    <slot />
  </div>
</template>
