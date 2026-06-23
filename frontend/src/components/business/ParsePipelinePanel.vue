<script setup lang="ts">
/**
 * 业务组件：ParsePipelinePanel
 * 文档上传解析全流程可视化面板
 *
 * 展示从"上传 → 校验 → 文本提取 → 结构分析 → 完成"的详细进度。
 * 包含：
 * - 当前阶段高亮 + 动画
 * - 进度百分比条
 * - 阶段耗时/预估时间
 * - 失败重试入口
 */
import { computed, ref, watch } from 'vue'
import {
  Upload,
  FileSearch,
  FileText,
  Layers,
  CheckCircle2,
  Loader2,
  AlertCircle,
  RefreshCw,
  Clock,
} from 'lucide-vue-next'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'

interface Props {
  /** 解析状态: pending | parsing | parsed | failed */
  parseStatus: string
  /** SSE 推送的实时进度百分比 (0-100) */
  progress: number | null
  /** 上传时间 ISO */
  uploadedAt?: string
  /** 解析完成时间 ISO (如有) */
  parsedAt?: string
  /** 错误信息 (failed 时) */
  errorMessage?: string
  /** 文件名 */
  filename?: string
  /** 文件类型 */
  fileType?: string
  class?: string
}

const props = withDefaults(defineProps<Props>(), {
  progress: null,
  uploadedAt: undefined,
  parsedAt: undefined,
  errorMessage: undefined,
  filename: undefined,
  fileType: undefined,
})

const emit = defineEmits<{
  (e: 'retry'): void
}>()

interface PipelineStage {
  key: string
  label: string
  description: string
  icon: typeof Upload
  state: 'done' | 'active' | 'idle' | 'failed'
}

// Elapsed timer for active parsing
const elapsed = ref(0)
let elapsedTimer: number | null = null

watch(
  () => props.parseStatus,
  (status) => {
    if (status === 'parsing') {
      elapsed.value = 0
      elapsedTimer = window.setInterval(() => {
        elapsed.value += 1
      }, 1000)
    } else {
      if (elapsedTimer) {
        clearInterval(elapsedTimer)
        elapsedTimer = null
      }
    }
  },
  { immediate: true },
)

const stages = computed<PipelineStage[]>(() => {
  const isParsing = props.parseStatus === 'parsing'
  const isDone = props.parseStatus === 'parsed'
  const isFailed = props.parseStatus === 'failed'
  const prog = props.progress ?? 0

  return [
    {
      key: 'upload',
      label: '文件接收',
      description: '已接收',
      icon: Upload,
      state: 'done',
    },
    {
      key: 'validate',
      label: '格式校验',
      description: isDone || isFailed || (isParsing && prog > 10) ? '已通过' : isParsing ? '校验中...' : '等待中',
      icon: FileSearch,
      state: isDone || isFailed || (isParsing && prog > 10) ? 'done' : isParsing ? 'active' : 'idle',
    },
    {
      key: 'extract',
      label: '文本提取',
      description: isDone || (isParsing && prog > 60)
        ? '已完成'
        : isFailed
          ? '提取失败'
          : isParsing && prog > 10
            ? `提取中 ${Math.min(prog, 60)}%`
            : '等待中',
      icon: FileText,
      state: isDone || (isParsing && prog > 60)
        ? 'done'
        : isFailed
          ? 'failed'
          : isParsing && prog > 10
            ? 'active'
            : 'idle',
    },
    {
      key: 'analyze',
      label: '结构分析',
      description: isDone
        ? '已完成'
        : isFailed
          ? '分析失败'
          : isParsing && prog > 60
            ? `分析中 ${Math.min(prog - 40, 60)}%`
            : '等待中',
      icon: Layers,
      state: isDone
        ? 'done'
        : isFailed && prog > 60
          ? 'failed'
          : isParsing && prog > 60
            ? 'active'
            : 'idle',
    },
    {
      key: 'complete',
      label: '解析完成',
      description: isDone ? '全部完成' : isFailed ? '解析失败' : '等待中',
      icon: CheckCircle2,
      state: isDone ? 'done' : isFailed ? 'failed' : 'idle',
    },
  ]
})

const overallProgress = computed(() => {
  if (props.parseStatus === 'parsed') return 100
  if (props.parseStatus === 'failed') return props.progress ?? 0
  if (props.parseStatus === 'parsing') return props.progress ?? 15
  return 0
})

const statusLabel = computed(() => {
  switch (props.parseStatus) {
    case 'parsed':
      return '解析完成'
    case 'parsing':
      return '正在解析'
    case 'failed':
      return '解析失败'
    default:
      return '等待解析'
  }
})

const statusColor = computed(() => {
  switch (props.parseStatus) {
    case 'parsed':
      return 'text-success'
    case 'parsing':
      return 'text-info'
    case 'failed':
      return 'text-danger'
    default:
      return 'text-muted-foreground'
  }
})

function formatElapsed(sec: number) {
  if (sec < 60) return `${sec}秒`
  return `${Math.floor(sec / 60)}分${sec % 60}秒`
}

function formatDuration() {
  if (!props.uploadedAt || !props.parsedAt) return null
  const diff = new Date(props.parsedAt).getTime() - new Date(props.uploadedAt).getTime()
  if (diff < 0) return null
  const sec = Math.round(diff / 1000)
  if (sec < 60) return `${sec}秒`
  return `${Math.floor(sec / 60)}分${sec % 60}秒`
}

function stageColor(state: PipelineStage['state']) {
  return ({
    done: 'bg-success/15 text-success border-success/30',
    active: 'bg-primary/10 text-primary border-primary/30',
    failed: 'bg-destructive/10 text-destructive border-destructive/30',
    idle: 'bg-muted/50 text-muted-foreground border-border',
  } as const)[state]
}

function stageIconBg(state: PipelineStage['state']) {
  return ({
    done: 'bg-success text-white',
    active: 'bg-primary text-white',
    failed: 'bg-destructive text-white',
    idle: 'bg-muted text-muted-foreground',
  } as const)[state]
}

function connectorColor(state: PipelineStage['state']) {
  return ({
    done: 'bg-success',
    active: 'bg-primary/50',
    failed: 'bg-destructive/50',
    idle: 'bg-border',
  } as const)[state]
}
</script>

<template>
  <div :class="cn('rounded-xl border bg-card overflow-hidden', $props.class)">
    <!-- Header -->
    <div class="px-5 py-4 border-b border-border flex items-center justify-between">
      <div class="flex items-center gap-3">
        <div class="relative">
          <div
            v-if="parseStatus === 'parsing'"
            class="absolute inset-0 rounded-full bg-primary/30 animate-ping"
          ></div>
          <div
            :class="cn(
              'w-8 h-8 rounded-full grid place-items-center relative z-10',
              parseStatus === 'parsed' ? 'bg-success/15 text-success' :
              parseStatus === 'parsing' ? 'bg-primary/15 text-primary' :
              parseStatus === 'failed' ? 'bg-destructive/15 text-destructive' :
              'bg-muted text-muted-foreground'
            )"
          >
            <Loader2 v-if="parseStatus === 'parsing'" class="w-4 h-4 animate-spin" />
            <CheckCircle2 v-else-if="parseStatus === 'parsed'" class="w-4 h-4" />
            <AlertCircle v-else-if="parseStatus === 'failed'" class="w-4 h-4" />
            <FileSearch v-else class="w-4 h-4" />
          </div>
        </div>
        <div>
          <div class="flex items-center gap-2">
            <span class="text-sm font-semibold text-ink">文档解析进度</span>
            <span :class="cn('text-xs font-medium', statusColor)">{{ statusLabel }}</span>
          </div>
          <div v-if="filename" class="text-[11px] text-muted-foreground mt-0.5">
            {{ filename }}
            <span v-if="fileType" class="uppercase">· {{ fileType }}</span>
          </div>
        </div>
      </div>
      <div class="flex items-center gap-3">
        <div v-if="parseStatus === 'parsing'" class="flex items-center gap-1.5 text-xs text-muted-foreground">
          <Clock class="w-3 h-3" />
          <span>已用 {{ formatElapsed(elapsed) }}</span>
        </div>
        <div v-else-if="parseStatus === 'parsed' && formatDuration()" class="flex items-center gap-1.5 text-xs text-success">
          <Clock class="w-3 h-3" />
          <span>耗时 {{ formatDuration() }}</span>
        </div>
        <Button
          v-if="parseStatus === 'failed'"
          size="sm"
          variant="outline"
          class="text-xs"
          @click="emit('retry')"
        >
          <RefreshCw class="w-3 h-3" />
          重试
        </Button>
      </div>
    </div>

    <!-- Progress bar -->
    <div class="h-1.5 bg-muted">
      <div
        :class="cn(
          'h-full rounded-r-full transition-all duration-700 ease-out',
          parseStatus === 'failed' ? 'bg-destructive' :
          parseStatus === 'parsed' ? 'bg-success' : 'bg-primary'
        )"
        :style="{ width: `${overallProgress}%` }"
      ></div>
    </div>

    <!-- Pipeline stages -->
    <div class="px-5 py-5">
      <div class="flex items-start gap-0">
        <template v-for="(stage, idx) in stages" :key="stage.key">
          <!-- Stage node -->
          <div class="flex flex-col items-center flex-1 min-w-0">
            <div
              :class="cn(
                'w-10 h-10 rounded-full grid place-items-center border-2 transition-all duration-300',
                stageColor(stage.state)
              )"
            >
              <Loader2 v-if="stage.state === 'active'" class="w-4 h-4 animate-spin" />
              <AlertCircle v-else-if="stage.state === 'failed'" class="w-4 h-4" />
              <CheckCircle2 v-else-if="stage.state === 'done'" class="w-4 h-4" />
              <component v-else :is="stage.icon" class="w-4 h-4" />
            </div>
            <span
              class="mt-2 text-xs font-medium text-center leading-tight"
              :class="stage.state === 'idle' ? 'text-muted-foreground' : 'text-ink'"
            >
              {{ stage.label }}
            </span>
            <span class="mt-0.5 text-[10px] text-center text-muted-foreground leading-tight max-w-[80px]">
              {{ stage.description }}
            </span>
          </div>
          <!-- Connector -->
          <div
            v-if="idx < stages.length - 1"
            class="flex-shrink-0 w-8 h-0.5 mt-5 rounded-full transition-colors duration-300"
            :class="connectorColor(stage.state)"
          ></div>
        </template>
      </div>
    </div>

    <!-- Error message -->
    <div v-if="parseStatus === 'failed' && errorMessage" class="px-5 pb-4">
      <div class="flex items-start gap-2 p-3 rounded-lg bg-destructive/5 border border-destructive/20">
        <AlertCircle class="w-4 h-4 text-destructive flex-shrink-0 mt-0.5" />
        <div class="text-xs text-destructive leading-relaxed">{{ errorMessage }}</div>
      </div>
    </div>
  </div>
</template>
