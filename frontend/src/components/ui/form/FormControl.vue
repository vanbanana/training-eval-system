<script setup lang="ts">
import { Slot } from 'reka-ui'
import { useFormItemId } from './use-form-item-id'

const id = useFormItemId()
</script>

<template>
  <Slot :id="id">
    <slot />
  </Slot>
</template>
