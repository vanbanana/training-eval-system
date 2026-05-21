<script setup lang="ts">
withDefaults(defineProps<{ width?: number; height?: number }>(), { width: 128, height: 128 })
</script>

<template>
  <svg :width="width" :height="height" viewBox="0 0 128 128" fill="none" xmlns="http://www.w3.org/2000/svg">
    <!-- 书本底层 -->
    <rect x="30" y="40" width="68" height="52" rx="4" fill="hsl(var(--muted))" stroke="hsl(var(--border-strong))" stroke-width="1.5" />
    <!-- 书脊 -->
    <line x1="64" y1="40" x2="64" y2="92" stroke="hsl(var(--border-strong))" stroke-width="1.5" />
    <!-- 左页线条 -->
    <rect x="38" y="50" width="20" height="3" rx="1.5" fill="hsl(var(--border))" opacity="0.5" />
    <rect x="38" y="58" width="16" height="3" rx="1.5" fill="hsl(var(--border))" opacity="0.4" />
    <rect x="38" y="66" width="18" height="3" rx="1.5" fill="hsl(var(--border))" opacity="0.3" />
    <!-- 右页线条 -->
    <rect x="70" y="50" width="20" height="3" rx="1.5" fill="hsl(var(--border))" opacity="0.5" />
    <rect x="70" y="58" width="16" height="3" rx="1.5" fill="hsl(var(--border))" opacity="0.4" />
    <!-- 学士帽 -->
    <path d="M54 28l10-6 10 6-10 6z" fill="hsl(var(--primary-soft))" stroke="hsl(var(--primary))" stroke-width="1" />
    <line x1="64" y1="28" x2="64" y2="36" stroke="hsl(var(--primary))" stroke-width="1" />
    <!-- 加号提示 -->
    <circle cx="88" cy="100" r="10" fill="hsl(var(--primary-soft))" stroke="hsl(var(--primary))" stroke-width="1" />
    <line x1="84" y1="100" x2="92" y2="100" stroke="hsl(var(--primary))" stroke-width="1.5" />
    <line x1="88" y1="96" x2="88" y2="104" stroke="hsl(var(--primary))" stroke-width="1.5" />
  </svg>
</template>
