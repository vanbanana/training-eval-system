<script setup lang="ts" generic="TName extends string">
import { Field } from 'vee-validate'

const props = defineProps<{
  name: TName
  /** Initial value used by Field */
  type?: string
  // 任意 vee-validate 的规则参数
  rules?: unknown
}>()
</script>

<template>
  <Field v-slot="slotProps" :name="props.name" :type="props.type" :rules="(props.rules as never)">
    <slot v-bind="slotProps" />
  </Field>
</template>
