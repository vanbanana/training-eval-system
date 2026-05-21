<script setup lang="ts">
withDefaults(defineProps<{ width?: number; height?: number }>(), { width: 128, height: 128 })
</script>

<template>
  <svg :width="width" :height="height" viewBox="0 0 128 128" fill="none" xmlns="http://www.w3.org/2000/svg">
    <!-- 文档底板 -->
    <rect x="36" y="20" width="56" height="72" rx="5" fill="hsl(var(--muted))" stroke="hsl(var(--border-strong))" stroke-width="1.5" />
    <!-- 星星评分（空心） -->
    <path d="M52 50l3 6 7 1-5 5 1 7-6-3-6 3 1-7-5-5 7-1z" stroke="hsl(var(--warning))" stroke-width="1.2" fill="none" />
    <path d="M68 50l3 6 7 1-5 5 1 7-6-3-6 3 1-7-5-5 7-1z" stroke="hsl(var(--warning))" stroke-width="1.2" fill="none" />
    <path d="M84 50l3 6 7 1-5 5 1 7-6-3-6 3 1-7-5-5 7-1z" stroke="hsl(var(--border))" stroke-width="1.2" fill="none" />
    <!-- 进度条（空） -->
    <rect x="46" y="72" width="36" height="4" rx="2" fill="hsl(var(--border))" opacity="0.5" />
    <!-- 问号 -->
    <circle cx="64" cy="100" r="8" stroke="hsl(var(--muted-foreground))" stroke-width="1.2" fill="none" />
    <text x="61" y="104" font-size="10" font-weight="600" fill="hsl(var(--muted-foreground))">?</text>
  </svg>
</template>
