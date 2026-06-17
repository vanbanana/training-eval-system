<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import axios from 'axios'
import AppShell from '@/components/layout/AppShell.vue'
import BreadcrumbNav from '@/components/business/BreadcrumbNav.vue'
import EmptyState from '@/components/business/EmptyState.vue'
import AnimatedNumber from '@/components/business/AnimatedNumber.vue'
import { useToast } from '@/components/ui/toast'
import { useAuthStore } from '@/stores/auth'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Badge } from '@/components/ui/badge'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Target, Sparkles, Share2, FileDown, BarChart3 } from 'lucide-vue-next'

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

const auth = useAuthStore()
const { toast } = useToast()
const profile = ref<ProfileOut | null>(null)
const loading = ref(true)
const timeRange = ref('6m')

const breadcrumbs = [
  { label: '我的', to: '/dashboard' },
  { label: '学习中心' },
  { label: '能力画像' },
]

const showShareDialog = ref(false)

const shareUrl = computed(() => {
  const origin = typeof window !== 'undefined' ? window.location.origin : ''
  return `${origin}/student/profile?student=${auth.user?.id ?? ''}`
})

async function fetchProfile() {
  loading.value = true
  try {
    const { data } = await axios.get<ProfileOut>(`/api/profiles/student/${auth.user?.id}`)
    profile.value = {
      ...data,
      radar_data: data.radar_data ?? {},
      weakness_list: data.weakness_list ?? [],
      suggestions: data.suggestions ?? [],
      score_trend: data.score_trend ?? [],
    }
  } catch {
    toast({ description: '加载画像失败', variant: 'destructive' })
  } finally {
    loading.value = false
  }
}

onMounted(fetchProfile)

watch(timeRange, fetchProfile)

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
  if (level === 'high') return 'bg-success'
  if (level === 'mid') return 'bg-warning'
  return 'bg-accent'
}

function scoreColor(level: 'low' | 'mid' | 'high'): string {
  if (level === 'high') return 'text-success'
  if (level === 'mid') return 'text-warning'
  return 'text-accent'
}

// Polygon for radar SVG (5+ axes)
const radarPolygons = computed(() => {
  if (radarEntries.value.length < 3) return null
  const cx = 150
  const cy = 150
  const max = 100
  const radius = 110
  const n = radarEntries.value.length
  const angleStep = (Math.PI * 2) / n

  const axisPoints: { x: number; y: number; label: string; angle: number }[] = []
  const valuePoints: string[] = []
  for (let i = 0; i < n; i++) {
    const angle = -Math.PI / 2 + i * angleStep
    const e = radarEntries.value[i]
    const r = (e.score / max) * radius
    axisPoints.push({
      x: cx + Math.cos(angle) * radius,
      y: cy + Math.sin(angle) * radius,
      label: e.name,
      angle,
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

async function exportPdf() {
  if (!auth.user) return
  try {
    const { data } = await axios.get(`/api/profiles/student/${auth.user.id}/pdf`, { responseType: 'blob' })
    const url = URL.createObjectURL(data)
    const a = document.createElement('a')
    a.href = url
    a.download = `profile_${auth.user.id}.pdf`
    a.click()
    URL.revokeObjectURL(url)
    toast({ description: '画像 PDF 已开始下载', variant: 'success' })
  } catch {
    toast({ description: 'PDF 导出失败，请稍后重试', variant: 'destructive' })
  }
}

function copyShareLink() {
  if (!auth.user) return
  const url = `${window.location.origin}/student/profile?student=${auth.user.id}`
  if (navigator.clipboard) {
    navigator.clipboard.writeText(url)
      .then(() => toast({ description: '画像链接已复制', variant: 'success' }))
      .catch(() => toast({ description: '复制失败', variant: 'destructive' }))
  } else {
    toast({ description: `链接：${url}`, variant: 'info' })
  }
  showShareDialog.value = false
}
</script>

<template>
  <AppShell>
    <BreadcrumbNav :items="breadcrumbs" />

    <div class="tes-page-header">
      <div class="min-w-0">
        <h1 class="tes-clamp-title text-2xl font-bold text-ink">我的能力画像</h1>
        <p class="mt-1.5 text-sm text-muted-foreground">
          <template v-if="profile && !profile.insufficient_data">
            基于近 <AnimatedNumber :value="profile.source_evaluation_count" /> 次实训综合分析 · 最近一次更新于 {{ formattedDate }}
          </template>
          <template v-else>个人能力发展轨迹与改进方向</template>
        </p>
      </div>
      <div class="tes-page-actions">
        <Select v-model="timeRange">
          <SelectTrigger class="w-32"><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem value="6m">近 6 个月</SelectItem>
            <SelectItem value="1y">近 1 年</SelectItem>
            <SelectItem value="all">全部</SelectItem>
          </SelectContent>
        </Select>
        <Button variant="outline" @click="showShareDialog = true">
          <Share2 class="w-3.5 h-3.5" />
          分享给老师
        </Button>
        <Button @click="exportPdf">
          <FileDown class="w-4 h-4" />
          导出 PDF
        </Button>
      </div>
    </div>

    <div v-if="loading" class="tes-grid-main-aside">
      <Skeleton class="h-[480px]" />
      <Skeleton class="h-[480px]" />
    </div>

    <EmptyState
      v-else-if="!profile || profile.insufficient_data"
      :icon="BarChart3"
      title="数据不足"
      description="至少完成 3 次评价后才能生成能力画像。请先去完成实训任务"
      action-label="查看任务"
      @action="$router.push('/student/tasks')"
    />

    <div v-else class="tes-grid-main-aside">
      <!-- LEFT: Weakness Analysis -->
      <Card class="tes-card-container overflow-hidden">
        <header class="px-6 py-4 border-b border-border flex justify-between items-center">
          <div class="flex items-center gap-2.5">
            <Target class="w-4 h-4 text-accent" />
            <h3 class="text-base font-bold text-ink">薄弱点分析 · 附学习建议</h3>
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
              <div class="flex min-w-0 items-center gap-2.5">
                <span class="w-6 h-6 rounded-full grid place-items-center text-xs font-bold bg-accent-soft text-accent-strong">
                  {{ idx + 1 }}
                </span>
                <span class="tes-breakable text-[15px] font-bold text-ink">{{ w }}</span>
              </div>
              <span
                v-if="profile.radar_data[w] !== undefined"
                class="text-sm font-semibold font-mono"
                :class="scoreColor(levelOf(profile.radar_data[w]))"
              >
                {{ profile.radar_data[w] }} 分
              </span>
            </div>
            <div
              v-if="profile.radar_data[w] !== undefined"
              class="h-[5px] bg-muted rounded-full overflow-hidden"
            >
              <div
                class="h-full rounded-full transition-all duration-700"
                :class="progressClass(levelOf(profile.radar_data[w]))"
                :style="{ width: profile.radar_data[w] + '%' }"
              ></div>
            </div>
            <div
              v-if="profile.suggestions[idx]"
              class="p-3.5 bg-accent-soft rounded-md flex flex-col gap-2"
            >
              <div class="flex items-center gap-1.5 text-xs font-semibold text-accent-strong">
                <Sparkles class="w-3.5 h-3.5" />
                <span>AI 学习建议</span>
              </div>
              <div class="text-xs leading-relaxed text-accent-strong">{{ profile.suggestions[idx] }}</div>
            </div>
          </div>
        </div>
      </Card>

      <!-- RIGHT -->
      <div class="flex flex-col gap-5">
        <!-- Radar -->
        <Card class="tes-card-container p-6">
          <div class="flex justify-between items-center mb-3.5">
            <div>
              <div class="text-[15px] font-semibold text-ink">能力雷达图</div>
              <div class="text-xs text-muted-foreground mt-1">{{ radarEntries.length }} 个维度均值</div>
            </div>
          </div>
          <div class="min-h-[220px] max-h-[300px] flex items-center justify-center">
            <svg v-if="radarPolygons" viewBox="0 0 300 300" class="w-full h-full max-w-[300px]">
              <!-- grid -->
              <polygon
                v-for="(g, i) in radarPolygons.grid"
                :key="i"
                :points="g"
                fill="none"
                stroke="hsl(var(--border))"
                stroke-width="1"
              />
              <!-- axes -->
              <line
                v-for="(a, i) in radarPolygons.axis"
                :key="`ax-${i}`"
                x1="150" y1="150"
                :x2="a.x" :y2="a.y"
                stroke="hsl(var(--border))"
                stroke-width="1"
              />
              <!-- labels -->
              <text
                v-for="(a, i) in radarPolygons.axis"
                :key="`lb-${i}`"
                :x="150 + (a.x - 150) * 1.15"
                :y="150 + (a.y - 150) * 1.15"
                text-anchor="middle"
                alignment-baseline="middle"
                class="text-[10px] fill-current text-muted-foreground"
                style="font-size: 10px"
              >
                {{ a.label }}
              </text>
              <!-- values -->
              <polygon
                :points="radarPolygons.valueStr"
                fill="hsl(var(--primary) / 0.20)"
                stroke="hsl(var(--primary))"
                stroke-width="2"
                class="transition-all duration-700"
              />
              <circle
                v-for="(p, i) in radarPolygons.valueStr.split(' ')"
                :key="`pt-${i}`"
                :cx="Number(p.split(',')[0])"
                :cy="Number(p.split(',')[1])"
                r="3"
                fill="hsl(var(--primary))"
              />
            </svg>
            <div v-else class="text-xs text-muted-foreground">至少需要 3 个维度</div>
          </div>
        </Card>

        <!-- Dimension Detail -->
        <Card class="tes-card-container overflow-hidden">
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

        <!-- Score Trend -->
        <Card v-if="profile.score_trend.length > 0" class="tes-card-container p-5">
          <div class="flex items-center justify-between mb-3">
            <span class="text-sm font-semibold text-ink">综合分趋势</span>
            <span class="text-[11px] text-muted-foreground">{{ profile.score_trend.length }} 个数据点</span>
          </div>
          <svg viewBox="0 0 300 100" class="w-full h-24">
            <polyline
              :points="(profile?.score_trend ?? [])
                .map((p, i) => `${(i / Math.max(1, (profile?.score_trend.length ?? 1) - 1)) * 280 + 10},${100 - (p.score / 100) * 80 - 10}`)
                .join(' ')"
              fill="none"
              stroke="hsl(var(--primary))"
              stroke-width="2"
              class="transition-all duration-1000"
            />
            <circle
              v-for="(p, i) in profile.score_trend"
              :key="i"
              :cx="(i / Math.max(1, profile.score_trend.length - 1)) * 280 + 10"
              :cy="100 - (p.score / 100) * 80 - 10"
              r="2.5"
              fill="hsl(var(--primary))"
            >
              <title>{{ p.period }}: {{ p.score }}</title>
            </circle>
          </svg>
        </Card>
      </div>
    </div>

    <Dialog v-model:open="showShareDialog">
      <DialogContent class="max-w-md">
        <DialogHeader>
          <DialogTitle>分享给老师</DialogTitle>
          <DialogDescription>复制下方链接发送给老师即可（仅老师本人有权访问）</DialogDescription>
        </DialogHeader>
        <p class="text-xs font-mono bg-surface-2 p-3 rounded-md break-all border border-border">
          {{ shareUrl }}
        </p>
        <DialogFooter class="gap-2">
          <Button variant="outline" @click="showShareDialog = false">关闭</Button>
          <Button @click="copyShareLink">复制链接</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </AppShell>
</template>
