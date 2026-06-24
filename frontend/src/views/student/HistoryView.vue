<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import axios from 'axios'
import AppShell from '@/components/layout/AppShell.vue'
import BreadcrumbNav from '@/components/business/BreadcrumbNav.vue'
import EmptyState from '@/components/business/EmptyState.vue'
import { useToast } from '@/components/ui/toast'
import { Input } from '@/components/ui/input'
import { Card } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Search, History as HistoryIcon, ChevronLeft, ChevronRight } from 'lucide-vue-next'

interface Evaluation {
  id: number
  task_id: number
  total_score: number | null
  status: string
  created_at: string
}

interface Task {
  id: number
  name: string
  course_id: number
}

const { toast } = useToast()
const evaluations = ref<Evaluation[]>([])
const taskMap = ref<Map<number, Task>>(new Map())
const loading = ref(true)

// Filters
const search = ref('')
const statusFilter = ref<string>('all')
const sortBy = ref<'date_desc' | 'date_asc' | 'score_desc' | 'score_asc'>('date_desc')
const currentPage = ref(1)
const pageSize = 10

async function fetchAll() {
  loading.value = true
  try {
    const { data } = await axios.get<Evaluation[]>('/api/evaluations/my')
    evaluations.value = data
    // 拉取所有相关 task name（join）
    const ids = Array.from(new Set(data.map((e) => e.task_id)))
    await Promise.all(
      ids.map((id) =>
        axios
          .get<Task>(`/api/tasks/${id}`)
          .then(({ data: t }) => taskMap.value.set(id, t))
          .catch(() => null),
      ),
    )
  } catch {
    toast({ description: '加载历史评价失败', variant: 'destructive' })
  } finally {
    loading.value = false
  }
}

onMounted(fetchAll)

const filtered = computed(() => {
  let list = [...evaluations.value]
  if (statusFilter.value !== 'all') {
    list = list.filter((e) => e.status === statusFilter.value)
  }
  if (search.value.trim()) {
    const q = search.value.trim().toLowerCase()
    list = list.filter((e) => {
      const name = taskMap.value.get(e.task_id)?.name ?? `任务 #${e.task_id}`
      return name.toLowerCase().includes(q)
    })
  }
  list.sort((a, b) => {
    if (sortBy.value === 'date_desc') return b.id - a.id
    if (sortBy.value === 'date_asc') return a.id - b.id
    if (sortBy.value === 'score_desc') return (b.total_score ?? -1) - (a.total_score ?? -1)
    return (a.total_score ?? 9999) - (b.total_score ?? 9999)
  })
  return list
})

const counts = computed(() => ({
  all: evaluations.value.length,
  scored: evaluations.value.filter((e) => e.status === 'scored').length,
  confirmed: evaluations.value.filter((e) => e.status === 'confirmed').length,
  rejected: evaluations.value.filter((e) => e.status === 'rejected').length,
}))

function statusVariant(s: string) {
  return ({ scored: 'info', confirmed: 'success', rejected: 'destructive', pending: 'secondary', failed: 'destructive' } as const)[s] ?? 'secondary'
}
function statusLabel(s: string) {
  return ({ scored: 'AI 评分', confirmed: '已确认', rejected: '已打回', pending: '评分中', failed: '评分失败' } as Record<string, string>)[s] ?? s
}
// Background tint for the leading score chip.
function scoreChipClass(score: number | null): string {
  if (score === null) return 'bg-muted text-muted-foreground'
  if (score >= 85) return 'bg-success-soft text-success'
  if (score >= 60) return 'bg-primary-soft text-primary'
  return 'bg-danger-soft text-danger'
}

const statusTabs = computed(() => [
  { key: 'all', label: '全部', count: counts.value.all },
  { key: 'scored', label: 'AI 评分', count: counts.value.scored },
  { key: 'confirmed', label: '已确认', count: counts.value.confirmed },
  { key: 'rejected', label: '已打回', count: counts.value.rejected },
])

function fmtTime(s: string): string {
  return s?.slice(0, 16).replace('T', ' ') ?? ''
}

// Pagination
const totalItems = computed(() => filtered.value.length)
const totalPages = computed(() => Math.max(1, Math.ceil(totalItems.value / pageSize)))
const paged = computed(() => {
  const start = (currentPage.value - 1) * pageSize
  return filtered.value.slice(start, start + pageSize)
})

function selectStatus(key: string) {
  statusFilter.value = key
  currentPage.value = 1
}
</script>

<template>
  <AppShell>
    <BreadcrumbNav
      :items="[
        { label: '我的', to: '/dashboard' },
        { label: '评价历史' },
      ]"
    />

    <div>
      <h1 class="text-2xl font-bold text-ink">评价历史</h1>
      <p class="mt-1.5 text-sm text-muted-foreground">查看所有历次评价 · 共 {{ counts.all }} 条记录</p>
    </div>

    <Card class="tes-card-container px-5 py-4">
      <div class="flex flex-wrap items-center gap-3">
        <div class="relative flex-1 min-w-[12rem]">
          <Search class="w-3.5 h-3.5 text-muted-foreground absolute left-3 top-1/2 -translate-y-1/2" />
          <Input v-model="search" placeholder="按任务名搜索" class="pl-9" @update:model-value="currentPage = 1" />
        </div>
        <Select v-model="sortBy">
          <SelectTrigger class="w-[9.5rem]"><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem value="date_desc">最新优先</SelectItem>
            <SelectItem value="date_asc">最早优先</SelectItem>
            <SelectItem value="score_desc">高分优先</SelectItem>
            <SelectItem value="score_asc">低分优先</SelectItem>
          </SelectContent>
        </Select>
      </div>
      <div class="mt-3 flex flex-wrap gap-2">
        <button
          v-for="t in statusTabs"
          :key="t.key"
          type="button"
          class="inline-flex items-center gap-1.5 rounded-pill px-3.5 py-1.5 text-[13px] font-medium transition-colors"
          :class="statusFilter === t.key ? 'bg-primary text-primary-foreground' : 'bg-surface-2 text-muted-foreground hover:text-ink'"
          @click="selectStatus(t.key)"
        >
          {{ t.label }}
          <span class="text-[11px] opacity-80">{{ t.count }}</span>
        </button>
      </div>
    </Card>

    <template v-if="loading">
      <div class="flex flex-col gap-3">
        <Card v-for="n in 5" :key="n" class="tes-card-container flex items-center gap-4 px-5 py-4">
          <Skeleton class="h-12 w-12 rounded-2xl" />
          <div class="flex-1 space-y-2">
            <Skeleton class="h-4 w-1/2" />
            <Skeleton class="h-3 w-1/3" />
          </div>
          <Skeleton class="h-6 w-16 rounded-pill" />
        </Card>
      </div>
    </template>

    <Card v-else-if="filtered.length === 0" class="tes-card-container">
      <EmptyState
        :icon="HistoryIcon"
        title="无符合条件的评价"
        :description="evaluations.length === 0 ? '尚未完成任何评价' : '调整筛选条件查看更多'"
      />
    </Card>

    <div v-else class="flex flex-col gap-3">
      <RouterLink
        v-for="(e, idx) in paged"
        :key="e.id"
        :to="`/student/evaluations/${e.id}`"
        class="tes-card-container group flex items-center gap-4 px-5 py-4 transition-all hover:-translate-y-0.5 hover:shadow-lg anim-in"
        :style="{ animationDelay: Math.min(idx * 30, 240) + 'ms' }"
      >
        <div
          class="grid h-12 w-12 shrink-0 place-items-center rounded-2xl font-mono text-lg font-bold leading-none"
          :class="scoreChipClass(e.total_score)"
        >
          {{ e.total_score ?? '—' }}
        </div>
        <div class="min-w-0 flex-1">
          <div class="tes-breakable text-[15px] font-semibold text-ink group-hover:text-primary transition-colors">
            {{ taskMap.get(e.task_id)?.name ?? `任务 #${e.task_id}` }}
          </div>
          <div class="mt-1 flex items-center gap-2 text-xs text-muted-foreground">
            <span class="font-mono">#{{ e.id }}</span>
            <span>·</span>
            <span class="font-mono">{{ fmtTime(e.created_at) }}</span>
          </div>
        </div>
        <Badge :variant="statusVariant(e.status)">{{ statusLabel(e.status) }}</Badge>
        <ChevronRight class="w-4 h-4 text-muted-foreground group-hover:text-primary transition-colors" />
      </RouterLink>

      <!-- Pagination -->
      <div v-if="totalItems > pageSize" class="flex flex-wrap justify-between items-center gap-3 pt-2">
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
    </div>
  </AppShell>
</template>
