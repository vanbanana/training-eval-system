<script setup lang="ts">
import { SelectGroup, type SelectGroupProps } from 'reka-ui'

const props = defineProps<SelectGroupProps & { class?: string }>()
</script>

<template>
  <SelectGroup v-bind="props">
    <slot />
  </SelectGroup>
</template>
