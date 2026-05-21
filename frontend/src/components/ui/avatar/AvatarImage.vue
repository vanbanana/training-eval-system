<script setup lang="ts">
import { AvatarImage, type AvatarImageProps } from 'reka-ui'

const props = defineProps<AvatarImageProps & { class?: string }>()
</script>

<template>
  <AvatarImage v-bind="props" class="aspect-square h-full w-full" />
</template>
