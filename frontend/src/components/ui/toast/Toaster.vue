<script setup lang="ts">
import { ToastClose, ToastDescription, ToastProvider, ToastTitle, ToastViewport } from 'reka-ui'
import { X } from 'lucide-vue-next'
import Toast from './Toast.vue'
import { useToast } from './use-toast'

const { toasts, dismiss } = useToast()
</script>

<template>
  <ToastProvider :duration="5000" swipe-direction="right">
    <Toast
      v-for="t in toasts"
      :key="t.id"
      :variant="t.variant"
      :open="t.open"
      :duration="t.duration"
      @update:open="(v) => { if (!v) dismiss(t.id) }"
    >
      <div class="grid gap-1">
        <ToastTitle v-if="t.title" class="text-sm font-semibold">{{ t.title }}</ToastTitle>
        <ToastDescription v-if="t.description" class="text-sm opacity-90">{{ t.description }}</ToastDescription>
      </div>
      <ToastClose class="absolute right-2 top-2 rounded-md p-1 text-foreground/60 transition-colors hover:text-foreground focus:outline-none focus:ring-2 focus:ring-ring" aria-label="关闭">
        <X class="w-4 h-4" />
      </ToastClose>
    </Toast>

    <ToastViewport
      class="fixed top-0 right-0 z-[100] flex max-h-screen w-full flex-col gap-2 p-4 sm:max-w-[420px] sm:bottom-auto sm:top-4 sm:right-4 sm:flex-col-reverse outline-none"
    />
  </ToastProvider>
</template>
