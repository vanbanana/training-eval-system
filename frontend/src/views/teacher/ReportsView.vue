<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import axios from 'axios'
import AppShell from '@/components/layout/AppShell.vue'
import BreadcrumbNav from '@/components/business/BreadcrumbNav.vue'
import EmptyState from '@/components/business/EmptyState.vue'
import { useToast } from '@/components/ui/toast'
import { useCourseMap } from '@/composables/useCourseMap'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Badge } from '@/components/ui/badge'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Search, FileSpreadsheet, FileText, MoreHorizontal, Download } from 'lucide-vue-next'

interface Task {
  id: number
  name: string
  status: string
  course_id: number
  deadline: string | null
}

const route = useRoute()
const { toast } = useToast()
const { load: loadCourseMap, courseName } = useCourseMap()

const tasks = ref<Task[]>([])
const loading = ref(true)
const search = ref('')
const exportingId = ref<number | null>(null)
const exportingFormat = ref<string>('')

async function fetchTasks() {
  loading.value = true
  try {
    const { data } = await axios.get('/api/tasks')
    tasks.value = data
  } catch {
    toast({ description: '加载任务失败', variant: 'destructive' })
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  void loadCourseMap()
  void fetchTasks()
})

const filtered = computed(() => {
  if (!search.value) return tasks.value
  const q = search.value.toLowerCase()
  return tasks.value.filter((t) => t.name.toLowerCase().includes(q))
})

watch(
  () => route.query.task_id,
  (newId) => {
    if (newId) {
      const tid = Number(newId)
      // smooth-scroll to row when arriving with ?task_id= 
      setTimeout(() => {
        document.getElementById(`task-row-${tid}`)?.scrollIntoView({ behavior: 'smooth', block: 'center' })
      }, 100)
    }
  },
  { immediate: true },
)

async function exportFile(taskId: number, format: 'pdf' | 'xlsx' | 'task') {
  exportingId.value = taskId
  exportingFormat.value = format
  try {
    const url = format === 'pdf'
      ? `/api/reports/statistics/${taskId}`
      : format === 'task'
        ? `/api/reports/task/${taskId}/csv`
        : `/api/reports/statistics/${taskId}/xlsx`
    const { data } = await axios.get(url, { responseType: 'blob' })
    const blobUrl = URL.createObjectURL(data)
    const a = document.createElement('a')
    a.href = blobUrl
    a.download = format === 'task'
      ? `report_task_${taskId}.xlsx`
      : `report_task_${taskId}.${format}`
    a.click()
    URL.revokeObjectURL(blobUrl)
    const label = format === 'task' ? 'Excel' : format.toUpperCase()
    toast({ description: `已导出 ${label}${format === 'xlsx' ? ' 统计报表' : ''}`, variant: 'success' })
  } catch (e) {
    const msg = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    toast({ description: msg ?? '导出失败', variant: 'destructive' })
  } finally {
    exportingId.value = null
    exportingFormat.value = ''
  }
}

function statusVariant(s: string) {
  return ({ draft: 'secondary', published: 'info', closed: 'secondary' } as const)[s] ?? 'secondary'
}

function statusLabel(s: string) {
  return { draft: '草稿', published: '已发布', closed: '已关闭' }[s] ?? s
}
</script>

<template>
  <AppShell>
    <BreadcrumbNav
      :items="[
        { label: '工作台', to: '/dashboard' },
        { label: '报表中心' },
      ]"
    />

    <div class="tes-page-header">
      <div class="min-w-0">
        <h1 class="tes-clamp-title text-2xl font-bold text-ink">报表中心</h1>
        <p class="mt-1.5 text-sm text-muted-foreground">选择任务导出评价 XLSX / 统计报表</p>
      </div>
    </div>

    <Card class="tes-card-container px-5 py-3.5">
      <div class="relative max-w-md">
        <Search class="w-3.5 h-3.5 text-muted-foreground absolute left-3 top-1/2 -translate-y-1/2" />
        <Input v-model="search" placeholder="搜索任务名" class="pl-9" />
      </div>
    </Card>

    <Card class="tes-card-container overflow-hidden">
      <div class="tes-table-shell">
      <div class="grid min-w-[760px] grid-cols-[minmax(18rem,1fr)_140px_120px_220px] items-center px-6 py-3 bg-surface-2 border-b border-border text-[11px] font-semibold text-muted-foreground tracking-wider">
        <span>任务名称</span>
        <span>课程</span>
        <span>状态</span>
        <span class="text-right">操作</span>
      </div>

      <template v-if="loading">
        <div v-for="n in 5" :key="n" class="grid min-w-[760px] grid-cols-[minmax(18rem,1fr)_140px_120px_220px] items-center px-6 py-3.5 border-b border-border">
          <Skeleton class="h-5 w-3/4" />
          <Skeleton class="h-4 w-20" />
          <Skeleton class="h-5 w-16" />
          <Skeleton class="h-8 w-32 ml-auto" />
        </div>
      </template>

      <EmptyState
        v-else-if="filtered.length === 0"
        title="暂无任务"
        description="发布任务后即可在此页面导出报表"
      />

      <div
        v-for="(t, idx) in filtered"
        v-else
        :key="t.id"
        :id="`task-row-${t.id}`"
        class="grid min-w-[760px] grid-cols-[minmax(18rem,1fr)_140px_120px_220px] items-center px-6 py-3.5 border-b border-border last:border-b-0 hover:bg-surface-2 transition-colors anim-in"
        :style="{ animationDelay: Math.min(idx * 25, 200) + 'ms' }"
      >
        <span class="tes-breakable font-medium text-ink">{{ t.name }}</span>
        <span class="text-xs text-muted-foreground">{{ courseName(t.course_id) }}</span>
        <Badge :variant="statusVariant(t.status)">{{ statusLabel(t.status) }}</Badge>
        <div class="flex items-center justify-end gap-2">
          <Button
            variant="outline"
            size="sm"
            :disabled="exportingId === t.id && exportingFormat === 'task'"
            @click="exportFile(t.id, 'task')"
          >
            <FileText class="w-3.5 h-3.5" />
            Excel
          </Button>
          <Button
            variant="outline"
            size="sm"
            :disabled="exportingId === t.id && exportingFormat === 'xlsx'"
            @click="exportFile(t.id, 'xlsx')"
          >
            <FileSpreadsheet class="w-3.5 h-3.5" />
            统计报表
          </Button>
          <DropdownMenu>
            <DropdownMenuTrigger as-child>
              <Button variant="ghost" size="icon-sm">
                <MoreHorizontal class="w-3.5 h-3.5" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem @select="exportFile(t.id, 'xlsx')">
                <Download class="text-muted-foreground" />
                导出 XLSX
              </DropdownMenuItem>
              <DropdownMenuItem @select="exportFile(t.id, 'pdf')">
                <Download class="text-muted-foreground" />
                导出 PDF
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>
      </div>
    </Card>

    <p class="text-xs text-muted-foreground">
      个人 PDF 报告导出（针对单个评价）请到「评价详情」页操作。
    </p>
  </AppShell>
</template>
