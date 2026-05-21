<script setup lang="ts">
/**
 * 列表/卡片入场动画包装。把子节点用 <transition-group> 包裹，自动 stagger fade+translateY。
 * 用法：
 *   <MotionList tag="div" class="grid gap-4">
 *     <Card v-for="(item, i) in items" :key="item.id" :data-motion-index="i">...</Card>
 *   </MotionList>
 *
 * 性能：纯 CSS transition + transform，不用 JS 计算。
 */
import { cn } from '@/lib/utils'

defineProps<{
  /** 渲染容器 tag，默认 div */
  tag?: string
  /** 透传给容器 */
  class?: string
  /** 每项错开毫秒，默认 50ms */
  stagger?: number
}>()
</script>

<template>
  <transition-group
    :tag="tag ?? 'div'"
    :class="cn($props.class)"
    enter-active-class="transition-all duration-300 ease-out"
    enter-from-class="opacity-0 translate-y-2"
    enter-to-class="opacity-100 translate-y-0"
    leave-active-class="transition-all duration-200 ease-in absolute"
    leave-from-class="opacity-100"
    leave-to-class="opacity-0 -translate-y-1"
    move-class="transition-transform duration-200"
  >
    <slot />
  </transition-group>
</template>
