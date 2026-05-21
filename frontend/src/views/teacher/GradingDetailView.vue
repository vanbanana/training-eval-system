<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import axios from 'axios'
import AppShell from '@/components/layout/AppShell.vue'
import BreadcrumbNav from '@/components/business/BreadcrumbNav.vue'
import EvaluationProgressPanel from '@/components/business/EvaluationProgressPanel.vue'
import RejectConfirmDialog from '@/components/business/RejectConfirmDialog.vue'
import { useToast } from '@/components/ui/toast'
import { confirm } from '@/composables/useConfirm'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Card } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import {
  ChevronLeft,
  ChevronRight,
  CheckCircle2,
  XCircle,
  FileSearch,
  Save,
  History,
} from 'lucide-vue-next'

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
  task_id: number
  student_id: number
  upload_id?: number
  total_score: number | null
  status: string
  teacher_comment: string
  created_at: string
  scores: DimensionScore[]
}

interface ParseResult {
  upload_id: number
  raw_text?: string
  pages?: number
  word_count?: number
  parse_status?: string
}

interface Submission {
  upload_id: number
  student_id: number
  student_name: string
  filename: string
  parse_status: string
  uploaded_at: string
  evaluation_id: number | null
  total_score: number | null
  eval_status: string | null
}

const route = useRoute()
const router = useRouter()
const { toast } = useToast()

const evalId = computed(() => Number(route.params.id))
const evaluation = ref<EvaluationDetail | null>(null)
const parseResult = ref<ParseResult | null>(null)
const submission = ref<Submission | null>(null)
const submissionsList = ref<Submission[]>([])
const loading = ref(true)
const teacherComment = ref('')
const subjScores = ref<Record<number, number>>({})
const submitting = ref(false)

// reject dialog
const rejectOpen = ref(false)
const rejectSubmitting = ref(false)

async function fetchAll() {
  loading.value = true
  try {
    const { data: ev } = await axios.get(`/api/evaluations/${evalId.value}`)
    evaluation.value = ev
    teacherComment.value = ev.teacher_comment ?? ''
    subjScores.value = {}
    for (const d of ev.scores ?? []) {
      subjScores.value[d.dimension_id] = d.subj_score ?? d.obj_score
    }

    // 拉解析文本（左栏）
    if (ev.upload_id) {
      try {
        const { data: pr } = await axios.get(`/api/parse/${ev.upload_id}/result`)
        parseResult.value = pr
      } catch {
        parseResult.value = null
      }
    }

    // 上下条切换：拉同任务的提交列表
    try {
      const { data: subs } = await axios.get(`/api/grading/tasks/${ev.task_id}/submissions`)
      submissionsList.value = subs
      submission.value = subs.find((s: Submission) => s.evaluation_id === evalId.value) ?? null
    } catch {
      /* ignore */
    }
  } catch (e) {
    const msg = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    toast({ description: msg ?? '加载评价失败', variant: 'destructive' })
  } finally {
    loading.value = false
  }
}

onMounted(fetchAll)

const currentIndex = computed(() => {
  if (!submission.value) return -1
  return submissionsList.value.findIndex((s) => s.evaluation_id === evalId.value)
})

const prevEvalId = computed(() => {
  const idx = currentIndex.value
  if (idx <= 0) return null
  for (let i = idx - 1; i >= 0; i--) {
    const e = submissionsList.value[i].evaluation_id
    if (e) return e
  }
  return null
})

const nextEvalId = computed(() => {
  const idx = currentIndex.value
  if (idx < 0) return null
  for (let i = idx + 1; i < submissionsList.value.length; i++) {
    const e = submissionsList.value[i].evaluation_id
    if (e) return e
  }
  return null
})

function goPrev() {
  if (prevEvalId.value) router.push(`/teacher/evaluations/${prevEvalId.value}`)
}
function goNext() {
  if (nextEvalId.value) router.push(`/teacher/evaluations/${nextEvalId.value}`)
}

const previewSubjTotal = computed(() => {
  if (!evaluation.value) return 0
  return Math.round(
    evaluation.value.scores.reduce((sum, d) => {
      const subj = subjScores.value[d.dimension_id] ?? d.obj_score
      return sum + subj * (d.weight / 100)
    }, 0),
  )
})

const previewObjTotal = computed(() => {
  if (!evaluation.value) return 0
  return Math.round(
    evaluation.value.scores.reduce((s, d) => s + d.obj_score * (d.weight / 100), 0),
  )
})

const previewFinalTotal = computed(() =>
  Math.round(previewObjTotal.value * 0.6 + previewSubjTotal.value * 0.4),
)

async function submitConfirm() {
  if (!evaluation.value) return
  const ok = await confirm({
    title: '确认评价',
    description: `综合分将记为 ${previewFinalTotal.value}（AI×60% + 教师×40%）。确认提交？`,
  })
  if (!ok) return
  submitting.value = true
  try {
    await axios.post(`/api/grading/evaluations/${evaluation.value.id}/confirm`, {
      teacher_comment: teacherComment.value,
      score_overrides: subjScores.value,
    })
    toast({ description: `已确认，综合分 ${previewFinalTotal.value}`, variant: 'success' })
    await fetchAll()
  } catch (e) {
    const msg = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    toast({ description: msg ?? '确认失败', variant: 'destructive' })
  } finally {
    submitting.value = false
  }
}

async function onReject(reason: string) {
  if (!evaluation.value) return
  rejectSubmitting.value = true
  try {
    await axios.post(`/api/grading/evaluations/${evaluation.value.id}/reject`, { reason })
    toast({ description: '已打回', variant: 'success' })
    rejectOpen.value = false
    await fetchAll()
  } catch (e) {
    const msg = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    toast({ description: msg ?? '打回失败', variant: 'destructive' })
  } finally {
    rejectSubmitting.value = false
  }
}

async function saveDraft() {
  if (!evaluation.value) return
  // 对每个改动的维度调 PATCH /api/evaluations/{id}/dimensions/{dim_id}
  submitting.value = true
  try {
    await Promise.all(
      evaluation.value.scores.map((d) => {
        const newVal = subjScores.value[d.dimension_id]
        if (newVal !== undefined && newVal !== d.subj_score) {
          return axios.patch(`/api/evaluations/${evalId.value}/dimensions/${d.dimension_id}`, {
            subj_score: newVal,
            comment: d.comment ?? '',
          })
        }
        return null
      }),
    )
    toast({ description: '草稿已保存', variant: 'success' })
  } catch (e) {
    const msg = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    toast({ description: msg ?? '保存失败', variant: 'destructive' })
  } finally {
    submitting.value = false
  }
}

const rejectTargets = computed(() =>
  submission.value
    ? [{ id: submission.value.upload_id, label: `${submission.value.student_name} 的提交` }]
    : [],
)

function openHistory() {
  if (!evaluation.value) return
  // 跳到 GradingView 的 sheet（暂时走 router push）
  router.push(`/teacher/tasks/${evaluation.value.task_id}/grading`)
}
</script>

<template>
  <AppShell>
    <BreadcrumbNav
      :items="[
        { label: '工作台', to: '/dashboard' },
        { label: '批改工作台', to: evaluation ? `/teacher/tasks/${evaluation.task_id}/grading` : '/teacher/tasks' },
        { label: '并排对比' },
      ]"
    />

    <div class="flex justify-between items-end">
      <div>
        <div class="flex items-center gap-3">
          <h1 class="text-2xl font-bold text-ink">并排对比</h1>
          <Badge v-if="submission" variant="info">{{ submission.student_name }}</Badge>
          <Badge v-if="evaluation" :variant="evaluation.status === 'confirmed' ? 'success' : evaluation.status === 'rejected' ? 'destructive' : 'warning'">
            {{ evaluation.status === 'confirmed' ? '已确认' : evaluation.status === 'rejected' ? '已打回' : '待批改' }}
          </Badge>
        </div>
        <p class="mt-1.5 text-sm text-muted-foreground">
          左栏文档原文 · 右栏 AI 评分及调整 · ← / → 切换上下条
        </p>
      </div>
      <div class="flex items-center gap-2">
        <Button variant="outline" size="icon" :disabled="!prevEvalId" @click="goPrev">
          <ChevronLeft class="w-4 h-4" />
        </Button>
        <Button variant="outline" size="icon" :disabled="!nextEvalId" @click="goNext">
          <ChevronRight class="w-4 h-4" />
        </Button>
        <Button variant="ghost" size="icon" @click="openHistory" title="查看修订历史">
          <History class="w-4 h-4" />
        </Button>
      </div>
    </div>

    <div v-if="loading" class="grid grid-cols-2 gap-4">
      <Skeleton class="h-[600px]" />
      <Skeleton class="h-[600px]" />
    </div>

    <div v-else-if="evaluation" class="grid grid-cols-2 gap-4 items-start">
      <!-- LEFT: parsed text -->
      <Card class="flex flex-col overflow-hidden h-[calc(100vh-280px)]">
        <header class="px-5 py-3.5 border-b border-border flex justify-between items-center bg-surface-2">
          <div class="flex items-center gap-2">
            <FileSearch class="w-4 h-4 text-primary" />
            <span class="text-sm font-semibold text-ink">{{ submission?.filename ?? '提交原文' }}</span>
          </div>
          <span v-if="parseResult?.pages" class="text-xs text-muted-foreground">{{ parseResult.pages }} 页</span>
        </header>
        <ScrollArea class="flex-1">
          <div class="p-5 text-sm leading-relaxed text-foreground whitespace-pre-wrap font-mono">
            <template v-if="parseResult?.raw_text">{{ parseResult.raw_text }}</template>
            <template v-else>
              <p class="text-center text-muted-foreground py-12">
                {{ parseResult ? '解析文本暂未提供（可能文件类型不支持文本提取）' : '加载中…' }}
              </p>
            </template>
          </div>
        </ScrollArea>
      </Card>

      <!-- RIGHT: AI scores + teacher input -->
      <div class="flex flex-col gap-4">
        <EvaluationProgressPanel
          v-if="submission"
          :parse-status="submission.parse_status"
          :eval-status="evaluation.status"
          :uploaded-at="submission.uploaded_at"
        />

        <Card>
          <header class="px-5 py-4 border-b border-border flex justify-between items-center">
            <span class="text-sm font-semibold text-ink">综合得分预览</span>
            <span class="text-2xl font-bold text-primary num-tabular">{{ previewFinalTotal }}</span>
          </header>
          <div class="px-5 py-3 grid grid-cols-2 gap-3 text-xs text-muted-foreground">
            <div>AI 客观分（×60%）：<span class="text-ink font-semibold">{{ previewObjTotal }}</span></div>
            <div>教师主观分（×40%）：<span class="text-ink font-semibold">{{ previewSubjTotal }}</span></div>
          </div>
        </Card>

        <Card class="overflow-hidden">
          <header class="px-5 py-4 border-b border-border">
            <span class="text-sm font-semibold text-ink">维度评分</span>
          </header>
          <div
            v-for="d in evaluation.scores"
            :key="d.dimension_id"
            class="px-5 py-3.5 border-b border-border last:border-b-0 grid grid-cols-[1fr_70px_70px_70px] items-center gap-3"
          >
            <div>
              <div class="text-sm font-semibold text-ink">{{ d.dimension_name }}</div>
              <div v-if="d.comment" class="text-[11px] text-muted-foreground mt-0.5 line-clamp-2">{{ d.comment }}</div>
            </div>
            <div class="text-center">
              <div class="text-[10px] text-muted-foreground">权重</div>
              <div class="font-mono text-xs text-foreground">{{ d.weight }}%</div>
            </div>
            <div class="text-center">
              <div class="text-[10px] text-muted-foreground">AI</div>
              <div class="font-mono text-sm font-semibold" :class="d.obj_score < 70 ? 'text-accent' : 'text-ink'">
                {{ d.obj_score }}
              </div>
            </div>
            <div>
              <Label class="text-[10px] text-muted-foreground block text-center mb-1">教师</Label>
              <Input
                v-model.number="subjScores[d.dimension_id]"
                type="number"
                min="0"
                max="100"
                class="h-8 text-center font-mono text-sm"
              />
            </div>
          </div>
        </Card>

        <Card class="p-5 flex flex-col gap-2">
          <Label class="text-sm font-semibold text-ink">教师评语</Label>
          <Textarea v-model="teacherComment" rows="3" placeholder="给学生的整体反馈和改进建议（可选）" />
        </Card>

        <div class="flex gap-3 sticky bottom-0">
          <Button variant="outline" :disabled="submitting" @click="saveDraft">
            <Save class="w-4 h-4" />
            保存草稿
          </Button>
          <Button
            variant="destructive"
            :disabled="submitting || !evaluation.id"
            @click="rejectOpen = true"
          >
            <XCircle class="w-4 h-4" />
            打回重做
          </Button>
          <Button class="flex-1" :disabled="submitting" @click="submitConfirm">
            <CheckCircle2 class="w-4 h-4" />
            确认评价并通知学生
          </Button>
        </div>
      </div>
    </div>

    <RejectConfirmDialog
      v-model:open="rejectOpen"
      :targets="rejectTargets"
      :submitting="rejectSubmitting"
      @confirm="onReject"
    />
  </AppShell>
</template>
