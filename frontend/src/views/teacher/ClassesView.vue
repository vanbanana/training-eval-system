<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import axios from 'axios'
import AppShell from '@/components/layout/AppShell.vue'
import BreadcrumbNav from '@/components/business/BreadcrumbNav.vue'
import EmptyState from '@/components/business/EmptyState.vue'
import { useToast } from '@/components/ui/toast'
import { confirm } from '@/composables/useConfirm'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Avatar } from '@/components/ui/avatar'
import { Badge } from '@/components/ui/badge'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
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
  Search,
  Upload,
  Pencil,
  Archive,
  Flag,
  ChevronLeft,
  ChevronRight,
  Plus,
  Download,
  MoreHorizontal,
  UserPlus,
  UserX,
} from 'lucide-vue-next'

interface Course { id: number; name: string; code: string }
interface ClassItem {
  id: number
  name: string
  course_id: number
  teacher_id: number
  student_count: number
  is_archived: boolean
}
interface StudentRow {
  id: number
  display_name: string
  username: string
  joined_at: string | null
}

const breadcrumbs = [
  { label: '工作台', to: '/dashboard' },
  { label: '组织' },
  { label: '班级管理' },
]

const { toast } = useToast()
const router = useRouter()
const courses = ref<Course[]>([])
const classes = ref<ClassItem[]>([])
const selectedClassId = ref<number | null>(null)
const students = ref<StudentRow[]>([])
const loadingClasses = ref(true)
const loadingStudents = ref(false)
const searchClass = ref('')
const searchStudent = ref('')
const filterType = ref('all')
const currentPage = ref(1)
const pageSize = 20

// Create class dialog
const showCreateDialog = ref(false)
const newClass = ref({ name: '', course_id: 0 })
const createSubmitting = ref(false)

// Add students dialog
const showAddStudentsDialog = ref(false)
const studentIdsInput = ref('')
const addingStudents = ref(false)

// Import students dialog
const showImportDialog = ref(false)
const importFile = ref<File | null>(null)
const importingStudents = ref(false)

const selectedClass = computed(() =>
  classes.value.find((c) => c.id === selectedClassId.value) ?? null,
)
const selectedCourse = computed(() =>
  selectedClass.value
    ? courses.value.find((c) => c.id === selectedClass.value!.course_id) ?? null
    : null,
)

function courseNameOf(id: number): string {
  return courses.value.find((c) => c.id === id)?.name ?? `课程 #${id}`
}

const filteredClasses = computed(() => {
  if (!searchClass.value) return classes.value
  const q = searchClass.value.toLowerCase()
  return classes.value.filter((c) => c.name.toLowerCase().includes(q))
})

const filteredStudents = computed(() => {
  let list = students.value
  if (searchStudent.value) {
    const q = searchStudent.value.toLowerCase()
    list = list.filter(
      (s) => s.display_name.toLowerCase().includes(q) || s.username.toLowerCase().includes(q),
    )
  }
  return list
})

const paginatedStudents = computed(() => {
  const start = (currentPage.value - 1) * pageSize
  return filteredStudents.value.slice(start, start + pageSize)
})

const totalPages = computed(() => Math.max(1, Math.ceil(filteredStudents.value.length / pageSize)))

async function loadCourses() {
  try {
    const { data } = await axios.get('/api/courses')
    courses.value = data
  } catch {
    /* ignore */
  }
}

async function loadClasses() {
  loadingClasses.value = true
  try {
    const { data } = await axios.get('/api/classes')
    classes.value = data
    if (data.length > 0 && !selectedClassId.value) {
      await selectClass(data[0].id)
    }
  } catch {
    toast({ description: '加载班级失败', variant: 'destructive' })
  } finally {
    loadingClasses.value = false
  }
}

async function selectClass(id: number) {
  selectedClassId.value = id
  currentPage.value = 1
  await loadStudents(id)
}

async function loadStudents(id: number) {
  loadingStudents.value = true
  try {
    const { data } = await axios.get(`/api/classes/${id}/students`)
    students.value = Array.isArray(data) ? data : (data.items ?? [])
  } catch {
    students.value = []
  } finally {
    loadingStudents.value = false
  }
}

function getInitial(name: string): string {
  return name.charAt(0)
}

function formatJoined(iso: string | null) {
  if (!iso) return '—'
  return iso.slice(0, 10)
}

async function archiveClass() {
  if (!selectedClass.value) return
  const ok = await confirm({
    title: '归档班级',
    description: `确定归档「${selectedClass.value.name}」？`,
    variant: 'destructive',
    confirmText: '归档',
  })
  if (!ok) return
  try {
    await axios.patch(`/api/classes/${selectedClass.value.id}/archive`)
    toast({ description: '班级已归档', variant: 'success' })
    await loadClasses()
  } catch (e) {
    const msg = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    toast({ description: msg ?? '归档失败', variant: 'destructive' })
  }
}

function openCreateDialog() {
  newClass.value = { name: '', course_id: courses.value[0]?.id ?? 0 }
  showCreateDialog.value = true
}

async function submitCreateClass() {
  if (!newClass.value.name || !newClass.value.course_id) {
    toast({ description: '请填写完整', variant: 'destructive' })
    return
  }
  createSubmitting.value = true
  try {
    await axios.post('/api/classes', {
      name: newClass.value.name,
      course_id: newClass.value.course_id,
    })
    toast({ description: '班级已创建', variant: 'success' })
    showCreateDialog.value = false
    await loadClasses()
  } catch (e) {
    const msg = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    toast({ description: msg ?? '创建失败', variant: 'destructive' })
  } finally {
    createSubmitting.value = false
  }
}

function openAddStudentsDialog() {
  studentIdsInput.value = ''
  showAddStudentsDialog.value = true
}

async function submitAddStudents() {
  if (!selectedClass.value) return
  const ids = studentIdsInput.value
    .split(/[\s,;\n]+/)
    .map((s) => Number(s.trim()))
    .filter((n) => Number.isFinite(n) && n > 0)
  if (ids.length === 0) {
    toast({ description: '请输入至少一个学生 ID', variant: 'destructive' })
    return
  }
  addingStudents.value = true
  try {
    const { data } = await axios.post(
      `/api/classes/${selectedClass.value.id}/students/bulk`,
      { student_ids: ids },
    )
    toast({
      description: `添加完成：成功 ${data.added}，失败 ${data.failed?.length ?? 0}`,
      variant: 'success',
    })
    showAddStudentsDialog.value = false
    await loadStudents(selectedClass.value.id)
    await loadClasses()
  } catch (e) {
    const msg = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    toast({ description: msg ?? '添加失败', variant: 'destructive' })
  } finally {
    addingStudents.value = false
  }
}

function openImportDialog() {
  importFile.value = null
  showImportDialog.value = true
}

function downloadStudentTemplate() {
  // Reuses the shared user import template (same columns).
  window.location.href = '/api/imports/template/user.xlsx'
  toast({ description: '模板下载已开始', variant: 'info' })
}

function onImportFileChange(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (file) importFile.value = file
}

async function submitImportStudents() {
  if (!selectedClass.value || !importFile.value) {
    toast({ description: '请先选择文件', variant: 'destructive' })
    return
  }
  importingStudents.value = true
  try {
    const form = new FormData()
    form.append('file', importFile.value)
    form.append('class_id', String(selectedClass.value.id))
    const { data } = await axios.post('/api/imports/students', form, {
      headers: { 'Content-Type': 'multipart/form-data' },
    })
    toast({
      description: `导入完成：成功 ${data.success ?? data.added ?? 0}`,
      variant: 'success',
    })
    showImportDialog.value = false
    await loadStudents(selectedClass.value.id)
    await loadClasses()
  } catch (e) {
    const msg = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    toast({ description: msg ?? '导入失败', variant: 'destructive' })
  } finally {
    importingStudents.value = false
  }
}

function exportClassRoster() {
  if (!selectedClass.value) return
  // 后端 §十一 提到 /api/imports/exports/class/{id}/students.xlsx
  window.location.href = `/api/imports/exports/class/${selectedClass.value.id}/students.xlsx`
  toast({ description: '名单下载已开始', variant: 'info' })
}

async function removeStudent(s: StudentRow) {
  if (!selectedClass.value) return
  const ok = await confirm({
    title: '移出班级',
    description: `确定将 ${s.display_name} 移出当前班级？`,
    variant: 'destructive',
    confirmText: '移出',
  })
  if (!ok) return
  try {
    await axios.delete(`/api/classes/${selectedClass.value.id}/students/${s.id}`)
    toast({ description: `已将 ${s.display_name} 移出班级`, variant: 'success' })
    await loadStudents(selectedClass.value.id)
    await loadClasses()
  } catch (e) {
    const msg = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    toast({ description: msg ?? '移出失败', variant: 'destructive' })
  }
}

function viewProfile(s: StudentRow) {
  router.push(`/teacher/students/${s.id}/profile`)
}

onMounted(async () => {
  await Promise.all([loadCourses(), loadClasses()])
})
</script>

<template>
  <AppShell>
    <BreadcrumbNav :items="breadcrumbs" />

    <div class="flex justify-between items-end">
      <div>
        <h1 class="text-2xl font-bold text-ink">班级管理</h1>
        <p class="mt-1.5 text-sm text-muted-foreground">按学年学期管理课程下的班级与学生名单</p>
      </div>
      <div class="flex items-center gap-3">
        <Button variant="outline" :disabled="!selectedClass" @click="openImportDialog">
          <Upload class="w-3.5 h-3.5" />
          导入学生
        </Button>
        <Button @click="openCreateDialog">
          <Plus class="w-4 h-4" />
          新建班级
        </Button>
      </div>
    </div>

    <div class="tes-grid-sidebar-main">
      <!-- LEFT: Class List -->
      <Card class="tes-card-container overflow-hidden">
        <div class="p-[18px] border-b border-border flex flex-col gap-2.5">
          <span class="text-sm font-semibold text-ink">班级列表</span>
          <div class="relative">
            <Search class="w-3.5 h-3.5 text-muted-foreground absolute left-3 top-1/2 -translate-y-1/2" />
            <Input v-model="searchClass" placeholder="搜索班级" class="pl-9 h-9" />
          </div>
        </div>
        <div v-if="loadingClasses" class="p-3 space-y-2">
          <Skeleton v-for="n in 4" :key="n" class="h-12" />
        </div>
        <div v-else-if="filteredClasses.length === 0" class="p-4 text-center text-xs text-muted-foreground">
          暂无班级
        </div>
        <div v-else class="flex flex-col">
          <button
            v-for="c in filteredClasses"
            :key="c.id"
            class="text-left px-[18px] py-3 border-b border-border flex flex-col gap-1 cursor-pointer hover:bg-surface-2 transition-colors"
            :class="[
              c.id === selectedClassId ? 'bg-primary-soft border-l-[3px] border-l-primary !pl-[15px]' : '',
              c.is_archived ? 'opacity-50' : '',
            ]"
            @click="selectClass(c.id)"
          >
            <span class="text-[13px] font-semibold" :class="c.id === selectedClassId ? 'text-primary' : 'text-ink'">
              {{ c.name }}
            </span>
            <span class="text-[11px] text-muted-foreground">
              {{ courseNameOf(c.course_id) }} · {{ c.is_archived ? '已归档' : `${c.student_count} 人` }}
            </span>
          </button>
        </div>
      </Card>

      <!-- RIGHT: Detail -->
      <Card class="tes-card-container overflow-hidden">
        <template v-if="selectedClass">
          <div class="px-6 py-[18px] border-b border-border flex justify-between items-center">
            <div>
              <div class="flex items-center gap-2.5">
                <h2 class="text-lg font-bold text-ink">{{ selectedClass.name }}</h2>
                <Badge variant="info">{{ selectedCourse?.name ?? `课程 #${selectedClass.course_id}` }}</Badge>
                <Badge v-if="selectedClass.is_archived" variant="secondary">已归档</Badge>
              </div>
              <div class="text-xs text-muted-foreground mt-1.5">
                {{ selectedClass.student_count }} 人
              </div>
            </div>
            <div class="flex items-center gap-2">
              <DropdownMenu>
                <DropdownMenuTrigger as-child>
                  <Button variant="outline" size="icon">
                    <Pencil class="w-4 h-4" />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  <DropdownMenuItem @select="openCreateDialog">
                    <Plus class="text-muted-foreground" />
                    复制并新建
                  </DropdownMenuItem>
                  <DropdownMenuItem
                    :disabled="selectedClass.is_archived"
                    class="text-danger focus:bg-danger-soft focus:text-danger"
                    @select="archiveClass"
                  >
                    <Archive class="text-current" />
                    归档班级
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </div>
          </div>

          <div class="bg-surface-2 border-b border-border px-6 py-[18px] tes-grid-kpi">
            <div class="flex min-w-0 flex-col gap-0.5">
              <span class="text-[11px] text-muted-foreground">学生总数</span>
              <span class="text-[22px] font-bold text-ink">{{ selectedClass.student_count }}</span>
            </div>
            <div class="flex min-w-0 flex-col gap-0.5">
              <span class="text-[11px] text-muted-foreground">课程</span>
              <span class="text-[22px] font-bold text-ink">{{ selectedCourse?.code ?? '—' }}</span>
            </div>
            <div class="flex min-w-0 flex-col gap-0.5">
              <span class="text-[11px] text-muted-foreground">状态</span>
              <span class="text-[22px] font-bold" :class="selectedClass.is_archived ? 'text-muted-foreground' : 'text-success'">
                {{ selectedClass.is_archived ? '归档' : '在用' }}
              </span>
            </div>
          </div>

          <div class="px-6 py-3.5 border-b border-border flex flex-wrap justify-between items-center gap-3">
            <div class="flex min-w-0 flex-wrap items-center gap-3">
              <div class="relative w-full sm:w-60">
                <Search class="w-3.5 h-3.5 text-muted-foreground absolute left-3 top-1/2 -translate-y-1/2" />
                <Input v-model="searchStudent" placeholder="按学号 / 姓名搜索" class="pl-9" />
              </div>
              <Select v-model="filterType">
                <SelectTrigger class="w-32"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">全部</SelectItem>
                  <SelectItem value="recent">最近加入</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div class="flex items-center gap-3">
              <Button variant="outline" size="sm" @click="exportClassRoster">
                <Download class="w-3.5 h-3.5" />
                导出名单
              </Button>
              <Button size="sm" @click="openAddStudentsDialog">
                <UserPlus class="w-3.5 h-3.5" />
                添加学生
              </Button>
            </div>
          </div>

          <div class="tes-table-shell">
            <div class="grid min-w-[860px] grid-cols-[36px_240px_140px_160px_140px_minmax(12rem,1fr)] items-center bg-surface-2 px-6 py-3 border-b border-border">
              <div></div>
              <div class="text-[11px] font-semibold tracking-wider text-muted-foreground">学生</div>
              <div class="text-[11px] font-semibold tracking-wider text-muted-foreground">学号</div>
              <div class="text-[11px] font-semibold tracking-wider text-muted-foreground">加入时间</div>
              <div class="text-[11px] font-semibold tracking-wider text-muted-foreground">状态</div>
              <div class="text-[11px] font-semibold tracking-wider text-muted-foreground text-right">操作</div>
            </div>

          <template v-if="loadingStudents">
            <div v-for="n in 5" :key="n" class="grid min-w-[860px] grid-cols-[36px_240px_140px_160px_140px_minmax(12rem,1fr)] items-center px-6 py-3.5 border-b border-border">
              <Skeleton class="h-4 w-4" />
              <Skeleton class="h-9 w-3/4" />
              <Skeleton class="h-4 w-20" />
              <Skeleton class="h-4 w-24" />
              <Skeleton class="h-5 w-12" />
              <Skeleton class="h-4 w-16 ml-auto" />
            </div>
          </template>

          <EmptyState
            v-else-if="paginatedStudents.length === 0"
            title="暂无学生"
            description="点击上方「添加学生」开始管理"
            :icon="Flag"
          />

          <div
            v-for="(s, idx) in paginatedStudents"
            v-else
            :key="s.id"
            class="grid min-w-[860px] grid-cols-[36px_240px_140px_160px_140px_minmax(12rem,1fr)] items-center px-6 py-3.5 border-b border-border last:border-b-0 hover:bg-surface-2 transition-colors anim-in"
            :style="{ animationDelay: Math.min(idx * 20, 200) + 'ms' }"
          >
            <div></div>
            <div class="flex items-center gap-2.5">
              <Avatar size="sm">{{ getInitial(s.display_name) }}</Avatar>
              <div class="flex flex-col">
                <span class="text-sm font-medium text-ink">{{ s.display_name }}</span>
                <span class="text-[11px] text-muted-foreground">ID #{{ s.id }}</span>
              </div>
            </div>
            <div class="font-mono text-xs text-muted-foreground">{{ s.username }}</div>
            <div class="font-mono text-xs text-muted-foreground">{{ formatJoined(s.joined_at) }}</div>
            <Badge variant="success">在班</Badge>
            <div class="flex items-center justify-end gap-1.5">
              <Button variant="ghost" size="sm" class="h-7 px-2 text-primary" @click="viewProfile(s)">画像</Button>
              <Button variant="ghost" size="sm" class="h-7 px-2 text-danger hover:text-danger" @click="removeStudent(s)">
                <UserX class="w-3 h-3" />
                移除
              </Button>
            </div>
          </div>

          </div>

          <div v-if="filteredStudents.length > 0" class="flex flex-wrap justify-between items-center gap-3 px-6 py-3.5 bg-surface-2 border-t border-border">
            <span class="text-xs text-muted-foreground">
              显示 {{ (currentPage - 1) * pageSize + 1 }} - {{ Math.min(currentPage * pageSize, filteredStudents.length) }} 共 {{ filteredStudents.length }} 名学生
            </span>
            <div class="flex items-center gap-2 text-xs text-ink">
              <Button variant="ghost" size="icon-sm" :disabled="currentPage <= 1" @click="currentPage--">
                <ChevronLeft class="w-3.5 h-3.5" />
              </Button>
              <span>{{ currentPage }} / {{ totalPages }}</span>
              <Button variant="ghost" size="icon-sm" :disabled="currentPage >= totalPages" @click="currentPage++">
                <ChevronRight class="w-3.5 h-3.5" />
              </Button>
            </div>
          </div>
        </template>

        <EmptyState
          v-else
          :icon="MoreHorizontal"
          title="未选择班级"
          :description="loadingClasses ? '加载中...' : '请从左侧选择一个班级，或新建班级'"
          action-label="新建班级"
          @action="openCreateDialog"
        />
      </Card>
    </div>

    <!-- Create Class Dialog -->
    <Dialog v-model:open="showCreateDialog">
      <DialogContent class="max-w-md">
        <DialogHeader>
          <DialogTitle>新建班级</DialogTitle>
          <DialogDescription>关联到课程后即可添加学生</DialogDescription>
        </DialogHeader>
        <div class="flex flex-col gap-4">
          <div class="space-y-2">
            <Label>班级名称<span class="text-danger ml-0.5">*</span></Label>
            <Input v-model="newClass.name" placeholder="如 软工 21-3 班" />
          </div>
          <div class="space-y-2">
            <Label>所属课程<span class="text-danger ml-0.5">*</span></Label>
            <Select v-model="newClass.course_id">
              <SelectTrigger><SelectValue placeholder="选择课程" /></SelectTrigger>
              <SelectContent>
                <SelectItem v-for="c in courses" :key="c.id" :value="c.id">
                  {{ c.name }}（{{ c.code }}）
                </SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>
        <DialogFooter class="gap-2">
          <Button variant="outline" @click="showCreateDialog = false">取消</Button>
          <Button :disabled="createSubmitting" @click="submitCreateClass">
            {{ createSubmitting ? '创建中...' : '确认创建' }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- Add Students Dialog -->
    <Dialog v-model:open="showAddStudentsDialog">
      <DialogContent class="max-w-md">
        <DialogHeader>
          <DialogTitle>添加学生</DialogTitle>
          <DialogDescription>
            输入学生 ID（每行一个，或逗号分隔）；后端将批量加入到 {{ selectedClass?.name }}
          </DialogDescription>
        </DialogHeader>
        <div class="space-y-2">
          <Label>学生 ID 列表</Label>
          <textarea
            v-model="studentIdsInput"
            rows="6"
            class="w-full font-mono text-sm border border-border-strong rounded-md p-3 outline-none focus:border-primary"
            placeholder="123&#10;456&#10;789"
          ></textarea>
          <p class="text-[11px] text-muted-foreground">不会重复添加；如需通过 username/姓名添加，请使用「导入学生」</p>
        </div>
        <DialogFooter class="gap-2">
          <Button variant="outline" @click="showAddStudentsDialog = false">取消</Button>
          <Button :disabled="addingStudents" @click="submitAddStudents">
            {{ addingStudents ? '添加中...' : '批量添加' }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- Import Students Dialog -->
    <Dialog v-model:open="showImportDialog">
      <DialogContent class="max-w-md">
        <DialogHeader>
          <DialogTitle>导入学生</DialogTitle>
          <DialogDescription>使用 Excel 模板将学生加入到 {{ selectedClass?.name }}</DialogDescription>
        </DialogHeader>
        <div class="space-y-3">
          <div class="bg-info-soft border border-info rounded-md p-3 text-xs text-info">
            模板列：username、display_name、（可选）password。系统将自动创建账号或关联现有账号。
          </div>
          <div class="space-y-2">
            <Label>选择文件</Label>
            <input
              type="file"
              accept=".xlsx,.csv"
              class="block w-full text-sm border border-border-strong rounded-md px-3 py-2"
              @change="onImportFileChange"
            />
            <p v-if="importFile" class="text-[11px] text-muted-foreground">
              已选 {{ importFile.name }}（{{ (importFile.size / 1024).toFixed(1) }} KB）
            </p>
          </div>
          <Button variant="link" size="sm" class="px-0" @click="downloadStudentTemplate">
            <Download class="w-3.5 h-3.5" />
            下载学生导入模板
          </Button>
        </div>
        <DialogFooter class="gap-2">
          <Button variant="outline" @click="showImportDialog = false">取消</Button>
          <Button :disabled="!importFile || importingStudents" @click="submitImportStudents">
            {{ importingStudents ? '导入中...' : '开始导入' }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </AppShell>
</template>
