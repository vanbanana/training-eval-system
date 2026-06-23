<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import axios from 'axios'
import AppShell from '@/components/layout/AppShell.vue'
import BreadcrumbNav from '@/components/business/BreadcrumbNav.vue'
import FileUploader from '@/components/business/FileUploader.vue'
import ParsePipelinePanel from '@/components/business/ParsePipelinePanel.vue'
import EvaluationProgressPanel from '@/components/business/EvaluationProgressPanel.vue'
import { useToast } from '@/components/ui/toast'
import { useCourseMap } from '@/composables/useCourseMap'
import { useParseProgress } from '@/composables/useParseProgress'
import { safeGet } from '@/lib/api-helpers'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Badge } from '@/components/ui/badge'
import {
  FileText,
  Info,
  ListChecks,
  PieChart,
  History as HistoryIcon,
  ShieldAlert,
  Archive,
  Image as ImageIcon,
  Check,
  Loader2,
  Sparkles,
} from 'lucide-vue-next'

interface Dimension {
  id: number
  name: string
  weight: number
  order_index: number
}
interface Task {
  id: number
  name: string
  description: string
  requirements: string
  status: string
  deadline: string | null
  course_id: number
  teacher_id: number
  dimensions: Dimension[]
}
interface Upload {
  id: number
  filename: string
  file_type: string
  file_size: number
  version: number
  parse_status: string
  created_at: string
}
interface EvalDimScore {
  dimension_id: number
  ai_score: number | null
  weight: number
}
interface Evaluation {
  id: number
  task_id: number
  total_score: number | null
  status: string
  created_at: string
  scores?: EvalDimScore[]
  /** AI 评分失败、已转人工评阅（status 仍为 pending） */
  ai_failed?: boolean
  overall_comment?: string
}
interface VerifyResult {
  is_valid: boolean
  required_sections?: string[]
  missing_sections?: string[]
  warnings?: string[]
  message?: string
}

const route = useRoute()
useRouter() // keep router available for future navigation
const taskId = computed(() => route.params.id as string)
const { toast } = useToast()
const { load: loadCourseMap, courseName } = useCourseMap()

const task = ref<Task | null>(null)
const uploads = ref<Upload[]>([])
const evaluation = ref<Evaluation | null>(null)
const verifyResults = ref<Record<number, VerifyResult | null>>({})
const loading = ref(true)
const triggering = ref<number | null>(null)
const taskDescExpanded = ref(false)

// SSE 实时进度接入
const uploadIds = computed(() => uploads.value.map(u => u.id))
const { getProgress: sseGetProgress, messages: sseMessages } = useParseProgress(uploadIds)

// Per-dimension evaluation progress from SSE
interface DimensionProgress {
  id: number
  name: string
  weight: number
  status: 'evaluating' | 'done' | 'failed' | 'idle'
  score?: number
}
const evalDimensions = ref<DimensionProgress[]>([])

// 当 SSE 推送解析/评分完成事件时自动刷新数据
watch(
  () => sseMessages.value.length,
  () => {
    const last = sseMessages.value[sseMessages.value.length - 1] as unknown as Record<string, unknown> | undefined
    if (!last) return

    // Handle eval_dimensions events for per-dimension progress
    if (last.stage === 'eval_dimensions' && Array.isArray(last.dimensions)) {
      evalDimensions.value = (last.dimensions as DimensionProgress[]).map(d => ({
        id: d.id,
        name: d.name,
        weight: d.weight,
        status: d.status,
        score: d.score,
      }))
      // Clear triggering state when all dimensions are done
      const allDone = evalDimensions.value.every(d => d.status === 'done' || d.status === 'failed')
      if (allDone && triggering.value) {
        triggering.value = null
      }
    }

    if (last.status === 'parsed' || last.status === 'failed' || last.status === 'scored') {
      setTimeout(() => fetchAll(), 500)
    }
  },
)

const now = ref(Date.now())
let timerId: number | null = null

async function fetchAll() {
  loading.value = true
  try {
    const [t, u] = await Promise.all([
      axios.get(`/api/tasks/${taskId.value}`),
      axios.get(`/api/uploads/${taskId.value}`),
    ])
    task.value = t.data
    uploads.value = u.data
    const evsResult = await safeGet<Evaluation[]>('/api/evaluations/my', [])
    if (evsResult.error && evsResult.status !== 404) {
      toast({
        description: `历史评价 ${evsResult.error}`,
        variant: 'warning',
      })
    }
    const myEvs = evsResult.data.filter(
      (e) => e.task_id === Number(taskId.value),
    )
    const latestEv = myEvs.sort((a, b) => b.id - a.id)[0] ?? null
    // Fetch full evaluation detail (with per-dimension scores) when scored or
    // when AI failed (so we can show the failure reason and hand-off notice).
    if (latestEv && (['scored', 'confirmed', 'rejected'].includes(latestEv.status) || latestEv.ai_failed)) {
      try {
        const fullEv = await axios.get(`/api/evaluations/${latestEv.id}`)
        evaluation.value = fullEv.data
      } catch {
        evaluation.value = latestEv
      }
    } else {
      evaluation.value = latestEv
    }

    const cur = uploads.value[0]
    if (cur) {
      const vr = await safeGet<VerifyResult>(
        `/api/uploads/${cur.id}/verify-result`,
        null as unknown as VerifyResult,
        undefined,
        { silent404: true },
      )
      verifyResults.value[cur.id] = vr.error ? null : vr.data
    }
  } catch (e) {
    const msg = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    toast({ description: msg ?? '加载任务失败', variant: 'destructive' })
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  void loadCourseMap()
  fetchAll()
  timerId = window.setInterval(() => (now.value = Date.now()), 30_000)
})
onUnmounted(() => {
  if (timerId) clearInterval(timerId)
})

const countdown = computed(() => {
  if (!task.value?.deadline) return null
  const diff = new Date(task.value.deadline).getTime() - now.value
  if (diff <= 0) return { days: 0, hours: 0, minutes: 0, expired: true }
  const days = Math.floor(diff / 86400000)
  const hours = Math.floor((diff % 86400000) / 3600000)
  const minutes = Math.floor((diff % 3600000) / 60000)
  return { days, hours, minutes, expired: false }
})

const isExpired = computed(() => countdown.value?.expired === true)
const canUpload = computed(() => task.value?.status === 'published' && !isExpired.value)

const deadlineLabel = computed(() => {
  if (!task.value?.deadline) return '——'
  return task.value.deadline.slice(0, 16).replace('T', ' ')
})

async function triggerEval(uploadId: number) {
  triggering.value = uploadId
  try {
    await axios.post(`/api/evaluations/trigger/${uploadId}`)
    toast({ description: 'AI 评价已开始，请等待各维度评分完成', variant: 'info' })
  } catch (e) {
    const msg = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    if (msg === 'Evaluation already triggered for this upload') {
      toast({ description: '该文件已触发过评价，正在刷新结果', variant: 'warning' })
      fetchAll()
    } else {
      toast({ description: msg ?? '触发评价失败', variant: 'destructive' })
    }
    triggering.value = null
  }
}

async function retryParse() {
  if (!currentUpload.value) return
  try {
    await axios.post(`/api/uploads/${currentUpload.value.id}/retry`)
    toast({ description: '已重新提交解析', variant: 'success' })
    fetchAll()
  } catch (e) {
    const msg = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    toast({ description: msg ?? '重试失败', variant: 'destructive' })
  }
}

function fileIcon(ext: string) {
  const e = ext.toLowerCase()
  if (e === 'pdf') return { cmp: FileText, color: 'bg-danger-soft text-danger' }
  if (e === 'zip' || e === 'rar') return { cmp: Archive, color: 'bg-info-soft text-info' }
  if (['png', 'jpg', 'jpeg', 'gif'].includes(e)) return { cmp: ImageIcon, color: 'bg-success-soft text-success' }
  return { cmp: FileText, color: 'bg-muted text-muted-foreground' }
}

function formatSize(bytes: number) {
  if (!bytes) return '0 B'
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  return (bytes / 1024 / 1024).toFixed(1) + ' MB'
}

function formatTime(iso: string) {
  return iso.slice(0, 16).replace('T', ' ')
}

function parseStatusBadge(status: string) {
  return ({
    parsed: { label: '已完成', variant: 'success' as const },
    parsing: { label: '解析中', variant: 'info' as const },
    failed: { label: '失败', variant: 'destructive' as const },
  } as const)[status] ?? { label: '待解析', variant: 'secondary' as const }
}

function getUploadStatus(upload: Upload): string {
  const wsProgress = sseGetProgress(upload.id)
  if (wsProgress) return wsProgress.status
  return upload.parse_status
}

function getUploadProgress(upload: Upload): number | null {
  const wsProgress = sseGetProgress(upload.id)
  if (wsProgress && wsProgress.status === 'parsing') return wsProgress.progress
  return null
}

const currentUpload = computed(() => uploads.value[0] ?? null)
const previousUploads = computed(() => uploads.value.slice(1))
const verifyForCurrent = computed(() =>
  currentUpload.value ? verifyResults.value[currentUpload.value.id] : null,
)

// Determine if we should show the parse pipeline prominently
const showParsePipeline = computed(() => {
  if (!currentUpload.value) return false
  const status = getUploadStatus(currentUpload.value)
  return status === 'parsing' || status === 'parsed' || status === 'failed' || status === 'pending'
})

// Build idle dimension list from task dimensions for initial display
const taskDimensionsAsIdle = computed<DimensionProgress[]>(() => {
  if (!task.value?.dimensions) return []
  const scoreMap = new Map<number, number>()
  if (evaluation.value?.scores) {
    for (const s of evaluation.value.scores) {
      if (s.ai_score != null) scoreMap.set(s.dimension_id, s.ai_score)
    }
  }
  const done = evaluation.value?.status === 'scored' || evaluation.value?.status === 'confirmed' || evaluation.value?.status === 'rejected'
  return task.value.dimensions.map(d => ({
    id: d.id,
    name: d.name,
    weight: d.weight,
    status: done ? 'done' as const : evaluation.value?.ai_failed ? 'failed' as const : 'idle' as const,
    score: scoreMap.get(d.id),
  }))
})

// AI failed => surface a clear failed macro state (status stays pending in DB).
const evalStatusForPanel = computed(() =>
  evaluation.value?.ai_failed ? 'failed' : (evaluation.value?.status ?? null),
)

function onUploadSuccess() {
  fetchAll()
}
</script>

<template>
  <AppShell>
    <div v-if="loading" class="space-y-4">
      <Skeleton class="h-12" />
      <Skeleton class="h-64" />
    </div>

    <template v-else-if="task">
      <BreadcrumbNav
        :items="[
          { label: '我的任务', to: '/student/tasks' },
          { label: task.name },
        ]"
      />

      <!-- Page header -->
      <div class="flex justify-between items-end">
        <div>
          <div class="flex items-center gap-3">
            <h1 class="text-2xl font-bold text-ink">{{ task.name }}</h1>
            <Badge v-if="countdown && !countdown.expired && countdown.days <= 1" variant="destructive">
              今日截止
            </Badge>
            <Badge v-else-if="countdown && !countdown.expired && countdown.days <= 3" variant="accent">
              即将截止
            </Badge>
            <Badge v-else-if="countdown?.expired" variant="secondary">已截止</Badge>
            <Badge v-else variant="info">进行中</Badge>
          </div>
          <p class="mt-2 text-sm text-muted-foreground flex items-center gap-2.5">
            <span>{{ courseName(task.course_id) }} · {{ task.dimensions.length }} 个评分维度</span>
            <span class="w-1 h-1 rounded-full bg-subtle-foreground"></span>
            <span class="text-accent font-mono">截止 {{ deadlineLabel }}</span>
          </p>
        </div>
        <Button variant="outline" size="sm" @click="fetchAll">刷新</Button>
      </div>

      <!-- Main layout: follow page-30 design reference -->
      <div class="tes-grid-main-aside" style="gap: 18px;">
        <!-- LEFT COLUMN: Description → Submission → Eval Progress → History -->
        <div class="flex flex-col gap-[18px]">

          <!-- === Task Description (follows page-30 panel structure) === -->
          <Card class="overflow-hidden">
            <header class="px-6 py-[18px] border-b border-border flex justify-between items-center">
              <div class="flex items-center gap-2.5">
                <FileText class="w-4 h-4 text-primary" />
                <span class="text-sm font-semibold text-ink">任务说明</span>
              </div>
              <button
                class="text-xs font-medium text-primary cursor-pointer hover:underline"
                @click="taskDescExpanded = !taskDescExpanded"
              >
                {{ taskDescExpanded ? '收起' : '展开详情' }}
              </button>
            </header>
            <div class="p-6 space-y-[18px]">
              <!-- Always show overview -->
              <div>
                <div class="flex items-center gap-2 mb-2 text-[13px] font-semibold text-ink">
                  <Info class="w-3.5 h-3.5 text-muted-foreground" />
                  <span>任务概述</span>
                </div>
                <div class="text-[13px] leading-[1.85] text-foreground whitespace-pre-wrap">
                  {{ task.description || '暂无描述' }}
                </div>
              </div>
              <!-- Expandable sections -->
              <template v-if="taskDescExpanded">
                <div v-if="task.requirements">
                  <div class="flex items-center gap-2 mb-2 text-[13px] font-semibold text-ink">
                    <ListChecks class="w-3.5 h-3.5 text-muted-foreground" />
                    <span>实训要求</span>
                  </div>
                  <pre class="text-[13px] leading-[1.85] text-foreground whitespace-pre-wrap font-sans">{{ task.requirements }}</pre>
                </div>
              </template>
              <!-- Scoring indicators (always visible, follows page-30 ds-grid-4 pattern) -->
              <div>
                <div class="flex items-center gap-2 mb-2 text-[13px] font-semibold text-ink">
                  <PieChart class="w-3.5 h-3.5 text-muted-foreground" />
                  <span>评分指标</span>
                </div>
                <div class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-2.5">
                  <div
                    v-for="d in task.dimensions"
                    :key="d.id"
                    class="bg-surface-2 border border-border rounded-lg p-3 flex flex-col gap-1"
                  >
                    <span class="text-[11px] text-muted-foreground leading-tight">{{ d.name }}</span>
                    <span class="text-lg font-bold text-ink">{{ d.weight }}%</span>
                  </div>
                </div>
              </div>
            </div>
          </Card>

          <!-- === My Submission (follows page-30 sf-row pattern) === -->
          <Card class="overflow-hidden">
            <header class="px-6 py-[18px] border-b border-border flex justify-between items-center">
              <div class="flex items-center gap-2.5">
                <FileText class="w-4 h-4 text-primary" />
                <span class="text-sm font-semibold text-ink">我的提交</span>
                <Badge v-if="uploads.length" variant="info" class="text-[10px]">v{{ uploads.length }}</Badge>
              </div>
              <span v-if="canUpload" class="text-xs font-medium text-primary cursor-pointer">补充提交 · 替换文件</span>
            </header>

            <!-- Upload area -->
            <div :class="currentUpload ? 'px-6 py-3' : 'p-6'">
              <FileUploader
                :endpoint="`/api/uploads/${taskId}`"
                :accept="['.pdf', '.docx', '.doc', '.zip', '.png', '.jpg', '.jpeg']"
                :max-size-mb="50"
                :disabled="!canUpload"
                :disabled-hint="isExpired ? '任务已截止，无法上传' : '任务未发布'"
                :compact="!!currentUpload"
                @success="onUploadSuccess"
              />
            </div>

            <!-- File rows (page-30 sf-row style) -->
            <template v-if="currentUpload">
              <div class="border-t border-border">
                <div class="flex items-center gap-3.5 px-6 py-3.5">
                  <div :class="['w-9 h-9 rounded-lg grid place-items-center flex-shrink-0', fileIcon(currentUpload.file_type).color]">
                    <component :is="fileIcon(currentUpload.file_type).cmp" class="w-4 h-4" />
                  </div>
                  <div class="flex-1 min-w-0">
                    <div class="flex items-center gap-2.5">
                      <span class="text-[13px] font-semibold text-ink truncate">{{ currentUpload.filename }}</span>
                      <Badge :variant="parseStatusBadge(getUploadStatus(currentUpload)).variant" class="text-[9px]">
                        {{ parseStatusBadge(getUploadStatus(currentUpload)).label }}
                      </Badge>
                    </div>
                    <div class="text-[11px] text-muted-foreground mt-0.5">
                      {{ formatSize(currentUpload.file_size) }} · {{ formatTime(currentUpload.created_at) }}
                    </div>
                  </div>
                  <Button
                    size="sm"
                    :disabled="triggering === currentUpload.id || currentUpload.parse_status !== 'parsed'"
                    @click="triggerEval(currentUpload.id)"
                  >
                    <Loader2 v-if="triggering === currentUpload.id" class="w-3.5 h-3.5 animate-spin" />
                    <Sparkles v-else class="w-3.5 h-3.5" />
                    {{ triggering === currentUpload.id ? '评价中...' : '触发 AI 评价' }}
                  </Button>
                </div>
              </div>
            </template>
          </Card>

          <!-- === Parse Pipeline (only when actively parsing / just parsed) === -->
          <ParsePipelinePanel
            v-if="showParsePipeline && currentUpload && getUploadStatus(currentUpload) === 'parsing'"
            :parse-status="getUploadStatus(currentUpload)"
            :progress="getUploadProgress(currentUpload)"
            :uploaded-at="currentUpload.created_at"
            :filename="currentUpload.filename"
            :file-type="currentUpload.file_type"
            @retry="retryParse"
          />

          <!-- === Evaluation Progress (vertical per-dimension list) === -->
          <EvaluationProgressPanel
            v-if="currentUpload && (currentUpload.parse_status === 'parsed' || evaluation)"
            :parse-status="currentUpload?.parse_status ?? 'pending'"
            :eval-status="evalStatusForPanel"
            :failure-reason="evaluation?.overall_comment"
            :uploaded-at="currentUpload?.created_at"
            :dimensions="evalDimensions.length > 0 ? evalDimensions : taskDimensionsAsIdle"
          />

          <!-- === Verify result === -->
          <Card v-if="verifyForCurrent" class="overflow-hidden">
            <div class="px-6 py-4">
              <div class="flex items-center gap-2 mb-3">
                <ShieldAlert class="w-4 h-4 text-info" />
                <span class="text-sm font-semibold text-ink">合规核查结果</span>
                <Badge :variant="verifyForCurrent.is_valid ? 'success' : 'warning'">
                  {{ verifyForCurrent.is_valid ? '通过' : '需关注' }}
                </Badge>
              </div>
              <p v-if="verifyForCurrent.message" class="text-xs text-muted-foreground mb-2">
                {{ verifyForCurrent.message }}
              </p>
              <ul v-if="verifyForCurrent.missing_sections?.length" class="text-xs text-danger space-y-1">
                <li v-for="s in verifyForCurrent.missing_sections" :key="s">缺失：{{ s }}</li>
              </ul>
              <ul v-if="verifyForCurrent.warnings?.length" class="mt-2 text-xs text-warning space-y-1">
                <li v-for="w in verifyForCurrent.warnings" :key="w">提醒：{{ w }}</li>
              </ul>
            </div>
          </Card>

          <!-- === Submission History (page-30 timeline style) === -->
          <Card v-if="previousUploads.length > 0" class="overflow-hidden">
            <header class="px-6 py-[18px] border-b border-border">
              <div class="flex items-center gap-2.5">
                <HistoryIcon class="w-4 h-4 text-muted-foreground" />
                <span class="text-sm font-semibold text-ink">提交历史</span>
                <span class="text-[11px] text-muted-foreground">截止前可继续替换</span>
              </div>
            </header>
            <div class="px-2 py-2">
              <div
                v-for="(u, idx) in previousUploads"
                :key="u.id"
                class="flex gap-3.5 px-4 py-3 relative"
              >
                <!-- Timeline dot + line -->
                <div class="w-6 flex flex-col items-center flex-shrink-0 pt-0.5">
                  <div class="w-2.5 h-2.5 rounded-full border-2 border-border bg-card flex-shrink-0"></div>
                  <div v-if="idx < previousUploads.length - 1" class="w-px flex-1 bg-border mt-1"></div>
                </div>
                <div class="flex-1 min-w-0 flex items-center gap-3">
                  <div class="flex-1 min-w-0">
                    <div class="flex items-center gap-2.5">
                      <span class="text-sm font-semibold text-ink">v{{ u.version }}</span>
                      <Badge variant="secondary" class="text-[10px]">已替换</Badge>
                      <span class="text-[11px] text-muted-foreground font-mono">{{ formatTime(u.created_at) }}</span>
                    </div>
                    <div class="text-[11px] text-muted-foreground mt-0.5">
                      {{ u.filename }} · {{ formatSize(u.file_size) }}
                    </div>
                  </div>
                  <Button variant="ghost" size="sm" :disabled="triggering === u.id" @click="triggerEval(u.id)">
                    重新评价
                  </Button>
                </div>
              </div>
            </div>
          </Card>
        </div>

        <!-- RIGHT COLUMN (follows page-30 right col: countdown + progress + tip) -->
        <div class="flex flex-col gap-[18px]">
          <!-- Countdown (page-30 cd-card) -->
          <div v-if="countdown && !countdown.expired" class="bg-primary text-primary-foreground rounded-xl p-6 flex flex-col gap-3.5">
            <span class="text-[11px] font-semibold tracking-[1.5px] text-[#B8C5D6] uppercase">距离截止还剩</span>
            <div class="flex items-end gap-3.5">
              <div class="flex flex-col items-center gap-0.5">
                <span class="text-4xl font-bold leading-none">{{ String(countdown.days).padStart(2, '0') }}</span>
                <span class="text-[11px] text-[#B8C5D6]">天</span>
              </div>
              <span class="text-2xl font-bold pb-3">:</span>
              <div class="flex flex-col items-center gap-0.5">
                <span class="text-4xl font-bold leading-none">{{ String(countdown.hours).padStart(2, '0') }}</span>
                <span class="text-[11px] text-[#B8C5D6]">小时</span>
              </div>
              <span class="text-2xl font-bold pb-3">:</span>
              <div class="flex flex-col items-center gap-0.5">
                <span class="text-4xl font-bold leading-none">{{ String(countdown.minutes).padStart(2, '0') }}</span>
                <span class="text-[11px] text-[#B8C5D6]">分钟</span>
              </div>
            </div>
            <div class="h-px bg-[#22344F]"></div>
            <span class="font-mono text-[11px] text-[#B8C5D6]">截止时间 {{ deadlineLabel }}</span>
          </div>

          <Card v-else-if="countdown?.expired" class="p-6 text-center bg-muted">
            <span class="text-sm font-semibold text-muted-foreground">已截止</span>
            <p class="text-xs text-muted-foreground mt-1 font-mono">{{ deadlineLabel }}</p>
          </Card>

          <!-- Task status summary (compact) -->
          <Card class="overflow-hidden">
            <header class="flex justify-between items-center px-5 py-4 border-b border-border">
              <span class="text-sm font-semibold text-ink">任务状态</span>
              <span class="text-xs font-semibold text-primary">{{ uploads.length }} 次提交</span>
            </header>
            <div class="px-5 py-4 flex flex-col gap-2">
              <div class="flex justify-between items-center">
                <span class="text-xs text-muted-foreground">任务状态</span>
                <Badge :variant="task.status === 'published' ? 'info' : 'secondary'" class="text-[10px]">
                  {{ task.status === 'published' ? '进行中' : task.status }}
                </Badge>
              </div>
              <div class="flex justify-between items-center">
                <span class="text-xs text-muted-foreground">已提交版本</span>
                <span class="text-xs font-semibold text-ink">v{{ uploads.length || 0 }}</span>
              </div>
              <div class="flex justify-between items-center">
                <span class="text-xs text-muted-foreground">评分维度</span>
                <span class="text-xs font-semibold text-ink">{{ task.dimensions.length }} 项</span>
              </div>
              <div v-if="evaluation" class="flex justify-between items-center pt-2 border-t border-border">
                <span class="text-xs text-muted-foreground">综合得分</span>
                <span class="text-base font-bold text-success">{{ evaluation.total_score ?? '—' }}</span>
              </div>
            </div>
          </Card>

          <!-- Submission checklist -->
          <Card v-if="uploads.length > 0" class="overflow-hidden">
            <header class="px-5 py-4 border-b border-border">
              <span class="text-sm font-semibold text-ink">提交清单</span>
            </header>
            <div class="px-5 py-3.5">
              <ul class="flex flex-col gap-2">
                <li
                  v-for="u in uploads.slice(0, 5)"
                  :key="u.id"
                  class="flex items-center gap-2.5 text-xs text-foreground leading-relaxed"
                >
                  <span class="w-4 h-4 rounded-full bg-success-soft text-success grid place-items-center flex-shrink-0">
                    <Check class="w-2.5 h-2.5" />
                  </span>
                  <span class="flex-1 truncate">{{ u.filename }}</span>
                  <span class="text-[10px] text-muted-foreground font-mono">v{{ u.version }}</span>
                </li>
              </ul>
            </div>
          </Card>

          <!-- Tip box (page-30 tip-box) -->
          <div class="bg-accent-soft border border-accent rounded-xl p-[18px] flex flex-col gap-2">
            <div class="flex items-center gap-2 text-accent-strong text-xs font-semibold">
              <ShieldAlert class="w-4 h-4" />
              <span>提交建议</span>
            </div>
            <div class="text-[11px] text-accent-strong leading-[1.7]">
              截止前 24 小时系统会自动提醒。系统将自动检测相似度，相似度过高会标记为疑似抄袭并通知教师人工复核。建议预留缓冲时间，请独立完成。
            </div>
          </div>
        </div>
      </div>
    </template>
  </AppShell>
</template>


