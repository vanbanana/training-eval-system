<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import ErrorPage from '@/components/business/ErrorPage.vue'
import { TriangleAlert } from 'lucide-vue-next'

const route = useRoute()
const traceId = computed(() => (route.query.trace as string) || undefined)

function reload() {
  window.location.reload()
}
</script>

<template>
  <ErrorPage
    code="500"
    title="服务暂时不可用"
    description="系统遇到了未预期的错误。我们已自动记录日志，运维同事将尽快处理。"
    icon-class="bg-danger-soft text-danger"
    :icon="TriangleAlert"
    :trace-id="traceId"
    primary-label="刷新重试"
    :primary-action="reload"
  />
</template>
