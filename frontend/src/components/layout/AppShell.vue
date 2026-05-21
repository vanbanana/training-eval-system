<script setup lang="ts">
import TopNav from './TopNav.vue'
</script>

<template>
  <div class="min-h-screen bg-background flex flex-col">
    <TopNav />
    <main class="flex-1 px-8 py-7 flex flex-col gap-6">
      <slot />
    </main>
  </div>
</template>
