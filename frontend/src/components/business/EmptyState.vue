<script setup lang="ts">
import { type Component } from 'vue'
import { Inbox, type LucideIcon } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

defineProps<{
  /** 空状态主文案 */
  title?: string
  /** 副文案 */
  description?: string
  /** SVG 插画 Vue 组件（优先于 icon） */
  illustration?: Component
  /** 顶部图标，默认 Inbox（illustration 不存在时使用） */
  icon?: LucideIcon
  /** 主操作按钮文字（可选） */
  actionLabel?: string
  /** 整体类名 */
  class?: string
  /** 主操作按钮类型 */
  actionVariant?: 'default' | 'outline' | 'secondary'
}>()

const emits = defineEmits<{ (e: 'action'): void }>()
</script>

<template>
  <div :class="cn('flex flex-col items-center justify-center px-6 py-12 text-center anim-in', $props.class)">
    <!-- 插画优先，无插画时回退到图标 -->
    <component
      v-if="illustration"
      :is="illustration"
      class="mb-4 h-32 w-32 text-muted-foreground"
    />
    <div v-else class="mb-3 flex h-12 w-12 items-center justify-center rounded-full bg-muted text-muted-foreground">
      <component :is="icon ?? Inbox" class="h-6 w-6" />
    </div>
    <p v-if="title" class="text-sm font-semibold text-ink">{{ title }}</p>
    <p v-if="description" class="mt-1 max-w-sm text-xs text-muted-foreground">{{ description }}</p>
    <slot />
    <Button
      v-if="actionLabel"
      :variant="actionVariant ?? 'outline'"
      class="mt-4"
      @click="emits('action')"
    >
      {{ actionLabel }}
    </Button>
  </div>
</template>
