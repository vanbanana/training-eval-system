<script setup lang="ts">
/**
 * 业务组件：EvaluationProgressPanel
 * 展示一份提交从"上传 → 解析 → 评分 → 教师确认"的 4 阶段进度。
 *
 * Props 来自 GradingView 的 submission 行；当 status 进入 finalized/confirmed/rejected 时进度满。
 * 视觉契约：frontend-preview/pages/25-parse-progress.html
 */
import { computed } from 'vue'
import { CheckCircle2, Loader2, FileSearch, Sparkles, ShieldCheck, Upload } from 'lucide-vue-next'
import { cn } from '@/lib/utils'

interface Props {
  parseStatus: string // pending | parsing | parsed | failed
  evalStatus: string | null // null | pending | scored | confirmed | rejected
  uploadedAt?: string
  class?: string
}

const props = defineProps<Props>()

interface Stage {
  key: 'uploaded' | 'parsing' | 'scoring' | 'finalized'
  label: string
  icon: typeof Upload
  state: 'done' | 'active' | 'idle' | 'failed'
  hint?: string
}

const stages = computed<Stage[]>(() => {
  const parseDone = props.parseStatus === 'parsed'
  const parseFailed = props.parseStatus === 'failed'
  const evaluating = props.evalStatus === 'pending'
  const scored = props.evalStatus === 'scored'
  const finalized = props.evalStatus === 'confirmed' || props.evalStatus === 'rejected'

  return [
    {
      key: 'uploaded',
      label: '已提交',
      icon: Upload,
      state: 'done',
      hint: props.uploadedAt ? formatTime(props.uploadedAt) : undefined,
    },
    {
      key: 'parsing',
      label: '解析文档',
      icon: FileSearch,
      state: parseFailed ? 'failed' : parseDone ? 'done' : props.parseStatus === 'parsing' ? 'active' : 'idle',
      hint: parseFailed ? '解析失败' : parseDone ? '已完成' : props.parseStatus === 'parsing' ? '正在解析…' : '等待中',
    },
    {
      key: 'scoring',
      label: 'AI 评分',
      icon: Sparkles,
      state:
        evaluating
          ? 'active'
          : scored || finalized
            ? 'done'
            : parseDone
              ? 'idle'
              : 'idle',
      hint: evaluating ? '正在评分…' : scored ? '已完成' : finalized ? '已完成' : '等待中',
    },
    {
      key: 'finalized',
      label: '教师确认',
      icon: ShieldCheck,
      state: finalized ? 'done' : scored ? 'active' : 'idle',
      hint:
        props.evalStatus === 'confirmed'
          ? '已确认'
          : props.evalStatus === 'rejected'
            ? '已打回'
            : scored
              ? '待教师审核'
              : '等待中',
    },
  ]
})

const overallPct = computed(() => {
  let done = 0
  for (const s of stages.value) if (s.state === 'done') done += 1
  return Math.round((done / stages.value.length) * 100)
})

function formatTime(iso: string) {
  return iso.slice(0, 16).replace('T', ' ')
}

function stageDot(state: Stage['state']) {
  return ({
    done: 'bg-success text-success-foreground',
    active: 'bg-info text-info-foreground animate-pulse',
    failed: 'bg-danger text-destructive-foreground',
    idle: 'bg-muted text-muted-foreground',
  } as const)[state]
}

function stageBar(state: Stage['state']) {
  return ({
    done: 'bg-success',
    active: 'bg-info',
    failed: 'bg-danger',
    idle: 'bg-border',
  } as const)[state]
}
</script>

<template>
  <section :class="cn('rounded-md bg-info-soft border border-info/30 p-5 flex flex-col gap-4', $props.class)">
    <div class="flex justify-between items-center">
      <div class="flex items-center gap-2.5">
        <div class="relative w-2 h-2 rounded-full bg-info">
          <span class="absolute inset-0 rounded-full bg-info animate-ping"></span>
        </div>
        <span class="text-sm font-semibold text-ink">评价进度</span>
      </div>
      <span class="font-mono text-xs text-muted-foreground">{{ overallPct }}% 完成</span>
    </div>

    <!-- 4 stages -->
    <div class="flex items-start">
      <template v-for="(s, idx) in stages" :key="s.key">
        <div class="flex flex-col items-center gap-2 min-w-[70px]">
          <div :class="cn('w-9 h-9 rounded-full grid place-items-center transition-colors', stageDot(s.state))">
            <Loader2 v-if="s.state === 'active'" class="w-4 h-4 animate-spin" />
            <CheckCircle2 v-else-if="s.state === 'done'" class="w-4 h-4" />
            <component v-else :is="s.icon" class="w-4 h-4" />
          </div>
          <div class="text-[11px] text-center font-semibold" :class="s.state === 'idle' ? 'text-muted-foreground' : 'text-ink'">
            {{ s.label }}
          </div>
          <div class="text-[10px] text-center text-muted-foreground line-clamp-1 max-w-[70px]">
            {{ s.hint }}
          </div>
        </div>
        <div
          v-if="idx < stages.length - 1"
          class="flex-1 h-0.5 mt-4 mx-1 rounded-pill transition-colors"
          :class="stageBar(s.state === 'done' ? 'done' : 'idle')"
        />
      </template>
    </div>
  </section>
</template>
