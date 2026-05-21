<script setup lang="ts">
import { cn } from '@/lib/utils'

defineProps<{
  class?: string
  colspan?: number
}>()
</script>

<template>
  <tr>
    <td :colspan="colspan ?? 99" :class="cn('h-32 text-center text-sm text-muted-foreground', $props.class)">
      <slot>暂无数据</slot>
    </td>
  </tr>
</template>
