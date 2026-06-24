<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import axios from 'axios'
import AppShell from '@/components/layout/AppShell.vue'
import BreadcrumbNav from '@/components/business/BreadcrumbNav.vue'
import EmptyState from '@/components/business/EmptyState.vue'
import IllustNoTasks from '@/components/illustrations/IllustNoTasks.vue'
import { useToast } from '@/components/ui/toast'
import { safeGet } from '@/lib/api-helpers'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Badge } from '@/components/ui/badge'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  Search,
  Calendar,
  AlarmClock,
  Award,
  FileText,
  Upload,
  CheckCircle2,
  ChevronRight,
  ChevronLeft,
} from 'lucide-vue-next'

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
  status: string
  deadline: string | null
  course_id: number
  teacher_id: number
  dimensions: Dimension[]
}
interface Evaluation {
  id: number
  task_id: number
  total_score: number | null
  status: string
  created_at: string
}

const { toast } = useToast()
const tasks = ref<Task[]>([])
const uploadCountByTask = ref<Record<number, number>>({})
const evalByTask = ref<Record<number, Evaluation>>({})
const loading = ref(true)
const searchQuery = ref('')
const filterStatus = ref<string>('all')

const now = ref(Date.now())
let timerId: number | null = null

async function fetchAll() {
  loading.value = true
  try {
    const tasksRes = await axios.get('/api/tasks')
    tasks.value = tasksRes.data

    // 评价加载失败不阻塞主流程，但要透出错误（不再静默 fallback）
    const evalsResult = await safeGet<Evaluation[]>('/api/evaluations/my', [])
    if (evalsResult.error) {
      toast({
        description: `评价记录 ${evalsResult.error}`,
        variant: 'warning',
      })
    }
    const evMap: Record<number, Evaluation> = {}
    for (const e of evalsResult.data) {
      if (!evMap[e.task_id]) evMap[e.task_id] = e
    }
    evalByTask.value = evMap

    const counts: Record<number, number> = {}
    await Promise.all(
      tasks.value.map(async (t) => {
        const r = await safeGet<unknown[]>(`/api/uploads/by-task/${t.id}`, [])
        counts[t.id] = (r.data ?? []).length
      }),
    )
    uploadCountByTask.value = counts
  } catch (e) {
    const msg = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    toast({ description: msg ?? '加载任务失败', variant: 'destructive' })
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchAll()
  timerId = window.setInterval(() => (now.value = Date.now()), 30_000)
})
onUnmounted(() => {
  if (timerId) clearInterval(timerId)
})

function getCountdown(deadline: string | null) {
  if (!deadline) return null
  const diff = new Date(deadline).getTime() - now.value
  if (diff <= 0) return { expired: true, days: 0, hours: 0, minutes: 0, label: '已截止' }
  const days = Math.floor(diff / 86400000)
  const hours = Math.floor((diff % 86400000) / 3600000)
  const minutes = Math.floor((diff % 3600000) / 60000)
  let label = ''
  if (days > 0) label = `${days} 天 ${hours} 小时`
  else if (hours > 0) label = `${hours} 小时 ${minutes} 分`
  else label = `${minutes} 分钟`
  return { expired: false, days, hours, minutes, label }
}

function urgencyRingClass(deadline: string | null) {
  const cd = getCountdown(deadline)
  if (!cd) return ''
  if (cd.expired) return 'opacity-70'
  if (cd.days < 1) return '!border-danger ring-2 ring-danger/30'
  if (cd.days < 3) return '!border-accent ring-1 ring-accent/30'
  return 'hover:!border-primary'
}

function urgencyBadge(deadline: string | null) {
  const cd = getCountdown(deadline)
  if (!cd) return null
  if (cd.expired) return { label: '已截止', variant: 'secondary' as const }
  if (cd.days < 1) return { label: '今日截止', variant: 'destructive' as const }
  if (cd.days < 3) return { label: '即将截止', variant: 'accent' as const }
  return null
}

function submissionState(t: Task): 'pending' | 'submitted' | 'evaluated' | 'closed' {
  if (t.status === 'closed') return 'closed'
  if (evalByTask.value[t.id]) return 'evaluated'
  if (uploadCountByTask.value[t.id] > 0) return 'submitted'
  return 'pending'
}

function statusBadge(t: Task) {
  const s = submissionState(t)
  return {
    pending: { label: '待提交', variant: 'warning' as const },
    submitted: { label: '已提交', variant: 'info' as const },
    evaluated: { label: '已评价', variant: 'success' as const },
    closed: { label: '已结束', variant: 'secondary' as const },
  }[s]
}

const counts = computed(() => ({
  all: tasks.value.length,
  pending: tasks.value.filter((t) => submissionState(t) === 'pending').length,
  submitted: tasks.value.filter((t) => submissionState(t) === 'submitted').length,
  evaluated: tasks.value.filter((t) => submissionState(t) === 'evaluated').length,
}))

const filtered = computed(() => {
  let list = tasks.value
  if (filterStatus.value !== 'all') {
    list = list.filter((t) => submissionState(t) === filterStatus.value)
  }
  if (searchQuery.value.trim()) {
    const q = searchQuery.value.trim().toLowerCase()
    list = list.filter(
      (t) => t.name.toLowerCase().includes(q) || (t.description ?? '').toLowerCase().includes(q),
    )
  }
  // 紧急的排前面：未截止 + 距离截止时间最近
  return [...list].sort((a, b) => {
    const ca = getCountdown(a.deadline)
    const cb = getCountdown(b.deadline)
    if (!ca && !cb) return 0
    if (!ca) return 1
    if (!cb) return -1
    if (ca.expired && !cb.expired) return 1
    if (cb.expired && !ca.expired) return -1
    return (
      new Date(a.deadline ?? 0).getTime() - new Date(b.deadline ?? 0).getTime()
    )
  })
})

function formatDeadline(iso: string | null) {
	  if (!iso) return '——'
	  return iso.slice(0, 16).replace('T', ' ')
	}

	// Pagination
	const pageSize = 8
	const currentPage = ref(1)
	const totalItems = computed(() => filtered.value.length)
	const totalPages = computed(() => Math.max(1, Math.ceil(totalItems.value / pageSize)))
	const paged = computed(() => {
	  const start = (currentPage.value - 1) * pageSize
	  return filtered.value.slice(start, start + pageSize)
	})
	// Reset to page 1 when filter/search changes
	function resetPagination() {
	  currentPage.value = 1
	}
</script>

<template>
  <AppShell>
    <BreadcrumbNav
      :items="[
        { label: '工作台', to: '/dashboard' },
        { label: '我的实训任务' },
      ]"
    />

    <div class="flex justify-between items-end">
      <div>
        <h1 class="text-2xl font-bold text-ink">我的实训任务</h1>
        <p class="mt-1.5 text-sm text-muted-foreground">
          按倒计时优先级提交成果 ·
          <span class="text-accent font-medium">{{ counts.pending }} 项待提交</span>
        </p>
      </div>
      <div class="flex items-center gap-3">
        <Button variant="outline" as-child>
          <RouterLink to="/student/history">
            <Award class="w-4 h-4" />
            评价历史
          </RouterLink>
        </Button>
      </div>
    </div>

    <Card class="px-5 py-3.5 flex justify-between items-center gap-4">
      <Tabs v-model="filterStatus" @update:model-value="currentPage = 1">
        <TabsList>
          <TabsTrigger value="all">全部 {{ counts.all }}</TabsTrigger>
          <TabsTrigger value="pending">待提交 {{ counts.pending }}</TabsTrigger>
          <TabsTrigger value="submitted">已提交 {{ counts.submitted }}</TabsTrigger>
          <TabsTrigger value="evaluated">已评价 {{ counts.evaluated }}</TabsTrigger>
        </TabsList>
      </Tabs>

      <div class="relative w-[280px]">
        <Search class="w-3.5 h-3.5 text-muted-foreground absolute left-3 top-1/2 -translate-y-1/2" />
        <Input v-model="searchQuery" placeholder="搜索任务名称 / 描述" class="pl-9" @input="currentPage = 1" />
      </div>
    </Card>

    <div v-if="loading" class="grid grid-cols-2 gap-4">
      <Skeleton v-for="n in 4" :key="n" class="h-56 rounded-lg" />
    </div>

    <EmptyState
      v-else-if="filtered.length === 0"
      :illustration="IllustNoTasks"
      :icon="FileText"
      title="暂无符合条件的任务"
      :description="filterStatus === 'all' ? '老师还未发布任务，敬请期待' : '试试切换其他状态筛选'"
    />

    <template v-else>
      <div class="grid grid-cols-2 gap-4">
        <RouterLink
          v-for="(t, idx) in paged"
          :key="t.id"
          :to="t.status === 'closed' ? '#' : `/student/tasks/${t.id}`"
          class="block anim-in"
          :style="{ animationDelay: Math.min(idx * 40, 240) + 'ms' }"
        >
          <!-- card content unchanged -->
          <Card
            :class="[
              'p-5 flex flex-col gap-3.5 transition-all duration-200',
              urgencyRingClass(t.deadline),
              t.status === 'closed' ? 'cursor-not-allowed pointer-events-none' : 'hover:shadow-lg cursor-pointer',
            ]"
          >
            <div class="flex justify-between items-start gap-3">
              <div class="flex-1 min-w-0">
                <div class="flex items-center gap-2 mb-1.5">
                  <span class="text-sm font-bold text-ink truncate">{{ t.name }}</span>
                  <Badge v-if="urgencyBadge(t.deadline)" :variant="urgencyBadge(t.deadline)!.variant" class="flex-shrink-0">
                    {{ urgencyBadge(t.deadline)!.label }}
                  </Badge>
                </div>
                <p class="text-xs text-muted-foreground line-clamp-2 leading-relaxed">
                  {{ t.description || '暂无描述' }}
                </p>
              </div>
              <Badge :variant="statusBadge(t).variant" class="flex-shrink-0">
                {{ statusBadge(t).label }}
              </Badge>
            </div>

            <div class="h-px bg-border"></div>

            <div class="flex flex-col gap-2">
              <div class="flex items-center justify-between text-xs">
                <span class="flex items-center gap-1.5 text-muted-foreground">
                  <Calendar class="w-3 h-3" />
                  <span>截止时间</span>
                </span>
                <span
                  class="font-mono font-semibold"
                  :class="getCountdown(t.deadline)?.expired
                    ? 'text-muted-foreground'
                    : (getCountdown(t.deadline)?.days ?? 99) < 3
                      ? 'text-accent'
                      : 'text-foreground'"
                >
                  {{ formatDeadline(t.deadline) }}
                </span>
              </div>
              <div class="flex items-center justify-between text-xs">
                <span class="flex items-center gap-1.5 text-muted-foreground">
                  <AlarmClock class="w-3 h-3" />
                  <span>剩余</span>
                </span>
                <span
                  class="font-semibold"
                  :class="getCountdown(t.deadline)?.expired
                    ? 'text-muted-foreground'
                    : (getCountdown(t.deadline)?.days ?? 99) < 1
                      ? 'text-danger'
                      : (getCountdown(t.deadline)?.days ?? 99) < 3
                        ? 'text-accent'
                        : 'text-foreground'"
                >
                  {{ getCountdown(t.deadline)?.label ?? '不限' }}
                </span>
              </div>
              <div class="flex items-center justify-between text-xs">
                <span class="flex items-center gap-1.5 text-muted-foreground">
                  <FileText class="w-3 h-3" />
                  <span>评价维度</span>
                </span>
                <span class="font-semibold text-foreground">{{ t.dimensions.length }} 项</span>
              </div>
              <div class="flex items-center justify-between text-xs">
                <span class="flex items-center gap-1.5 text-muted-foreground">
                  <Upload class="w-3 h-3" />
                  <span>已提交</span>
                </span>
                <span class="font-semibold text-foreground">{{ uploadCountByTask[t.id] || 0 }} 次</span>
              </div>
            </div>

            <div class="flex items-center justify-between mt-auto pt-2 border-t border-border">
              <div class="flex items-center gap-2">
                <CheckCircle2 v-if="evalByTask[t.id]" class="w-4 h-4 text-success" />
                <span class="text-[11px] text-muted-foreground">
                  <template v-if="evalByTask[t.id]">
                    综合得分 <span class="text-success font-bold">{{ evalByTask[t.id].total_score ?? '—' }}</span>
                  </template>
                  <template v-else-if="(uploadCountByTask[t.id] || 0) > 0">等待评分</template>
                  <template v-else-if="t.status === 'closed'">任务已关闭</template>
                  <template v-else>尚未提交</template>
                </span>
              </div>
              <span class="text-xs font-medium text-primary flex items-center gap-1">
                {{ t.status === 'closed' ? '已结束' : '查看详情' }}
                <ChevronRight v-if="t.status !== 'closed'" class="w-3 h-3" />
              </span>
            </div>
          </Card>
        </RouterLink>
      </div>

      <!-- Pagination -->
      <div v-if="totalItems > pageSize" class="flex flex-wrap justify-between items-center gap-3 px-6 py-4 bg-surface-2 border border-border rounded-lg">
        <div class="text-xs text-muted-foreground">
          显示 {{ (currentPage - 1) * pageSize + 1 }} - {{ Math.min(currentPage * pageSize, totalItems) }} 共 {{ totalItems }} 条
        </div>
        <div class="flex items-center gap-1.5">
          <Button variant="outline" size="icon-sm" :disabled="currentPage <= 1" @click="currentPage--">
            <ChevronLeft class="w-3.5 h-3.5" />
          </Button>
          <Button
            v-for="page in totalPages"
            :key="page"
            :variant="page === currentPage ? 'default' : 'outline'"
            size="sm"
            class="h-8 min-w-[32px]"
            @click="currentPage = page"
          >
            {{ page }}
          </Button>
          <Button variant="outline" size="icon-sm" :disabled="currentPage >= totalPages" @click="currentPage++">
            <ChevronRight class="w-3.5 h-3.5" />
          </Button>
        </div>
      </div>
    </template>
  </AppShell>
</template>
