<script setup lang="ts">
import { computed } from 'vue'
import { icons3d } from './index'

const props = withDefaults(defineProps<{
  name: keyof typeof icons3d
  size?: number
  color?: string
}>(), { size: 52, color: 'blue' })

const wrapSize = computed(() => Math.round(props.size * 1.4))
</script>

<template>
  <div
    class="i3d"
    :style="{
      width: `${wrapSize}px`,
      height: `${wrapSize}px`,
    }"
  >
    <img :src="icons3d[props.name]" :width="props.size" :height="props.size" :alt="props.name" />
  </div>
</template>

<style scoped>
.i3d {
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 16px;
  flex-shrink: 0;
}
.i3d img {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: contain;
}
</style>
