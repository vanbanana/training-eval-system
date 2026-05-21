<script setup lang="ts">
import { type Component, computed } from 'vue'
import { TrendingUp, TrendingDown, Minus } from 'lucide-vue-next'
import AnimatedNumber from './AnimatedNumber.vue'
import { cn } from '@/lib/utils'

export type AccentColor = 'primary' | 'accent' | 'success' | 'warning' | 'danger' | 'info'

const props = withDefaults(
  defineProps<{
    label: string
    value: number | string
    icon: Component
    trend?: { direction: 'up' | 'down' | 'neutral'; text: string }
    accentColor?: AccentColor
    animateValue?: boolean
    delay?: number
    class?: string
  }>(),
  {
    accentColor: 'primary',
    animateValue: false,
    delay: 0,
  },
)

const gradientVar = computed(() => `var(--gradient-${props.accentColor})`)

const trendIcon = computed(() => {
  if (!props.trend) return null
  switch (props.trend.direction) {
    case 'up': return TrendingUp
    case 'down': return TrendingDown
    default: return Minus
  }
})

const trendColor = computed(() => {
  if (!props.trend) return ''
  switch (props.trend.direction) {
    case 'up': return 'text-success'
    case 'down': return 'text-danger'
    default: return 'text-muted-foreground'
  }
})
</script>

<template>
  <div
    :class="cn(
      'stat-card relative overflow-hidden rounded-lg border border-border bg-card p-4 transition-all duration-200 ease-out [box-shadow:var(--shadow-sm)] hover:-translate-y-0.5 hover:[box-shadow:var(--shadow-md)]',
      $props.class,
    )"
    :style="{ transitionDelay: `${delay}ms`, '--_accent-gradient': gradientVar }"
  >
    <!-- 左侧渐变色条 -->
    <div
      class="absolute left-0 top-0 h-full w-1 rounded-l-lg"
      :style="{ background: gradientVar }"
    />

    <div class="flex items-start justify-between pl-3">
      <div class="flex-1">
        <p class="text-xs font-medium text-muted-foreground">{{ label }}</p>
        <p class="mt-1 text-2xl font-bold text-ink num-tabular">
          <AnimatedNumber
            v-if="animateValue && typeof value === 'number'"
            :value="(value as number)"
            :duration="700"
          />
          <span v-else>{{ value }}</span>
        </p>
        <div v-if="trend" class="mt-1 flex items-center gap-1">
          <component :is="trendIcon" :class="cn('h-3.5 w-3.5', trendColor)" />
          <span :class="cn('text-xs font-medium', trendColor)">{{ trend.text }}</span>
        </div>
      </div>
      <div class="flex h-9 w-9 items-center justify-center rounded-md bg-muted text-muted-foreground">
        <component :is="icon" class="h-4.5 w-4.5" />
      </div>
    </div>
  </div>
</template>

<style scoped>
/* 入场动效：CSS @starting-style */
.stat-card {
  opacity: 1;
  transform: translateY(0);
  transition: opacity 300ms ease-out, transform 300ms ease-out, box-shadow 200ms ease-out, translate 200ms ease-out;
}

@starting-style {
  .stat-card {
    opacity: 0;
    transform: translateY(12px);
  }
}
</style>
