<script setup lang="ts">
import { Dialog, DialogContent } from '@/components/ui/dialog'
import Command from './Command.vue'

defineProps<{
  open?: boolean
}>()
const emits = defineEmits<{ (e: 'update:open', value: boolean): void }>()

function setOpen(v: boolean) {
  emits('update:open', v)
}
</script>

<template>
  <Dialog :open="open" @update:open="setOpen">
    <DialogContent class="overflow-hidden p-0 max-w-2xl shadow-lg">
      <Command class="[&_[cmdk-input-wrapper]]:border-b" :model-value="''">
        <slot />
      </Command>
    </DialogContent>
  </Dialog>
</template>
