<script setup lang="ts">
import ErrorPage from '@/components/business/ErrorPage.vue'
import { Lock } from 'lucide-vue-next'
</script>

<template>
  <ErrorPage
    code="403"
    title="权限不足"
    description="你没有访问该页面的权限。请联系学院管理员申请相应角色，或返回工作台。"
    icon-class="bg-accent-soft text-accent"
    :icon="Lock"
  />
</template>
