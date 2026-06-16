<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import axios from 'axios'
import { GripVertical, Trash2, Bookmark, Save, CalendarClock, Info, Plus, X } from 'lucide-vue-next'
import AppShell from '@/components/layout/AppShell.vue'
import BreadcrumbNav from '@/components/business/BreadcrumbNav.vue'
import { useToast } from '@/components/ui/toast'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Card } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
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
import { Checkbox } from '@/components/ui/checkbox'
import { ScrollArea } from '@/components/ui/scroll-area'

interface Dimension {
  id?: number
  name: string
  description: string
  weight: number
}
interface Course { id: number; name: string; code: string }
interface ClassItem { id: number; name: string; course_id: number; student_count: number }
interface Template {
  id: number
  name: string
  description?: string
  visibility?: string
  dimensions: Dimension[]
  is_system?: boolean
}

const route = useRoute()
const router = useRouter()
const { toast } = useToast()

const editId = computed(() => Number(route.query.edit) || 0)
const editing = computed(() => editId.value > 0)
const loadingTask = ref(false)

const name = ref('')
const courseId = ref<number>(0)
const deadline = ref('')
const description = ref('')
const requirements = ref('')

const allCourses = ref<Course[]>([])
const allClasses = ref<ClassItem[]>([])
const selectedClassIds = ref<Set<number>>(new Set())

function courseNameOf(id: number | null | undefined): string {
  if (id == null) return '——'
  return allCourses.value.find((c) => c.id === id)?.name ?? `课程 #${id}`
}

const dimensions = ref<Dimension[]>([
  { name: '代码规范性', description: '命名、注释、复用性、错误处理与风格统一', weight: 30 },
  { name: '功能完整性', description: '需求覆盖度与功能模块的完整实现', weight: 35 },
  { name: '测试与质量', description: '单元测试覆盖率与边界处理', weight: 20 },
  { name: '实验报告与文档', description: '报告结构、内容清晰度与图表表达', weight: 15 },
])

const weightSum = computed(() => dimensions.value.reduce((s, d) => s + (d.weight || 0), 0))
const nameLength = computed(() => name.value.length)
const requirementsLength = computed(() => requirements.value.length)

const submitting = ref(false)
const error = ref('')

// Templates
const templates = ref<Template[]>([])
const selectedTemplateId = ref<number | null>(null)
const showTemplatePicker = ref(false)
const showSaveTemplateDialog = ref(false)
const templateForm = ref({ name: '', description: '', visibility: 'private' as 'private' | 'shared' })
const submittingTpl = ref(false)
const taskCreatedId = ref<number | null>(null)

// Class picker
const showClassPicker = ref(false)
const classPickerSearch = ref('')

const breadcrumbs = computed(() => [
  { label: '工作台', to: '/dashboard' },
  { label: '实训任务', to: '/teacher/tasks' },
  { label: editing.value ? '编辑实训任务' : '创建实训任务' },
])

const selectedClasses = computed(() =>
  allClasses.value.filter((c) => selectedClassIds.value.has(c.id)),
)

const filteredClassesForPicker = computed(() => {
  if (!classPickerSearch.value.trim()) return allClasses.value
  const q = classPickerSearch.value.trim().toLowerCase()
  return allClasses.value.filter((c) => c.name.toLowerCase().includes(q))
})

async function loadCourses() {
  try {
    const { data } = await axios.get('/api/courses')
    allCourses.value = data
    if (allCourses.value.length > 0 && !courseId.value) {
      courseId.value = allCourses.value[0].id
    }
  } catch {
    /* ignore */
  }
}

async function loadClasses() {
  try {
    const { data } = await axios.get('/api/classes')
    allClasses.value = data
  } catch {
    /* ignore */
  }
}

async function loadTemplates() {
  try {
    const { data } = await axios.get('/api/templates')
    templates.value = data
  } catch {
    /* ignore */
  }
}

async function loadTaskForEdit(id: number) {
  loadingTask.value = true
  try {
    const { data } = await axios.get(`/api/tasks/${id}`)
    name.value = data.name
    description.value = data.description ?? ''
    requirements.value = data.requirements ?? ''
    courseId.value = data.course_id
    deadline.value = data.deadline ? data.deadline.slice(0, 16) : ''
    if (Array.isArray(data.dimensions) && data.dimensions.length > 0) {
      dimensions.value = data.dimensions.map((d: { id?: number; name: string; description?: string; weight: number }) => ({
        id: d.id,
        name: d.name,
        description: d.description ?? '',
        weight: d.weight,
      }))
    }
    if (Array.isArray(data.class_ids)) {
      selectedClassIds.value = new Set(data.class_ids)
    }
  } catch (e) {
    const msg = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    toast({ description: msg ?? '加载任务失败', variant: 'destructive' })
  } finally {
    loadingTask.value = false
  }
}

onMounted(async () => {
  await Promise.all([loadCourses(), loadClasses(), loadTemplates()])
  if (editing.value) {
    await loadTaskForEdit(editId.value)
  }
  // Deep-link: /teacher/tasks/new?template_id=<id> auto-applies that template.
  const tplId = Number(route.query.template_id) || 0
  if (tplId > 0) {
    const tpl = templates.value.find((t) => t.id === tplId)
    if (tpl) applyTemplateLocal(tpl)
  }
})

function addDimension() {
  if (dimensions.value.length >= 10) return
  dimensions.value.push({ name: '', description: '', weight: 0 })
}

function removeDimension(i: number) {
  if (dimensions.value.length <= 2) return
  dimensions.value.splice(i, 1)
}

// HTML5 drag-and-drop reordering
const dragIndex = ref<number | null>(null)

function onDragStart(i: number, e: DragEvent) {
  dragIndex.value = i
  if (e.dataTransfer) {
    e.dataTransfer.effectAllowed = 'move'
    e.dataTransfer.setData('text/plain', String(i))
  }
}
function onDragOver(e: DragEvent) {
  e.preventDefault()
  if (e.dataTransfer) e.dataTransfer.dropEffect = 'move'
}
function onDrop(target: number) {
  if (dragIndex.value === null || dragIndex.value === target) {
    dragIndex.value = null
    return
  }
  const arr = [...dimensions.value]
  const [moved] = arr.splice(dragIndex.value, 1)
  arr.splice(target, 0, moved)
  dimensions.value = arr
  dragIndex.value = null
}

function toggleClass(id: number, v: boolean) {
  if (v) selectedClassIds.value.add(id)
  else selectedClassIds.value.delete(id)
  selectedClassIds.value = new Set(selectedClassIds.value)
}

function applyTemplateLocal(tpl: Template) {
  if (!tpl.dimensions || tpl.dimensions.length === 0) {
    toast({ description: '该模板无评价维度', variant: 'warning' })
    return
  }
  dimensions.value = tpl.dimensions.map((d) => ({
    name: d.name,
    description: d.description ?? '',
    weight: d.weight,
  }))
  selectedTemplateId.value = tpl.id
  showTemplatePicker.value = false
  toast({ description: `已应用模板「${tpl.name}」`, variant: 'success' })
}

async function saveAsTemplate() {
  if (!taskCreatedId.value) {
    toast({
      description: '请先保存任务（保存为草稿或发布），再保存为模板',
      variant: 'warning',
    })
    return
  }
  if (!templateForm.value.name) {
    toast({ description: '请输入模板名称', variant: 'destructive' })
    return
  }
  submittingTpl.value = true
  try {
    await axios.post('/api/templates/from-task', {
      task_id: taskCreatedId.value,
      name: templateForm.value.name,
      description: templateForm.value.description || undefined,
      visibility: templateForm.value.visibility,
      course_id: courseId.value || undefined,
    })
    toast({ description: '模板保存成功', variant: 'success' })
    showSaveTemplateDialog.value = false
    await loadTemplates()
  } catch (e) {
    const msg = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    toast({ description: msg ?? '保存模板失败', variant: 'destructive' })
  } finally {
    submittingTpl.value = false
  }
}

async function saveDraft() { await doSubmit('draft') }
async function publish() { await doSubmit('published') }

async function doSubmit(status: 'draft' | 'published') {
  error.value = ''
  if (!name.value) { error.value = '请输入任务名称'; return }
  if (weightSum.value !== 100) { error.value = '维度权重和必须为 100%'; return }
  submitting.value = true
  try {
    const payload = {
      name: name.value,
      description: description.value,
      requirements: requirements.value,
      course_id: courseId.value,
      deadline: deadline.value || null,
      dimensions: dimensions.value.map((d, i) => ({
        name: d.name,
        description: d.description,
        weight: d.weight,
        order_index: i,
      })),
      class_ids: Array.from(selectedClassIds.value),
      template_id: selectedTemplateId.value ?? null,
    }
    let createdId: number
    if (editing.value) {
      await axios.patch(`/api/tasks/${editId.value}`, payload)
      createdId = editId.value
      // 维度同步
      await axios.put(`/api/tasks/${editId.value}/dimensions`, {
        dimensions: payload.dimensions,
      })
    } else {
      const { data } = await axios.post('/api/tasks', payload)
      createdId = data.id
    }
    taskCreatedId.value = createdId
    if (status === 'published') {
      await axios.post(`/api/tasks/${createdId}/publish`)
    }
    toast({
      description: status === 'published' ? '任务已发布' : '草稿已保存',
      variant: 'success',
    })
    router.push('/teacher/tasks')
  } catch (err) {
    const msg = (err as { response?: { data?: { detail?: string; message?: string } } })?.response?.data?.detail
    error.value = msg ?? '保存失败'
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <AppShell>
    <BreadcrumbNav :items="breadcrumbs" />

    <div class="flex justify-between items-end">
      <div>
        <h1 class="text-2xl font-bold text-ink">{{ editing ? '编辑实训任务' : '创建实训任务' }}</h1>
        <p class="mt-1.5 text-sm text-muted-foreground">
          配置任务信息与多维度评价指标，发布后将通知关联班级所有学生
        </p>
      </div>
      <div class="flex gap-3 items-center">
        <Button variant="outline" :disabled="submitting" @click="saveDraft">保存为草稿</Button>
        <Button :disabled="submitting" @click="publish">{{ editing ? '保存并发布' : '发布任务' }}</Button>
      </div>
    </div>

    <p v-if="error" class="text-xs text-danger anim-in">{{ error }}</p>

    <div v-if="loadingTask" class="space-y-3">
      <Skeleton class="h-32" />
      <Skeleton class="h-64" />
    </div>

    <div v-else class="tes-grid-main-aside">
      <!-- LEFT -->
      <div class="flex flex-col gap-5">
        <Card class="tes-card-container overflow-hidden">
          <header class="flex items-center gap-2.5 px-6 py-4 border-b border-border">
            <span class="w-[22px] h-[22px] rounded-full bg-primary-soft text-primary grid place-items-center text-[11px] font-semibold">1</span>
            <span class="text-[15px] font-semibold text-ink">基本信息</span>
          </header>
          <div class="p-6 flex flex-col gap-4">
            <div class="space-y-2">
              <div class="flex justify-between items-center">
                <Label>任务名称<span class="text-danger ml-0.5">*</span></Label>
                <span class="text-[11px] text-subtle-foreground">{{ nameLength }} / 100</span>
              </div>
              <Input v-model="name" maxlength="100" placeholder="如：软件工程实践 · 第三次实训" />
              <span class="text-[11px] text-muted-foreground">1-100 字符</span>
            </div>

            <div class="grid grid-cols-[repeat(auto-fit,minmax(min(100%,14rem),1fr))] gap-4">
              <div class="space-y-2">
                <Label>所属课程<span class="text-danger ml-0.5">*</span></Label>
                <Select v-model="courseId">
                  <SelectTrigger><SelectValue placeholder="选择课程" /></SelectTrigger>
                  <SelectContent>
                    <SelectItem v-for="c in allCourses" :key="c.id" :value="c.id">
                      {{ c.name }}（{{ c.code }}）
                    </SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div class="space-y-2">
                <Label>截止时间<span class="text-danger ml-0.5">*</span></Label>
                <div class="flex items-center gap-2 h-9 px-3 border border-border-strong rounded-md bg-surface focus-within:border-primary">
                  <CalendarClock class="w-3.5 h-3.5 text-muted-foreground shrink-0" />
                  <input v-model="deadline" type="datetime-local" class="border-0 outline-none bg-transparent flex-1 text-sm" />
                </div>
              </div>
            </div>

            <div class="space-y-2">
              <div class="flex justify-between items-center">
                <Label>发布班级<span class="text-danger ml-0.5">*</span></Label>
                <button class="text-xs text-primary font-medium hover:underline" @click="showClassPicker = true">
                  + 添加更多班级
                </button>
              </div>
              <div class="flex items-center gap-2 flex-wrap min-h-[40px] px-2.5 py-2 border border-border-strong rounded-md bg-surface">
                <Badge
                  v-for="cls in selectedClasses"
                  :key="cls.id"
                  variant="default"
                  class="gap-1.5 px-2 py-1"
                >
                  {{ cls.name }} · {{ cls.student_count }} 人
                  <button class="opacity-60 hover:opacity-100" @click="toggleClass(cls.id, false)">
                    <X class="w-3 h-3" />
                  </button>
                </Badge>
                <span v-if="selectedClasses.length === 0" class="text-xs text-muted-foreground">尚未选择班级</span>
              </div>
            </div>

            <div class="space-y-2">
              <Label>任务描述</Label>
              <Textarea v-model="description" rows="3" placeholder="向学生说明任务背景与目标" />
            </div>

            <div class="space-y-2">
              <div class="flex justify-between items-center">
                <Label>实训要求<span class="text-danger ml-0.5">*</span></Label>
                <span class="text-[11px] text-subtle-foreground">{{ requirementsLength }} / 5000</span>
              </div>
              <Textarea v-model="requirements" rows="6" maxlength="5000" placeholder="按步骤列出学生应完成的内容" />
            </div>
          </div>
        </Card>

        <Card class="tes-card-container overflow-hidden">
          <header class="flex items-center gap-2.5 px-6 py-4 border-b border-border">
            <span class="w-[22px] h-[22px] rounded-full bg-primary-soft text-primary grid place-items-center text-[11px] font-semibold">2</span>
            <span class="text-[15px] font-semibold text-ink">评价指标</span>
            <Badge :variant="weightSum === 100 ? 'success' : 'destructive'" class="ml-2">
              权重总和 {{ weightSum }}%
            </Badge>
            <div class="ml-auto flex gap-2">
              <Button variant="ghost" size="sm" @click="showTemplatePicker = true">
                <Bookmark class="w-3.5 h-3.5" />
                从模板加载
              </Button>
              <Button variant="ghost" size="sm" :disabled="!taskCreatedId" @click="showSaveTemplateDialog = true">
                <Save class="w-3.5 h-3.5" />
                保存为模板
              </Button>
            </div>
          </header>

          <div class="tes-table-shell">
          <div class="grid min-w-[820px] grid-cols-[60px_240px_minmax(18rem,1fr)_120px_80px] items-center px-5 py-3.5 bg-surface-2 border-b border-border text-[11px] font-semibold text-muted-foreground tracking-wider">
            <div></div>
            <div>指标名称</div>
            <div>评分依据</div>
            <div>权重</div>
            <div class="text-right">操作</div>
          </div>

          <div
            v-for="(d, i) in dimensions"
            :key="i"
            class="grid min-w-[820px] grid-cols-[60px_240px_minmax(18rem,1fr)_120px_80px] items-center px-5 py-3.5 border-b border-border last:border-b-0 transition-colors"
            :class="dragIndex === i ? 'bg-primary-soft/40' : ''"
            draggable="true"
            @dragstart="onDragStart(i, $event)"
            @dragover="onDragOver"
            @drop="onDrop(i)"
          >
            <span class="w-6 h-6 grid place-items-center text-subtle-foreground cursor-grab active:cursor-grabbing">
              <GripVertical class="w-4 h-4" />
            </span>
            <Input v-model="d.name" placeholder="维度名称" class="border-0 shadow-none bg-transparent px-2 font-semibold text-ink" />
            <Input v-model="d.description" placeholder="评分依据说明" class="border-0 shadow-none bg-transparent px-2 text-xs text-muted-foreground" />
            <div class="flex items-center gap-1.5 w-20 h-8 px-2.5 border border-border-strong rounded-md bg-surface">
              <input
                v-model.number="d.weight"
                type="number"
                min="1"
                max="100"
                class="border-0 outline-none bg-transparent w-[30px] text-sm text-right"
              />
              <span class="text-xs text-muted-foreground">%</span>
            </div>
            <div class="text-right">
              <Button variant="ghost" size="icon-sm" @click="removeDimension(i)">
                <Trash2 class="w-3.5 h-3.5" />
              </Button>
            </div>
          </div>
          </div>

          <button
            class="w-full px-5 py-3.5 text-center text-sm font-medium text-primary bg-surface-2 hover:bg-primary-soft transition-colors"
            @click="addDimension"
          >
            <Plus class="inline w-4 h-4 mr-1" />
            添加新评价维度（最多 10 项）
          </button>
        </Card>
      </div>

      <!-- RIGHT -->
      <div class="flex flex-col gap-5">
        <Card class="overflow-hidden">
          <header class="flex items-center justify-between px-5 py-4 border-b border-border">
            <span class="text-[15px] font-semibold text-ink">评价模板</span>
            <RouterLink to="/templates" class="text-xs text-primary font-medium">管理 ›</RouterLink>
          </header>
          <div v-if="templates.length === 0" class="px-5 py-6 text-center text-xs text-muted-foreground">
            暂无可用模板
          </div>
          <div v-else class="max-h-[280px] overflow-auto">
            <button
              v-for="tpl in templates"
              :key="tpl.id"
              class="w-full flex gap-3 items-start px-5 py-3.5 border-b border-border last:border-b-0 cursor-pointer hover:bg-surface-2 transition-colors text-left"
              :class="{ 'bg-primary-soft': selectedTemplateId === tpl.id }"
              @click="applyTemplateLocal(tpl)"
            >
              <span
                class="w-4 h-4 rounded-full border-2 shrink-0 mt-0.5 transition-colors"
                :class="selectedTemplateId === tpl.id ? 'border-primary bg-primary' : 'border-border-strong'"
              ></span>
              <div class="flex-1 min-w-0">
                <div class="text-sm font-semibold text-ink truncate">
                  {{ tpl.name }}
                  <Badge v-if="tpl.is_system" variant="gold" class="ml-1.5 text-[10px]">系统</Badge>
                </div>
                <div class="text-[11px] text-muted-foreground mt-1">
                  {{ tpl.dimensions.length }} 个维度
                  {{ tpl.description ? '· ' + tpl.description : '' }}
                </div>
              </div>
            </button>
          </div>
        </Card>

        <Card class="p-6 flex flex-col gap-3.5">
          <div>
            <div class="text-sm font-semibold text-ink">客观与主观比例</div>
            <div class="text-xs text-muted-foreground mt-1">系统将按以下比例计算综合得分</div>
          </div>
          <div class="h-9 rounded-md overflow-hidden flex">
            <div class="flex-[0_0_60%] flex items-center justify-center bg-primary text-primary-foreground text-xs font-semibold">AI 客观 60%</div>
            <div class="flex-[0_0_40%] flex items-center justify-center bg-accent text-accent-foreground text-xs font-semibold">教师主观 40%</div>
          </div>
        </Card>

        <div class="flex flex-col gap-2.5 bg-accent-soft border border-accent rounded-lg p-4">
          <div class="flex items-center gap-2 text-accent-strong text-sm font-semibold">
            <Info class="w-4 h-4" />
            <span>发布前请确认以下事项</span>
          </div>
          <div class="text-xs text-accent-strong leading-relaxed">
            <p>• 任务名称、评价指标在发布后将无法修改</p>
            <p>• 截止时间必须晚于当前时间</p>
            <p>• 系统将自动通知关联班级所有学生</p>
          </div>
        </div>
      </div>
    </div>

    <!-- Class picker dialog -->
    <Dialog v-model:open="showClassPicker">
      <DialogContent class="max-w-md">
        <DialogHeader>
          <DialogTitle>选择班级</DialogTitle>
          <DialogDescription>勾选要发布到的班级</DialogDescription>
        </DialogHeader>
        <Input v-model="classPickerSearch" placeholder="搜索班级名" class="mb-2" />
        <ScrollArea class="h-72 border border-border rounded-md">
          <ul>
            <li
              v-for="cls in filteredClassesForPicker"
              :key="cls.id"
              class="flex items-center gap-3 px-4 py-3 border-b border-border last:border-b-0 cursor-pointer hover:bg-surface-2"
              @click="toggleClass(cls.id, !selectedClassIds.has(cls.id))"
            >
              <Checkbox
                :model-value="selectedClassIds.has(cls.id)"
                @update:model-value="(v) => toggleClass(cls.id, v === true)"
              />
              <div class="flex-1">
                <span class="text-sm font-semibold text-ink">{{ cls.name }}</span>
                <div class="text-[11px] text-muted-foreground">{{ courseNameOf(cls.course_id) }} · {{ cls.student_count }} 人</div>
              </div>
            </li>
            <li v-if="filteredClassesForPicker.length === 0" class="px-4 py-8 text-center text-xs text-muted-foreground">
              无匹配班级
            </li>
          </ul>
        </ScrollArea>
        <DialogFooter class="gap-2">
          <Button variant="outline" @click="showClassPicker = false">完成</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- Template picker dialog -->
    <Dialog v-model:open="showTemplatePicker">
      <DialogContent class="max-w-md">
        <DialogHeader>
          <DialogTitle>从模板加载维度</DialogTitle>
          <DialogDescription>选择后将覆盖当前指标列表</DialogDescription>
        </DialogHeader>
        <ScrollArea class="h-72 border border-border rounded-md">
          <ul>
            <li
              v-for="tpl in templates"
              :key="tpl.id"
              class="px-4 py-3 border-b border-border last:border-b-0 cursor-pointer hover:bg-surface-2"
              @click="applyTemplateLocal(tpl)"
            >
              <div class="text-sm font-semibold text-ink">{{ tpl.name }}</div>
              <div class="text-[11px] text-muted-foreground mt-0.5">{{ tpl.dimensions.length }} 个维度</div>
            </li>
            <li v-if="templates.length === 0" class="px-4 py-8 text-center text-xs text-muted-foreground">
              暂无模板
            </li>
          </ul>
        </ScrollArea>
      </DialogContent>
    </Dialog>

    <!-- Save as template dialog -->
    <Dialog v-model:open="showSaveTemplateDialog">
      <DialogContent class="max-w-md">
        <DialogHeader>
          <DialogTitle>保存为模板</DialogTitle>
          <DialogDescription>将当前任务的维度设置保存为可复用模板</DialogDescription>
        </DialogHeader>
        <div class="flex flex-col gap-3">
          <div class="space-y-2">
            <Label>模板名称<span class="text-danger ml-0.5">*</span></Label>
            <Input v-model="templateForm.name" placeholder="如 软件工程通用评价" />
          </div>
          <div class="space-y-2">
            <Label>说明</Label>
            <Textarea v-model="templateForm.description" rows="2" placeholder="可选" />
          </div>
          <div class="space-y-2">
            <Label>可见性</Label>
            <Select v-model="templateForm.visibility">
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="private">私有（仅自己）</SelectItem>
                <SelectItem value="shared">共享（所在课程教师）</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>
        <DialogFooter class="gap-2">
          <Button variant="outline" @click="showSaveTemplateDialog = false">取消</Button>
          <Button :disabled="submittingTpl" @click="saveAsTemplate">
            {{ submittingTpl ? '保存中...' : '保存模板' }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </AppShell>
</template>
