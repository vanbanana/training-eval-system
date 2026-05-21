<script setup lang="ts">
import { Inbox, type LucideIcon } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

defineProps<{
  /** 空状态主文案 */
  title?: string
  /** 副文案 */
  description?: string
  /** 顶部图标，默认 Inbox */
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
    <div class="mb-3 flex h-12 w-12 items-center justify-center rounded-full bg-muted text-muted-foreground">
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
