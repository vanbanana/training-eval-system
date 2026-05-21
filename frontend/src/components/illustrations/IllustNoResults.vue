<script setup lang="ts">
withDefaults(defineProps<{ width?: number; height?: number }>(), { width: 128, height: 128 })
</script>

<template>
  <svg :width="width" :height="height" viewBox="0 0 128 128" fill="none" xmlns="http://www.w3.org/2000/svg">
    <!-- 放大镜圆圈 -->
    <circle cx="56" cy="56" r="24" stroke="hsl(var(--border-strong))" stroke-width="2" fill="hsl(var(--muted))" />
    <!-- 放大镜手柄 -->
    <line x1="74" y1="74" x2="92" y2="92" stroke="hsl(var(--border-strong))" stroke-width="3" stroke-linecap="round" />
    <!-- 内部 X 表示无结果 -->
    <line x1="48" y1="48" x2="64" y2="64" stroke="hsl(var(--muted-foreground))" stroke-width="1.5" stroke-linecap="round" />
    <line x1="64" y1="48" x2="48" y2="64" stroke="hsl(var(--muted-foreground))" stroke-width="1.5" stroke-linecap="round" />
    <!-- 底部虚线 -->
    <rect x="36" y="104" width="56" height="3" rx="1.5" fill="hsl(var(--border))" opacity="0.4" />
  </svg>
</template>
