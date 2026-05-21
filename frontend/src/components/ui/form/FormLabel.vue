<script setup lang="ts">
import { Label } from '@/components/ui/label'
import { useFormItemId } from './use-form-item-id'
import { cn } from '@/lib/utils'

defineProps<{ class?: string; required?: boolean }>()

const id = useFormItemId()
</script>

<template>
  <Label :for="id" :class="cn('text-sm font-medium text-ink', $props.class)">
    <slot />
    <span v-if="required" class="text-danger ml-0.5">*</span>
  </Label>
</template>
