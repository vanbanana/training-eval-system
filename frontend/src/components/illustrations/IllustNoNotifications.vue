<script setup lang="ts">
withDefaults(defineProps<{ width?: number; height?: number }>(), { width: 128, height: 128 })
</script>

<template>
  <svg :width="width" :height="height" viewBox="0 0 128 128" fill="none" xmlns="http://www.w3.org/2000/svg">
    <!-- 铃铛主体 -->
    <path
      d="M64 28c-14 0-24 10-24 24v16c0 4-2 8-6 10h60c-4-2-6-6-6-10V52c0-14-10-24-24-24z"
      fill="hsl(var(--muted))"
      stroke="hsl(var(--border-strong))"
      stroke-width="1.5"
    />
    <!-- 铃铛底部弧 -->
    <path d="M56 78c0 4.4 3.6 8 8 8s8-3.6 8-8" stroke="hsl(var(--border-strong))" stroke-width="1.5" fill="none" />
    <!-- 铃铛顶部圆点 -->
    <circle cx="64" cy="26" r="3" fill="hsl(var(--primary-soft))" stroke="hsl(var(--primary))" stroke-width="1" />
    <!-- 静音斜线 -->
    <line x1="40" y1="96" x2="88" y2="96" stroke="hsl(var(--border))" stroke-width="1" stroke-dasharray="3 3" />
    <!-- ZZZ 标记 -->
    <text x="82" y="40" font-size="10" font-weight="600" fill="hsl(var(--muted-foreground))" opacity="0.5">z</text>
    <text x="88" y="34" font-size="8" font-weight="600" fill="hsl(var(--muted-foreground))" opacity="0.35">z</text>
  </svg>
</template>
