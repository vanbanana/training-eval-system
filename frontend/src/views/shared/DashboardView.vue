<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import axios from 'axios'
import AppShell from '@/components/layout/AppShell.vue'
import TeacherDashboard from '@/components/dashboard/TeacherDashboard.vue'
import { useAuthStore } from '@/stores/auth'
import { useCourseMap } from '@/composables/useCourseMap'
import { safeGet } from '@/lib/api-helpers'
import {
  TrendingUp,
  TrendingDown,
  Plus,
  Sparkles,
  Users,
  BookOpen,
  Award,
  AlertTriangle,
  FileText,
  FileClock,
  AlarmClock,
  BarChart3,
  RefreshCw,
} from 'lucide-vue-next'

// ============ Types ============
interface DashboardTask {
  id: number
  name: string
  status: string
  deadline: string | null
  course_id: number
  total_students?: number
  submitted?: number
  graded?: number
}

interface Notification {
  id: number
  title: string
  type: string
  is_read: boolean
  created_at: string
}

interface ScoreTrend {
  label: string
  score: number
  task_id: number
}

interface WeaknessItem {
  name: string
  score: number
}

interface ActivityDay {
  date: string
  count: number
}

interface StudentDashboard {
  role: string
  pending_tasks: { id: number; name: string; deadline: string | null; course_id: number }[]
  pending_task_count: number
  latest_score: number | null
  score_diff: number | null
  score_trend: ScoreTrend[]
  rank: number | null
  class_size: number | null
  radar_data: Record<string, number>
  weakness_list: WeaknessItem[]
  ai_used_today: number
  ai_daily_limit: number
  recent_evaluations: { id: number; task_id: number; total_score: number | null; status: string }[]
  recent_notifications: Notification[]
}

interface TeacherDashboard {
  role: string
  my_tasks: number
  pending_grading: number
  graded_this_week: number
  class_avg_score: number | null
  activity_7d: ActivityDay[]
  recent_tasks: DashboardTask[]
  recent_notifications: Notification[]
}

interface AdminDashboard {
  role: string
  user_count: number
  task_count: number
  eval_count: number
  monthly_active_students: number
  system_resources: { cpu_percent: number | null; mem_percent: number | null; disk_percent: number | null }
}

// ============ State ============
const auth = useAuthStore()
const { load: loadCourseMap, courseName } = useCourseMap()
const stats = ref<StudentDashboard | TeacherDashboard | AdminDashboard | null>(null)
const loading = ref(true)
const loadErrors = ref<string[]>([])

// Teacher extra
const classes = ref<{ id: number; name: string; is_archived: boolean; student_count?: number }[]>([])
const suspectCount = ref(0)

async function fetchAll() {
  loading.value = true
  loadErrors.value = []
  try {
    const { data } = await axios.get('/api/dashboard')
    // 确保 score_trend / pending_tasks 不为 null
    if (data) {
      if (data.score_trend == null) data.score_trend = []
      if (data.pending_tasks == null) data.pending_tasks = []
      if (data.weakness_list == null) data.weakness_list = []
      if (data.radar_data == null) data.radar_data = {}
    }
    stats.value = data

    if (auth.user?.role === 'teacher') {
      const classesResult = await safeGet<typeof classes.value>('/api/classes', [])
      if (classesResult.error) loadErrors.value.push(`班级 ${classesResult.error}`)
      classes.value = classesResult.data ?? []
      await fetchSuspectCount()
    }
  } catch (e) {
    console.error(e)
    loadErrors.value.push('仪表盘数据加载失败')
  } finally {
    loading.value = false
  }
}

async function fetchSuspectCount() {
  if (auth.user?.role !== 'teacher') return
  const tStats = stats.value as TeacherDashboard | null
  if (!tStats?.recent_tasks) return
  let total = 0
  const published = tStats.recent_tasks.filter((t) => t.status === 'published').slice(0, 8)
  await Promise.all(
    published.map(async (t) => {
      const r = await safeGet<Array<{ state: string }>>(`/api/similarity/task/${t.id}`, [])
      total += (r.data ?? []).filter((p) => p.state === 'suspect').length
    }),
  )
  suspectCount.value = total
}

onMounted(() => {
  void loadCourseMap()
  void fetchAll()
})

const role = computed(() => auth.user?.role ?? 'student')
const greeting = computed(() => {
  const h = new Date().getHours()
  if (h < 6) return '凌晨好'
  if (h < 12) return '早上好'
  if (h < 14) return '中午好'
  if (h < 18) return '下午好'
  return '晚上好'
})

// ============ Student helpers ============
const studentData = computed(() => stats.value as StudentDashboard | null)

// 分数显示统一保留 1 位小数，规避二进制浮点尾差（如 7.500000000000001）
function fmtScore(v: number | null | undefined): string {
  if (v == null) return '—'
  return (Math.round(v * 10) / 10).toString()
}
const radarEntries = computed(() => {
  if (!studentData.value?.radar_data) return []
  return Object.entries(studentData.value.radar_data).map(([name, score]) => ({ name, score }))
})
const maxRadarScore = computed(() => Math.max(100, ...radarEntries.value.map((e) => e.score)))

// ============ Teacher helpers ============
// @ts-ignore unused - kept for future teacher dashboard in this shared view
const teacherData = computed(() => stats.value as TeacherDashboard | null) // eslint-disable-line

// ============ Admin helpers ============
const adminData = computed(() => stats.value as AdminDashboard | null)

// ============ Shared helpers ============
function formatDeadline(iso: string | null): string {
  if (!iso) return '——'
  const d = new Date(iso)
  const now = new Date()
  const diff = d.getTime() - now.getTime()
  const days = Math.ceil(diff / (1000 * 60 * 60 * 24))
  if (days < 0) return '已截止'
  if (days === 0) return '今天截止'
  if (days <= 3) return `${days} 天后截止`
  return iso.slice(5, 10) + ' 截止'
}

function deadlineUrgency(iso: string | null): 'danger' | 'warning' | 'muted' {
  if (!iso) return 'muted'
  const days = Math.ceil((new Date(iso).getTime() - Date.now()) / (1000 * 60 * 60 * 24))
  if (days <= 3) return 'danger'
  if (days <= 7) return 'warning'
  return 'muted'
}

// ============ Radar chart geometry ============
const CX = 150
const CY = 150
const R = 90

function radarPoint(index: number, ratio: number): { x: number; y: number } {
  const n = radarEntries.value.length || 5
  const angle = (2 * Math.PI * index) / n - Math.PI / 2
  return {
    x: CX + R * ratio * Math.cos(angle),
    y: CY + R * ratio * Math.sin(angle),
  }
}

function radarPolygon(ratio: number): string {
  const n = radarEntries.value.length || 5
  const pts: string[] = []
  for (let i = 0; i < n; i++) {
    const p = radarPoint(i, ratio)
    pts.push(`${p.x.toFixed(1)},${p.y.toFixed(1)}`)
  }
  return pts.join(' ')
}

const radarDataPoints = computed(() => {
  return radarEntries.value
    .map((entry, i) => {
      const p = radarPoint(i, entry.score / maxRadarScore.value)
      return `${p.x.toFixed(1)},${p.y.toFixed(1)}`
    })
    .join(' ')
})

function radarLabelPos(index: number): { x: number; y: number; anchor: string } {
  const n = radarEntries.value.length || 5
  const angle = (2 * Math.PI * index) / n - Math.PI / 2
  const labelR = R + 35
  const x = CX + labelR * Math.cos(angle)
  const y = CY + labelR * Math.sin(angle)
  let anchor = 'middle'
  if (x < CX - 15) anchor = 'end'
  else if (x > CX + 15) anchor = 'start'
  return { x, y: y + 4, anchor }
}
</script>

<template>
  <AppShell>
    <!-- Load errors -->
    <div
      v-if="loadErrors.length > 0"
      class="flex flex-wrap items-center gap-2 px-4 py-2 bg-warning-soft border border-warning rounded-md"
    >
      <AlertTriangle class="w-4 h-4 text-warning" />
      <span class="text-xs text-warning font-medium">{{ loadErrors.join('、') }}</span>
      <button class="ml-auto text-xs text-warning underline" @click="fetchAll">重试</button>
    </div>

    <div v-if="loading" class="text-sm text-muted-foreground py-12 text-center">加载中...</div>

    <!-- ==================== STUDENT DASHBOARD ==================== -->
    <template v-else-if="role === 'student' && studentData">
      <!-- Header -->
      <div class="tes-page-header">
        <div class="min-w-0">
          <h1 class="tes-clamp-title text-2xl font-bold text-ink">{{ greeting }}，{{ auth.user?.display_name }}{{ (auth.user?.display_name ?? '').endsWith('同学') ? '' : ' 同学' }}</h1>
          <p class="mt-1 text-sm text-muted-foreground">
            <template v-if="studentData.rank && studentData.class_size">
              班级排名第 {{ studentData.rank }} · 共 {{ studentData.class_size }} 人 ·
            </template>
            已完成 {{ studentData.score_trend.length }} 次实训<template v-if="studentData.latest_score">，最近得分 {{ fmtScore(studentData.latest_score) }}</template>
          </p>
        </div>
        <div class="tes-page-actions">
          <RouterLink to="/student/history" class="inline-flex items-center gap-1.5 h-9 px-4 bg-surface border border-border-strong rounded-md text-sm font-semibold text-ink hover:bg-surface-2 transition-colors">
            我的评价历史
          </RouterLink>
          <RouterLink to="/student/tasks" class="inline-flex items-center gap-1.5 h-9 px-4 bg-primary text-primary-foreground rounded-md text-sm font-semibold hover:bg-primary/90 transition-colors">
            <Plus class="w-4 h-4" />
            提交实训成果
          </RouterLink>
        </div>
      </div>

      <!-- Student Stat Cards -->
      <div class="tes-grid-kpi">
        <div class="tes-card-container bg-surface border border-border rounded-lg p-5 flex flex-col gap-2.5">
          <div class="flex justify-between items-center">
            <span class="text-xs font-medium tracking-wider text-muted-foreground">待提交任务</span>
            <FileClock class="w-4 h-4 text-accent" />
          </div>
          <div class="text-3xl font-bold text-ink leading-none">{{ studentData.pending_task_count }}</div>
          <div class="flex items-center gap-1.5 text-xs font-medium text-accent">
            <AlarmClock class="w-3.5 h-3.5" />
            <span v-if="studentData.pending_tasks.length > 0">最近一项 {{ formatDeadline(studentData.pending_tasks[0]?.deadline) }}</span>
            <span v-else>暂无待提交</span>
          </div>
        </div>
        <div class="tes-card-container bg-surface border border-border rounded-lg p-5 flex flex-col gap-2.5">
          <div class="flex justify-between items-center">
            <span class="text-xs font-medium tracking-wider text-muted-foreground">近期评分</span>
            <Award class="w-4 h-4 text-subtle-foreground" />
          </div>
          <div class="text-3xl font-bold text-ink leading-none">{{ fmtScore(studentData.latest_score) }}</div>
          <div class="flex items-center gap-1.5 text-xs font-medium" :class="(studentData.score_diff ?? 0) >= 0 ? 'text-success' : 'text-danger'">
            <TrendingUp v-if="(studentData.score_diff ?? 0) >= 0" class="w-3.5 h-3.5" />
            <TrendingDown v-else class="w-3.5 h-3.5" />
            <span v-if="studentData.score_diff != null">较上次 {{ studentData.score_diff >= 0 ? '+' : '' }}{{ fmtScore(studentData.score_diff) }}</span>
            <span v-else>首次评价</span>
          </div>
        </div>
        <div class="tes-card-container bg-surface border border-border rounded-lg p-5 flex flex-col gap-2.5">
          <div class="flex justify-between items-center">
            <span class="text-xs font-medium tracking-wider text-muted-foreground">班级排名</span>
            <BarChart3 class="w-4 h-4 text-subtle-foreground" />
          </div>
          <div class="text-3xl font-bold text-ink leading-none">{{ studentData.rank ?? '—' }}</div>
          <div class="flex items-center gap-1.5 text-xs font-medium text-success">
            <TrendingUp class="w-3.5 h-3.5" />
            <span v-if="studentData.rank && studentData.class_size">前 {{ Math.round((studentData.rank / studentData.class_size) * 100) }}% / {{ studentData.class_size }} 人</span>
            <span v-else>暂无排名</span>
          </div>
        </div>
        <div class="tes-card-container bg-surface border border-border rounded-lg p-5 flex flex-col gap-2.5">
          <div class="flex justify-between items-center">
            <span class="text-xs font-medium tracking-wider text-muted-foreground">AI 助手剩余次数</span>
            <Sparkles class="w-4 h-4 text-subtle-foreground" />
          </div>
          <div class="text-3xl font-bold text-ink leading-none">{{ (studentData.ai_daily_limit ?? 0) - (studentData.ai_used_today ?? 0) }}</div>
          <div class="flex items-center gap-1.5 text-xs font-medium text-muted-foreground">
            <RefreshCw class="w-3.5 h-3.5" />
            <span>每日 {{ studentData.ai_daily_limit }} 次额度</span>
          </div>
        </div>
      </div>

      <!-- Student Main Row -->
      <div class="tes-grid-main-aside">
        <!-- LEFT: Tasks + Score Trend -->
        <div class="flex flex-col gap-[18px]">
          <!-- 待提交任务 -->
          <section class="bg-surface border border-border rounded-lg overflow-hidden">
            <header class="px-6 py-5 border-b border-border flex justify-between items-center">
              <span class="text-md font-semibold text-ink">待提交任务</span>
              <RouterLink to="/student/tasks" class="text-xs text-primary font-medium">全部 ›</RouterLink>
            </header>
            <div v-if="studentData.pending_tasks.length === 0" class="p-12 text-center text-sm text-muted-foreground">
              暂无待提交任务 🎉
            </div>
            <div
              v-for="(t, idx) in studentData.pending_tasks"
              :key="t.id"
              class="grid grid-cols-[36px_minmax(0,1fr)_minmax(8rem,17.5rem)_7.5rem] items-center gap-4 px-6 py-4 border-b border-border last:border-b-0 max-lg:grid-cols-[36px_minmax(0,1fr)_7.5rem] max-lg:[&_.deadline-copy]:hidden max-sm:grid-cols-1"
              :class="idx === 0 && deadlineUrgency(t.deadline) === 'danger' ? 'bg-accent-soft' : 'hover:bg-surface-2'"
            >
              <div class="w-9 h-9 rounded-md grid place-items-center" :class="deadlineUrgency(t.deadline) === 'danger' ? 'bg-accent-soft text-accent' : deadlineUrgency(t.deadline) === 'warning' ? 'bg-primary-soft text-primary' : 'bg-muted text-muted-foreground'">
                <FileText class="w-4 h-4" />
              </div>
              <div class="min-w-0">
                <div class="flex items-center gap-2">
                  <span class="tes-breakable text-sm font-semibold text-ink">{{ t.name }}</span>
                  <span v-if="deadlineUrgency(t.deadline) === 'danger'" class="inline-flex items-center px-2 py-0.5 rounded-full text-[10px] font-semibold bg-danger-soft text-danger">{{ formatDeadline(t.deadline) }}</span>
                </div>
                <div class="text-xs text-muted-foreground mt-1 font-mono">{{ t.deadline?.slice(0, 10) ?? '——' }} · {{ courseName(t.course_id) }}</div>
              </div>
              <div class="deadline-copy text-xs text-muted-foreground">{{ formatDeadline(t.deadline) }}</div>
              <RouterLink :to="`/student/tasks/${t.id}`" class="inline-flex items-center justify-center h-[34px] px-4 rounded-md text-sm font-semibold transition-colors" :class="idx === 0 ? 'bg-primary text-primary-foreground hover:bg-primary/90' : 'bg-surface border border-border-strong text-ink hover:bg-surface-2'">
                {{ idx === 0 ? '提交成果' : '查看' }}
              </RouterLink>
            </div>
          </section>

          <!-- 评分趋势 -->
          <section class="bg-surface border border-border rounded-lg p-6">
            <div class="flex justify-between items-center mb-[18px]">
              <div>
                <div class="text-base font-semibold text-ink">评分趋势</div>
                <div class="text-xs text-muted-foreground mt-1">最近 {{ studentData.score_trend.length }} 次实训综合得分</div>
              </div>
              <RouterLink to="/student/history" class="text-xs text-primary font-medium">查看全部 ›</RouterLink>
            </div>
            <div v-if="studentData.score_trend.length === 0" class="h-[160px] flex items-center justify-center text-sm text-muted-foreground">
              暂无评分数据
            </div>
            <div v-else class="h-[160px] flex items-end justify-between gap-3 px-1">
              <div
                v-for="(point, idx) in studentData.score_trend"
                :key="idx"
                class="flex-1 flex flex-col items-center gap-2"
              >
                <div
                  class="w-full max-w-[36px] rounded-t relative transition-[height] duration-500"
                  :class="idx === studentData.score_trend.length - 1 ? 'bg-primary' : 'bg-primary-soft'"
                  :style="{ height: `${Math.max(20, (point.score / 100) * 140)}px` }"
                >
                  <span class="absolute -top-5 left-1/2 -translate-x-1/2 text-[11px] font-semibold text-ink whitespace-nowrap">{{ point.score }}</span>
                </div>
                <span class="text-[11px] text-muted-foreground">{{ point.label }}</span>
              </div>
            </div>
          </section>
        </div>

        <!-- RIGHT: Radar + Weaknesses -->
        <div class="flex flex-col gap-[18px]">
          <!-- 个人能力雷达 -->
          <section class="bg-surface border border-border rounded-lg p-6 overflow-visible">
            <div class="flex justify-between items-center mb-3.5">
              <div>
                <div class="text-base font-semibold text-ink">个人能力雷达</div>
                <div class="text-xs text-muted-foreground mt-1">基于近 {{ studentData.score_trend.length }} 次评价数据</div>
              </div>
            </div>
            <div v-if="radarEntries.length === 0" class="h-[260px] flex items-center justify-center text-sm text-muted-foreground">
              评价数据不足
            </div>
            <div v-else class="flex items-center justify-center py-4">
              <svg viewBox="0 0 300 300" class="w-full max-w-[300px] h-auto" style="overflow: visible;">
                <!-- Background polygons -->
                <g stroke="hsl(var(--border))" fill="none" stroke-width="1">
                  <polygon :points="radarPolygon(1.0)" />
                  <polygon :points="radarPolygon(0.8)" />
                  <polygon :points="radarPolygon(0.6)" />
                  <polygon :points="radarPolygon(0.4)" />
                  <polygon :points="radarPolygon(0.2)" />
                </g>
                <!-- Axes -->
                <g stroke="hsl(var(--border))" stroke-width="1">
                  <line v-for="(_, i) in radarEntries" :key="'ax'+i" x1="150" y1="150" :x2="radarPoint(i, 1.0).x" :y2="radarPoint(i, 1.0).y" />
                </g>
                <!-- Data polygon -->
                <polygon :points="radarDataPoints" fill="hsl(var(--primary) / 0.18)" stroke="hsl(var(--primary))" stroke-width="2" />
                <g fill="hsl(var(--primary))">
                  <circle v-for="(entry, i) in radarEntries" :key="'dot'+i" :cx="radarPoint(i, entry.score / maxRadarScore).x" :cy="radarPoint(i, entry.score / maxRadarScore).y" r="3" />
                </g>
                <!-- Labels -->
                <g font-size="11" fill="hsl(var(--muted-foreground))">
                  <text v-for="(entry, i) in radarEntries" :key="'lbl'+i" :x="radarLabelPos(i).x" :y="radarLabelPos(i).y" :text-anchor="radarLabelPos(i).anchor">{{ entry.name }} {{ entry.score }}</text>
                </g>
              </svg>
            </div>
          </section>

          <!-- 薄弱点 TOP 3 -->
          <section class="bg-surface border border-border rounded-lg overflow-hidden">
            <header class="px-5 py-4 border-b border-border flex justify-between items-center">
              <span class="text-md font-semibold text-ink">薄弱点 TOP 3</span>
              <RouterLink to="/student/profile" class="text-xs text-primary font-medium">详情 ›</RouterLink>
            </header>
            <div v-if="!studentData.weakness_list?.length" class="p-8 text-center text-xs text-muted-foreground">
              评价数据不足，暂无薄弱点分析
            </div>
            <div v-else class="flex flex-col">
              <div
                v-for="(w, idx) in (studentData.weakness_list ?? [])"
                :key="idx"
                class="flex gap-3.5 px-5 py-3.5 border-b border-border last:border-b-0 hover:bg-surface-2 cursor-pointer"
              >
                <span class="w-[22px] h-[22px] bg-accent-soft text-accent font-bold text-[11px] rounded-full grid place-items-center flex-shrink-0 mt-0.5">{{ idx + 1 }}</span>
                <div class="flex-1">
                  <div class="flex justify-between items-center">
                    <span class="text-[13px] font-semibold text-ink">{{ w.name }}</span>
                    <span class="text-xs text-accent font-semibold font-mono">{{ w.score }}</span>
                  </div>
                  <div class="h-1 bg-muted rounded-full mt-1.5 overflow-hidden">
                    <div class="h-full bg-accent rounded-full" :style="{ width: `${w.score}%` }"></div>
                  </div>
                </div>
              </div>
            </div>
          </section>
        </div>
      </div>
    </template>

    <!-- ==================== TEACHER DASHBOARD ==================== -->
    <template v-else-if="role === 'teacher'">
      <TeacherDashboard />
    </template>

    <!-- ==================== ADMIN DASHBOARD ==================== -->
    <template v-else-if="role === 'admin' && adminData">
      <!-- Admin redirects to dedicated AdminDashboardView -->
      <div class="tes-page-header">
        <div class="min-w-0">
          <h1 class="tes-clamp-title text-2xl font-bold text-ink">{{ greeting }}，{{ auth.user?.display_name }}</h1>
          <p class="mt-1 text-sm text-muted-foreground">系统共有 {{ adminData.user_count }} 名用户，{{ adminData.task_count }} 个任务，{{ adminData.eval_count }} 份评价。</p>
        </div>
        <div class="tes-page-actions">
          <RouterLink to="/admin/dashboard" class="inline-flex items-center gap-1.5 h-9 px-4 bg-primary text-white rounded-md text-sm font-semibold hover:bg-primary-strong transition-colors">
            运行总览 →
          </RouterLink>
        </div>
      </div>
      <div class="tes-grid-kpi">
        <div class="tes-card-container bg-surface border border-border rounded-lg p-5 flex flex-col gap-2.5">
          <div class="flex justify-between items-center"><span class="text-xs font-medium tracking-wider text-muted-foreground">用户总数</span><Users class="w-4 h-4 text-subtle-foreground" /></div>
          <div class="text-3xl font-bold text-ink leading-none">{{ adminData.user_count }}</div>
          <div class="text-xs font-medium text-success">系统注册账号</div>
        </div>
        <div class="tes-card-container bg-surface border border-border rounded-lg p-5 flex flex-col gap-2.5">
          <div class="flex justify-between items-center"><span class="text-xs font-medium tracking-wider text-muted-foreground">实训任务</span><BookOpen class="w-4 h-4 text-subtle-foreground" /></div>
          <div class="text-3xl font-bold text-ink leading-none">{{ adminData.task_count }}</div>
          <div class="text-xs font-medium text-info">含历史任务</div>
        </div>
        <div class="tes-card-container bg-surface border border-border rounded-lg p-5 flex flex-col gap-2.5">
          <div class="flex justify-between items-center"><span class="text-xs font-medium tracking-wider text-muted-foreground">评价总数</span><Award class="w-4 h-4 text-subtle-foreground" /></div>
          <div class="text-3xl font-bold text-ink leading-none">{{ adminData.eval_count }}</div>
          <div class="text-xs font-medium text-success">系统累计</div>
        </div>
        <div class="tes-card-container bg-surface border border-border rounded-lg p-5 flex flex-col gap-2.5">
          <div class="flex justify-between items-center"><span class="text-xs font-medium tracking-wider text-muted-foreground">月活学生</span><TrendingUp class="w-4 h-4 text-subtle-foreground" /></div>
          <div class="text-3xl font-bold text-ink leading-none">{{ adminData.monthly_active_students }}</div>
          <div class="text-xs font-medium text-muted-foreground">近 30 天</div>
        </div>
      </div>
    </template>

    <!-- Fallback -->
    <template v-else-if="!loading">
      <div class="text-center py-12 text-muted-foreground">无法加载仪表盘数据</div>
    </template>
  </AppShell>
</template>
