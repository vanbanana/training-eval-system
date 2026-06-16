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
import { Checkbox } from '@/components/ui/checkbox'
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
  Info,
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
const selected = ref<Set<number>>(new Set())

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

const allSelectedOnPage = computed({
  get: () =>
    paginatedTasks.value.length > 0 &&
    paginatedTasks.value.every((t) => selected.value.has(t.id)),
  set: (v: boolean) => {
    if (v) paginatedTasks.value.forEach((t) => selected.value.add(t.id))
    else paginatedTasks.value.forEach((t) => selected.value.delete(t.id))
    selected.value = new Set(selected.value)
  },
})
const someSelected = computed(
  () => paginatedTasks.value.some((t) => selected.value.has(t.id)) && !allSelectedOnPage.value,
)
function toggleRow(id: number, v: boolean) {
  if (v) selected.value.add(id)
  else selected.value.delete(id)
  selected.value = new Set(selected.value)
}

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
  toast({
    description: '任务批量导入接口暂未在后端开放，将在 Epic 31 后接入。如急需可在批改工作台单条添加',
    variant: 'info',
  })
}
</script>

<template>
  <AppShell>
    <BreadcrumbNav
      :items="[
        { label: '工作台', to: '/dashboard' },
        { label: '实训任务', to: '/teacher/tasks' },
        { label: '管理列表' },
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

    <!-- Selected toolbar -->
    <div v-if="selected.size > 0" class="px-4 py-2 bg-info-soft border border-info rounded-md flex items-center gap-3 anim-in">
      <Info class="w-4 h-4 text-info" />
      <span class="text-xs text-info">已选 {{ selected.size }} 项</span>
      <span class="text-xs text-info opacity-60">（批量操作即将开放）</span>
      <Button variant="ghost" size="sm" class="ml-auto" @click="selected = new Set()">取消选择</Button>
    </div>

    <!-- Table -->
    <Card class="tes-card-container overflow-hidden">
      <div class="tes-table-shell">
      <div class="grid min-w-[980px] grid-cols-[40px_minmax(18rem,1fr)_160px_160px_140px_120px_180px] items-center px-5 py-3.5 bg-surface-2 border-b border-border">
        <Checkbox
          :model-value="allSelectedOnPage ? true : someSelected ? 'indeterminate' : false"
          @update:model-value="(v) => allSelectedOnPage = v === true"
          aria-label="全选当前页"
        />
        <div class="text-[11px] font-semibold tracking-wider text-muted-foreground">任务名称</div>
        <div class="text-[11px] font-semibold tracking-wider text-muted-foreground">所属课程</div>
        <div class="text-[11px] font-semibold tracking-wider text-muted-foreground">截止时间</div>
        <div class="text-[11px] font-semibold tracking-wider text-muted-foreground">维度数</div>
        <div class="text-[11px] font-semibold tracking-wider text-muted-foreground">状态</div>
        <div class="text-[11px] font-semibold tracking-wider text-muted-foreground text-right">操作</div>
      </div>

      <template v-if="loading">
        <div
          v-for="n in 6"
          :key="n"
          class="grid min-w-[980px] grid-cols-[40px_minmax(18rem,1fr)_160px_160px_140px_120px_180px] items-center px-5 py-4 border-b border-border"
        >
          <Skeleton class="h-4 w-4" />
          <Skeleton class="h-10 w-3/4" />
          <Skeleton class="h-4 w-20" />
          <Skeleton class="h-4 w-24" />
          <Skeleton class="h-4 w-16" />
          <Skeleton class="h-5 w-12" />
          <Skeleton class="h-4 w-24 ml-auto" />
        </div>
      </template>

      <EmptyState
        v-else-if="paginatedTasks.length === 0"
        title="暂无任务"
        description="创建任务后可在此页面跟踪批改进度"
        action-label="创建任务"
        @action="goToCreate"
      />

      <div
        v-for="(task, idx) in paginatedTasks"
        v-else
        :key="task.id"
        class="grid min-w-[980px] grid-cols-[40px_minmax(18rem,1fr)_160px_160px_140px_120px_180px] items-center px-5 py-4 border-b border-border last:border-b-0 hover:bg-surface-2 transition-colors anim-in"
        :style="{ animationDelay: idx * 30 + 'ms' }"
      >
        <Checkbox
          :model-value="selected.has(task.id)"
          @update:model-value="(v) => toggleRow(task.id, v === true)"
          :aria-label="`选择 ${task.name}`"
        />
        <div class="min-w-0">
          <div class="tes-breakable text-sm font-semibold text-ink">{{ task.name }}</div>
          <div class="text-xs text-muted-foreground mt-1 line-clamp-1">{{ task.description || '暂无描述' }}</div>
        </div>
        <div class="text-sm text-foreground">{{ courseName(task.course_id) }}</div>
        <div>
          <div class="font-mono text-xs" :class="task.deadline ? 'text-accent' : 'text-subtle-foreground'">
            {{ formatDeadline(task.deadline) }}
          </div>
          <div class="text-xs text-muted-foreground mt-1">{{ deadlineHint(task.deadline, task.status) }}</div>
        </div>
        <div class="text-sm text-ink font-semibold">{{ task.dimensions.length }} 个维度</div>
        <div>
          <Badge :variant="statusVariant(task.status)">{{ statusLabel(task.status) }}</Badge>
        </div>
        <div class="flex items-center justify-end gap-1">
          <Button
            v-if="task.status === 'published'"
            variant="ghost" size="sm"
            class="h-7 px-2 text-primary"
            @click="goToGrading(task.id)"
          >批改</Button>
          <Button
            v-else-if="task.status === 'draft'"
            variant="ghost" size="sm"
            class="h-7 px-2 text-primary"
            @click="router.push(`/teacher/tasks/new?edit=${task.id}`)"
          >编辑</Button>
          <Button v-else variant="ghost" size="sm" class="h-7 px-2 text-primary" @click="goToReports(task.id)">报表</Button>

          <Button
            v-if="task.status === 'draft'"
            variant="ghost" size="sm" class="h-7 px-2"
            @click="publishTask(task)"
          >发布</Button>
          <Button
            v-else-if="task.status === 'published'"
            variant="ghost" size="sm" class="h-7 px-2"
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
      </div>

      <!-- Pagination -->
      </div>

      <div v-if="!loading && totalItems > 0" class="flex flex-wrap justify-between items-center gap-3 px-6 py-4 bg-surface-2 border-t border-border">
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
    </Card>
  </AppShell>
</template>
