<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import axios from 'axios'
import AppShell from '@/components/layout/AppShell.vue'
import BreadcrumbNav from '@/components/business/BreadcrumbNav.vue'
import EmptyState from '@/components/business/EmptyState.vue'
import AnimatedNumber from '@/components/business/AnimatedNumber.vue'
import { useToast } from '@/components/ui/toast'
import { Card } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { BarChart3, GraduationCap, Building2, Sparkles, TriangleAlert } from 'lucide-vue-next'

interface Course {
  id: number
  name: string
  code: string
}
interface DimStat {
  name: string
  average_score: number
  weak_student_ratio?: number
}
interface ClassBrief {
  class_id: number
  class_name: string
  avg_score: number
  student_count: number
}

const { toast } = useToast()
const scope = ref<'school' | 'course'>('school')
const range = ref<'30d' | '90d' | '1y' | 'all'>('90d')
const courses = ref<Course[]>([])
const selectedCourseId = ref<number | null>(null)
const data = ref<Record<string, unknown> | null>(null)
const loading = ref(true)
const summaryText = ref('')
const summaryLoading = ref(false)

async function loadCourses() {
  try {
    const { data: cs } = await axios.get('/api/courses')
    courses.value = cs
    if (cs.length > 0 && !selectedCourseId.value) {
      selectedCourseId.value = cs[0].id
    }
  } catch {
    /* ignore */
  }
}

function profileUrl() {
  return scope.value === 'school' ? '/api/profiles/school' : `/api/profiles/course/${selectedCourseId.value}`
}

async function fetchSummary() {
  // The AI summary is slow (LLM call), so it is loaded separately after the stats render.
  summaryText.value = ''
  if (scope.value === 'course' && !selectedCourseId.value) return
  summaryLoading.value = true
  const url = profileUrl()
  try {
    const { data: d } = await axios.get(url, { params: { range: range.value, summary: 1 } })
    summaryText.value = typeof d?.llm_summary === 'string' ? d.llm_summary : ''
  } catch {
    /* summary is best-effort */
  } finally {
    summaryLoading.value = false
  }
}

async function fetchProfile() {
  loading.value = true
  data.value = null
  if (scope.value === 'course' && !selectedCourseId.value) {
    loading.value = false
    return
  }
  try {
    const { data: d } = await axios.get(profileUrl(), { params: { range: range.value } })
    data.value = d
  } catch (e) {
    const msg = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    toast({ description: msg ?? '加载画像失败', variant: 'destructive' })
  } finally {
    loading.value = false
  }
  fetchSummary()
}

onMounted(async () => {
  await loadCourses()
  await fetchProfile()
})

watch([scope, range, selectedCourseId], fetchProfile)

const num = (v: unknown) => (typeof v === 'number' && Number.isFinite(v) ? v : 0)

const totalStudents = computed(() => num(data.value?.total_students))
const evalCount = computed(() => num(data.value?.eval_count))
const avgScore = computed(() => num(data.value?.average_score))
const adoptionRate = computed(() => num(data.value?.adoption_rate))
const completionRate = computed(() => num(data.value?.completion_rate))
const llmSummary = computed(() => summaryText.value)

const distribution = computed<number[]>(() => {
  const d = data.value?.score_distribution
  return Array.isArray(d) ? d.map((n) => Number(n) || 0) : []
})
const distLabels = ['0-59', '60-69', '70-79', '80-89', '90-100']
const distMax = computed(() => Math.max(1, ...distribution.value))
const distTotal = computed(() => distribution.value.reduce((a, b) => a + b, 0))
const distColors = ['bg-accent', 'bg-accent/70', 'bg-primary/60', 'bg-primary', 'bg-success']

const dimensions = computed<DimStat[]>(() => {
  const arr = data.value?.top_dimensions
  if (!Array.isArray(arr)) return []
  return (arr as DimStat[])
    .map((d) => ({
      name: d.name,
      average_score: Number(d.average_score) || 0,
      weak_student_ratio: Number(d.weak_student_ratio) || 0,
    }))
    .sort((a, b) => b.average_score - a.average_score)
})

const classComparisons = computed<ClassBrief[]>(() => {
  const arr = data.value?.class_comparisons
  if (!Array.isArray(arr)) return []
  return (arr as ClassBrief[])
    .map((c) => ({
      class_id: c.class_id,
      class_name: c.class_name,
      avg_score: Number(c.avg_score) || 0,
      student_count: Number(c.student_count) || 0,
    }))
    .filter((c) => c.student_count > 0)
    .sort((a, b) => b.avg_score - a.avg_score)
})

const weaknesses = computed<string[]>(() => {
  const arr = data.value?.recommend_teaching_for
  return Array.isArray(arr) ? (arr as string[]) : []
})

// Radar chart geometry for the (up to 8) dimensions.
const RADAR_SIZE = 260
const RADAR_R = 96
const radarDims = computed(() => dimensions.value.slice(0, 8))
function radarPoint(score: number, i: number, n: number, ratio = 1) {
  const cx = RADAR_SIZE / 2
  const cy = RADAR_SIZE / 2
  const angle = -Math.PI / 2 + (i * 2 * Math.PI) / n
  const r = RADAR_R * Math.max(0, Math.min(1, score / 100)) * ratio
  return { x: cx + r * Math.cos(angle), y: cy + r * Math.sin(angle) }
}
const radarPolygon = computed(() => {
  const n = radarDims.value.length
  if (n < 3) return ''
  return radarDims.value
    .map((d, i) => {
      const p = radarPoint(d.average_score, i, n)
      return `${p.x.toFixed(1)},${p.y.toFixed(1)}`
    })
    .join(' ')
})
const radarAxes = computed(() => {
  const n = radarDims.value.length
  return radarDims.value.map((d, i) => {
    const outer = radarPoint(100, i, n)
    const label = radarPoint(118, i, n, 1)
    return { x2: outer.x, y2: outer.y, lx: label.x, ly: label.y, name: d.name, score: d.average_score }
  })
})
const radarRings = [0.25, 0.5, 0.75, 1]
function ringPolygon(ratio: number) {
  const n = radarDims.value.length
  if (n < 3) return ''
  return radarDims.value
    .map((_, i) => {
      const p = radarPoint(100, i, n, ratio)
      return `${p.x.toFixed(1)},${p.y.toFixed(1)}`
    })
    .join(' ')
}

const scopeLabel = computed(() =>
  scope.value === 'school' ? '全校' : typeof data.value?.course_name === 'string' ? data.value.course_name : '课程',
)
function scoreColor(s: number) {
  return s >= 85 ? 'bg-success' : s >= 70 ? 'bg-primary' : 'bg-accent'
}
</script>

<template>
  <AppShell>
    <BreadcrumbNav :items="[{ label: '工作台', to: '/dashboard' }, { label: '教学画像' }]" />

    <div class="tes-page-header">
      <div class="min-w-0">
        <h1 class="tes-clamp-title text-2xl font-bold text-ink">教学画像</h1>
        <p class="mt-1.5 text-sm text-muted-foreground">学校 / 课程级教学质量综合分析</p>
      </div>
      <div class="tes-page-actions">
        <div class="space-y-1">
          <Label class="text-[11px] text-muted-foreground">时间范围</Label>
          <Select v-model="range">
            <SelectTrigger class="w-32"><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem value="30d">近 30 天</SelectItem>
              <SelectItem value="90d">近 90 天</SelectItem>
              <SelectItem value="1y">近 1 年</SelectItem>
              <SelectItem value="all">全部</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>
    </div>

    <Card class="tes-card-container px-5 py-3.5 flex flex-wrap items-center gap-4">
      <Tabs v-model="scope" class="min-w-0">
        <TabsList>
          <TabsTrigger value="school">
            <Building2 class="w-3.5 h-3.5" />
            学校级
          </TabsTrigger>
          <TabsTrigger value="course">
            <GraduationCap class="w-3.5 h-3.5" />
            课程级
          </TabsTrigger>
        </TabsList>
      </Tabs>

      <div v-if="scope === 'course'" class="flex min-w-0 flex-wrap items-center gap-2">
        <Label class="text-xs">课程：</Label>
        <Select v-model="selectedCourseId">
          <SelectTrigger class="w-64"><SelectValue placeholder="选择课程" /></SelectTrigger>
          <SelectContent>
            <SelectItem v-for="c in courses" :key="c.id" :value="c.id"> {{ c.name }}（{{ c.code }}） </SelectItem>
          </SelectContent>
        </Select>
      </div>
    </Card>

    <div v-if="loading" class="flex flex-col gap-5">
      <div class="tes-grid-kpi">
        <Skeleton v-for="n in 4" :key="n" class="h-28" />
      </div>
      <Skeleton class="h-72" />
    </div>

    <EmptyState
      v-else-if="!data || (evalCount === 0 && totalStudents === 0)"
      :icon="BarChart3"
      title="暂无评价数据"
      description="该范围内尚无可分析的评价记录"
    />

    <div v-else class="flex flex-col gap-5">
      <!-- KPIs -->
      <div class="tes-grid-kpi">
        <Card
          v-for="(kpi, i) in [
            { label: '学生总数', value: totalStudents, color: 'text-ink' },
            { label: '评价总数', value: evalCount, color: 'text-ink' },
            { label: '平均分', value: avgScore, color: 'text-primary', decimals: 1 },
            { label: '完成率', value: completionRate, color: 'text-success', suffix: '%', decimals: 1 },
          ]"
          :key="kpi.label"
          class="tes-card-container p-5 anim-in"
          :style="{ animationDelay: i * 50 + 'ms' }"
        >
          <div class="text-xs text-muted-foreground">{{ kpi.label }}</div>
          <div class="text-3xl font-bold mt-1 num-tabular" :class="kpi.color">
            <AnimatedNumber :value="kpi.value" :decimals="kpi.decimals ?? 0" :suffix="kpi.suffix ?? ''" />
          </div>
        </Card>
      </div>

      <div class="grid grid-cols-1 lg:grid-cols-2 gap-5">
        <!-- Score distribution column chart -->
        <Card v-if="distribution.length > 0" class="tes-card-container p-6 anim-in">
          <div class="flex items-center justify-between mb-5">
            <span class="text-sm font-semibold text-ink">成绩分布</span>
            <span class="text-xs text-muted-foreground">{{ scopeLabel }} · 共 {{ distTotal }} 份</span>
          </div>
          <div class="flex items-end gap-3 h-44">
            <div
              v-for="(v, i) in distribution"
              :key="i"
              class="flex-1 flex flex-col items-center gap-2 h-full justify-end"
            >
              <span class="text-xs font-mono font-semibold text-ink">{{ v }}</span>
              <div
                class="w-full rounded-t-md transition-all duration-700"
                :class="distColors[i]"
                :style="{ height: Math.max(2, (v / distMax) * 100) + '%' }"
              ></div>
              <span class="text-[11px] text-muted-foreground">{{ distLabels[i] }}</span>
            </div>
          </div>
        </Card>

        <!-- Dimension radar -->
        <Card v-if="radarDims.length >= 3" class="tes-card-container p-6 anim-in">
          <div class="flex items-center justify-between mb-2">
            <span class="text-sm font-semibold text-ink">维度能力雷达</span>
            <span class="text-xs text-muted-foreground">{{ radarDims.length }} 个维度</span>
          </div>
          <div class="flex justify-center">
            <svg :viewBox="`0 0 ${RADAR_SIZE} ${RADAR_SIZE}`" class="w-full max-w-[320px] h-auto overflow-visible">
              <polygon
                v-for="r in radarRings"
                :key="r"
                :points="ringPolygon(r)"
                fill="none"
                stroke="hsl(var(--border))"
                stroke-width="1"
              />
              <line
                v-for="(a, i) in radarAxes"
                :key="'ax' + i"
                :x1="RADAR_SIZE / 2"
                :y1="RADAR_SIZE / 2"
                :x2="a.x2"
                :y2="a.y2"
                stroke="hsl(var(--border))"
                stroke-width="1"
              />
              <polygon
                :points="radarPolygon"
                fill="hsl(var(--primary) / 0.18)"
                stroke="hsl(var(--primary))"
                stroke-width="2"
              />
              <text
                v-for="(a, i) in radarAxes"
                :key="'lb' + i"
                :x="a.lx"
                :y="a.ly"
                :text-anchor="a.lx > RADAR_SIZE / 2 + 4 ? 'start' : a.lx < RADAR_SIZE / 2 - 4 ? 'end' : 'middle'"
                dominant-baseline="middle"
                class="fill-muted-foreground"
                style="font-size: 9px"
              >
                {{ a.name }}
              </text>
            </svg>
          </div>
        </Card>
      </div>

      <!-- Dimension averages bars -->
      <Card v-if="dimensions.length > 0" class="tes-card-container overflow-hidden anim-in">
        <header class="px-5 py-4 border-b border-border flex justify-between items-center">
          <span class="text-sm font-semibold text-ink">各维度平均分</span>
          <span class="text-xs text-muted-foreground">{{ dimensions.length }} 个维度</span>
        </header>
        <div class="max-h-[26rem] overflow-y-auto">
          <div
            v-for="(d, idx) in dimensions"
            :key="d.name"
            class="px-5 py-3.5 border-b border-border last:border-b-0 flex items-center gap-3.5"
          >
            <span class="w-[min(10rem,35vw)] text-sm text-ink font-medium shrink-0 truncate">{{ d.name }}</span>
            <div class="flex-1 h-1.5 bg-muted rounded-full overflow-hidden">
              <div
                class="h-full rounded-full transition-all duration-700"
                :class="scoreColor(d.average_score)"
                :style="{ width: d.average_score + '%', transitionDelay: Math.min(idx * 30, 200) + 'ms' }"
              ></div>
            </div>
            <span class="w-[80px] text-right text-sm font-semibold font-mono text-ink">
              <AnimatedNumber :value="d.average_score" :decimals="1" /> / 100
            </span>
          </div>
        </div>
      </Card>

      <!-- Class comparison (course scope) -->
      <Card v-if="classComparisons.length > 0" class="tes-card-container overflow-hidden anim-in">
        <header class="px-5 py-4 border-b border-border flex justify-between items-center">
          <span class="text-sm font-semibold text-ink">班级对比</span>
          <span class="text-xs text-muted-foreground">{{ classComparisons.length }} 个班级</span>
        </header>
        <div
          v-for="c in classComparisons"
          :key="c.class_id"
          class="px-5 py-3.5 border-b border-border last:border-b-0 flex items-center gap-3.5"
        >
          <span class="w-[min(12rem,40vw)] text-sm text-ink font-medium shrink-0 truncate">{{ c.class_name }}</span>
          <span class="text-[11px] text-muted-foreground shrink-0 w-12">{{ c.student_count }} 人</span>
          <div class="flex-1 h-1.5 bg-muted rounded-full overflow-hidden">
            <div
              class="h-full rounded-full transition-all duration-700"
              :class="scoreColor(c.avg_score)"
              :style="{ width: c.avg_score + '%' }"
            ></div>
          </div>
          <span class="w-[64px] text-right text-sm font-semibold font-mono text-ink">
            <AnimatedNumber :value="c.avg_score" :decimals="1" />
          </span>
        </div>
      </Card>

      <div class="grid grid-cols-1 lg:grid-cols-2 gap-5">
        <!-- Adoption rate gauge -->
        <Card class="tes-card-container p-6 anim-in">
          <div class="flex justify-between items-center mb-3">
            <span class="text-sm font-semibold text-ink">教师采纳率</span>
            <span class="text-sm font-mono text-success">{{ adoptionRate.toFixed(1) }}%</span>
          </div>
          <div class="h-2 bg-muted rounded-full overflow-hidden">
            <div
              class="h-full bg-success rounded-full transition-all duration-1000"
              :style="{ width: adoptionRate + '%' }"
            ></div>
          </div>
          <p class="mt-2 text-xs text-muted-foreground">
            指 AI 评分被教师直接确认（未手动改分）的比例。高采纳率表示 AI 评分质量稳定。
          </p>
        </Card>

        <!-- Recommended teaching focus -->
        <Card class="tes-card-container p-6 anim-in">
          <div class="flex items-center gap-2 mb-3">
            <TriangleAlert class="w-4 h-4 text-accent" />
            <span class="text-sm font-semibold text-ink">重点教学建议</span>
          </div>
          <div v-if="weaknesses.length > 0" class="flex flex-wrap gap-2">
            <span
              v-for="w in weaknesses"
              :key="w"
              class="px-2.5 py-1 rounded-full bg-accent/10 text-accent text-xs font-medium"
              >{{ w }}</span
            >
          </div>
          <p v-else class="text-xs text-muted-foreground">未发现明显薄弱维度（无超过 30% 学生低于 60 分的维度）。</p>
        </Card>
      </div>

      <!-- LLM summary (loaded lazily) -->
      <Card v-if="summaryLoading || llmSummary" class="tes-card-container p-6 anim-in">
        <div class="flex items-center gap-2 mb-3">
          <Sparkles class="w-4 h-4 text-primary" />
          <span class="text-sm font-semibold text-ink">AI 教学质量总结</span>
          <span v-if="summaryLoading" class="text-xs text-muted-foreground">生成中…</span>
        </div>
        <div v-if="summaryLoading" class="space-y-2">
          <Skeleton class="h-4 w-3/4" />
          <Skeleton class="h-4 w-full" />
          <Skeleton class="h-4 w-5/6" />
        </div>
        <p v-else class="text-sm leading-relaxed text-foreground whitespace-pre-line">{{ llmSummary }}</p>
      </Card>
    </div>
  </AppShell>
</template>
