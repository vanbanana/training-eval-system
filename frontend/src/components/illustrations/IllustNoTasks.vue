<script setup lang="ts">
withDefaults(defineProps<{ width?: number; height?: number }>(), { width: 128, height: 128 })
</script>

<template>
  <svg :width="width" :height="height" viewBox="0 0 128 128" fill="none" xmlns="http://www.w3.org/2000/svg">
    <!-- 剪贴板底板 -->
    <rect x="32" y="24" width="64" height="84" rx="6" fill="hsl(var(--muted))" stroke="hsl(var(--border-strong))" stroke-width="1.5" />
    <!-- 剪贴板夹子 -->
    <rect x="48" y="18" width="32" height="12" rx="4" fill="hsl(var(--primary-soft))" stroke="hsl(var(--primary))" stroke-width="1" />
    <!-- 空行占位 -->
    <rect x="42" y="46" width="44" height="4" rx="2" fill="hsl(var(--border))" opacity="0.6" />
    <rect x="42" y="56" width="36" height="4" rx="2" fill="hsl(var(--border))" opacity="0.4" />
    <rect x="42" y="66" width="40" height="4" rx="2" fill="hsl(var(--border))" opacity="0.3" />
    <!-- 对勾圆圈（空） -->
    <circle cx="64" cy="88" r="10" stroke="hsl(var(--muted-foreground))" stroke-width="1.5" fill="none" stroke-dasharray="4 3" />
  </svg>
</template>
