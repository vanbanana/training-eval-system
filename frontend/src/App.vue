<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { Toaster } from '@/components/ui/toast'
import ConfirmDialog from '@/components/business/ConfirmDialog.vue'
import { setConfirm } from '@/composables/useConfirm'
import { TooltipProvider } from '@/components/ui/tooltip'
import { useTheme } from '@/composables/useTheme'

const confirmRef = ref<InstanceType<typeof ConfirmDialog> | null>(null)

// 初始化主题（持久化在 localStorage 的 tes-theme）
useTheme()

onMounted(() => {
  setConfirm(confirmRef.value)
})
</script>

<template>
  <TooltipProvider>
    <RouterView v-slot="{ Component, route }">
      <transition name="fade" mode="out-in">
        <component :is="Component" :key="route.fullPath" />
      </transition>
    </RouterView>
    <Toaster />
    <ConfirmDialog ref="confirmRef" />
  </TooltipProvider>
</template>
