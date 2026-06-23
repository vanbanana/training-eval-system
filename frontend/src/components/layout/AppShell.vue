<script setup lang="ts">
import { computed } from 'vue'
import TopNav from './TopNav.vue'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const isTeacher = computed(() => auth.user?.role === 'teacher')
</script>

<template>
  <div class="min-h-dvh bg-background flex flex-col overflow-x-clip">
    <TopNav />
    <main
      class="app-shell-content relative w-full min-w-0 flex flex-col items-start"
      :class="isTeacher ? 'px-4 sm:px-5 lg:px-6 py-4 gap-4' : 'px-4 sm:px-6 lg:px-8 py-5 lg:py-7 gap-5 lg:gap-6'"
    >
      <div class="app-shell-inner relative z-[1] w-full min-w-0">
        <slot />
      </div>
    </main>
  </div>
</template>

<style scoped>
.app-shell-content {
  background-image: var(--gradient-page-bg);
  background-attachment: fixed;
}

.app-shell-inner {
  /* Constrain + center content on ultra-wide displays so pages read like a
     focused product surface rather than an edge-to-edge admin console. */
  max-width: 1600px;
  margin-inline: auto;
  display: flex;
  flex-direction: column;
  align-items: stretch;
  gap: inherit;
}
</style>
