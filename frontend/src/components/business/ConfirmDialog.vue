<script setup lang="ts">
import { ref } from 'vue'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'

interface ConfirmOptions {
  title?: string
  description?: string
  confirmText?: string
  cancelText?: string
  /** 主按钮颜色 */
  variant?: 'default' | 'destructive'
}

const open = ref(false)
const options = ref<ConfirmOptions>({})
let resolveFn: ((v: boolean) => void) | null = null

function show(opts: ConfirmOptions): Promise<boolean> {
  options.value = opts
  open.value = true
  return new Promise<boolean>((resolve) => {
    resolveFn = resolve
  })
}

function close(result: boolean) {
  open.value = false
  if (resolveFn) {
    resolveFn(result)
    resolveFn = null
  }
}

defineExpose({ show })
</script>

<template>
  <Dialog :open="open" @update:open="(v) => { if (!v) close(false) }">
    <DialogContent class="max-w-md">
      <DialogHeader>
        <DialogTitle>{{ options.title ?? '确认操作' }}</DialogTitle>
        <DialogDescription v-if="options.description">{{ options.description }}</DialogDescription>
      </DialogHeader>
      <DialogFooter class="mt-2 gap-2">
        <Button variant="outline" @click="close(false)">{{ options.cancelText ?? '取消' }}</Button>
        <Button :variant="options.variant === 'destructive' ? 'destructive' : 'default'" @click="close(true)">
          {{ options.confirmText ?? '确认' }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
