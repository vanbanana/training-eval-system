<script setup lang="ts">
import ErrorPage from '@/components/business/ErrorPage.vue'
import { FileQuestion } from 'lucide-vue-next'
</script>

<template>
  <ErrorPage
    code="404"
    title="页面未找到"
    description="可能链接已失效，或该页面尚未创建。请检查 URL 或返回工作台。"
    icon-class="bg-info-soft text-info"
    :icon="FileQuestion"
  />
</template>
