<script setup lang="ts">
import { computed } from 'vue'
import { icons3d } from './index'

const props = withDefaults(defineProps<{
  name: keyof typeof icons3d
  size?: number
  color?: string
}>(), { size: 52, color: 'blue' })

const themes: Record<string, { light: string; dark: string; shadow: string }> = {
  blue:   { light: '#dbeafe', dark: '#93c5fd', shadow: 'rgba(37,99,235,0.3)' },
  purple: { light: '#ede9fe', dark: '#c4b5fd', shadow: 'rgba(124,58,237,0.3)' },
  green:  { light: '#dcfce7', dark: '#86efac', shadow: 'rgba(22,163,74,0.3)' },
  orange: { light: '#fef3c7', dark: '#fcd34d', shadow: 'rgba(217,119,6,0.3)' },
  red:    { light: '#fee2e2', dark: '#fca5a5', shadow: 'rgba(220,38,38,0.3)' },
  teal:   { light: '#ccfbf1', dark: '#5eead4', shadow: 'rgba(13,148,136,0.3)' },
  pink:   { light: '#fce7f3', dark: '#f9a8d4', shadow: 'rgba(219,39,119,0.3)' },
}

const theme = computed(() => themes[props.color] || themes.blue)
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
