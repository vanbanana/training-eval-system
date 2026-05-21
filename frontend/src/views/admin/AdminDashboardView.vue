<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import axios from 'axios'
import AppShell from '@/components/layout/AppShell.vue'
import BreadcrumbNav from '@/components/business/BreadcrumbNav.vue'
import AnimatedNumber from '@/components/business/AnimatedNumber.vue'
import { useToast } from '@/components/ui/toast'
import { safeGet } from '@/lib/api-helpers'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { ScrollArea } from '@/components/ui/scroll-area'
import {
  Users,
  Upload,
  Sparkles,
  CircleAlert,
  TrendingUp,
  TrendingDown,
  Check,
  Settings,
  RefreshCw,
} from 'lucide-vue-next'

interface SystemResources {
  cpu_percent: number | null
  mem_percent: number | null
  disk_percent: number | null
}

interface AdminDashboardData {
  user_count: number
  task_count: number
  eval_count: number
  monthly_active_students: number
  system_resources: SystemResources
}

interface AuditLog {
  id: number
  username: string
  role: string
  action: string
  target?: string | null
  ip?: string | null
  created_at: string
}

interface LogRow {
  ts: string
  level: 'info' | 'warn' | 'error' | 'debug'
  message: string
}

const router = useRouter()
const { toast } = useToast()

const data = ref<AdminDashboardData | null>(null)
const loading = ref(true)
const recentLogs = ref<LogRow[]>([])
const startTime = ref<number>(0)
const now = ref(Date.now())
let timerId: number | null = null

// 真实 QPS 折线（基于 audit 日志近 N 小时的调用计数派生）
// 每个 bucket = 1 小时；http = 非 llm/login 类的接口调用，llm = llm.* 调用计数
const qpsSeries = ref<{ http: number; llm: number; label: string }[]>([])

function buildQpsFromAudit(items: AuditLog[]): {
  http: number
  llm: number
  label: string
}[] {
  const buckets: { http: number; llm: number; label: string }[] = []
  const now = new Date()
  const HOURS = 10
  for (let i = HOURS - 1; i >= 0; i--) {
    const start = new Date(now.getTime() - i * 60 * 60 * 1000)
    buckets.push({
      http: 0,
      llm: 0,
      label: String(start.getHours()).padStart(2, '0'),
    })
  }
  for (const log of items) {
    if (!log.created_at) continue
    const t = new Date(log.created_at).getTime()
    const offsetH = Math.floor((now.getTime() - t) / (60 * 60 * 1000))
    if (offsetH < 0 || offsetH >= HOURS) continue
    const idx = HOURS - 1 - offsetH
    if (idx < 0 || idx >= buckets.length) continue
    if (/^llm\./i.test(log.action)) buckets[idx].llm += 1
    else buckets[idx].http += 1
  }
  return buckets
}

async function fetchAll() {
  loading.value = true
  auditError.value = null
  try {
    const dashRes = await axios.get<AdminDashboardData>('/api/dashboard')
    data.value = dashRes.data

    // audit 失败时显式提示，不再静默"今日活动全 0"
    const auditResult = await safeGet<{ items: AuditLog[] }>(
      '/api/audit',
      { items: [] },
      { params: { page: 1, page_size: 200 } },
    )
    if (auditResult.error) auditError.value = auditResult.error
    const items: AuditLog[] = auditResult.data.items ?? []

    recentLogs.value = items.slice(0, 10).map((log) => ({
      ts: log.created_at?.slice(11, 19) ?? '',
      level: deriveLevel(log.action),
      message: `${log.action} user=${log.username} ${log.target ? 'target=' + log.target : ''}${log.ip ? ' ip=' + log.ip : ''}`.trim(),
    }))
    qpsSeries.value = buildQpsFromAudit(items)
  } catch {
    toast({ description: '加载系统数据失败', variant: 'destructive' })
  } finally {
    loading.value = false
  }
}

function deriveLevel(action: string): LogRow['level'] {
  if (/error|fail|timeout/i.test(action)) return 'error'
  if (/warn|reject|denied|locked/i.test(action)) return 'warn'
  if (/debug/i.test(action)) return 'debug'
  return 'info'
}

const uptimeText = computed(() => {
  if (!startTime.value) return '初始化中…'
  const sec = Math.floor((now.value - startTime.value) / 1000)
  const days = Math.floor(sec / 86400)
  const hours = Math.floor((sec % 86400) / 3600)
  const mins = Math.floor((sec % 3600) / 60)
  return `在线 ${days} 天 ${String(hours).padStart(2, '0')}h ${String(mins).padStart(2, '0')}m`
})

const cpuPct = computed(() => Math.round(data.value?.system_resources.cpu_percent ?? 0))
const memPct = computed(() => Math.round(data.value?.system_resources.mem_percent ?? 0))
const diskPct = computed(() => Math.round(data.value?.system_resources.disk_percent ?? 0))

function colorBar(pct: number): string {
  if (pct >= 85) return 'bg-danger'
  if (pct >= 60) return 'bg-accent'
  if (pct >= 40) return 'bg-warning'
  return 'bg-success'
}

const onlineUsers = computed(() => data.value?.monthly_active_students ?? 0)

const todayUploads = ref(0)
const todayLLMCalls = ref(0)
const todayErrors = ref(0)
const auditError = ref<string | null>(null)

async function refreshActivity() {
  // 通过 audit 派生今日活动数
  const r = await safeGet<{ items: AuditLog[] }>(
    '/api/audit',
    { items: [] },
    { params: { page: 1, page_size: 100 } },
  )
  if (r.error) auditError.value = r.error
  const items = r.data.items ?? []
  const today = new Date().toISOString().slice(0, 10)
  const todays = items.filter((l) => l.created_at?.startsWith(today))
  todayUploads.value = todays.filter((l) => l.action.startsWith('upload.')).length
  todayLLMCalls.value = todays.filter((l) => l.action.startsWith('llm.')).length
  todayErrors.value = todays.filter((l) => /error|fail|timeout/i.test(l.action)).length
}

onMounted(async () => {
  startTime.value = Date.now()
  await fetchAll()
  await refreshActivity()
  timerId = window.setInterval(() => {
    now.value = Date.now()
  }, 30_000)
})
onUnmounted(() => {
  if (timerId) clearInterval(timerId)
})

function levelBadgeClass(level: LogRow['level']): string {
  return ({
    info: 'bg-info-soft text-info',
    warn: 'bg-warning-soft text-warning',
    error: 'bg-danger-soft text-danger',
    debug: 'bg-muted text-muted-foreground',
  } as const)[level]
}

const maxQps = computed(() => {
  let max = 1
  for (const p of qpsSeries.value) max = Math.max(max, p.http + p.llm)
  return max
})

function bar(pct: number): string {
  return `${Math.min(100, Math.max(0, pct))}%`
}
</script>

<template>
  <AppShell>
    <BreadcrumbNav
      :items="[
        { label: '管理控制台', to: '/dashboard' },
        { label: '运行总览' },
      ]"
    />

    <div class="flex justify-between items-end">
      <div>
        <h1 class="text-2xl font-bold text-ink">运行总览 · 系统健康度</h1>
        <p class="mt-1.5 text-sm text-muted-foreground">实时监测系统资源、用户活跃与 LLM 服务调用</p>
      </div>
      <div class="flex items-center gap-3">
        <span class="inline-flex items-center gap-1.5 px-3 py-1.5 bg-success-soft rounded-pill">
          <span class="w-2 h-2 rounded-full bg-success animate-pulse"></span>
          <span class="font-mono text-xs font-semibold text-success">{{ uptimeText }}</span>
        </span>
        <Button variant="outline" @click="router.push('/admin/llm')">
          <Settings class="w-4 h-4" />
          系统设置
        </Button>
        <Button variant="ghost" size="icon" @click="fetchAll">
          <RefreshCw class="w-4 h-4" />
        </Button>
      </div>
    </div>

    <!-- audit 加载失败提示 -->
    <div
      v-if="auditError"
      class="flex flex-wrap items-center gap-2 px-4 py-2 bg-warning-soft border border-warning rounded-md"
    >
      <CircleAlert class="w-4 h-4 text-warning" />
      <span class="text-xs text-warning font-medium">
        审计日志 {{ auditError }} · "今日活动" 与 QPS 折线可能为空
      </span>
      <button
        class="ml-auto text-xs text-warning underline hover:opacity-80"
        @click="fetchAll(); refreshActivity()"
      >
        重试
      </button>
    </div>

    <!-- KPI -->
    <div v-if="loading" class="grid grid-cols-4 gap-[18px]">
      <Skeleton v-for="n in 4" :key="n" class="h-28" />
    </div>
    <div v-else class="grid grid-cols-4 gap-[18px]">
      <Card class="anim-in" :style="{ animationDelay: '0ms' }">
        <CardContent class="p-5 flex flex-col gap-2.5">
          <div class="flex justify-between items-center">
            <span class="text-xs font-medium tracking-wider text-muted-foreground">在线用户</span>
            <Users class="w-4 h-4 text-subtle-foreground" />
          </div>
          <div class="text-3xl font-bold text-ink leading-none">
            <AnimatedNumber :value="onlineUsers" />
          </div>
          <div class="flex items-center gap-1.5 text-xs font-medium text-success">
            <TrendingUp class="w-3.5 h-3.5" />
            <span>近 30 日活跃学生</span>
          </div>
        </CardContent>
      </Card>
      <Card class="anim-in" :style="{ animationDelay: '50ms' }">
        <CardContent class="p-5 flex flex-col gap-2.5">
          <div class="flex justify-between items-center">
            <span class="text-xs font-medium tracking-wider text-muted-foreground">今日提交</span>
            <Upload class="w-4 h-4 text-subtle-foreground" />
          </div>
          <div class="text-3xl font-bold text-ink leading-none">
            <AnimatedNumber :value="todayUploads" />
          </div>
          <div class="flex items-center gap-1.5 text-xs font-medium text-muted-foreground">
            <Check class="w-3.5 h-3.5" />
            <span>来自审计日志聚合</span>
          </div>
        </CardContent>
      </Card>
      <Card class="anim-in" :style="{ animationDelay: '100ms' }">
        <CardContent class="p-5 flex flex-col gap-2.5">
          <div class="flex justify-between items-center">
            <span class="text-xs font-medium tracking-wider text-muted-foreground">LLM 调用</span>
            <Sparkles class="w-4 h-4 text-subtle-foreground" />
          </div>
          <div class="text-3xl font-bold text-ink leading-none">
            <AnimatedNumber :value="todayLLMCalls" />
          </div>
          <div class="flex items-center gap-1.5 text-xs font-medium text-success">
            <TrendingUp class="w-3.5 h-3.5" />
            <span>详见 LLM 配置页</span>
          </div>
        </CardContent>
      </Card>
      <Card class="anim-in" :style="{ animationDelay: '150ms' }">
        <CardContent class="p-5 flex flex-col gap-2.5">
          <div class="flex justify-between items-center">
            <span class="text-xs font-medium tracking-wider text-muted-foreground">今日错误</span>
            <CircleAlert class="w-4 h-4 text-accent" />
          </div>
          <div class="text-3xl font-bold leading-none" :class="todayErrors > 0 ? 'text-danger' : 'text-ink'">
            <AnimatedNumber :value="todayErrors" />
          </div>
          <div class="flex items-center gap-1.5 text-xs font-medium" :class="todayErrors === 0 ? 'text-success' : 'text-danger'">
            <TrendingDown v-if="todayErrors === 0" class="w-3.5 h-3.5" />
            <CircleAlert v-else class="w-3.5 h-3.5" />
            <span>{{ todayErrors === 0 ? '系统平稳运行' : '需检查日志' }}</span>
          </div>
        </CardContent>
      </Card>
    </div>

    <!-- Resources -->
    <div v-if="!loading" class="grid grid-cols-4 gap-[18px]">
      <Card class="p-6 flex flex-col gap-3.5 anim-in" :style="{ animationDelay: '50ms' }">
        <span class="text-sm font-medium text-muted-foreground">CPU 使用率</span>
        <span class="text-[32px] font-bold text-ink leading-none">{{ cpuPct }}%</span>
        <div class="h-2 bg-muted rounded-pill overflow-hidden">
          <div class="h-full rounded-pill transition-[width] duration-700" :class="colorBar(cpuPct)" :style="{ width: bar(cpuPct) }" />
        </div>
        <div class="flex justify-between text-[11px] text-muted-foreground">
          <span>4 核 LoongArch</span>
          <span>实时</span>
        </div>
      </Card>
      <Card class="p-6 flex flex-col gap-3.5 anim-in" :style="{ animationDelay: '100ms' }">
        <span class="text-sm font-medium text-muted-foreground">内存使用</span>
        <span class="text-[32px] font-bold text-ink leading-none">{{ memPct }}%</span>
        <div class="h-2 bg-muted rounded-pill overflow-hidden">
          <div class="h-full rounded-pill transition-[width] duration-700" :class="colorBar(memPct)" :style="{ width: bar(memPct) }" />
        </div>
        <div class="flex justify-between text-[11px] text-muted-foreground">
          <span>{{ memPct }}% 已使用</span>
          <span>余量 {{ Math.max(0, 100 - memPct) }}%</span>
        </div>
      </Card>
      <Card class="p-6 flex flex-col gap-3.5 anim-in" :style="{ animationDelay: '150ms' }">
        <span class="text-sm font-medium text-muted-foreground">磁盘使用</span>
        <span class="text-[32px] font-bold text-ink leading-none">{{ diskPct }}%</span>
        <div class="h-2 bg-muted rounded-pill overflow-hidden">
          <div class="h-full rounded-pill transition-[width] duration-700" :class="colorBar(diskPct)" :style="{ width: bar(diskPct) }" />
        </div>
        <div class="flex justify-between text-[11px] text-muted-foreground">
          <span>{{ diskPct }}% 已使用</span>
          <span>含备份分区</span>
        </div>
      </Card>
      <Card class="p-6 flex flex-col gap-3.5 anim-in" :style="{ animationDelay: '200ms' }">
        <span class="text-sm font-medium text-muted-foreground">实体规模</span>
        <span class="text-[32px] font-bold text-ink leading-none">
          <AnimatedNumber :value="data?.task_count ?? 0" />
        </span>
        <div class="h-2 bg-muted rounded-pill overflow-hidden">
          <div class="h-full rounded-pill bg-info" style="width: 100%" />
        </div>
        <div class="flex justify-between text-[11px] text-muted-foreground">
          <span>用户 {{ data?.user_count ?? 0 }}</span>
          <span>评价 {{ data?.eval_count ?? 0 }}</span>
        </div>
      </Card>
    </div>

    <!-- Chart + Log -->
    <div v-if="!loading" class="grid grid-cols-[1fr_440px] gap-[18px]">
      <!-- QPS Chart -->
      <Card class="p-6">
        <div class="flex justify-between items-center mb-[18px]">
          <div>
            <div class="text-base font-semibold text-ink">近 10 小时调用数（按 1 小时聚合）</div>
            <div class="text-xs text-muted-foreground mt-1">来源：审计日志中 HTTP 与 LLM 调用计数 · 与右侧"今日活动"卡片同源</div>
          </div>
          <div class="flex gap-3.5 text-[11px]">
            <span class="flex items-center gap-1.5 text-muted-foreground">
              <span class="w-2.5 h-2.5 rounded-sm bg-primary" />
              <span>HTTP</span>
            </span>
            <span class="flex items-center gap-1.5 text-muted-foreground">
              <span class="w-2.5 h-2.5 rounded-sm bg-accent" />
              <span>LLM</span>
            </span>
          </div>
        </div>
        <div v-if="qpsSeries.length === 0 || maxQps <= 1" class="h-[140px] flex items-center justify-center text-sm text-muted-foreground">
          暂无审计日志数据
        </div>
        <div v-else class="h-[140px] flex items-end justify-between gap-1">
          <div
            v-for="(p, idx) in qpsSeries"
            :key="idx"
            class="flex-1 max-w-[60px] flex flex-col-reverse gap-0.5 h-[120px]"
            :title="`${p.label}:00 · HTTP ${p.http} · LLM ${p.llm}`"
          >
            <div
              class="w-full bg-primary transition-[height] duration-500"
              :style="{ height: `${(p.http / maxQps) * 120}px` }"
            ></div>
            <div
              class="w-full bg-accent rounded-t-sm transition-[height] duration-500"
              :style="{ height: `${(p.llm / maxQps) * 120}px` }"
            ></div>
          </div>
        </div>
      </Card>

      <!-- System Log -->
      <Card class="overflow-hidden flex flex-col">
        <header class="px-5 py-[18px] border-b border-border flex justify-between items-center">
          <span class="text-sm font-semibold text-ink">系统日志</span>
          <button class="text-xs text-primary font-medium hover:underline" @click="router.push('/admin/audit')">
            实时 →
          </button>
        </header>
        <ScrollArea class="max-h-[420px]">
          <div v-if="recentLogs.length === 0" class="px-5 py-8 text-center text-xs text-muted-foreground">
            暂无日志
          </div>
          <div
            v-for="(log, idx) in recentLogs"
            :key="idx"
            class="px-5 py-3 border-b border-border last:border-b-0 flex flex-col gap-1 anim-in"
            :style="{ animationDelay: Math.min(idx * 30, 200) + 'ms' }"
          >
            <div class="flex items-center gap-2 font-mono text-[11px] text-muted-foreground">
              <span :class="['px-1.5 py-px rounded-sm font-bold text-[10px]', levelBadgeClass(log.level)]">
                {{ log.level.toUpperCase() }}
              </span>
              <span>{{ log.ts }}</span>
            </div>
            <div class="font-mono text-[11px] text-foreground leading-relaxed break-all">{{ log.message }}</div>
          </div>
        </ScrollArea>
      </Card>
    </div>
  </AppShell>
</template>
