<script setup lang="ts">
/**
 * 教师视角看学生薄弱点画像。
 * 直接调用 GET /api/profiles/student/{user_id} 后端会校验权限：
 *   - 学生：仅自己
 *   - 教师：仅班级内学生
 *   - 管理员：所有
 *
 * UI 布局复用 student/MyProfileView.vue 的视觉，但通过 user_id 路由参数区分对象，
 * 顶部多一栏"返回"+ 学生信息卡。
 */
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import axios from 'axios'
import AppShell from '@/components/layout/AppShell.vue'
import BreadcrumbNav from '@/components/business/BreadcrumbNav.vue'
import EmptyState from '@/components/business/EmptyState.vue'
import AnimatedNumber from '@/components/business/AnimatedNumber.vue'
import { useToast } from '@/components/ui/toast'
import { safeGet } from '@/lib/api-helpers'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Badge } from '@/components/ui/badge'
import { Avatar } from '@/components/ui/avatar'
import { ArrowLeft, Target, Sparkles, BarChart3 } from 'lucide-vue-next'

interface ProfileOut {
  student_id: number
  radar_data: Record<string, number>
  weakness_list: string[]
  suggestions: string[]
  score_trend: { period: string; score: number }[]
  source_evaluation_count: number
  computed_at: string | null
  insufficient_data: boolean
}

interface User {
  id: number
  username: string
  display_name: string
  role: string
}

const route = useRoute()
const router = useRouter()
const { toast } = useToast()

const studentId = computed(() => Number(route.params.id))
const profile = ref<ProfileOut | null>(null)
const student = ref<User | null>(null)
const loading = ref(true)

async function fetchProfile() {
  loading.value = true
  try {
    // 画像必须成功
    const pRes = await axios.get<ProfileOut>(
      `/api/profiles/student/${studentId.value}`,
    )
    profile.value = pRes.data
    // 用户信息可降级（仅用于展示姓名/账号）
    const uRes = await safeGet<User[]>('/api/users', [])
    if (uRes.error) {
      // eslint-disable-next-line no-console
      console.warn('[StudentProfile] users lookup failed:', uRes.error)
    }
    student.value =
      uRes.data.find((u) => u.id === studentId.value) ?? null
  } catch (e) {
    const status = (e as { response?: { status?: number } })?.response?.status
    if (status === 403) {
      toast({ description: '无权查看该学生画像', variant: 'destructive' })
      router.back()
      return
    }
    const msg = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    toast({ description: msg ?? '加载画像失败', variant: 'destructive' })
  } finally {
    loading.value = false
  }
}

onMounted(fetchProfile)

const radarEntries = computed(() => {
  if (!profile.value) return []
  return Object.entries(profile.value.radar_data)
    .map(([name, score]) => ({ name, score: Number(score) }))
    .sort((a, b) => b.score - a.score)
})

const formattedDate = computed(() => {
  if (!profile.value?.computed_at) return '——'
  return profile.value.computed_at.slice(0, 16).replace('T', ' ')
})

function levelOf(score: number): 'low' | 'mid' | 'high' {
  if (score >= 85) return 'high'
  if (score >= 70) return 'mid'
  return 'low'
}
function progressClass(level: 'low' | 'mid' | 'high'): string {
  return ({ high: 'bg-success', mid: 'bg-warning', low: 'bg-accent' } as const)[level]
}
function scoreColor(level: 'low' | 'mid' | 'high'): string {
  return ({ high: 'text-success', mid: 'text-warning', low: 'text-accent' } as const)[level]
}

const radarPolygons = computed(() => {
  if (radarEntries.value.length < 3) return null
  const cx = 150
  const cy = 150
  const radius = 110
  const n = radarEntries.value.length
  const angleStep = (Math.PI * 2) / n

  const axisPoints: { x: number; y: number; label: string }[] = []
  const valuePoints: string[] = []
  for (let i = 0; i < n; i++) {
    const angle = -Math.PI / 2 + i * angleStep
    const e = radarEntries.value[i]
    const r = (e.score / 100) * radius
    axisPoints.push({
      x: cx + Math.cos(angle) * radius,
      y: cy + Math.sin(angle) * radius,
      label: e.name,
    })
    valuePoints.push(`${cx + Math.cos(angle) * r},${cy + Math.sin(angle) * r}`)
  }
  return {
    axis: axisPoints,
    valueStr: valuePoints.join(' '),
    grid: [0.25, 0.5, 0.75, 1].map((scale) =>
      Array.from({ length: n })
        .map((_, i) => {
          const angle = -Math.PI / 2 + i * angleStep
          return `${cx + Math.cos(angle) * radius * scale},${cy + Math.sin(angle) * radius * scale}`
        })
        .join(' '),
    ),
  }
})
</script>

<template>
  <AppShell>
    <BreadcrumbNav
      :items="[
        { label: '工作台', to: '/dashboard' },
        { label: '班级管理', to: '/teacher/classes' },
        { label: '学生画像' },
      ]"
    />

    <div class="flex justify-between items-end">
      <div class="flex items-center gap-3">
        <Button variant="outline" size="icon-sm" @click="router.back()">
          <ArrowLeft class="w-4 h-4" />
        </Button>
        <Avatar size="lg" v-if="student">{{ student.display_name.charAt(0) }}</Avatar>
        <div>
          <h1 class="text-2xl font-bold text-ink">{{ student?.display_name ?? '学生画像' }}</h1>
          <p class="mt-1 text-sm text-muted-foreground">
            <span v-if="student" class="font-mono">{{ student.username }}</span>
            <span v-if="profile && !profile.insufficient_data">
              · 基于 {{ profile.source_evaluation_count }} 次评价综合分析 · 更新于 {{ formattedDate }}
            </span>
          </p>
        </div>
      </div>
    </div>

    <div v-if="loading" class="grid grid-cols-[1fr_420px] gap-5">
      <Skeleton class="h-[480px]" />
      <Skeleton class="h-[480px]" />
    </div>

    <EmptyState
      v-else-if="!profile || profile.insufficient_data"
      :icon="BarChart3"
      title="数据不足"
      description="该学生完成评价不足，暂无法生成画像（建议至少 3 次评价）"
    />

    <div v-else class="grid grid-cols-[1fr_420px] gap-5">
      <Card class="overflow-hidden">
        <header class="px-6 py-4 border-b border-border flex justify-between items-center">
          <div class="flex items-center gap-2.5">
            <Target class="w-4 h-4 text-accent" />
            <h3 class="text-base font-bold text-ink">薄弱点分析</h3>
            <Badge variant="accent">AI 智能分析</Badge>
          </div>
          <span class="text-xs text-muted-foreground">{{ profile.weakness_list.length }} 项</span>
        </header>

        <div v-if="profile.weakness_list.length === 0" class="px-6 py-12 text-center text-sm text-muted-foreground">
          表现均衡，未识别出明显薄弱点
        </div>

        <div v-else class="flex flex-col">
          <div
            v-for="(w, idx) in profile.weakness_list"
            :key="idx"
            class="px-6 py-4 border-b border-border last:border-b-0 flex flex-col gap-3 anim-in"
            :style="{ animationDelay: idx * 60 + 'ms' }"
          >
            <div class="flex justify-between items-center">
              <div class="flex items-center gap-2.5">
                <span class="w-6 h-6 rounded-full grid place-items-center text-xs font-bold bg-accent-soft text-accent-strong">
                  {{ idx + 1 }}
                </span>
                <span class="text-[15px] font-bold text-ink">{{ w }}</span>
              </div>
              <span
                v-if="profile.radar_data[w] !== undefined"
                class="text-sm font-semibold font-mono"
                :class="scoreColor(levelOf(profile.radar_data[w]))"
              >
                {{ profile.radar_data[w] }} 分
              </span>
            </div>
            <div v-if="profile.radar_data[w] !== undefined" class="h-[5px] bg-muted rounded-full overflow-hidden">
              <div
                class="h-full rounded-full transition-all duration-700"
                :class="progressClass(levelOf(profile.radar_data[w]))"
                :style="{ width: profile.radar_data[w] + '%' }"
              ></div>
            </div>
            <div v-if="profile.suggestions[idx]" class="p-3.5 bg-accent-soft rounded-md flex flex-col gap-2">
              <div class="flex items-center gap-1.5 text-xs font-semibold text-accent-strong">
                <Sparkles class="w-3.5 h-3.5" />
                <span>AI 学习建议</span>
              </div>
              <div class="text-xs leading-relaxed text-accent-strong">{{ profile.suggestions[idx] }}</div>
            </div>
          </div>
        </div>
      </Card>

      <div class="flex flex-col gap-5">
        <Card class="p-6">
          <div class="text-[15px] font-semibold text-ink mb-3.5">能力雷达图</div>
          <div class="h-[300px] flex items-center justify-center">
            <svg v-if="radarPolygons" viewBox="0 0 300 300" class="w-full h-full max-w-[300px]">
              <polygon
                v-for="(g, i) in radarPolygons.grid"
                :key="i"
                :points="g"
                fill="none"
                stroke="hsl(var(--border))"
                stroke-width="1"
              />
              <line
                v-for="(a, i) in radarPolygons.axis"
                :key="`ax-${i}`"
                x1="150" y1="150" :x2="a.x" :y2="a.y"
                stroke="hsl(var(--border))" stroke-width="1"
              />
              <text
                v-for="(a, i) in radarPolygons.axis"
                :key="`lb-${i}`"
                :x="150 + (a.x - 150) * 1.15"
                :y="150 + (a.y - 150) * 1.15"
                text-anchor="middle" alignment-baseline="middle"
                style="font-size: 10px"
                class="fill-current text-muted-foreground"
              >
                {{ a.label }}
              </text>
              <polygon
                :points="radarPolygons.valueStr"
                fill="hsl(var(--primary) / 0.20)"
                stroke="hsl(var(--primary))"
                stroke-width="2"
                class="transition-all duration-700"
              />
            </svg>
            <div v-else class="text-xs text-muted-foreground">至少需要 3 个维度</div>
          </div>
        </Card>

        <Card class="overflow-hidden">
          <header class="px-5 py-4 border-b border-border flex justify-between items-center">
            <span class="text-sm font-semibold text-ink">维度明细</span>
            <span class="text-xs text-muted-foreground">基于 {{ profile.source_evaluation_count }} 次评价</span>
          </header>
          <div
            v-for="(d, idx) in radarEntries"
            :key="d.name"
            class="px-5 py-3.5 border-b border-border last:border-b-0 flex items-center gap-3.5 anim-in"
            :style="{ animationDelay: Math.min(idx * 30, 200) + 'ms' }"
          >
            <span class="w-[100px] text-sm text-ink font-medium shrink-0 truncate">{{ d.name }}</span>
            <div class="flex-1 h-1.5 bg-muted rounded-full overflow-hidden">
              <div
                class="h-full rounded-full transition-all duration-700"
                :class="progressClass(levelOf(d.score))"
                :style="{ width: d.score + '%' }"
              ></div>
            </div>
            <span class="w-[70px] text-right text-sm font-semibold font-mono text-ink">
              <AnimatedNumber :value="d.score" /> / 100
            </span>
          </div>
        </Card>
      </div>
    </div>
  </AppShell>
</template>
