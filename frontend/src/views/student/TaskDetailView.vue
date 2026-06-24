<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import axios from 'axios'
import AppShell from '@/components/layout/AppShell.vue'
import BreadcrumbNav from '@/components/business/BreadcrumbNav.vue'
import FileUploader from '@/components/business/FileUploader.vue'
import EvaluationProgressPanel from '@/components/business/EvaluationProgressPanel.vue'
import { useToast } from '@/components/ui/toast'
import { useCourseMap } from '@/composables/useCourseMap'
import { useParseProgress } from '@/composables/useParseProgress'
import { safeGet } from '@/lib/api-helpers'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
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
interface Evaluation {
  id: number
  task_id: number
  total_score: number | null
  status: string
  created_at: string
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
const router = useRouter()
const taskId = computed(() => route.params.id as string)
const { toast } = useToast()
const { load: loadCourseMap, courseName } = useCourseMap()

const task = ref<Task | null>(null)
const uploads = ref<Upload[]>([])
const evaluation = ref<Evaluation | null>(null)
const verifyResults = ref<Record<number, VerifyResult | null>>({})
const loading = ref(true)
const triggering = ref<number | null>(null)

// SSE 实时进度接入
const uploadIds = computed(() => uploads.value.map(u => u.id))
const { getProgress: sseGetProgress, messages: sseMessages } = useParseProgress(uploadIds)

// 当 SSE 推送解析/评分完成事件时自动刷新数据
watch(
  () => sseMessages.value.length,
  () => {
    const last = sseMessages.value[sseMessages.value.length - 1] as { status?: string } | undefined
    if (last && (last.status === 'parsed' || last.status === 'failed' || last.status === 'scored')) {
      // 延迟 500ms 刷新，让后端有时间写入 DB
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
      axios.get(`/api/uploads/by-task/${taskId.value}`),
    ])
    task.value = t.data
    uploads.value = u.data
    // 评价记录可降级（学生未提交时无评价是正常的）
    const evsResult = await safeGet<Evaluation[]>('/api/evaluations/my', [])
    if (evsResult.error && evsResult.status !== 404) {
      // 仅在非 404 时通知用户
      toast({
        description: `历史评价 ${evsResult.error}`,
        variant: 'warning',
      })
    }
    const myEvs = evsResult.data.filter(
      (e) => e.task_id === Number(taskId.value),
    )
    evaluation.value = myEvs.sort((a, b) => b.id - a.id)[0] ?? null

    // verify-result for current upload
    const cur = uploads.value[0]
    if (cur) {
      const vr = await safeGet<VerifyResult>(
        `/api/uploads/${cur.id}/verify-result`,
        null as unknown as VerifyResult,
        undefined,
        { silent404: true },
      )
      // 404 = 该上传不需要核查/未生成；其他错误仅记录到控制台
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

// AI 评分失败时 status 仍为 pending，需显式映射为 failed 让进度面板呈现失败态。
const evalStatusForPanel = computed(() =>
  evaluation.value?.ai_failed ? 'failed' : (evaluation.value?.status ?? null),
)

async function triggerEval(uploadId: number) {
  triggering.value = uploadId
  try {
    const { data } = await axios.post(`/api/evaluations/trigger/${uploadId}`)
    toast({ description: `评价完成，综合分 ${data.total_score}`, variant: 'success' })
    setTimeout(() => router.push(`/student/evaluations/${data.evaluation_id}`), 600)
  } catch (e) {
    const msg = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    toast({ description: msg ?? '触发评价失败', variant: 'destructive' })
  } finally {
    triggering.value = null
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
    parsed: { label: '已完成解析', variant: 'success' as const },
    parsing: { label: '解析中', variant: 'info' as const },
    failed: { label: '解析失败', variant: 'destructive' as const },
  } as const)[status] ?? { label: '待解析', variant: 'secondary' as const }
}

/** 获取上传的实时状态（优先 SSE 推送，降级为 DB 状态） */
function getUploadStatus(upload: Upload): string {
  const wsProgress = sseGetProgress(upload.id)
  if (wsProgress) return wsProgress.status
  return upload.parse_status
}

/** 获取上传的实时进度百分比 */
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

function onUploadSuccess() {
  // 重新拉取
  fetchAll()
}
</script>

<template>
  <AppShell>
    <div v-if="loading" class="space-y-3">
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
          <p class="mt-2.5 text-sm text-muted-foreground flex items-center gap-2.5">
            <span>{{ courseName(task.course_id) }} · {{ task.dimensions.length }} 个评分维度</span>
            <span class="w-1 h-1 rounded-full bg-subtle-foreground"></span>
            <span class="text-accent font-mono">截止 {{ deadlineLabel }}</span>
          </p>
        </div>
        <div class="flex items-center gap-3">
          <Button variant="outline" @click="fetchAll">刷新</Button>
        </div>
      </div>

      <div class="tes-grid-main-aside">
        <!-- LEFT -->
        <div class="flex flex-col gap-5">
          <!-- Task description -->
          <Card class="tes-card-container">
            <header class="flex justify-between items-center px-6 py-4 border-b border-border">
              <div class="flex items-center gap-2.5">
                <FileText class="w-4 h-4 text-primary" />
                <span class="text-base font-semibold text-ink">任务说明</span>
              </div>
            </header>
            <div class="p-6 space-y-5">
              <div>
                <div class="flex items-center gap-2 mb-2 text-sm font-semibold text-ink">
                  <Info class="w-3.5 h-3.5 text-muted-foreground" />
                  <span>任务概述</span>
                </div>
                <div class="text-sm leading-relaxed text-foreground whitespace-pre-wrap">
                  {{ task.description || '暂无描述' }}
                </div>
              </div>
              <div v-if="task.requirements">
                <div class="flex items-center gap-2 mb-2 text-sm font-semibold text-ink">
                  <ListChecks class="w-3.5 h-3.5 text-muted-foreground" />
                  <span>实训要求</span>
                </div>
                <pre class="text-sm leading-relaxed text-foreground whitespace-pre-wrap font-sans">{{ task.requirements }}</pre>
              </div>
              <div>
                <div class="flex items-center gap-2 mb-3 text-sm font-semibold text-ink">
                  <PieChart class="w-3.5 h-3.5 text-muted-foreground" />
                  <span>评分指标</span>
                </div>
                <div class="tes-grid-kpi">
                  <div
                    v-for="d in task.dimensions"
                    :key="d.id"
                    class="bg-surface-2 border border-border rounded-md p-3 flex flex-col gap-1"
                  >
                    <span class="text-[11px] text-muted-foreground">{{ d.name }}</span>
                    <span class="text-lg font-bold text-ink">{{ d.weight }}%</span>
                  </div>
                </div>
              </div>
            </div>
          </Card>

          <!-- Uploader -->
          <Card class="p-6">
            <FileUploader
              :endpoint="`/api/uploads/by-task/${taskId}`"
              :accept="['.pdf', '.docx', '.doc', '.zip', '.png', '.jpg', '.jpeg']"
              :max-size-mb="50"
              :disabled="!canUpload"
              :disabled-hint="isExpired ? '任务已截止，无法上传' : '任务未发布'"
              @success="onUploadSuccess"
            />
          </Card>

          <!-- Verify result -->
          <Card v-if="verifyForCurrent" class="p-5">
            <div class="flex items-center gap-2 mb-2">
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
          </Card>

          <!-- Evaluation progress panel -->
          <EvaluationProgressPanel
            v-if="currentUpload"
            :parse-status="currentUpload.parse_status"
            :eval-status="evalStatusForPanel"
            :failure-reason="evaluation?.overall_comment"
            :uploaded-at="currentUpload.created_at"
          />

          <!-- Uploaded files -->
          <Card v-if="uploads.length > 0" class="overflow-hidden">
            <header class="px-6 py-4 border-b border-border flex justify-between items-center">
              <div class="flex items-center gap-2.5">
                <FileText class="w-4 h-4 text-primary" />
                <span class="text-base font-semibold text-ink">已上传文件</span>
                <span class="text-xs text-muted-foreground">{{ uploads.length }} 个</span>
              </div>
              <span v-if="currentUpload" class="text-xs text-primary font-medium">
                当前版本 v{{ currentUpload.version }}
              </span>
            </header>

            <div
              v-if="currentUpload"
              class="grid grid-cols-[40px_1fr_140px_auto] items-center gap-4 px-6 py-4 border-b border-border"
            >
              <div :class="['w-10 h-10 rounded-md grid place-items-center flex-shrink-0', fileIcon(currentUpload.file_type).color]">
                <component :is="fileIcon(currentUpload.file_type).cmp" class="w-4 h-4" />
              </div>
              <div>
                <div class="flex items-center gap-2">
                  <span class="text-sm font-semibold text-ink">{{ currentUpload.filename }}</span>
                  <Badge :variant="parseStatusBadge(currentUpload.parse_status).variant" class="text-[10px]">
                    {{ parseStatusBadge(currentUpload.parse_status).label }}
                  </Badge>
                </div>
                <div class="text-[11px] text-muted-foreground font-mono mt-1">
                  {{ formatSize(currentUpload.file_size) }} · {{ formatTime(currentUpload.created_at) }} · v{{ currentUpload.version }}
                </div>
              </div>
              <div class="text-xs font-medium" :class="parseStatusBadge(getUploadStatus(currentUpload)).variant === 'success' ? 'text-success' : 'text-info'">
                <template v-if="getUploadProgress(currentUpload) !== null">
                  <div class="flex items-center gap-2">
                    <Loader2 class="w-3 h-3 animate-spin" />
                    <span>解析中 {{ getUploadProgress(currentUpload) }}%</span>
                  </div>
                  <div class="mt-1 h-1.5 bg-muted rounded-full overflow-hidden">
                    <div
                      class="h-full bg-primary rounded-full transition-all duration-500"
                      :style="{ width: `${getUploadProgress(currentUpload)}%` }"
                    ></div>
                  </div>
                </template>
                <template v-else>
                  {{ parseStatusBadge(getUploadStatus(currentUpload)).label }}
                </template>
              </div>
              <div class="flex gap-1.5 justify-end">
                <Button
                  size="sm"
                  :disabled="triggering === currentUpload.id || currentUpload.parse_status !== 'parsed'"
                  @click="triggerEval(currentUpload.id)"
                >
                  <Loader2 v-if="triggering === currentUpload.id" class="w-3 h-3 animate-spin" />
                  <Sparkles v-else class="w-3 h-3" />
                  {{ triggering === currentUpload.id ? '评价中' : '触发评价' }}
                </Button>
              </div>
            </div>

            <div
              v-for="u in previousUploads"
              :key="u.id"
              class="grid grid-cols-[40px_1fr_140px_auto] items-center gap-4 px-6 py-3 border-b border-border last:border-b-0 opacity-75"
            >
              <div :class="['w-10 h-10 rounded-md grid place-items-center flex-shrink-0', fileIcon(u.file_type).color]">
                <component :is="fileIcon(u.file_type).cmp" class="w-4 h-4" />
              </div>
              <div>
                <div class="flex items-center gap-2">
                  <span class="text-sm font-medium text-ink">{{ u.filename }}</span>
                  <Badge variant="secondary" class="text-[10px]">已替换</Badge>
                </div>
                <div class="text-[11px] text-muted-foreground font-mono mt-1">
                  {{ formatSize(u.file_size) }} · {{ formatTime(u.created_at) }} · v{{ u.version }}
                </div>
              </div>
              <div class="text-xs text-muted-foreground">历史版本</div>
              <Button variant="ghost" size="sm" :disabled="triggering === u.id" @click="triggerEval(u.id)">
                重新评价
              </Button>
            </div>
          </Card>

          <!-- Submission timeline -->
          <Card v-if="uploads.length > 0" class="overflow-hidden">
            <header class="px-6 py-4 border-b border-border">
              <div class="flex items-center gap-2.5">
                <HistoryIcon class="w-4 h-4 text-muted-foreground" />
                <span class="text-base font-semibold text-ink">提交历史</span>
                <span class="text-[11px] text-muted-foreground">截止前可继续替换</span>
              </div>
            </header>
            <div class="px-6 py-2">
              <div
                v-for="(u, idx) in uploads"
                :key="u.id"
                class="flex gap-3.5 py-3 relative"
              >
                <div
                  v-if="idx < uploads.length - 1"
                  class="absolute left-[11px] top-[28px] bottom-[-8px] w-px bg-border"
                ></div>
                <div class="w-6 flex-shrink-0 flex flex-col items-center pt-0.5">
                  <div
                    class="w-2.5 h-2.5 rounded-full bg-card border-2 flex-shrink-0"
                    :class="idx === 0 ? 'border-primary' : 'border-border-strong'"
                  ></div>
                </div>
                <div class="flex-1 flex flex-col gap-0.5">
                  <div class="flex items-center gap-2.5">
                    <span class="text-sm font-semibold text-ink">
                      v{{ u.version }}{{ idx === 0 ? ' · 当前版本' : '' }}
                    </span>
                    <Badge :variant="idx === 0 ? 'success' : 'secondary'" class="text-[10px]">
                      {{ idx === 0 ? '已提交' : '已替换' }}
                    </Badge>
                    <span class="font-mono text-[11px] text-muted-foreground">{{ formatTime(u.created_at) }}</span>
                  </div>
                  <div class="text-xs text-ink">{{ u.filename }}</div>
                  <div class="text-[11px] text-muted-foreground">
                    {{ formatSize(u.file_size) }} · {{ u.file_type.toUpperCase() }}
                  </div>
                </div>
              </div>
            </div>
          </Card>
        </div>

        <!-- RIGHT -->
        <div class="flex flex-col gap-5">
          <div v-if="countdown && !countdown.expired" class="bg-primary text-primary-foreground rounded-lg p-6 flex flex-col gap-3.5">
            <span class="text-[11px] font-semibold tracking-widest text-primary-foreground/70">距离截止还剩</span>
            <div class="flex items-end gap-3.5">
              <div class="flex flex-col items-center gap-0.5">
                <span class="text-4xl font-bold leading-none num-tabular">{{ String(countdown.days).padStart(2, '0') }}</span>
                <span class="text-[11px] text-primary-foreground/70">天</span>
              </div>
              <span class="text-2xl font-bold pb-3">:</span>
              <div class="flex flex-col items-center gap-0.5">
                <span class="text-4xl font-bold leading-none num-tabular">{{ String(countdown.hours).padStart(2, '0') }}</span>
                <span class="text-[11px] text-primary-foreground/70">小时</span>
              </div>
              <span class="text-2xl font-bold pb-3">:</span>
              <div class="flex flex-col items-center gap-0.5">
                <span class="text-4xl font-bold leading-none num-tabular">{{ String(countdown.minutes).padStart(2, '0') }}</span>
                <span class="text-[11px] text-primary-foreground/70">分钟</span>
              </div>
            </div>
            <div class="h-px my-1 bg-primary-foreground/15"></div>
            <span class="font-mono text-[11px] text-primary-foreground/70">截止时间 {{ deadlineLabel }}</span>
          </div>

          <Card v-else-if="countdown?.expired" class="p-5 text-center bg-muted">
            <span class="text-sm font-semibold text-muted-foreground">已截止</span>
            <p class="text-xs text-muted-foreground mt-1 font-mono">{{ deadlineLabel }}</p>
          </Card>

          <Card class="overflow-hidden">
            <header class="flex justify-between items-center px-5 py-4 border-b border-border">
              <span class="text-sm font-semibold text-ink">任务状态</span>
              <span class="text-xs font-semibold text-primary">{{ uploads.length }} 次提交</span>
            </header>
            <div class="px-5 py-4 flex flex-col gap-2.5">
              <div class="flex justify-between items-center">
                <span class="text-xs text-muted-foreground">任务状态</span>
                <Badge :variant="task.status === 'published' ? 'info' : 'secondary'">
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
              <div v-if="evaluation" class="flex justify-between items-center">
                <span class="text-xs text-muted-foreground">综合得分</span>
                <span class="text-xs font-semibold text-success">{{ evaluation.total_score ?? '—' }}</span>
              </div>
            </div>
          </Card>

          <Card class="p-6 flex flex-col gap-3.5">
            <div class="text-base font-semibold text-ink">提交清单</div>
            <ScrollArea class="bg-surface-2 rounded-md p-3.5 max-h-48">
              <div v-if="uploads.length === 0" class="text-xs text-muted-foreground py-2">
                还未上传任何文件
              </div>
              <ul class="flex flex-col gap-2">
                <li
                  v-for="u in uploads.slice(0, 5)"
                  :key="u.id"
                  class="flex items-start gap-2 text-xs text-foreground leading-relaxed"
                >
                  <span class="w-3.5 h-3.5 rounded-full bg-success-soft text-success grid place-items-center flex-shrink-0 mt-0.5">
                    <Check class="w-2.5 h-2.5" />
                  </span>
                  <span class="flex-1 truncate">{{ u.filename }} (v{{ u.version }})</span>
                </li>
              </ul>
            </ScrollArea>
          </Card>

          <div class="bg-accent-soft border border-accent rounded-lg p-4 flex flex-col gap-2.5">
            <div class="flex items-center gap-2 text-accent-strong text-sm font-semibold">
              <ShieldAlert class="w-4 h-4" />
              <span>诚信提示</span>
            </div>
            <div class="text-xs text-accent-strong leading-relaxed">
              系统将自动检测相似度，相似度过高会标记为疑似抄袭并通知教师人工复核。请独立完成你的实训。
            </div>
          </div>
        </div>
      </div>
    </template>
  </AppShell>
</template>
