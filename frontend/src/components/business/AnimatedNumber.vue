<script setup lang="ts">
import { computed } from 'vue'
import { useTransition, TransitionPresets } from '@vueuse/core'

const props = withDefaults(
  defineProps<{
    value: number
    duration?: number
    /** 小数位 */
    decimals?: number
    /** 数字前缀 */
    prefix?: string
    /** 数字后缀 */
    suffix?: string
    /** 类名 */
    class?: string
  }>(),
  {
    duration: 800,
    decimals: 0,
    prefix: '',
    suffix: '',
  },
)

const animated = useTransition(
  computed(() => props.value),
  {
    duration: props.duration,
    transition: TransitionPresets.easeOutQuart,
  },
)

const display = computed(() => {
  const v = animated.value ?? 0
  return `${props.prefix}${v.toFixed(props.decimals)}${props.suffix}`
})
</script>

<template>
  <span :class="$props.class" class="num-tabular">{{ display }}</span>
</template>
