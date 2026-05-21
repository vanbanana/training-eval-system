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
import { Textarea } from '@/components/ui/textarea'
import { Card } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Badge } from '@/components/ui/badge'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
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
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  Search,
  Code2,
  FileText,
  Users,
  Zap,
  Network,
  Plus,
  Download,
  Upload,
  Trash2,
  Copy,
  MoreHorizontal,
  CheckCircle2,
} from 'lucide-vue-next'

interface TemplateDimension {
  name: string
  weight: number
  description?: string
}

interface Template {
  id: number
  name: string
  description: string
  visibility: 'system' | 'shared' | 'private'
  dimensions: TemplateDimension[]
  usage_count?: number
  created_at?: string
  creator_name?: string
}

const router = useRouter()
const { toast } = useToast()
const templates = ref<Template[]>([])
const loading = ref(true)
const activeTab = ref<'all' | 'system' | 'mine' | 'shared'>('all')
const searchQuery = ref('')
const categoryFilter = ref('all')

// Create dialog
const showCreateDialog = ref(false)
const newTpl = ref({
  name: '',
  description: '',
  visibility: 'private' as 'private' | 'shared',
  dimensions: [
    { name: '', description: '', weight: 50 },
    { name: '', description: '', weight: 50 },
  ] as TemplateDimension[],
})
const submittingCreate = ref(false)

const breadcrumbs = [
  { label: '工作台', to: '/dashboard' },
  { label: '教学资源' },
  { label: '评价模板' },
]

const filteredTemplates = computed(() => {
  let list = templates.value
  if (activeTab.value === 'system') list = list.filter((t) => t.visibility === 'system')
  else if (activeTab.value === 'mine') list = list.filter((t) => t.visibility === 'private')
  else if (activeTab.value === 'shared') list = list.filter((t) => t.visibility === 'shared')

  if (searchQuery.value) {
    const q = searchQuery.value.toLowerCase()
    list = list.filter(
      (t) =>
        t.name.toLowerCase().includes(q) ||
        t.dimensions.some((d) => d.name.toLowerCase().includes(q)),
    )
  }
  if (categoryFilter.value !== 'all') {
    if (categoryFilter.value === 'code') {
      list = list.filter((t) => /代码|编程|测试/.test(t.name + (t.description ?? '')))
    } else if (categoryFilter.value === 'doc') {
      list = list.filter((t) => /文档|报告|论文/.test(t.name + (t.description ?? '')))
    }
  }
  return list
})

const tabCounts = computed(() => ({
  all: templates.value.length,
  system: templates.value.filter((t) => t.visibility === 'system').length,
  mine: templates.value.filter((t) => t.visibility === 'private').length,
  shared: templates.value.filter((t) => t.visibility === 'shared').length,
}))

const newTplWeightSum = computed(() => newTpl.value.dimensions.reduce((s, d) => s + (d.weight || 0), 0))

async function fetchTemplates() {
  loading.value = true
  try {
    const { data } = await axios.get('/api/templates')
    templates.value = data
  } catch {
    toast({ description: '加载模板失败', variant: 'destructive' })
  } finally {
    loading.value = false
  }
}

onMounted(fetchTemplates)

function getIconComponent(index: number) {
  const icons = [Code2, FileText, Users, Zap, Network]
  return icons[index % icons.length]
}

function getIconStyle(index: number) {
  const styles = [
    'bg-info-soft text-info',
    'bg-gold-soft text-gold',
    'bg-success-soft text-success',
    'bg-primary-soft text-primary',
    'bg-accent-soft text-accent',
  ]
  return styles[index % styles.length]
}

function getVisibilityVariant(visibility: string) {
  if (visibility === 'system') return 'info' as const
  if (visibility === 'shared') return 'success' as const
  return 'warning' as const
}
function getVisibilityLabel(visibility: string) {
  if (visibility === 'system') return '系统预置'
  if (visibility === 'shared') return '团队共享'
  return '我创建的'
}

function openCreateDialog() {
  newTpl.value = {
    name: '',
    description: '',
    visibility: 'private',
    dimensions: [
      { name: '', description: '', weight: 50 },
      { name: '', description: '', weight: 50 },
    ],
  }
  showCreateDialog.value = true
}

function addDimension() {
  if (newTpl.value.dimensions.length >= 10) return
  newTpl.value.dimensions.push({ name: '', description: '', weight: 0 })
}
function removeDimension(i: number) {
  if (newTpl.value.dimensions.length <= 2) return
  newTpl.value.dimensions.splice(i, 1)
}

async function submitCreate() {
  if (!newTpl.value.name) {
    toast({ description: '请输入模板名称', variant: 'destructive' })
    return
  }
  if (newTplWeightSum.value !== 100) {
    toast({ description: '维度权重和必须为 100%', variant: 'destructive' })
    return
  }
  if (newTpl.value.dimensions.some((d) => !d.name)) {
    toast({ description: '请填写每个维度名称', variant: 'destructive' })
    return
  }
  submittingCreate.value = true
  try {
    await axios.post('/api/templates', {
      name: newTpl.value.name,
      description: newTpl.value.description || undefined,
      visibility: newTpl.value.visibility,
      dimensions: newTpl.value.dimensions,
    })
    toast({ description: '模板创建成功', variant: 'success' })
    showCreateDialog.value = false
    await fetchTemplates()
  } catch (e) {
    const msg = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    toast({ description: msg ?? '创建失败', variant: 'destructive' })
  } finally {
    submittingCreate.value = false
  }
}

function useTemplate(t: Template) {
  // 将模板应用到新任务表单（query 携带 template_id）
  router.push({ path: '/teacher/tasks/new', query: { template_id: t.id } })
}

function copyTemplate(t: Template) {
  newTpl.value = {
    name: t.name + ' · 副本',
    description: t.description,
    visibility: 'private',
    dimensions: t.dimensions.map((d) => ({ ...d })),
  }
  showCreateDialog.value = true
}

async function deleteTemplate(t: Template) {
  if (t.visibility === 'system') {
    toast({ description: '系统预置模板不可删除', variant: 'warning' })
    return
  }
  const ok = await confirm({
    title: '删除模板',
    description: `确定删除「${t.name}」？此操作不可撤销`,
    variant: 'destructive',
    confirmText: '删除',
  })
  if (!ok) return
  try {
    await axios.delete(`/api/templates/${t.id}`)
    toast({ description: '模板已删除', variant: 'success' })
    await fetchTemplates()
  } catch (e) {
    const msg = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    toast({ description: msg ?? '删除失败', variant: 'destructive' })
  }
}

function exportAll() {
  const json = JSON.stringify(templates.value, null, 2)
  const blob = new Blob([json], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `templates-${new Date().toISOString().slice(0, 10)}.json`
  a.click()
  URL.revokeObjectURL(url)
  toast({ description: '已导出 JSON', variant: 'success' })
}

const fileInput = ref<HTMLInputElement | null>(null)

function pickImportFile() {
  fileInput.value?.click()
}

async function onImportFile(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (!file) return
  try {
    const text = await file.text()
    const arr = JSON.parse(text) as Template[]
    if (!Array.isArray(arr)) throw new Error('JSON 必须是数组')
    let ok = 0
    let fail = 0
    for (const t of arr) {
      try {
        await axios.post('/api/templates', {
          name: t.name + ' · 导入',
          description: t.description,
          visibility: 'private',
          dimensions: t.dimensions,
        })
        ok++
      } catch {
        fail++
      }
    }
    toast({
      description: `导入完成：成功 ${ok}${fail > 0 ? `，失败 ${fail}` : ''}`,
      variant: fail === 0 ? 'success' : 'warning',
    })
    await fetchTemplates()
  } catch (err) {
    const msg = err instanceof Error ? err.message : '解析 JSON 失败'
    toast({ description: msg, variant: 'destructive' })
  } finally {
    if (e.target) (e.target as HTMLInputElement).value = ''
  }
}
</script>

<template>
  <AppShell>
    <BreadcrumbNav :items="breadcrumbs" />

    <div class="flex justify-between items-end">
      <div>
        <h1 class="text-2xl font-bold text-ink">评价模板库</h1>
        <p class="mt-1.5 text-sm text-muted-foreground">沉淀常用评价指标方案，创建任务时可一键复用</p>
      </div>
      <div class="flex gap-3 items-center">
        <input ref="fileInput" type="file" accept=".json" class="hidden" @change="onImportFile" />
        <DropdownMenu>
          <DropdownMenuTrigger as-child>
            <Button variant="outline">
              导入 / 导出
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem @select="exportAll">
              <Download class="text-muted-foreground" />
              导出全部模板 (JSON)
            </DropdownMenuItem>
            <DropdownMenuItem @select="pickImportFile">
              <Upload class="text-muted-foreground" />
              导入模板 JSON
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
        <Button @click="openCreateDialog">
          <Plus class="w-4 h-4" />
          新建模板
        </Button>
      </div>
    </div>

    <Card class="px-5 py-3.5 flex justify-between items-center gap-4">
      <Tabs v-model="activeTab">
        <TabsList>
          <TabsTrigger value="all">全部 {{ tabCounts.all }}</TabsTrigger>
          <TabsTrigger value="system">系统预置 {{ tabCounts.system }}</TabsTrigger>
          <TabsTrigger value="mine">我创建的 {{ tabCounts.mine }}</TabsTrigger>
          <TabsTrigger value="shared">团队共享 {{ tabCounts.shared }}</TabsTrigger>
        </TabsList>
      </Tabs>

      <div class="flex items-center gap-3">
        <div class="relative w-[260px]">
          <Search class="w-3.5 h-3.5 text-muted-foreground absolute left-3 top-1/2 -translate-y-1/2" />
          <Input v-model="searchQuery" placeholder="搜索模板名称 / 维度" class="pl-9" />
        </div>
        <Select v-model="categoryFilter">
          <SelectTrigger class="w-32"><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem value="all">全部分类</SelectItem>
            <SelectItem value="code">编程类</SelectItem>
            <SelectItem value="doc">文档类</SelectItem>
          </SelectContent>
        </Select>
      </div>
    </Card>

    <div v-if="loading" class="grid grid-cols-3 gap-[18px]">
      <Skeleton v-for="n in 6" :key="n" class="h-64" />
    </div>

    <EmptyState
      v-else-if="filteredTemplates.length === 0"
      title="暂无模板"
      description="点击下方「新建模板」沉淀你的第一个评价方案"
      action-label="新建模板"
      @action="openCreateDialog"
    />

    <div v-else class="grid grid-cols-3 gap-[18px]">
      <Card
        v-for="(t, idx) in filteredTemplates"
        :key="t.id"
        class="overflow-hidden hover:border-primary anim-in"
        :style="{ animationDelay: Math.min(idx * 30, 240) + 'ms' }"
      >
        <div class="px-[22px] pt-5 pb-4 flex flex-col gap-2.5">
          <div class="flex justify-between items-center">
            <div :class="['w-8 h-8 rounded-md grid place-items-center', getIconStyle(idx)]">
              <component :is="getIconComponent(idx)" class="w-4 h-4" />
            </div>
            <Badge :variant="getVisibilityVariant(t.visibility)">{{ getVisibilityLabel(t.visibility) }}</Badge>
          </div>
          <div class="text-[15px] font-bold text-ink line-clamp-1">{{ t.name }}</div>
          <div class="text-xs leading-relaxed text-muted-foreground line-clamp-2">{{ t.description || '暂无描述' }}</div>
        </div>
        <div class="bg-surface-2 px-[22px] py-4 border-t border-b border-border flex flex-col gap-2.5">
          <div
            v-for="d in t.dimensions"
            :key="d.name"
            class="flex justify-between items-center text-xs"
          >
            <span class="text-foreground truncate mr-2">{{ d.name }}</span>
            <span class="text-ink font-semibold font-mono shrink-0">{{ d.weight }}%</span>
          </div>
        </div>
        <div class="px-[22px] py-3.5 flex justify-between items-center">
          <span class="text-[11px] text-muted-foreground truncate">
            已被 {{ t.usage_count ?? 0 }} 个任务使用
          </span>
          <div class="flex items-center gap-1">
            <Button variant="ghost" size="sm" class="h-7 px-2 text-primary" @click="useTemplate(t)">
              <CheckCircle2 class="w-3 h-3" />
              使用
            </Button>
            <DropdownMenu>
              <DropdownMenuTrigger as-child>
                <Button variant="ghost" size="icon-sm">
                  <MoreHorizontal class="w-3.5 h-3.5" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuItem @select="copyTemplate(t)">
                  <Copy class="text-muted-foreground" />
                  复制
                </DropdownMenuItem>
                <DropdownMenuSeparator v-if="t.visibility !== 'system'" />
                <DropdownMenuItem
                  v-if="t.visibility !== 'system'"
                  class="text-danger focus:bg-danger-soft focus:text-danger"
                  @select="deleteTemplate(t)"
                >
                  <Trash2 class="text-current" />
                  删除
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </div>
      </Card>

      <button
        class="bg-surface-2 border border-dashed border-border-strong rounded-lg p-6 flex flex-col items-center justify-center gap-3.5 cursor-pointer min-h-[260px] hover:border-primary hover:bg-primary-soft transition-colors active:scale-[0.99]"
        @click="openCreateDialog"
      >
        <div class="w-12 h-12 bg-card border border-border rounded-full grid place-items-center text-primary">
          <Plus class="w-5 h-5" />
        </div>
        <div class="text-sm font-semibold text-ink">创建新模板</div>
        <div class="text-xs text-muted-foreground text-center">自定义评价维度与权重，沉淀你的教学经验</div>
      </button>
    </div>

    <!-- Create Dialog -->
    <Dialog v-model:open="showCreateDialog">
      <DialogContent class="max-w-xl max-h-[85vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>新建评价模板</DialogTitle>
          <DialogDescription>定义可复用的评价维度方案</DialogDescription>
        </DialogHeader>
        <div class="flex flex-col gap-4">
          <div class="space-y-2">
            <Label>模板名称<span class="text-danger ml-0.5">*</span></Label>
            <Input v-model="newTpl.name" placeholder="如 软件工程通用评价" />
          </div>
          <div class="space-y-2">
            <Label>说明</Label>
            <Textarea v-model="newTpl.description" rows="2" placeholder="简要介绍此模板的适用场景" />
          </div>
          <div class="space-y-2">
            <Label>可见性</Label>
            <Select v-model="newTpl.visibility">
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="private">私有（仅自己）</SelectItem>
                <SelectItem value="shared">共享（团队成员）</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div class="space-y-2">
            <div class="flex justify-between items-center">
              <Label>评价维度 ({{ newTpl.dimensions.length }})</Label>
              <Badge :variant="newTplWeightSum === 100 ? 'success' : 'destructive'">
                权重和 {{ newTplWeightSum }}%
              </Badge>
            </div>
            <div
              v-for="(d, i) in newTpl.dimensions"
              :key="i"
              class="flex items-center gap-2 p-2 bg-surface-2 rounded-md"
            >
              <Input v-model="d.name" placeholder="维度名称" class="flex-1" />
              <Input
                v-model.number="d.weight"
                type="number"
                min="1"
                max="100"
                class="w-20"
                placeholder="%"
              />
              <Button variant="ghost" size="icon-sm" :disabled="newTpl.dimensions.length <= 2" @click="removeDimension(i)">
                <Trash2 class="w-3 h-3" />
              </Button>
            </div>
            <Button variant="outline" size="sm" :disabled="newTpl.dimensions.length >= 10" @click="addDimension">
              <Plus class="w-3 h-3" />
              添加维度
            </Button>
          </div>
        </div>
        <DialogFooter class="gap-2">
          <Button variant="outline" @click="showCreateDialog = false">取消</Button>
          <Button :disabled="submittingCreate" @click="submitCreate">
            {{ submittingCreate ? '创建中...' : '确认创建' }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </AppShell>
</template>
