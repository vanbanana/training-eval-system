<script setup lang="ts">
/**
 * 业务组件：通用错误页面（403 / 404 / 500 共用版面）
 * 视觉契约：frontend-preview/pages/26-error-states.html
 *  - 圆形图标背景
 *  - 状态码大字
 *  - 标题 / 说明 / 操作按钮
 *  - 可选：复制 trace_id
 */
import { Copy, Home, RotateCcw, ArrowLeft } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { useToast } from '@/components/ui/toast'
import type { Component } from 'vue'

interface Props {
  /** 状态码（用作大字显示）*/
  code: number | string
  /** 标题 */
  title: string
  /** 说明文字 */
  description?: string
  /** 圆形图标背景类（如 'bg-accent-soft text-accent'）*/
  iconClass?: string
  /** 图标组件 */
  icon?: Component
  /** Emoji 备用（无 icon 时用）*/
  emoji?: string
  /** trace_id（可选，用于错误追踪）*/
  traceId?: string
  /** 主按钮：返回工作台 / 刷新等。默认返回工作台 */
  primaryLabel?: string
  primaryAction?: () => void
  /** 副按钮：返回 / 上一步 */
  secondaryLabel?: string
  secondaryAction?: () => void
}

const props = withDefaults(defineProps<Props>(), {
  iconClass: 'bg-muted text-muted-foreground',
  emoji: '⚠',
  primaryLabel: '返回工作台',
  secondaryLabel: '上一页',
})

const { toast } = useToast()

function copyTrace() {
  if (!props.traceId) return
  if (navigator.clipboard) {
    navigator.clipboard.writeText(props.traceId)
      .then(() => toast({ description: 'Trace ID 已复制', variant: 'success' }))
      .catch(() => toast({ description: '复制失败', variant: 'destructive' }))
  }
}

function goPrimary() {
  if (props.primaryAction) props.primaryAction()
  else window.location.href = '/dashboard'
}

function goSecondary() {
  if (props.secondaryAction) props.secondaryAction()
  else if (window.history.length > 1) window.history.back()
  else window.location.href = '/'
}
</script>

<template>
  <div class="min-h-screen flex flex-col items-center justify-center bg-background gap-5 px-6 py-12">
    <div class="anim-in" :style="{ animationDelay: '0ms' }">
      <div :class="['w-24 h-24 rounded-full grid place-items-center', iconClass]">
        <component v-if="icon" :is="icon" class="w-10 h-10" />
        <span v-else class="text-5xl">{{ emoji }}</span>
      </div>
    </div>

    <span class="text-6xl font-bold text-ink tracking-wider num-tabular anim-in" :style="{ animationDelay: '50ms' }">
      {{ code }}
    </span>

    <h1 class="text-xl font-semibold text-ink m-0 anim-in" :style="{ animationDelay: '100ms' }">
      {{ title }}
    </h1>

    <p
      v-if="description"
      class="text-sm text-muted-foreground text-center max-w-md leading-relaxed anim-in"
      :style="{ animationDelay: '150ms' }"
    >
      {{ description }}
    </p>

    <div v-if="traceId" class="flex items-center gap-2 mt-1 anim-in" :style="{ animationDelay: '200ms' }">
      <code class="text-[11px] font-mono bg-surface-2 border border-border px-2 py-1 rounded-sm text-muted-foreground">
        trace_id: {{ traceId }}
      </code>
      <Button variant="ghost" size="icon-sm" @click="copyTrace">
        <Copy class="w-3 h-3" />
      </Button>
    </div>

    <div class="flex gap-3 mt-4 anim-in" :style="{ animationDelay: '250ms' }">
      <Button variant="outline" @click="goSecondary">
        <ArrowLeft class="w-4 h-4" />
        {{ secondaryLabel }}
      </Button>
      <Button @click="goPrimary">
        <component :is="primaryLabel.includes('刷新') ? RotateCcw : Home" class="w-4 h-4" />
        {{ primaryLabel }}
      </Button>
    </div>
  </div>
</template>
