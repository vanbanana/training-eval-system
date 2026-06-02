<script setup lang="ts">
import { computed } from 'vue'
import TopNav from './TopNav.vue'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const isTeacher = computed(() => auth.user?.role === 'teacher')
</script>

<template>
  <div class="min-h-screen bg-background flex flex-col">
    <TopNav />
    <main
      class="app-shell-content relative flex-1 flex flex-col"
      :class="isTeacher ? 'px-6 py-4 gap-4' : 'px-8 py-7 gap-6'"
    >
      <slot />
    </main>
  </div>
</template>

<style scoped>
.app-shell-content {
  background-image: var(--gradient-page-bg);
  background-attachment: fixed;
}

/* 装饰性渐变球 */
.app-shell-content::before {
  content: '';
  position: fixed;
  top: -20%;
  right: -10%;
  width: 600px;
  height: 600px;
  border-radius: 50%;
  background: radial-gradient(circle, hsl(var(--accent) / 0.03), transparent 70%);
  pointer-events: none;
  z-index: 0;
}
</style>
