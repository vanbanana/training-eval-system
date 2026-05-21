<script setup lang="ts">
import { AvatarRoot } from 'reka-ui'
import { cn } from '@/lib/utils'
import { type AvatarVariants, avatarVariant } from './variants'

defineProps<{
  size?: AvatarVariants['size']
  shape?: AvatarVariants['shape']
  class?: string
}>()
</script>

<template>
  <AvatarRoot :class="cn(avatarVariant({ size, shape }), $props.class)">
    <slot />
  </AvatarRoot>
</template>
