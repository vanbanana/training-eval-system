<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { avatarInitial } from '@/lib/utils'
import { useRoute, useRouter } from 'vue-router'
import axios from 'axios'
import AppShell from '@/components/layout/AppShell.vue'
import BreadcrumbNav from '@/components/business/BreadcrumbNav.vue'
import EmptyState from '@/components/business/EmptyState.vue'
import AnimatedNumber from '@/components/business/AnimatedNumber.vue'
import { useToast } from '@/components/ui/toast'
import { confirm } from '@/composables/useConfirm'
import { safeGet } from '@/lib/api-helpers'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Card } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Badge } from '@/components/ui/badge'
import { Checkbox } from '@/components/ui/checkbox'
import { Avatar } from '@/components/ui/avatar'
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
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import {
  AlertTriangle,
  Loader2,
  Search,
  History,
  Edit3,
  Eye,
  Sparkles,
} from 'lucide-vue-next'

interface Submission {
  upload_id: number
  student_id: number
  student_name: string
  filename: string
  file_size: number
  version: number
  parse_status: string
  uploaded_at: string
  evaluation_id: number | null
  eval_status: string | null
  total_score: number | null
}

interface SimilarityRecord {
  id: number
  task_id: number
  upload_a_id: number
  upload_b_id: number
  hamming_distance: number | null
  cosine_similarity: number | null
  state: string
  created_at?: string
}

interface Task {
  id: number
  name: string
  status: string
  deadline: string | null
  course_id: number
  dimensions: { id: number; name: string; weight: number }[]
}

interface DimensionScore {
  dimension_id: number
  dimension_name: string
  obj_score: number
  subj_score: number | null
  comment: string | null
  weight: number
}

interface EvaluationDetail {
  id: number
  total_score: number | null
  status: string
  scores: DimensionScore[]
}

interface HistoryItem {
  id: number
  action: string
  before_value: string | null
  after_value: string | null
  changed_at: string
  operator_id: number
}

const route = useRoute()
const router = useRouter()
const taskId = computed(() => route.params.id as string)
const { toast } = useToast()

const submissions = ref<Submission[]>([])
const similarityPairs = ref<SimilarityRecord[]>([])
const task = ref<Task | null>(null)
const loading = ref(true)
const filterTab = ref<string>('all')
const searchQuery = ref('')
const selectedIds = ref<Set<number>>(new Set())

// Reject dialog
const showRejectDialog = ref(false)
const rejectTarget = ref<Submission | null>(null)
const bulkRejectTargets = ref<Submission[]>([])
const rejectReason = ref('')
const rejectSubmitting = ref(false)
const rejectReasonChips = [
  '解析失败，请重新提交',
  '内容与要求不符',
  '相似度过高，疑似抄袭',
  '文档格式不规范',
  '内容过于简略',
]

// Detail / dimension edit Sheet
const showDetail = ref(false)
const detailLoading = ref(false)
const detailTarget = ref<Submission | null>(null)
const detailEvaluation = ref<EvaluationDetail | null>(null)
const detailHistory = ref<HistoryItem[]>([])

const editingDim = ref<DimensionScore | null>(null)
const showDimDialog = ref(false)
const dimSubjScore = ref(0)
const dimComment = ref('')
const dimSubmitting = ref(false)

// 任务级聚合统计
interface TaskSummary {
  task_id: number
  total_uploads: number
  parsed_count: number
  scored_count: number
  confirmed_count: number
  rejected_count: number
  similarity_warnings: number
  progress_percent: number
}
const taskSummary = ref<TaskSummary | null>(null)
const autoScoring = ref(false)

// ─── SSE 实时评分进度 ───
interface ScoringProgress {
  upload_id: number
  student_name: string
  status: 'queued' | 'scoring' | 'scored' | 'failed'
  score?: number
}
const scoringProgress = ref<Map<number, ScoringProgress>>(new Map())
const scoringAnimReady = ref(false)

// 分数露出动画队列 — 「刷刷刷」
const scoreRevealQueue = ref<Array<{ upload_id: number; score: number }>>([])
const revealActive = ref(false)

let sseEventSource: EventSource | null = null

function connectSSE() {
  const raw = localStorage.getItem('tes_token')
  let token = ''
  if (raw) {
    try { token = JSON.parse(raw) } catch { token = raw }
  }
  if (!token) return

  const url = `${window.location.protocol === 'https:' ? 'https:' : 'http:'}//${window.location.host}/api/sse/events?token=${encodeURIComponent(token)}`
  sseEventSource = new EventSource(url)

  sseEventSource.addEventListener('progress', (e: MessageEvent) => {
    try {
      const data = JSON.parse(e.data) as {
        user_id: number
        upload_id: number
        stage: string
        status: string
        evaluation_id?: number
        total_score?: number
      }
      if (data.stage === 'eval' || data.stage === 'eval_dimensions') {
        scoringProgress.value.set(data.upload_id, {
          upload_id: data.upload_id,
          student_name: '',
          status: data.status === 'scored' ? 'scored' : 'scoring',
          score: data.total_score,
        })
        scoringProgress.value = new Map(scoringProgress.value)
      }
    } catch {}
  })

  sseEventSource.addEventListener('score_complete', (e: MessageEvent) => {
    try {
      const data = JSON.parse(e.data) as {
        evaluation_id: number
        upload_id: number
        total_score: number
      }
      // 更新评分状态
      scoringProgress.value.set(data.upload_id, {
        upload_id: data.upload_id,
        student_name: '',
        status: 'scored',
        score: data.total_score,
      })
      scoringProgress.value = new Map(scoringProgress.value)

      // 加入露出动画队列
      scoreRevealQueue.value.push({ upload_id: data.upload_id, score: data.total_score })
      if (!revealActive.value) playRevealQueue()
    } catch {}
  })
}

function playRevealQueue() {
  if (scoreRevealQueue.value.length === 0) {
    revealActive.value = false
    return
  }
  revealActive.value = true
  const item = scoreRevealQueue.value.shift()!
  // 找到对应提交行，更新总分为动画值
  const sub = submissionIndex.value.get(item.upload_id)
  if (sub) {
    sub.total_score = item.score
    sub.eval_status = 'scored'
  }
  // 每个间隔 300-500ms，「刷刷刷」的感觉
  setTimeout(() => playRevealQueue(), 300 + Math.random() * 200)
}

onMounted(() => {
  fetchAll()
  connectSSE()
})

onUnmounted(() => {
  if (sseEventSource) sseEventSource.close()
})

async function triggerAutoScore() {
  if (autoScoring.value) return
  const ok = await confirm({
    title: '一键 AI 批改',
    description: '将对所有已解析但未评分的提交执行 AI 批改，评分结果将实时推送展示。',
  })
  if (!ok) return
  autoScoring.value = true
  scoringProgress.value = new Map()
  try {
    const { data } = await axios.post(`/api/grading/tasks/${taskId.value}/auto-score`, { mode: 'unscored' })
    // 把排队中的先标记为 queued
    if (data.items) {
      for (const item of data.items) {
        if (item.status === 'queued') {
          const sub = submissions.value.find(s => s.upload_id === item.upload_id)
          scoringProgress.value.set(item.upload_id, {
            upload_id: item.upload_id,
            student_name: sub?.student_name ?? '',
            status: 'queued',
          })
        }
      }
      scoringProgress.value = new Map(scoringProgress.value)
    }
    toast({
      description: `⏳ 队列 ${data.queued} 份，跳过 ${data.skipped} 份${data.failed ? `，失败 ${data.failed} 份` : ''}`,
      variant: data.failed ? 'warning' : 'success',
    })
  } catch (e) {
    const msg = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    toast({ description: msg ?? '一键批改请求失败', variant: 'destructive' })
    autoScoring.value = false
  }
}

async function fetchAll() {
  loading.value = true
  try {
    const [taskRes, subsRes, summaryRes] = await Promise.all([
      axios.get(`/api/tasks/${taskId.value}`),
      axios.get(`/api/grading/tasks/${taskId.value}/submissions`),
      safeGet<TaskSummary>(`/api/grading/tasks/${taskId.value}/summary`, null as unknown as TaskSummary),
    ])
    task.value = taskRes.data
    submissions.value = Array.isArray(subsRes.data) ? subsRes.data : []
    if (!summaryRes.error) {
      taskSummary.value = summaryRes.data
    }

    // 相似度记录可降级（404=任务无相似度记录是常态）
    const simResult = await safeGet<SimilarityRecord[]>(
      `/api/similarity/task/${taskId.value}`,
      [],
    )
    if (simResult.error && simResult.status !== 404) {
      toast({
        description: `相似度数据 ${simResult.error}`,
        variant: 'warning',
      })
    }
    similarityPairs.value = simResult.data
  } catch (e) {
    const msg = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    toast({ description: msg ?? '加载批改数据失败', variant: 'destructive' })
  } finally {
    loading.value = false
  }
}

onMounted(fetchAll)

// 计算疑似抄袭：以后端 state==='suspect' 且 cosine_similarity 高 / hamming 低为信号
function pairIsSuspicious(p: SimilarityRecord): boolean {
  return (
    p.state === 'suspect' ||
    (p.cosine_similarity !== null && p.cosine_similarity >= 0.85) ||
    (p.hamming_distance !== null && p.hamming_distance <= 8)
  )
}

const suspiciousUploadIds = computed(() => {
  const ids = new Set<number>()
  for (const p of similarityPairs.value) {
    if (pairIsSuspicious(p)) {
      ids.add(p.upload_a_id)
      ids.add(p.upload_b_id)
    }
  }
  return ids
})

const submissionIndex = computed(() => {
  const m = new Map<number, Submission>()
  for (const s of submissions.value) m.set(s.upload_id, s)
  return m
})

const similarityHint = computed<Record<number, string>>(() => {
  const map: Record<number, string> = {}
  for (const p of similarityPairs.value) {
    if (!pairIsSuspicious(p)) continue
    const ratio = p.cosine_similarity != null ? Math.round(p.cosine_similarity * 100) + '%' : `汉明 ${p.hamming_distance}`
    const a = submissionIndex.value.get(p.upload_a_id)
    const b = submissionIndex.value.get(p.upload_b_id)
    if (a && b) {
      map[p.upload_a_id] = `与 ${b.student_name} 相似度 ${ratio}`
      map[p.upload_b_id] = `与 ${a.student_name} 相似度 ${ratio}`
    }
  }
  return map
})

const counts = computed(() => ({
  all: submissions.value.length,
  pending: submissions.value.filter((s) => !s.eval_status || s.eval_status === 'pending').length,
  scored: submissions.value.filter((s) => s.eval_status === 'scored').length,
  confirmed: submissions.value.filter((s) => s.eval_status === 'confirmed').length,
  rejected: submissions.value.filter((s) => s.eval_status === 'rejected').length,
  suspicious: submissions.value.filter((s) => suspiciousUploadIds.value.has(s.upload_id)).length,
}))

const filtered = computed(() => {
  let list = submissions.value
  if (filterTab.value === 'pending') {
    list = list.filter((s) => !s.eval_status || s.eval_status === 'pending')
  } else if (filterTab.value === 'scored') {
    list = list.filter((s) => s.eval_status === 'scored')
  } else if (filterTab.value === 'confirmed') {
    list = list.filter((s) => s.eval_status === 'confirmed')
  } else if (filterTab.value === 'rejected') {
    list = list.filter((s) => s.eval_status === 'rejected')
  } else if (filterTab.value === 'suspicious') {
    list = list.filter((s) => suspiciousUploadIds.value.has(s.upload_id))
  }
  if (searchQuery.value.trim()) {
    const q = searchQuery.value.trim().toLowerCase()
    list = list.filter(
      (s) =>
        s.student_name.toLowerCase().includes(q) ||
        String(s.student_id).includes(q),
    )
  }
  return list
})

const stats = computed(() => {
  const total = submissions.value.length
  const aiScored = submissions.value.filter((s) => s.eval_status === 'scored' || s.eval_status === 'confirmed').length
  const teacherConfirmed = submissions.value.filter((s) => s.eval_status === 'confirmed').length
  const suspicious = similarityPairs.value.filter(pairIsSuspicious).length
  return {
    submitted: total,
    aiScored,
    aiPending: total - aiScored,
    teacherConfirmed,
    confirmRate: aiScored > 0 ? Math.round((teacherConfirmed / aiScored) * 100) : 0,
    suspicious,
  }
})

const allSelectedOnList = computed({
  get: () => filtered.value.length > 0 && filtered.value.every((s) => selectedIds.value.has(s.upload_id)),
  set: (v: boolean) => {
    if (v) filtered.value.forEach((s) => selectedIds.value.add(s.upload_id))
    else filtered.value.forEach((s) => selectedIds.value.delete(s.upload_id))
    selectedIds.value = new Set(selectedIds.value)
  },
})
const someSelectedOnList = computed(
  () => filtered.value.some((s) => selectedIds.value.has(s.upload_id)) && !allSelectedOnList.value,
)
function toggleRow(uploadId: number, v: boolean) {
  if (v) selectedIds.value.add(uploadId)
  else selectedIds.value.delete(uploadId)
  selectedIds.value = new Set(selectedIds.value)
}

const selectedCount = computed(() => selectedIds.value.size)

function statusBadge(s: Submission) {
  if (suspiciousUploadIds.value.has(s.upload_id)) return { label: '疑似相似', variant: 'destructive' as const }
  switch (s.eval_status) {
    case 'confirmed': return { label: '已确认', variant: 'success' as const }
    case 'scored': return { label: '待批改', variant: 'warning' as const }
    case 'rejected': return { label: '已打回', variant: 'destructive' as const }
    case 'pending': return { label: 'AI 评分中', variant: 'info' as const }
    default:
      if (s.parse_status === 'pending' || s.parse_status === 'parsing') return { label: '解析中', variant: 'info' as const }
      return { label: '待批改', variant: 'warning' as const }
  }
}

function formatDate(iso: string) {
  if (!iso) return '——'
  const d = new Date(iso)
  return `${(d.getMonth() + 1).toString().padStart(2, '0')}-${d.getDate().toString().padStart(2, '0')} ${d.getHours().toString().padStart(2, '0')}:${d.getMinutes().toString().padStart(2, '0')}`
}

function formatDeadline(iso: string | null) {
  if (!iso) return ''
  return iso.slice(0, 16).replace('T', ' ')
}

async function confirmEval(s: Submission) {
  if (!s.evaluation_id) return
  try {
    const { data } = await axios.post(`/api/grading/evaluations/${s.evaluation_id}/confirm`, {
      teacher_comment: '确认通过',
      score_overrides: {},
    })
    toast({ description: `已确认，最终分 ${data.total_score}`, variant: 'success' })
    s.eval_status = 'confirmed'
    s.total_score = data.total_score
  } catch (e) {
    const msg = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    toast({ description: msg ?? '操作失败', variant: 'destructive' })
  }
}

function rejectEval(s: Submission) {
  if (!s.evaluation_id) return
  rejectTarget.value = s
  bulkRejectTargets.value = []
  rejectReason.value = ''
  showRejectDialog.value = true
}

async function submitReject() {
  if (!rejectTarget.value?.evaluation_id) return
  if (rejectReason.value.trim().length < 20) {
    toast({ description: '原因至少 20 字', variant: 'destructive' })
    return
  }
  rejectSubmitting.value = true
  try {
    await axios.post(`/api/grading/evaluations/${rejectTarget.value.evaluation_id}/reject`, {
      reason: rejectReason.value.trim(),
    })
    rejectTarget.value.eval_status = 'rejected'
    toast({ description: '已打回', variant: 'success' })
    showRejectDialog.value = false
    rejectTarget.value = null
    rejectReason.value = ''
  } catch (e) {
    const msg = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    toast({ description: msg ?? '操作失败', variant: 'destructive' })
  } finally {
    rejectSubmitting.value = false
  }
}

async function bulkConfirm() {
  if (selectedCount.value === 0) {
    toast({ description: '请先勾选要确认的提交', variant: 'info' })
    return
  }
  const targets = submissions.value.filter(
    (s) => selectedIds.value.has(s.upload_id) && s.evaluation_id && s.eval_status === 'scored',
  )
  if (targets.length === 0) {
    toast({ description: '所选项目无可确认的评价', variant: 'warning' })
    return
  }
  const ok = await confirm({
    title: `确认 ${targets.length} 项评价？`,
    description: '将使用 /api/evaluations/bulk-action 批量提交',
  })
  if (!ok) return
  try {
    const { data } = await axios.post('/api/evaluations/bulk-action', {
      evaluation_ids: targets.map((t) => t.evaluation_id),
      action: 'confirm',
      reason: '批量确认',
    })
    toast({
      description: `批量确认完成：${data.confirmed ?? data.affected ?? targets.length}/${targets.length}`,
      variant: 'success',
    })
    selectedIds.value = new Set()
    await fetchAll()
  } catch (e) {
    const msg = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    toast({ description: msg ?? '批量确认失败', variant: 'destructive' })
  }
}

function bulkReject() {
  if (selectedCount.value === 0) {
    toast({ description: '请先勾选要打回的提交', variant: 'info' })
    return
  }
  const targets = submissions.value.filter(
    (s) => selectedIds.value.has(s.upload_id) && s.evaluation_id,
  )
  if (targets.length === 0) {
    toast({ description: '所选项目没有可打回的评价', variant: 'warning' })
    return
  }
  rejectTarget.value = null
  bulkRejectTargets.value = targets
  rejectReason.value = ''
  showRejectDialog.value = true
}

async function submitBulkReject() {
  if (bulkRejectTargets.value.length === 0) return
  if (rejectReason.value.trim().length < 20) {
    toast({ description: '原因至少 20 字', variant: 'destructive' })
    return
  }
  rejectSubmitting.value = true
  try {
    const { data } = await axios.post('/api/evaluations/bulk-action', {
      evaluation_ids: bulkRejectTargets.value.map((t) => t.evaluation_id),
      action: 'reject',
      reason: rejectReason.value.trim(),
    })
    toast({
      description: `批量打回完成：${data.rejected ?? data.affected ?? bulkRejectTargets.value.length}/${bulkRejectTargets.value.length}`,
      variant: 'success',
    })
    showRejectDialog.value = false
    bulkRejectTargets.value = []
    rejectReason.value = ''
    selectedIds.value = new Set()
    await fetchAll()
  } catch (e) {
    const msg = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    toast({ description: msg ?? '批量打回失败', variant: 'destructive' })
  } finally {
    rejectSubmitting.value = false
  }
}

async function openDetail(s: Submission) {
  if (!s.evaluation_id) {
    toast({ description: '该提交尚未生成评价', variant: 'info' })
    return
  }
  detailTarget.value = s
  detailEvaluation.value = null
  detailHistory.value = []
  detailLoading.value = true
  showDetail.value = true
  try {
    const evRes = await axios.get(`/api/evaluations/${s.evaluation_id}`)
    detailEvaluation.value = { ...evRes.data, scores: evRes.data.scores ?? [] }
    // history 可降级（404=新评价无历史是常态）
    const histResult = await safeGet<HistoryItem[]>(
      `/api/evaluations/${s.evaluation_id}/history`,
      [],
    )
    detailHistory.value = histResult.data
  } catch (e) {
    const msg = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    toast({ description: msg ?? '加载评价失败', variant: 'destructive' })
  } finally {
    detailLoading.value = false
  }
}

function startEditDim(d: DimensionScore) {
  editingDim.value = d
  dimSubjScore.value = d.subj_score ?? d.obj_score
  dimComment.value = d.comment ?? ''
  showDimDialog.value = true
}

async function submitDimUpdate() {
  if (!detailTarget.value?.evaluation_id || !editingDim.value) return
  if (dimSubjScore.value < 0 || dimSubjScore.value > 100) {
    toast({ description: '分数应在 0-100 之间', variant: 'destructive' })
    return
  }
  dimSubmitting.value = true
  try {
    await axios.patch(
      `/api/evaluations/${detailTarget.value.evaluation_id}/dimensions/${editingDim.value.dimension_id}`,
      { subj_score: dimSubjScore.value, comment: dimComment.value },
    )
    toast({ description: '已保存维度调整', variant: 'success' })
    showDimDialog.value = false
    // 刷新详情
    if (detailTarget.value) await openDetail(detailTarget.value)
  } catch (e) {
    const msg = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    toast({ description: msg ?? '保存失败', variant: 'destructive' })
  } finally {
    dimSubmitting.value = false
  }
}

function viewFullEvaluation() {
  if (detailTarget.value?.evaluation_id) {
    router.push(`/teacher/evaluations/${detailTarget.value.evaluation_id}`)
  }
}

function goToSimilarity(uploadId: number) {
  // 找包含该 upload 的第一条 suspect 记录
  const pair = similarityPairs.value.find(
    (p) => pairIsSuspicious(p) && (p.upload_a_id === uploadId || p.upload_b_id === uploadId),
  )
  if (!pair) {
    toast({ description: '未找到相似度比对记录', variant: 'warning' })
    return
  }
  router.push({ path: `/teacher/similarity/${pair.id}`, query: { task_id: taskId.value } })
}
</script>

<template>
  <AppShell>
    <BreadcrumbNav
      :items="[
        { label: '工作台', to: '/dashboard' },
        { label: task?.name ?? '加载中...', to: '/teacher/tasks' },
        { label: '批改工作台' },
      ]"
    />

    <div class="flex justify-between items-end">
      <div>
        <div class="flex items-center gap-3">
          <h1 class="text-2xl font-bold text-ink">批改工作台</h1>
          <Badge variant="secondary">{{ task?.name ?? '——' }}</Badge>
        </div>
        <p class="mt-1.5 text-sm text-muted-foreground">
          {{ submissions.length }} 名学生 · 截止 {{ formatDeadline(task?.deadline ?? null) || '未设定' }}
        </p>
      </div>
      <div class="flex items-center gap-3">
        <Button variant="outline" :disabled="autoScoring" @click="triggerAutoScore">
          <Loader2 v-if="autoScoring" class="w-4 h-4 mr-1.5 animate-spin" />
          <Sparkles v-else class="w-4 h-4 mr-1.5" />
          一键 AI 批改未评分提交
        </Button>
        <Button variant="outline" :disabled="selectedCount === 0" @click="bulkReject">
          批量打回
        </Button>
        <Button :disabled="selectedCount === 0" @click="bulkConfirm">
          一键确认所选 ({{ selectedCount }})
        </Button>
      </div>
    </div>

    <!-- Stats Bar -->
    <Card class="tes-card-container">
      <div class="tes-grid-kpi px-5 py-5">
        <div class="flex min-w-0 flex-col gap-1.5 anim-in" :style="{ animationDelay: '0ms' }">
          <span class="text-[11px] font-semibold tracking-wider text-muted-foreground">批改进度</span>
          <div class="flex items-end gap-2">
            <span class="text-2xl font-bold text-ink leading-none">
              <AnimatedNumber :value="stats.submitted" />
            </span>
            <span class="text-sm text-muted-foreground">已提交</span>
          </div>
          <div class="h-1.5 bg-muted rounded-full overflow-hidden mt-1">
            <div class="h-full bg-primary rounded-full transition-all duration-700" :style="{ width: `${stats.submitted > 0 ? 100 : 0}%` }"></div>
          </div>
        </div>
        <div class="hidden w-px bg-border self-stretch"></div>
        <div class="flex min-w-0 flex-col gap-1.5 anim-in" :style="{ animationDelay: '50ms' }">
          <span class="text-[11px] font-semibold tracking-wider text-muted-foreground">已自动评分</span>
          <div class="flex items-end gap-2">
            <span class="text-2xl font-bold text-ink leading-none">
              <AnimatedNumber :value="stats.aiScored" />
            </span>
            <span class="text-sm text-muted-foreground">/ {{ stats.submitted }}</span>
          </div>
          <span class="text-[11px] text-info">另有 {{ stats.aiPending }} 份待解析</span>
        </div>
        <div class="hidden w-px bg-border self-stretch"></div>
        <div class="flex min-w-0 flex-col gap-1.5 anim-in" :style="{ animationDelay: '100ms' }">
          <span class="text-[11px] font-semibold tracking-wider text-muted-foreground">教师已确认</span>
          <div class="flex items-end gap-2">
            <span class="text-2xl font-bold text-success leading-none">
              <AnimatedNumber :value="stats.teacherConfirmed" />
            </span>
            <span class="text-sm text-muted-foreground">/ {{ stats.aiScored }}</span>
          </div>
          <span class="text-[11px] text-muted-foreground">确认率 {{ stats.confirmRate }}%</span>
        </div>
        <div class="hidden w-px bg-border self-stretch"></div>
        <div class="flex min-w-0 flex-col gap-1.5 anim-in" :style="{ animationDelay: '150ms' }">
          <span class="text-[11px] font-semibold tracking-wider text-muted-foreground">疑似抄袭警告</span>
          <div class="flex items-end gap-2">
            <span class="text-2xl font-bold text-danger leading-none">
              <AnimatedNumber :value="stats.suspicious" />
            </span>
            <span class="text-sm text-muted-foreground">组</span>
          </div>
          <span class="text-[11px] text-danger">建议人工复核</span>
        </div>
      </div>
    </Card>

    <Card class="overflow-hidden">
      <Tabs v-model="filterTab" class="px-2 pt-2 border-b border-border">
        <TabsList>
          <TabsTrigger value="all">全部 {{ counts.all }}</TabsTrigger>
          <TabsTrigger value="pending">待批改 {{ counts.pending }}</TabsTrigger>
          <TabsTrigger value="scored">已评分 {{ counts.scored }}</TabsTrigger>
          <TabsTrigger value="confirmed">已确认 {{ counts.confirmed }}</TabsTrigger>
          <TabsTrigger value="rejected">已打回 {{ counts.rejected }}</TabsTrigger>
          <TabsTrigger v-if="counts.suspicious > 0" value="suspicious" class="data-[state=active]:bg-danger data-[state=active]:text-destructive-foreground">
            <AlertTriangle class="w-3 h-3 mr-1" />
            疑似 {{ counts.suspicious }}
          </TabsTrigger>
        </TabsList>
      </Tabs>

      <div class="px-5 py-3 bg-surface-2 border-b border-border flex flex-wrap items-center gap-3">
        <label class="flex items-center gap-2 text-xs text-muted-foreground cursor-pointer select-none">
          <Checkbox
            :model-value="allSelectedOnList ? true : someSelectedOnList ? 'indeterminate' : false"
            @update:model-value="(v) => allSelectedOnList = v === true"
            aria-label="全选"
          />
          全选
        </label>
        <div class="relative w-full sm:w-[280px]">
          <Search class="w-3.5 h-3.5 text-muted-foreground absolute left-3 top-1/2 -translate-y-1/2" />
          <Input v-model="searchQuery" placeholder="按学号 / 姓名搜索" class="pl-9" />
        </div>
        <span v-if="selectedCount > 0" class="text-xs text-primary font-medium">已选 {{ selectedCount }} 项</span>
      </div>

      <div class="flex flex-col">
        <template v-if="loading">
          <div
            v-for="n in 6"
            :key="n"
            class="flex items-center gap-4 px-5 py-4 border-b border-border"
          >
            <Skeleton class="h-4 w-4" />
            <Skeleton class="h-9 w-9 rounded-full" />
            <div class="flex-1 space-y-2">
              <Skeleton class="h-4 w-40" />
              <Skeleton class="h-3 w-24" />
            </div>
            <Skeleton class="h-8 w-12" />
            <Skeleton class="h-7 w-28" />
          </div>
        </template>

        <EmptyState
          v-else-if="filtered.length === 0"
          title="无符合条件的提交"
          description="调整筛选 / 搜索条件查看更多结果"
        />

        <div
          v-for="s in filtered"
          v-else
          :key="s.upload_id"
          class="flex flex-wrap items-center gap-x-4 gap-y-3 px-5 py-4 border-b border-border last:border-b-0 transition-colors"
          :class="suspiciousUploadIds.has(s.upload_id) ? 'bg-danger-soft hover:bg-danger-soft/80' : 'hover:bg-surface-2'"
        >
          <Checkbox
            :model-value="selectedIds.has(s.upload_id)"
            @update:model-value="(v) => toggleRow(s.upload_id, v === true)"
            :aria-label="`选择 ${s.student_name}`"
          />

          <Avatar size="sm" :class="suspiciousUploadIds.has(s.upload_id) ? '!bg-danger-soft !text-danger' : ''">
            {{ avatarInitial(s.student_name) }}
          </Avatar>

          <div class="flex min-w-[10rem] flex-1 flex-col gap-1">
            <div class="flex items-center gap-2">
              <span class="text-sm font-semibold text-ink truncate">{{ s.student_name }}</span>
              <Badge :variant="statusBadge(s).variant">{{ statusBadge(s).label }}</Badge>
            </div>
            <span class="text-[11px] text-muted-foreground truncate">
              学号 {{ s.student_id }} · {{ formatDate(s.uploaded_at) }}<template v-if="similarityHint[s.upload_id]"> · <span class="text-danger">{{ similarityHint[s.upload_id] }}</span></template>
            </span>
          </div>

          <div class="flex w-16 flex-col items-center">
            <div v-if="s.eval_status === 'pending'" class="flex items-center gap-1.5 text-info">
              <Loader2 class="w-3.5 h-3.5 animate-spin" />
              <span class="text-xs font-medium">评分中</span>
            </div>
            <template v-else-if="s.total_score !== null">
              <span
                class="text-xl font-bold leading-none"
                :class="s.eval_status === 'confirmed' ? 'text-success' : (s.total_score < 70 ? 'text-accent' : 'text-ink')"
              >{{ s.total_score }}</span>
              <span class="mt-0.5 text-[10px] text-muted-foreground">综合得分</span>
            </template>
            <span v-else class="font-mono text-sm text-subtle-foreground">——</span>
          </div>

          <div class="flex items-center justify-end gap-1">
            <Button
              v-if="suspiciousUploadIds.has(s.upload_id)"
              variant="ghost"
              size="sm"
              class="h-7 px-2 text-danger hover:text-danger"
              @click="goToSimilarity(s.upload_id)"
              title="查看相似度对比"
            >
              <AlertTriangle class="w-3 h-3" />
            </Button>
            <Button v-if="s.eval_status === 'scored'" variant="ghost" size="sm" class="h-7 px-2 text-primary" @click="confirmEval(s)">确认</Button>
            <Button variant="ghost" size="sm" class="h-7 px-2" @click="openDetail(s)">详情</Button>
            <Button v-if="s.evaluation_id && s.eval_status !== 'rejected'" variant="ghost" size="sm" class="h-7 px-2 text-danger hover:text-danger" @click="rejectEval(s)">打回</Button>
          </div>
        </div>
      </div>
    </Card>

    <!-- Reject Dialog -->
    <Dialog v-model:open="showRejectDialog">
      <DialogContent class="max-w-lg">
        <DialogHeader>
          <DialogTitle class="flex items-center gap-2">
            <AlertTriangle class="w-4 h-4 text-danger" />
            {{ bulkRejectTargets.length > 0
                ? `批量打回（${bulkRejectTargets.length} 项）`
                : `打回 ${rejectTarget?.student_name ?? ''} 的提交` }}
          </DialogTitle>
          <DialogDescription>打回操作不可撤销。学生将收到通知并被要求重新提交。</DialogDescription>
        </DialogHeader>
        <div class="flex flex-col gap-4">
          <div>
            <Label class="text-xs">快速选择原因</Label>
            <div class="flex flex-wrap gap-2 mt-2">
              <Button
                v-for="chip in rejectReasonChips"
                :key="chip"
                variant="outline"
                size="sm"
                class="h-7 text-[11px]"
                @click="rejectReason = chip + '。请根据要求重新整理后提交。'"
              >
                {{ chip }}
              </Button>
            </div>
          </div>
          <div class="space-y-2">
            <Label class="flex items-center justify-between">
              详细说明 <span class="text-danger">*</span>
              <span class="font-mono text-[11px]" :class="rejectReason.trim().length >= 20 ? 'text-success' : 'text-danger'">
                {{ rejectReason.trim().length }}/20
              </span>
            </Label>
            <Textarea v-model="rejectReason" rows="5" placeholder="请详细说明打回原因..." />
          </div>
        </div>
        <DialogFooter class="gap-2">
          <Button variant="outline" @click="showRejectDialog = false">取消</Button>
          <Button
            variant="destructive"
            :disabled="rejectSubmitting || rejectReason.trim().length < 20"
            @click="bulkRejectTargets.length > 0 ? submitBulkReject() : submitReject()"
          >
            {{ rejectSubmitting ? '打回中...' : '确认打回' }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- Detail Sheet -->
    <Sheet v-model:open="showDetail">
      <SheetContent side="right" class="w-[560px] sm:max-w-[560px] overflow-y-auto">
        <SheetHeader>
          <SheetTitle>{{ detailTarget?.student_name ?? '评价详情' }}</SheetTitle>
          <SheetDescription>
            {{ detailTarget?.filename }} · 综合得分 {{ detailEvaluation?.total_score ?? '——' }}
          </SheetDescription>
        </SheetHeader>

        <div v-if="detailLoading" class="space-y-3 mt-4">
          <Skeleton class="h-12" />
          <Skeleton class="h-32" />
        </div>

        <div v-else-if="detailEvaluation" class="mt-4 flex flex-col gap-4">
          <!-- Action bar -->
          <div class="flex gap-2">
            <Button variant="outline" size="sm" @click="viewFullEvaluation">
              <Eye class="w-3.5 h-3.5" />
              完整评价页
            </Button>
            <Button v-if="detailTarget && detailTarget.eval_status === 'scored'" size="sm" @click="confirmEval(detailTarget)">
              确认评价
            </Button>
          </div>

          <!-- Dimensions -->
          <div>
            <h4 class="text-sm font-semibold text-ink mb-2">维度评分</h4>
            <div class="tes-table-shell border border-border rounded-md">
              <div class="grid min-w-[460px] grid-cols-[minmax(12rem,1fr)_60px_60px_60px_70px] px-3 py-2 bg-surface-2 text-[11px] font-semibold text-muted-foreground border-b border-border">
                <span>维度</span>
                <span class="text-right">权重</span>
                <span class="text-right">AI</span>
                <span class="text-right">教师</span>
                <span></span>
              </div>
              <div
                v-for="d in detailEvaluation.scores"
                :key="d.dimension_id"
                class="grid min-w-[460px] grid-cols-[minmax(12rem,1fr)_60px_60px_60px_70px] px-3 py-2.5 border-b border-border last:border-b-0 text-sm"
              >
                <span class="text-ink font-medium truncate">{{ d.dimension_name }}</span>
                <span class="text-right text-muted-foreground font-mono">{{ d.weight }}%</span>
                <span class="text-right font-mono">{{ d.obj_score }}</span>
                <span class="text-right font-mono">{{ d.subj_score ?? '—' }}</span>
                <Button variant="ghost" size="icon-sm" @click="startEditDim(d)">
                  <Edit3 class="w-3 h-3" />
                </Button>
              </div>
            </div>
          </div>

          <!-- History -->
          <div>
            <h4 class="text-sm font-semibold text-ink mb-2 flex items-center gap-1.5">
              <History class="w-3.5 h-3.5" />
              修订历史 ({{ detailHistory.length }})
            </h4>
            <div v-if="detailHistory.length === 0" class="text-xs text-muted-foreground border border-border rounded-md px-3 py-4 text-center">
              暂无修订记录
            </div>
            <div v-else class="space-y-1.5">
              <div
                v-for="h in detailHistory"
                :key="h.id"
                class="px-3 py-2 bg-surface-2 border border-border rounded-md text-xs"
              >
                <div class="flex justify-between">
                  <span class="font-mono text-primary">{{ h.action }}</span>
                  <span class="text-muted-foreground">{{ formatDate(h.changed_at) }}</span>
                </div>
                <div v-if="h.before_value || h.after_value" class="mt-1 text-muted-foreground">
                  <span v-if="h.before_value">前: <code class="font-mono">{{ h.before_value }}</code></span>
                  <span v-if="h.before_value && h.after_value"> → </span>
                  <span v-if="h.after_value">后: <code class="font-mono">{{ h.after_value }}</code></span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </SheetContent>
    </Sheet>

    <!-- Dimension edit dialog -->
    <Dialog v-model:open="showDimDialog">
      <DialogContent class="max-w-md">
        <DialogHeader>
          <DialogTitle>调整维度主观分</DialogTitle>
          <DialogDescription>
            维度「{{ editingDim?.dimension_name }}」 · 权重 {{ editingDim?.weight }}%
          </DialogDescription>
        </DialogHeader>
        <div class="flex flex-col gap-3">
          <div class="space-y-2">
            <Label>主观分（0-100）</Label>
            <Input v-model.number="dimSubjScore" type="number" min="0" max="100" />
            <p class="text-[11px] text-muted-foreground">AI 客观分 {{ editingDim?.obj_score }}，最终 = AI×60% + 教师×40%</p>
          </div>
          <div class="space-y-2">
            <Label>批注</Label>
            <Textarea v-model="dimComment" rows="3" placeholder="可选 · 解释调整原因" />
          </div>
        </div>
        <DialogFooter class="gap-2">
          <Button variant="outline" @click="showDimDialog = false">取消</Button>
          <Button :disabled="dimSubmitting" @click="submitDimUpdate">
            {{ dimSubmitting ? '保存中...' : '保存' }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </AppShell>
</template>
