<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import axios from 'axios'
import AppShell from '@/components/layout/AppShell.vue'
import BreadcrumbNav from '@/components/business/BreadcrumbNav.vue'
import EmptyState from '@/components/business/EmptyState.vue'
import IllustNoCourses from '@/components/illustrations/IllustNoCourses.vue'
import { useToast } from '@/components/ui/toast'
import { confirm } from '@/composables/useConfirm'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  Code2,
  Network,
  Globe,
  Database,
  Cpu,
  Plus,
  Search,
  Download,
  Archive,
  ArchiveRestore,
  Eye,
  MoreHorizontal,
  BookOpen,
} from 'lucide-vue-next'

interface Course {
  id: number
  code: string
  name: string
  is_archived: boolean
  class_count: number
  // 后端返回的派生字段（前端按 id 拉 classes 后聚合）
  student_count?: number
  task_count?: number
}

const { toast } = useToast()
const courses = ref<Course[]>([])
const loading = ref(true)
const activeTab = ref<'all' | 'active' | 'archived'>('all')
const searchQuery = ref('')

// Create modal
const showCreateDialog = ref(false)
const newCourse = ref({ code: '', name: '' })
const submittingCreate = ref(false)
const createErrors = ref<Record<string, string>>({})

// Detail dialog
const showDetailDialog = ref(false)
const detailLoading = ref(false)
const detailCourse = ref<Course | null>(null)
const detailClasses = ref<{ id: number; name: string; teacher_id: number | null; student_count: number; is_archived: boolean }[]>([])

const iconList = [Code2, Network, Globe, Database, Cpu, BookOpen]
const iconColorList = [
  'bg-primary-soft text-primary',
  'bg-success-soft text-success',
  'bg-info-soft text-info',
  'bg-accent-soft text-accent',
  'bg-gold-soft text-gold',
  'bg-warning-soft text-warning',
]

function iconForIndex(index: number) {
  return iconList[index % iconList.length]
}
function iconColorForIndex(index: number) {
  return iconColorList[index % iconColorList.length]
}

const filteredCourses = computed(() => {
  let list = courses.value
  if (activeTab.value === 'active') list = list.filter((c) => !c.is_archived)
  if (activeTab.value === 'archived') list = list.filter((c) => c.is_archived)
  if (searchQuery.value) {
    const q = searchQuery.value.toLowerCase()
    list = list.filter(
      (c) =>
        c.name.toLowerCase().includes(q) || c.code.toLowerCase().includes(q),
    )
  }
  return list
})

const tabCounts = computed(() => ({
  all: courses.value.length,
  active: courses.value.filter((c) => !c.is_archived).length,
  archived: courses.value.filter((c) => c.is_archived).length,
}))

async function fetchCourses() {
  loading.value = true
  try {
    const { data } = await axios.get<Course[]>('/api/courses')
    courses.value = data
    // 并发拉每个课程的 classes 和 tasks 派生 student_count / task_count
    await enrichCourseStats()
  } catch {
    toast({ description: '加载课程失败', variant: 'destructive' })
  } finally {
    loading.value = false
  }
}

async function enrichCourseStats() {
  // 拉所有任务一次（任务列表对管理员是完整的）
  const taskCountByCourse = new Map<number, number>()
  try {
    const { data: tasks } = await axios.get<{ course_id: number }[]>('/api/tasks')
    for (const t of tasks) {
      taskCountByCourse.set(
        t.course_id,
        (taskCountByCourse.get(t.course_id) ?? 0) + 1,
      )
    }
  } catch {
    /* 任务接口失败时退化为空 */
  }
  // 并发拉每个课程的 classes，求 student_count 总和
  await Promise.all(
    courses.value.map(async (c) => {
      try {
        const { data: clsList } = await axios.get<
          { student_count: number }[]
        >(`/api/courses/${c.id}/classes`)
        c.student_count = clsList.reduce(
          (sum, cls) => sum + (cls.student_count || 0),
          0,
        )
      } catch {
        c.student_count = undefined
      }
      c.task_count = taskCountByCourse.get(c.id) ?? 0
    }),
  )
  // 触发响应式更新
  courses.value = [...courses.value]
}

onMounted(fetchCourses)

function openCreate() {
  newCourse.value = { code: '', name: '' }
  createErrors.value = {}
  showCreateDialog.value = true
}

function validateCreate() {
  createErrors.value = {}
  if (!newCourse.value.code || newCourse.value.code.length < 2) {
    createErrors.value.code = '编号至少 2 个字符'
  }
  if (!newCourse.value.name) {
    createErrors.value.name = '课程名必填'
  }
  return Object.keys(createErrors.value).length === 0
}

async function submitCreate() {
  if (!validateCreate()) return
  submittingCreate.value = true
  try {
    await axios.post('/api/courses', newCourse.value)
    toast({ description: '课程创建成功', variant: 'success' })
    showCreateDialog.value = false
    await fetchCourses()
  } catch (e) {
    const msg = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    toast({ description: msg ?? '创建失败', variant: 'destructive' })
  } finally {
    submittingCreate.value = false
  }
}

async function archiveCourse(c: Course) {
  const action = c.is_archived ? '取消归档' : '归档'
  const ok = await confirm({
    title: `${action}课程`,
    description: `确定${action}「${c.name}」？`,
    confirmText: action,
  })
  if (!ok) return
  try {
    await axios.patch(`/api/courses/${c.id}/archive`)
    toast({ description: `课程已${action}`, variant: 'success' })
    await fetchCourses()
  } catch (e) {
    const msg = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    toast({ description: msg ?? `${action}失败`, variant: 'destructive' })
  }
}

async function viewDetail(c: Course) {
  detailCourse.value = c
  detailClasses.value = []
  detailLoading.value = true
  showDetailDialog.value = true
  try {
    const { data } = await axios.get(`/api/courses/${c.id}/classes`)
    detailClasses.value = data
  } catch {
    detailClasses.value = []
  } finally {
    detailLoading.value = false
  }
}

function exportCourses() {
  const rows = [['ID', '编号', '名称', '班级数', '学生数', '任务数', '状态']]
  courses.value.forEach((c) => {
    rows.push([
      String(c.id),
      c.code,
      c.name,
      String(c.class_count ?? 0),
      String(c.student_count ?? 0),
      String(c.task_count ?? 0),
      c.is_archived ? '已归档' : '开课中',
    ])
  })
  const csv = rows
    .map((r) => r.map((cell) => `"${cell.replace(/"/g, '""')}"`).join(','))
    .join('\n')
  const blob = new Blob(['\uFEFF' + csv], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `courses-${new Date().toISOString().slice(0, 10)}.csv`
  a.click()
  URL.revokeObjectURL(url)
  toast({ description: '已导出 CSV', variant: 'success' })
}
</script>

<template>
  <AppShell>
    <BreadcrumbNav
      :items="[
        { label: '工作台', to: '/dashboard' },
        { label: '组织' },
        { label: '课程管理' },
      ]"
    />

    <!-- Page Header -->
    <div class="flex justify-between items-end">
      <div>
        <h1 class="text-2xl font-bold text-ink">课程管理</h1>
        <p class="mt-1 text-sm text-muted-foreground">维护课程信息，关联班级与实训任务</p>
      </div>
      <div class="flex items-center gap-3">
        <Button variant="outline" @click="exportCourses">
          <Download class="w-4 h-4" />
          导出
        </Button>
        <Button @click="openCreate">
          <Plus class="w-4 h-4" />
          新建课程
        </Button>
      </div>
    </div>

    <!-- Toolbar -->
    <Card>
      <CardContent class="px-5 py-3.5 flex justify-between items-center gap-4">
        <Tabs v-model="activeTab">
          <TabsList>
            <TabsTrigger value="all">全部 {{ tabCounts.all }}</TabsTrigger>
            <TabsTrigger value="active">开课中 {{ tabCounts.active }}</TabsTrigger>
            <TabsTrigger value="archived">已归档 {{ tabCounts.archived }}</TabsTrigger>
          </TabsList>
        </Tabs>

        <div class="relative w-[280px]">
          <Search class="w-3.5 h-3.5 text-muted-foreground absolute left-3 top-1/2 -translate-y-1/2" />
          <Input v-model="searchQuery" placeholder="按课程名 / 编号搜索" class="pl-9" />
        </div>
      </CardContent>
    </Card>

    <!-- Loading -->
    <div v-if="loading" class="tes-grid-cards">
      <Skeleton v-for="n in 6" :key="n" class="h-[280px] rounded-lg" />
    </div>

    <!-- Empty -->
    <EmptyState
      v-else-if="filteredCourses.length === 0"
      :illustration="IllustNoCourses"
      title="暂无课程"
      description="点击下方「新建课程」开始创建"
      action-label="新建课程"
      @action="openCreate"
    />

    <!-- Course Grid -->
    <div v-else class="tes-grid-cards">
      <Card
        v-for="(course, idx) in filteredCourses"
        :key="course.id"
        :class="['overflow-hidden hover:border-primary anim-in', course.is_archived ? 'opacity-60' : '']"
        :style="{ animationDelay: Math.min(idx * 40, 240) + 'ms' }"
      >
        <CardContent class="p-5 pb-4 flex flex-col gap-2.5">
          <div class="flex justify-between items-center">
            <div :class="['w-8 h-8 rounded-md grid place-items-center', iconColorForIndex(idx)]">
              <component :is="iconForIndex(idx)" class="h-4 w-4" />
            </div>
            <Badge :variant="course.is_archived ? 'secondary' : 'success'">
              {{ course.is_archived ? '已归档' : '开课中' }}
            </Badge>
          </div>
          <div class="text-lg font-bold text-ink">{{ course.name }}</div>
          <div class="font-mono text-[11px] text-muted-foreground">
            {{ course.code }}
          </div>
        </CardContent>
        <div class="bg-surface-2 px-5 py-3.5 border-t border-b border-border flex justify-around">
          <div class="flex flex-col items-center gap-0.5">
            <span class="text-lg font-bold text-ink">{{ course.class_count ?? 0 }}</span>
            <span class="text-[11px] text-muted-foreground">班级</span>
          </div>
          <div class="flex flex-col items-center gap-0.5">
            <span class="text-lg font-bold text-ink">{{ course.student_count ?? '—' }}</span>
            <span class="text-[11px] text-muted-foreground">学生</span>
          </div>
          <div class="flex flex-col items-center gap-0.5">
            <span class="text-lg font-bold text-ink">{{ course.task_count ?? '—' }}</span>
            <span class="text-[11px] text-muted-foreground">实训任务</span>
          </div>
        </div>
        <div class="px-5 py-3.5 flex justify-between items-center">
          <span class="text-[11px] text-muted-foreground truncate flex-1 mr-2">
            {{ course.is_archived ? '已归档' : `共 ${course.class_count} 个班级` }}
          </span>
          <div class="flex items-center gap-1">
            <Button variant="ghost" size="sm" class="h-7 px-2 text-primary" @click="viewDetail(course)">
              查看
            </Button>
            <DropdownMenu>
              <DropdownMenuTrigger as-child>
                <Button variant="ghost" size="icon-sm">
                  <MoreHorizontal class="w-3.5 h-3.5" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuItem @select="viewDetail(course)">
                  <Eye class="text-muted-foreground" />
                  查看班级
                </DropdownMenuItem>
                <DropdownMenuItem
                  :class="course.is_archived ? 'text-primary focus:bg-primary-soft focus:text-primary' : 'text-danger focus:bg-danger-soft focus:text-danger'"
                  @select="archiveCourse(course)"
                >
                  <Archive v-if="!course.is_archived" class="text-current" />
                  <ArchiveRestore v-else class="text-current" />
                  {{ course.is_archived ? '取消归档' : '归档课程' }}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </div>
      </Card>

      <!-- New Course Card -->
      <button
        class="bg-surface-2 border border-dashed border-border-strong rounded-lg p-6 flex flex-col items-center justify-center gap-3.5 min-h-[280px] cursor-pointer hover:border-primary hover:bg-primary-soft transition-colors active:scale-[0.99]"
        @click="openCreate"
      >
        <div class="w-12 h-12 bg-card border border-border rounded-full grid place-items-center text-primary">
          <Plus class="h-5 w-5" />
        </div>
        <div class="text-sm font-semibold text-ink">新建课程</div>
        <div class="text-xs text-muted-foreground">创建新课程并关联班级</div>
      </button>
    </div>

    <!-- Create Course Dialog -->
    <Dialog v-model:open="showCreateDialog">
      <DialogContent class="max-w-md">
        <DialogHeader>
          <DialogTitle>新建课程</DialogTitle>
          <DialogDescription>创建后可在「班级管理」为其关联班级</DialogDescription>
        </DialogHeader>
        <div class="flex flex-col gap-4">
          <div class="space-y-2">
            <Label>课程编号 <span class="text-danger">*</span></Label>
            <Input v-model="newCourse.code" placeholder="如 CS-101" class="font-mono" />
            <p v-if="createErrors.code" class="text-xs text-danger">{{ createErrors.code }}</p>
          </div>
          <div class="space-y-2">
            <Label>课程名称 <span class="text-danger">*</span></Label>
            <Input v-model="newCourse.name" placeholder="如 计算机程序设计" />
            <p v-if="createErrors.name" class="text-xs text-danger">{{ createErrors.name }}</p>
          </div>
        </div>
        <DialogFooter class="gap-2">
          <Button variant="outline" @click="showCreateDialog = false">取消</Button>
          <Button :disabled="submittingCreate" @click="submitCreate">
            {{ submittingCreate ? '提交中...' : '确认创建' }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- Detail Dialog -->
    <Dialog v-model:open="showDetailDialog">
      <DialogContent class="max-w-2xl">
        <DialogHeader>
          <DialogTitle>{{ detailCourse?.name ?? '课程详情' }}</DialogTitle>
          <DialogDescription>
            <span class="font-mono">{{ detailCourse?.code }}</span> · 共 {{ detailClasses.length }} 个班级
          </DialogDescription>
        </DialogHeader>
        <div v-if="detailLoading" class="space-y-2">
          <Skeleton v-for="n in 3" :key="n" class="h-10 w-full" />
        </div>
        <div v-else-if="detailClasses.length === 0" class="text-center text-sm text-muted-foreground py-8">
          该课程下尚无班级
        </div>
        <div v-else class="tes-table-shell border border-border rounded-md">
          <div class="grid min-w-[420px] grid-cols-[minmax(12rem,1fr)_120px_100px] px-4 py-2 bg-surface-2 text-[11px] font-semibold text-muted-foreground border-b border-border">
            <span>班级名称</span>
            <span>学生数</span>
            <span>状态</span>
          </div>
          <div
            v-for="cls in detailClasses"
            :key="cls.id"
            class="grid min-w-[420px] grid-cols-[minmax(12rem,1fr)_120px_100px] px-4 py-2.5 border-b border-border last:border-b-0 text-sm"
          >
            <span class="font-medium text-ink">{{ cls.name }}</span>
            <span>{{ cls.student_count }}</span>
            <Badge :variant="cls.is_archived ? 'secondary' : 'success'">
              {{ cls.is_archived ? '已归档' : '在用' }}
            </Badge>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  </AppShell>
</template>
