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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { BarChart3, GraduationCap, Building2 } from 'lucide-vue-next'

interface Course { id: number; name: string; code: string }

const { toast } = useToast()
const scope = ref<'school' | 'course'>('school')
const range = ref<'30d' | '90d' | '1y' | 'all'>('90d')
const courses = ref<Course[]>([])
const selectedCourseId = ref<number | null>(null)
const data = ref<Record<string, unknown> | null>(null)
const loading = ref(true)

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

async function fetchProfile() {
  loading.value = true
  data.value = null
  try {
    if (scope.value === 'school') {
      const { data: d } = await axios.get('/api/profiles/school', { params: { range: range.value } })
      data.value = d
    } else if (selectedCourseId.value) {
      const { data: d } = await axios.get(`/api/profiles/course/${selectedCourseId.value}`, {
        params: { range: range.value },
      })
      data.value = d
    }
  } catch (e) {
    const msg = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    toast({ description: msg ?? '加载画像失败', variant: 'destructive' })
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  await loadCourses()
  await fetchProfile()
})

watch([scope, range, selectedCourseId], fetchProfile)

const totalStudents = computed(() => Number(data.value?.total_students ?? data.value?.student_count ?? 0))
const evalCount = computed(() => Number(data.value?.eval_count ?? data.value?.evaluation_count ?? 0))
const avgScore = computed(() => {
  const v = data.value?.avg_score
  return typeof v === 'number' ? v : 0
})
const adoptionRate = computed(() => {
  const v = data.value?.adoption_rate
  return typeof v === 'number' ? v : 0
})
const completionRate = computed(() => {
  const v = data.value?.completion_rate
  return typeof v === 'number' ? v : 0
})

const dimensionAverages = computed(() => {
  const obj = data.value?.dimension_averages as Record<string, number> | undefined
  if (!obj) return []
  return Object.entries(obj)
    .map(([name, score]) => ({ name, score: Number(score) }))
    .sort((a, b) => b.score - a.score)
})
</script>

<template>
  <AppShell>
    <BreadcrumbNav
      :items="[
        { label: '工作台', to: '/dashboard' },
        { label: '教学画像' },
      ]"
    />

    <div class="flex justify-between items-end">
      <div>
        <h1 class="text-2xl font-bold text-ink">教学画像</h1>
        <p class="mt-1.5 text-sm text-muted-foreground">学校 / 课程级教学质量综合分析</p>
      </div>
      <div class="flex items-center gap-3">
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

    <Card class="px-5 py-3.5 flex items-center gap-4">
      <Tabs v-model="scope">
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

      <div v-if="scope === 'course'" class="flex items-center gap-2">
        <Label class="text-xs">课程：</Label>
        <Select v-model="selectedCourseId">
          <SelectTrigger class="w-64"><SelectValue placeholder="选择课程" /></SelectTrigger>
          <SelectContent>
            <SelectItem v-for="c in courses" :key="c.id" :value="c.id">
              {{ c.name }}（{{ c.code }}）
            </SelectItem>
          </SelectContent>
        </Select>
      </div>
    </Card>

    <div v-if="loading" class="grid grid-cols-3 gap-4">
      <Skeleton v-for="n in 3" :key="n" class="h-32" />
    </div>

    <EmptyState
      v-else-if="!data || (evalCount === 0 && totalStudents === 0)"
      :icon="BarChart3"
      title="暂无评价数据"
      description="该范围内尚无可分析的评价记录"
    />

    <div v-else class="flex flex-col gap-5">
      <!-- KPIs -->
      <div class="grid grid-cols-4 gap-4">
        <Card v-for="(kpi, i) in [
            { label: '学生总数', value: totalStudents, color: 'text-ink' },
            { label: '评价总数', value: evalCount, color: 'text-ink' },
            { label: '平均分', value: avgScore, color: 'text-primary', decimals: 1 },
            { label: '完成率', value: completionRate, color: 'text-success', suffix: '%' },
          ]" :key="kpi.label" class="p-5 anim-in" :style="{ animationDelay: i * 50 + 'ms' }">
          <div class="text-xs text-muted-foreground">{{ kpi.label }}</div>
          <div class="text-3xl font-bold mt-1" :class="kpi.color">
            <AnimatedNumber :value="kpi.value" :decimals="kpi.decimals ?? 0" :suffix="kpi.suffix ?? ''" />
          </div>
        </Card>
      </div>

      <!-- Adoption rate -->
      <Card v-if="adoptionRate > 0" class="p-6">
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
          指 AI 评分被教师确认的比例。高采纳率表示 AI 评分质量稳定。
        </p>
      </Card>

      <!-- Dimension Averages -->
      <Card v-if="dimensionAverages.length > 0" class="overflow-hidden">
        <header class="px-5 py-4 border-b border-border flex justify-between items-center">
          <span class="text-sm font-semibold text-ink">各维度平均分</span>
          <span class="text-xs text-muted-foreground">{{ dimensionAverages.length }} 个维度</span>
        </header>
        <div
          v-for="(d, idx) in dimensionAverages"
          :key="d.name"
          class="px-5 py-3.5 border-b border-border last:border-b-0 flex items-center gap-3.5 anim-in"
          :style="{ animationDelay: Math.min(idx * 30, 200) + 'ms' }"
        >
          <span class="w-[160px] text-sm text-ink font-medium shrink-0 truncate">{{ d.name }}</span>
          <div class="flex-1 h-1.5 bg-muted rounded-full overflow-hidden">
            <div
              class="h-full rounded-full transition-all duration-700"
              :class="d.score >= 85 ? 'bg-success' : d.score >= 70 ? 'bg-primary' : 'bg-accent'"
              :style="{ width: d.score + '%' }"
            ></div>
          </div>
          <span class="w-[80px] text-right text-sm font-semibold font-mono text-ink">
            <AnimatedNumber :value="d.score" :decimals="1" /> / 100
          </span>
        </div>
      </Card>
    </div>
  </AppShell>
</template>
