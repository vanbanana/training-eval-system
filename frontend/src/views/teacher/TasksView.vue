<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import axios from 'axios'
import AppShell from '@/components/layout/AppShell.vue'
import BreadcrumbNav from '@/components/business/BreadcrumbNav.vue'
import EmptyState from '@/components/business/EmptyState.vue'
import { useToast } from '@/components/ui/toast'
import { confirm } from '@/composables/useConfirm'
import { useCourseMap } from '@/composables/useCourseMap'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Badge } from '@/components/ui/badge'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Search,
  Upload,
  SlidersHorizontal,
  ChevronLeft,
  ChevronRight,
  MoreHorizontal,
  Trash2,
  Eye,
  BarChart3,
  Plus,
  BookOpen,
  CalendarClock,
  Layers,
} from 'lucide-vue-next'

interface Task {
  id: number
  name: string
  description: string
  status: string
  deadline: string | null
  course_id: number
  teacher_id: number
  dimensions: { id: number; name: string; weight: number; order_index: number }[]
  created_at: string
}

const router = useRouter()
const { toast } = useToast()
const { load: loadCourseMap, courseName } = useCourseMap()

const tasks = ref<Task[]>([])
const loading = ref(true)
const searchQuery = ref('')
const statusFilter = ref<string>('all')
const sortBy = ref<'created_at_desc' | 'created_at_asc' | 'deadline_asc' | 'deadline_desc'>('created_at_desc')
const minDimensions = ref<number | ''>('')

const currentPage = ref(1)
const pageSize = 6

async function fetchTasks() {
  loading.value = true
  try {
    const { data } = await axios.get('/api/tasks')
    tasks.value = data
  } catch {
    toast({ description: '加载任务列表失败', variant: 'destructive' })
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  void loadCourseMap()
  void fetchTasks()
})

const filteredTasks = computed(() => {
  let result = tasks.value
  if (statusFilter.value !== 'all') {
    result = result.filter((t) => t.status === statusFilter.value)
  }
  if (searchQuery.value.trim()) {
    const q = searchQuery.value.trim().toLowerCase()
    result = result.filter((t) => t.name.toLowerCase().includes(q))
  }
  if (minDimensions.value !== '' && Number(minDimensions.value) > 0) {
    const min = Number(minDimensions.value)
    result = result.filter((t) => t.dimensions.length >= min)
  }
  result = [...result].sort((a, b) => {
    if (sortBy.value === 'created_at_desc') return b.id - a.id
    if (sortBy.value === 'created_at_asc') return a.id - b.id
    const ad = a.deadline ? new Date(a.deadline).getTime() : Infinity
    const bd = b.deadline ? new Date(b.deadline).getTime() : Infinity
    return sortBy.value === 'deadline_asc' ? ad - bd : bd - ad
  })
  return result
})

const totalItems = computed(() => filteredTasks.value.length)
const totalPages = computed(() => Math.max(1, Math.ceil(totalItems.value / pageSize)))
const paginatedTasks = computed(() => {
  const start = (currentPage.value - 1) * pageSize
  return filteredTasks.value.slice(start, start + pageSize)
})

watch([statusFilter, searchQuery, minDimensions, sortBy], () => {
  currentPage.value = 1
})

const statusCounts = computed(() => ({
  all: tasks.value.length,
  published: tasks.value.filter((t) => t.status === 'published').length,
  draft: tasks.value.filter((t) => t.status === 'draft').length,
  closed: tasks.value.filter((t) => t.status === 'closed').length,
}))

function statusLabel(s: string) {
  return { draft: '草稿', published: '已发布', closed: '已关闭' }[s] ?? s
}
function statusVariant(s: string) {
  return ({ draft: 'secondary', published: 'info', closed: 'secondary' } as const)[s] ?? 'secondary'
}

function formatDeadline(deadline: string | null) {
  if (!deadline) return '——'
  return deadline.slice(0, 16).replace('T', ' ')
}

function deadlineHint(deadline: string | null, status: string) {
  if (!deadline) return status === 'draft' ? '未发布' : ''
  const diff = new Date(deadline).getTime() - Date.now()
  if (diff < 0) return '已结束'
  const days = Math.ceil(diff / (1000 * 60 * 60 * 24))
  return `剩余 ${days} 天`
}

async function publishTask(task: Task) {
  try {
    await axios.patch(`/api/tasks/${task.id}/publish`)
    toast({ description: `"${task.name}" 已发布`, variant: 'success' })
    await fetchTasks()
  } catch (e) {
    const msg = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    toast({ description: msg ?? '发布失败', variant: 'destructive' })
  }
}

async function closeTask(task: Task) {
  try {
    await axios.patch(`/api/tasks/${task.id}/close`)
    toast({ description: `"${task.name}" 已关闭`, variant: 'success' })
    await fetchTasks()
  } catch (e) {
    const msg = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    toast({ description: msg ?? '关闭失败', variant: 'destructive' })
  }
}

async function deleteTask(task: Task) {
  const ok = await confirm({
    title: '删除任务',
    description: `确定删除任务 "${task.name}"？此操作不可撤销。`,
    variant: 'destructive',
    confirmText: '删除',
  })
  if (!ok) return
  try {
    await axios.delete(`/api/tasks/${task.id}`)
    toast({ description: `"${task.name}" 已删除`, variant: 'success' })
    await fetchTasks()
  } catch (e) {
    const msg = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    toast({ description: msg ?? '删除失败', variant: 'destructive' })
  }
}

function goToCreate() {
  router.push('/teacher/tasks/new')
}

function goToGrading(taskId: number) {
  router.push(`/teacher/tasks/${taskId}/grading`)
}

function goToReports(taskId: number) {
  router.push({ path: '/teacher/reports', query: { task_id: taskId } })
}

function notifyImportPlanned() {
  showImportDialog.value = true
}

// ── Task Import ──
const showImportDialog = ref(false)
const importFile = ref<File | null>(null)
const importing = ref(false)
const importResult = ref<{ total: number; success: number; failed: number } | null>(null)

function downloadTaskTemplate() {
  window.location.href = '/api/imports/template/task.xlsx'
}

function onImportFileChange(e: Event) {
  const input = e.target as HTMLInputElement
  importFile.value = input.files?.[0] ?? null
  importResult.value = null
}

async function submitImport() {
  if (!importFile.value) return
  importing.value = true
  importResult.value = null
  try {
    const fd = new FormData()
    fd.append('file', importFile.value)
    const { data } = await axios.post('/api/imports/tasks', fd, {
      headers: { 'Content-Type': 'multipart/form-data' },
    })
    importResult.value = {
      total: data.total_count,
      success: data.success_count,
      failed: data.failed_count,
    }
    if (data.success_count > 0) {
      await fetchTasks()
    }
  } catch (e) {
    const msg = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    toast({ description: msg ?? '导入失败', variant: 'destructive' })
  } finally {
    importing.value = false
  }
}

function closeImportDialog() {
  showImportDialog.value = false
  importFile.value = null
  importResult.value = null
}
</script>

<template>
  <AppShell>
    <BreadcrumbNav
      :items="[
        { label: '工作台', to: '/dashboard' },
        { label: '实训任务' },
      ]"
    />

    <!-- Page Header -->
    <div class="tes-page-header">
      <div class="min-w-0">
        <h1 class="tes-clamp-title text-2xl font-bold text-ink">实训任务</h1>
        <p class="mt-1.5 text-sm text-muted-foreground">创建、发布并跟踪所有实训任务的批改进度</p>
      </div>
      <div class="tes-page-actions">
        <Button variant="outline" @click="notifyImportPlanned">
          <Upload class="w-4 h-4" />
          导入任务
        </Button>
        <Button @click="goToCreate">
          <Plus class="w-4 h-4" />
          创建任务
        </Button>
      </div>
    </div>

    <!-- Toolbar -->
    <Card class="tes-card-container px-5 py-3.5 flex flex-wrap justify-between items-center gap-4">
      <Tabs v-model="statusFilter" class="min-w-0">
        <TabsList>
          <TabsTrigger value="all">全部 {{ statusCounts.all }}</TabsTrigger>
          <TabsTrigger value="published">已发布 {{ statusCounts.published }}</TabsTrigger>
          <TabsTrigger value="draft">草稿 {{ statusCounts.draft }}</TabsTrigger>
          <TabsTrigger value="closed">已关闭 {{ statusCounts.closed }}</TabsTrigger>
        </TabsList>
      </Tabs>

      <div class="flex min-w-0 flex-wrap items-center gap-3">
        <div class="relative w-full sm:w-[280px]">
          <Search class="w-3.5 h-3.5 text-muted-foreground absolute left-3 top-1/2 -translate-y-1/2" />
          <Input v-model="searchQuery" placeholder="搜索任务名称 / 课程" class="pl-9" />
        </div>
        <Popover>
          <PopoverTrigger as-child>
            <Button variant="ghost" size="sm">
              <SlidersHorizontal class="w-3.5 h-3.5" />
              筛选
            </Button>
          </PopoverTrigger>
          <PopoverContent align="end" class="w-72">
            <div class="space-y-3">
              <div class="space-y-1.5">
                <Label>排序</Label>
                <Select v-model="sortBy">
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="created_at_desc">创建时间（最新）</SelectItem>
                    <SelectItem value="created_at_asc">创建时间（最早）</SelectItem>
                    <SelectItem value="deadline_asc">截止时间（最近）</SelectItem>
                    <SelectItem value="deadline_desc">截止时间（最远）</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div class="space-y-1.5">
                <Label>最少维度数</Label>
                <Input v-model="minDimensions" type="number" min="0" placeholder="不限" />
              </div>
            </div>
          </PopoverContent>
        </Popover>
      </div>
    </Card>

    <!-- Loading skeletons -->
    <div v-if="loading" class="grid grid-cols-[repeat(auto-fill,minmax(22rem,1fr))] gap-4">
      <Card v-for="n in 6" :key="n" class="tes-card-container flex flex-col gap-4 p-5">
        <div class="flex items-start gap-3">
          <div class="flex-1 space-y-2">
            <Skeleton class="h-5 w-3/4" />
            <Skeleton class="h-3 w-full" />
          </div>
          <Skeleton class="h-6 w-14 rounded-pill" />
        </div>
        <Skeleton class="h-9 w-full rounded-md" />
      </Card>
    </div>

    <Card v-else-if="paginatedTasks.length === 0" class="tes-card-container">
      <EmptyState
        title="暂无任务"
        description="创建任务后可在此页面跟踪批改进度"
        action-label="创建任务"
        @action="goToCreate"
      />
    </Card>

    <!-- Task cards -->
    <div v-else class="grid grid-cols-[repeat(auto-fill,minmax(22rem,1fr))] gap-4">
      <Card
        v-for="(task, idx) in paginatedTasks"
        :key="task.id"
        class="tes-card-container flex flex-col gap-4 p-5 transition-all hover:-translate-y-0.5 hover:shadow-lg anim-in"
        :style="{ animationDelay: Math.min(idx * 30, 240) + 'ms' }"
      >
        <div class="flex items-start justify-between gap-3">
          <div class="min-w-0">
            <h3 class="tes-breakable text-base font-semibold text-ink leading-snug">{{ task.name }}</h3>
            <p class="mt-1 text-xs text-muted-foreground line-clamp-2">{{ task.description || '暂无描述' }}</p>
          </div>
          <Badge :variant="statusVariant(task.status)" class="shrink-0">{{ statusLabel(task.status) }}</Badge>
        </div>

        <div class="flex flex-col gap-2 text-xs text-muted-foreground">
          <div class="flex items-center gap-2">
            <BookOpen class="w-3.5 h-3.5 shrink-0" />
            <span class="tes-breakable">{{ courseName(task.course_id) }}</span>
          </div>
          <div class="flex items-center gap-2">
            <CalendarClock class="w-3.5 h-3.5 shrink-0" />
            <span class="font-mono">{{ formatDeadline(task.deadline) }}</span>
            <span v-if="deadlineHint(task.deadline, task.status)" class="text-[11px] text-subtle-foreground">· {{ deadlineHint(task.deadline, task.status) }}</span>
          </div>
          <div class="flex items-center gap-2">
            <Layers class="w-3.5 h-3.5 shrink-0" />
            <span><b class="font-semibold text-ink">{{ task.dimensions.length }}</b> 个评价维度</span>
          </div>
        </div>

        <div class="mt-auto flex items-center gap-2 border-t border-border pt-4">
          <Button
            v-if="task.status === 'published'"
            size="sm" class="flex-1"
            @click="goToGrading(task.id)"
          >进入批改</Button>
          <Button
            v-else-if="task.status === 'draft'"
            size="sm" class="flex-1"
            @click="router.push(`/teacher/tasks/new?edit=${task.id}`)"
          >编辑任务</Button>
          <Button v-else size="sm" class="flex-1" @click="goToReports(task.id)">查看报表</Button>

          <Button
            v-if="task.status === 'draft'"
            variant="outline" size="sm"
            @click="publishTask(task)"
          >发布</Button>
          <Button
            v-else-if="task.status === 'published'"
            variant="outline" size="sm"
            @click="closeTask(task)"
          >关闭</Button>

          <DropdownMenu>
            <DropdownMenuTrigger as-child>
              <Button variant="ghost" size="icon-sm">
                <MoreHorizontal class="w-3.5 h-3.5" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" class="w-44">
              <DropdownMenuItem @select="goToGrading(task.id)">
                <Eye class="text-muted-foreground" />
                查看批改
              </DropdownMenuItem>
              <DropdownMenuItem @select="goToReports(task.id)">
                <BarChart3 class="text-muted-foreground" />
                报表中心
              </DropdownMenuItem>
              <DropdownMenuSeparator v-if="task.status === 'draft'" />
              <DropdownMenuItem
                v-if="task.status === 'draft'"
                class="text-danger focus:bg-danger-soft focus:text-danger"
                @select="deleteTask(task)"
              >
                <Trash2 class="text-current" />
                删除任务
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </Card>
    </div>

    <!-- Pagination -->
    <div v-if="!loading && totalItems > pageSize" class="flex flex-wrap justify-between items-center gap-3">
      <div class="text-xs text-muted-foreground">
        显示 {{ (currentPage - 1) * pageSize + 1 }} - {{ Math.min(currentPage * pageSize, totalItems) }} 共 {{ totalItems }} 条
      </div>
      <div class="flex items-center gap-1.5">
        <Button variant="outline" size="icon-sm" :disabled="currentPage <= 1" @click="currentPage--">
          <ChevronLeft class="w-3.5 h-3.5" />
        </Button>
        <Button
          v-for="page in totalPages"
          :key="page"
          :variant="page === currentPage ? 'default' : 'outline'"
          size="sm"
          class="h-8 min-w-[32px]"
          @click="currentPage = page"
        >
          {{ page }}
        </Button>
        <Button variant="outline" size="icon-sm" :disabled="currentPage >= totalPages" @click="currentPage++">
          <ChevronRight class="w-3.5 h-3.5" />
        </Button>
      </div>
    </div>

    <!-- Import Dialog -->
    <Teleport to="body">
      <div v-if="showImportDialog" class="fixed inset-0 z-50 flex items-center justify-center bg-black/40" @click.self="closeImportDialog">
        <div class="bg-white rounded-xl shadow-2xl w-[460px] max-h-[80vh] overflow-y-auto p-6">
          <h2 class="text-lg font-bold text-ink">导入实训任务</h2>
          <p class="text-sm text-muted-foreground mt-1">通过 Excel 文件批量创建任务（创建后为草稿状态）</p>

          <div class="mt-4 space-y-4">
            <button class="text-sm text-primary font-medium hover:underline" @click="downloadTaskTemplate">
              下载导入模板 (task_template.xlsx)
            </button>

            <div class="border-2 border-dashed border-border rounded-lg p-6 text-center">
              <input
                ref="fileInput"
                type="file"
                accept=".xlsx,.csv"
                class="hidden"
                @change="onImportFileChange"
              />
              <button
                class="text-sm text-muted-foreground hover:text-primary transition-colors"
                @click="($refs.fileInput as HTMLInputElement)?.click()"
              >
                {{ importFile ? importFile.name : '点击选择或拖放 xlsx / csv 文件' }}
              </button>
            </div>

            <div v-if="importResult" class="rounded-lg border p-4 text-sm space-y-1"
              :class="importResult.failed === 0 ? 'border-success bg-success-soft' : 'border-warning bg-warning-soft'"
            >
              <p>共 {{ importResult.total }} 条记录</p>
              <p class="text-success">成功: {{ importResult.success }}</p>
              <p v-if="importResult.failed > 0" class="text-danger">失败: {{ importResult.failed }}</p>
            </div>
          </div>

          <div class="flex justify-end gap-3 mt-6">
            <Button variant="outline" @click="closeImportDialog">
              {{ importResult ? '关闭' : '取消' }}
            </Button>
            <Button v-if="!importResult" :disabled="!importFile || importing" @click="submitImport">
              {{ importing ? '导入中...' : '开始导入' }}
            </Button>
          </div>
        </div>
      </div>
    </Teleport>
  </AppShell>
</template>
