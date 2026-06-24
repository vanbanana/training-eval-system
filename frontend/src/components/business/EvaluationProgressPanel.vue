<script setup lang="ts">
/**
 * 业务组件：EvaluationProgressPanel
 * 竖向展示各评分维度的真实AI评价进度。
 *
 * 从 SSE 接收 eval_dimensions 事件，实时展示每个维度的状态：
 * - evaluating: AI正在评价此维度
 * - done: 已完成，显示得分
 * - failed: 评价失败
 * - idle: 等待中
 *
 * 同时保留宏观阶段概览（已提交 → 解析 → AI评分 → 教师确认）。
 */
import { computed } from 'vue'
import {
  CheckCircle2,
  Loader2,
  AlertCircle,
  Sparkles,
  Clock,
} from 'lucide-vue-next'
import { cn } from '@/lib/utils'

interface DimensionProgress {
  id: number
  name: string
  weight: number
  status: 'evaluating' | 'done' | 'failed' | 'idle'
  score?: number
}

interface Props {
  parseStatus: string
  evalStatus: string | null
  uploadedAt?: string
  /** 从 SSE 推送的各维度进度 */
  dimensions?: DimensionProgress[]
  /** AI 评分失败原因（status=failed 时展示，指引转人工评阅） */
  failureReason?: string
  class?: string
}

const props = defineProps<Props>()

// Macro stage for top-level summary
type StageState = 'done' | 'active' | 'idle' | 'failed'

const parseState = computed<StageState>(() => {
  if (props.parseStatus === 'parsed') return 'done'
  if (props.parseStatus === 'parsing') return 'active'
  if (props.parseStatus === 'failed') return 'failed'
  return 'idle'
})

const evalState = computed<StageState>(() => {
  if (!props.evalStatus) return 'idle'
  if (['scored', 'confirmed', 'rejected'].includes(props.evalStatus)) return 'done'
  if (['pending', 'scoring'].includes(props.evalStatus)) return 'active'
  if (props.evalStatus === 'failed') return 'failed'
  return 'idle'
})

const teacherState = computed<StageState>(() => {
  if (props.evalStatus === 'confirmed' || props.evalStatus === 'rejected') return 'done'
  if (props.evalStatus === 'scored') return 'active'
  return 'idle'
})

const overallLabel = computed(() => {
  if (teacherState.value === 'done') return props.evalStatus === 'confirmed' ? '教师已确认' : '已打回修改'
  if (evalState.value === 'done') return '待教师确认'
  if (evalState.value === 'failed') return 'AI 评分失败 · 待人工评阅'
  if (evalState.value === 'active') return 'AI 评价中'
  if (parseState.value === 'active') return '文档解析中'
  if (parseState.value === 'done') return '解析完成，等待评价'
  if (parseState.value === 'failed') return '解析失败'
  return '等待处理'
})

const hasDimensions = computed(() => (props.dimensions?.length ?? 0) > 0)

function stateIcon(state: StageState) {
  if (state === 'done') return CheckCircle2
  if (state === 'active') return Loader2
  if (state === 'failed') return AlertCircle
  return Clock
}

function stateColor(state: StageState) {
  return ({
    done: 'text-success',
    active: 'text-info',
    failed: 'text-destructive',
    idle: 'text-muted-foreground',
  } as const)[state]
}

function dimStateLabel(status: DimensionProgress['status']) {
  return ({
    evaluating: '评价中',
    done: '已完成',
    failed: '失败',
    idle: '等待中',
  } as const)[status]
}

function dimStateColor(status: DimensionProgress['status']) {
  return ({
    evaluating: 'text-info',
    done: 'text-success',
    failed: 'text-destructive',
    idle: 'text-muted-foreground',
  } as const)[status]
}

function formatTime(iso: string) {
  return iso.slice(0, 16).replace('T', ' ')
}
</script>

<template>
  <div :class="cn('rounded-xl border bg-card overflow-hidden', $props.class)">
    <!-- Header -->
    <div class="px-5 py-4 border-b border-border flex items-center justify-between">
      <div class="flex items-center gap-2.5">
        <Sparkles class="w-4 h-4 text-primary" />
        <span class="text-sm font-semibold text-ink">评价进度</span>
      </div>
      <span
        :class="cn(
          'text-xs font-medium',
          evalState === 'done' || teacherState === 'done' ? 'text-success' :
          evalState === 'active' ? 'text-info' :
          evalState === 'failed' || parseState === 'failed' ? 'text-destructive' : 'text-muted-foreground'
        )"
      >
        {{ overallLabel }}
      </span>
    </div>

    <!-- Macro stages: compact horizontal summary -->
    <div class="px-5 py-3 bg-surface-2/50 border-b border-border">
      <div class="flex items-center gap-2">
        <!-- Upload: always done -->
        <div class="flex items-center gap-1.5">
          <CheckCircle2 class="w-3.5 h-3.5 text-success" />
          <span class="text-[11px] text-ink font-medium">已提交</span>
        </div>
        <div class="flex-1 h-px bg-success max-w-[2rem]"></div>

        <!-- Parse -->
        <div class="flex items-center gap-1.5">
          <component
            :is="stateIcon(parseState)"
            :class="cn('w-3.5 h-3.5', stateColor(parseState), parseState === 'active' ? 'animate-spin' : '')"
          />
          <span :class="cn('text-[11px] font-medium', parseState === 'idle' ? 'text-muted-foreground' : 'text-ink')">解析</span>
        </div>
        <div :class="cn('flex-1 h-px max-w-[2rem]', parseState === 'done' ? 'bg-success' : 'bg-border')"></div>

        <!-- Eval -->
        <div class="flex items-center gap-1.5">
          <component
            :is="stateIcon(evalState)"
            :class="cn('w-3.5 h-3.5', stateColor(evalState), evalState === 'active' ? 'animate-spin' : '')"
          />
          <span :class="cn('text-[11px] font-medium', evalState === 'idle' ? 'text-muted-foreground' : 'text-ink')">AI 评分</span>
        </div>
        <div :class="cn('flex-1 h-px max-w-[2rem]', evalState === 'done' ? 'bg-success' : 'bg-border')"></div>

        <!-- Teacher -->
        <div class="flex items-center gap-1.5">
          <component
            :is="stateIcon(teacherState)"
            :class="cn('w-3.5 h-3.5', stateColor(teacherState), teacherState === 'active' ? 'animate-spin' : '')"
          />
          <span :class="cn('text-[11px] font-medium', teacherState === 'idle' ? 'text-muted-foreground' : 'text-ink')">教师确认</span>
        </div>
      </div>
    </div>

    <!-- Per-dimension progress list -->
    <div v-if="hasDimensions" class="divide-y divide-border">
      <div
        v-for="dim in dimensions"
        :key="dim.id"
        class="px-5 py-3 flex items-center gap-3"
      >
        <!-- Status indicator -->
        <div
          :class="cn(
            'w-7 h-7 rounded-md grid place-items-center flex-shrink-0',
            dim.status === 'done' ? 'bg-success/10' :
            dim.status === 'evaluating' ? 'bg-info/10' :
            dim.status === 'failed' ? 'bg-destructive/10' : 'bg-muted/50'
          )"
        >
          <Loader2 v-if="dim.status === 'evaluating'" class="w-3.5 h-3.5 text-info animate-spin" />
          <CheckCircle2 v-else-if="dim.status === 'done'" class="w-3.5 h-3.5 text-success" />
          <AlertCircle v-else-if="dim.status === 'failed'" class="w-3.5 h-3.5 text-destructive" />
          <Clock v-else class="w-3.5 h-3.5 text-muted-foreground" />
        </div>

        <!-- Dimension info -->
        <div class="flex-1 min-w-0">
          <div class="flex items-center gap-2">
            <span class="text-xs font-semibold text-ink">{{ dim.name }}</span>
            <span class="text-[10px] text-muted-foreground">{{ dim.weight }}%</span>
          </div>
          <span :class="cn('text-[10px]', dimStateColor(dim.status))">{{ dimStateLabel(dim.status) }}</span>
        </div>

        <!-- Score (when done) -->
        <div v-if="dim.status === 'done' && dim.score != null" class="text-right">
          <span class="text-sm font-bold text-success">{{ dim.score }}</span>
          <span class="text-[10px] text-muted-foreground">/100</span>
        </div>
      </div>
    </div>

    <!-- Fallback: no dimension data yet (show simple message) -->
    <div v-else-if="evalState === 'active'" class="px-5 py-6 flex flex-col items-center gap-2">
      <Loader2 class="w-5 h-5 text-info animate-spin" />
      <span class="text-xs text-muted-foreground">AI 正在分析各评分维度...</span>
    </div>
    <div v-else-if="evalState === 'idle' && parseState !== 'failed'" class="px-5 py-4">
      <span class="text-xs text-muted-foreground">评价完成后将展示各维度得分</span>
    </div>

    <!-- AI scoring failed: clearly explain hand-off to manual teacher review -->
    <div v-if="evalState === 'failed'" class="mx-5 my-4 rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3">
      <div class="flex items-start gap-2.5">
        <AlertCircle class="w-4 h-4 text-destructive flex-shrink-0 mt-0.5" />
        <div class="min-w-0">
          <p class="text-xs font-semibold text-destructive">AI 自动评分暂时不可用</p>
          <p class="mt-1 text-[11px] leading-relaxed text-muted-foreground">
            本次提交已转交任课教师人工评阅，结果稍后可见，无需重复提交。
          </p>
          <p v-if="failureReason" class="mt-1.5 text-[10px] leading-relaxed text-muted-foreground/80 font-mono break-all">
            原因：{{ failureReason }}
          </p>
        </div>
      </div>
    </div>

    <!-- Upload time footer -->
    <div v-if="uploadedAt" class="px-5 py-2.5 bg-surface-2/30 border-t border-border">
      <span class="text-[10px] text-muted-foreground font-mono">提交于 {{ formatTime(uploadedAt) }}</span>
    </div>
  </div>
</template>
