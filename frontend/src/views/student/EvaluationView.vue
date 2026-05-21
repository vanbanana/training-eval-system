<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import axios from 'axios'
import AppShell from '@/components/layout/AppShell.vue'
import BreadcrumbNav from '@/components/business/BreadcrumbNav.vue'
import ChatDialog from '@/components/business/ChatDialog.vue'
import { useToast } from '@/components/ui/toast'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Badge } from '@/components/ui/badge'
import {
  Sparkles,
  AlertTriangle,
  Info,
  XCircle,
  CheckCircle2,
  FileDown,
  MessageSquareText,
} from 'lucide-vue-next'

interface DimensionScore {
  dimension_id: number
  ai_score: number | null
  teacher_score: number | null
  rationale: string
}
interface Evaluation {
  id: number
  task_id: number
  student_id: number
  total_score: number | null
  status: string
  teacher_comment: string
  created_at: string
  scores: DimensionScore[]
}
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
  course_id: number
  teacher_id: number
  dimensions: Dimension[]
}

const route = useRoute()
const evalId = computed(() => route.params.id as string)
const { toast } = useToast()

const evaluation = ref<Evaluation | null>(null)
const task = ref<Task | null>(null)
const loading = ref(true)
const chatRef = ref<InstanceType<typeof ChatDialog> | null>(null)

const dimMap = computed(() => {
  const m: Record<number, Dimension> = {}
  for (const d of task.value?.dimensions ?? []) m[d.id] = d
  return m
})

const displayScores = computed(() => {
  if (!evaluation.value) return []
  return evaluation.value.scores
    .map((s) => {
      const dim = dimMap.value[s.dimension_id]
      return {
        id: s.dimension_id,
        name: dim?.name ?? `维度 ${s.dimension_id}`,
        weight: dim?.weight ?? 0,
        order: dim?.order_index ?? 0,
        ai_score: s.ai_score,
        teacher_score: s.teacher_score,
        final_score: s.teacher_score ?? s.ai_score,
        rationale: s.rationale,
      }
    })
    .sort((a, b) => a.order - b.order)
})

const issues = computed(() => {
  const list: { type: 'warn' | 'info' | 'danger'; title: string; severity?: string; description: string; meta: string }[] = []
  for (const s of displayScores.value) {
    if (s.final_score === null) continue
    if (s.final_score < 60) {
      list.push({
        type: 'danger',
        title: `${s.name} 得分较低`,
        severity: '高',
        description: s.rationale || `${s.name}维度得分仅 ${s.final_score} 分，需重点改进`,
        meta: `影响维度：${s.name} -${Math.round((100 - s.final_score) * s.weight / 100)} 分`,
      })
    } else if (s.final_score < 75) {
      list.push({
        type: 'warn',
        title: `${s.name} 仍有提升空间`,
        severity: '中',
        description: s.rationale || `${s.name}维度得分 ${s.final_score} 分，可参考改进建议`,
        meta: `维度权重 ${s.weight}%`,
      })
    } else if (s.rationale) {
      list.push({
        type: 'info',
        title: `${s.name}：${s.rationale.slice(0, 30)}${s.rationale.length > 30 ? '...' : ''}`,
        description: s.rationale,
        meta: `当前得分 ${s.final_score} / 100`,
      })
    }
  }
  return list
})

const objectiveScore = computed(() => {
  const list = displayScores.value.filter((s) => s.ai_score !== null)
  if (list.length === 0) return null
  return Math.round(list.reduce((sum, s) => sum + (s.ai_score ?? 0) * s.weight / 100, 0))
})
const subjectiveScore = computed(() => {
  const list = displayScores.value.filter((s) => s.teacher_score !== null)
  if (list.length === 0) return null
  return Math.round(list.reduce((sum, s) => sum + (s.teacher_score ?? 0) * s.weight / 100, 0))
})

async function fetchAll() {
  loading.value = true
  try {
    const { data: evalData } = await axios.get(`/api/evaluations/${evalId.value}`)
    evaluation.value = evalData
    if (evalData.task_id) {
      const { data: taskData } = await axios.get(`/api/tasks/${evalData.task_id}`)
      task.value = taskData
    }
  } catch (e) {
    const msg = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    toast({ description: msg ?? '加载评价失败', variant: 'destructive' })
  } finally {
    loading.value = false
  }
}

onMounted(fetchAll)

function issueIconCmp(type: string) {
  if (type === 'warn') return AlertTriangle
  if (type === 'danger') return XCircle
  return Info
}

function issueIconColor(type: string) {
  if (type === 'warn') return 'bg-warning-soft text-warning'
  if (type === 'danger') return 'bg-danger-soft text-danger'
  return 'bg-info-soft text-info'
}

function formatScoredAt() {
  if (!evaluation.value?.created_at) return ''
  return evaluation.value.created_at.slice(0, 16).replace('T', ' ')
}

async function exportPdf() {
  try {
    toast({ description: '正在生成 PDF 报告…', variant: 'info' })
    const { data } = await axios.get(`/api/reports/personal/${evalId.value}`, { responseType: 'blob' })
    const url = URL.createObjectURL(data)
    const a = document.createElement('a')
    a.href = url
    a.download = `evaluation_${evalId.value}.pdf`
    a.click()
    URL.revokeObjectURL(url)
    toast({ description: 'PDF 已下载', variant: 'success' })
  } catch (e) {
    const msg = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    toast({ description: msg ?? '导出 PDF 失败', variant: 'destructive' })
  }
}

function dimBarColor(score: number | null): string {
  if (score === null) return 'bg-muted'
  if (score >= 85) return 'bg-success'
  if (score >= 70) return 'bg-primary'
  return 'bg-accent'
}

function scoreColor(score: number | null) {
  if (score === null) return 'text-muted-foreground'
  if (score < 70) return 'text-accent'
  return 'text-ink'
}

function openChat() {
  chatRef.value?.open()
}
</script>

<template>
  <AppShell>
    <div v-if="loading" class="space-y-3">
      <Skeleton class="h-12" />
      <Skeleton class="h-64" />
    </div>

    <template v-else-if="evaluation && task">
      <BreadcrumbNav
        :items="[
          { label: '我的任务', to: '/student/tasks' },
          { label: task.name, to: `/student/tasks/${task.id}` },
          { label: '评价结果' },
        ]"
      />

      <div class="flex justify-between items-end">
        <div>
          <h1 class="text-2xl font-bold text-ink">评价结果</h1>
          <p class="mt-1.5 text-sm text-muted-foreground">{{ task.name }} · 评价生成于 {{ formatScoredAt() }}</p>
        </div>
        <div class="flex items-center gap-3">
          <Button variant="outline" @click="exportPdf">
            <FileDown class="w-4 h-4" />
            导出 PDF 报告
          </Button>
          <Button @click="openChat">
            <Sparkles class="w-3.5 h-3.5" />
            问问 AI 助手
          </Button>
        </div>
      </div>

      <div class="flex flex-col gap-5 max-w-[1100px]">
        <!-- Score Card -->
        <Card class="p-7 flex flex-col gap-5">
          <div class="flex justify-between items-center">
            <div>
              <div class="text-xs text-muted-foreground font-medium">综合得分</div>
              <div class="text-[56px] font-bold text-ink leading-none tabular-nums num-tabular">
                {{ evaluation.total_score ?? '—' }}
                <span class="text-sm text-muted-foreground font-normal ml-1">/ 100</span>
              </div>
            </div>
            <div class="flex flex-col gap-1.5 items-end">
              <Badge v-if="evaluation.status === 'confirmed'" variant="success" class="px-3 py-1">
                <CheckCircle2 class="w-3.5 h-3.5 mr-1" />
                已确认
              </Badge>
              <Badge v-else-if="evaluation.status === 'rejected'" variant="destructive" class="px-3 py-1">
                <XCircle class="w-3.5 h-3.5 mr-1" />
                已打回
              </Badge>
              <Badge v-else variant="info" class="px-3 py-1">
                <Sparkles class="w-3.5 h-3.5 mr-1" />
                AI 已评分
              </Badge>
              <span class="text-xs text-muted-foreground">
                客观 {{ objectiveScore ?? '—' }} · 主观 {{ subjectiveScore ?? '—' }}
              </span>
            </div>
          </div>

          <div class="h-px w-full bg-border"></div>

          <!-- Dimension list with progress bars -->
          <div class="flex flex-col gap-4">
            <div
              v-for="(s, idx) in displayScores"
              :key="s.id"
              class="flex flex-col gap-2 anim-in"
              :style="{ animationDelay: idx * 60 + 'ms' }"
            >
              <div class="flex justify-between items-center">
                <div class="flex items-center gap-2">
                  <span class="text-sm font-semibold text-ink">{{ s.name }}</span>
                  <Badge variant="secondary" class="text-[10px]">权重 {{ s.weight }}%</Badge>
                </div>
                <div class="flex items-center gap-3 font-mono">
                  <span class="text-[11px] text-muted-foreground">AI {{ s.ai_score ?? '—' }}</span>
                  <span class="text-[11px] text-muted-foreground">教师 {{ s.teacher_score ?? '—' }}</span>
                  <span class="text-base font-bold" :class="scoreColor(s.final_score)">{{ s.final_score ?? '—' }} / 100</span>
                </div>
              </div>
              <div class="h-2 bg-muted rounded-pill overflow-hidden">
                <div
                  class="h-full rounded-pill transition-[width] duration-700"
                  :class="dimBarColor(s.final_score)"
                  :style="{ width: ((s.final_score ?? 0) / 100) * 100 + '%' }"
                />
              </div>
            </div>
          </div>
        </Card>

        <!-- Teacher comment -->
        <Card v-if="evaluation.teacher_comment" class="p-6 flex flex-col gap-3.5">
          <div class="flex items-center gap-2.5">
            <span class="w-8 h-8 rounded-full bg-primary-soft text-primary grid place-items-center font-semibold text-sm">师</span>
            <span class="text-sm font-semibold text-ink">教师评语</span>
            <Badge variant="default">教师评语</Badge>
          </div>
          <p class="text-sm leading-[1.85] text-foreground whitespace-pre-wrap">{{ evaluation.teacher_comment }}</p>
        </Card>

        <!-- Rationale list -->
        <Card class="overflow-hidden">
          <header class="px-6 py-4 border-b border-border flex items-center gap-2">
            <Sparkles class="w-4 h-4 text-primary" />
            <span class="text-base font-semibold text-ink">评分依据</span>
          </header>
          <div class="flex flex-col">
            <div
              v-for="s in displayScores"
              :key="s.id"
              class="px-6 py-4 border-b border-border last:border-b-0"
            >
              <div class="flex justify-between items-center mb-2">
                <span class="text-sm font-semibold text-ink">{{ s.name }}</span>
                <span class="text-sm font-bold font-mono" :class="scoreColor(s.final_score)">
                  {{ s.final_score ?? '—' }} / 100
                </span>
              </div>
              <p class="text-xs text-muted-foreground leading-relaxed">{{ s.rationale || '暂无评分说明' }}</p>
            </div>
          </div>
        </Card>

        <!-- AI Issues -->
        <Card v-if="issues.length > 0" class="overflow-hidden">
          <header class="px-6 py-4 border-b border-border flex justify-between items-center">
            <div class="flex items-center gap-2">
              <Sparkles class="w-4 h-4 text-accent" />
              <span class="text-base font-semibold text-ink">AI 智能反馈</span>
            </div>
            <span class="text-xs text-primary font-medium">共 {{ issues.length }} 条</span>
          </header>
          <div class="flex flex-col">
            <div
              v-for="(issue, idx) in issues"
              :key="idx"
              class="flex gap-3 px-6 py-4 border-b border-border last:border-b-0 anim-in"
              :style="{ animationDelay: idx * 50 + 'ms' }"
            >
              <div :class="['w-8 h-8 rounded-md grid place-items-center flex-shrink-0', issueIconColor(issue.type)]">
                <component :is="issueIconCmp(issue.type)" class="w-4 h-4" />
              </div>
              <div class="flex-1">
                <div class="text-sm font-semibold text-ink flex items-center gap-1.5">
                  <span>{{ issue.title }}</span>
                  <Badge
                    v-if="issue.severity"
                    :variant="issue.severity === '高' ? 'destructive' : 'warning'"
                  >
                    {{ issue.severity }}
                  </Badge>
                </div>
                <p class="text-xs text-muted-foreground mt-1 leading-[1.7]">{{ issue.description }}</p>
                <p class="text-[11px] text-subtle-foreground mt-1.5 font-mono">{{ issue.meta }}</p>
              </div>
              <Button variant="outline" size="sm" class="h-7 self-start" @click="openChat">
                <MessageSquareText class="w-3 h-3" />
                追问
              </Button>
            </div>
          </div>
        </Card>
      </div>

      <!-- Floating AI Chat Sidebar -->
      <ChatDialog ref="chatRef" :evaluation-id="Number(evalId)" />
    </template>
  </AppShell>
</template>
