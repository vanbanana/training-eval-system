<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import axios from 'axios'
import AppShell from '@/components/layout/AppShell.vue'
import BreadcrumbNav from '@/components/business/BreadcrumbNav.vue'
import EmptyState from '@/components/business/EmptyState.vue'
import { useToast } from '@/components/ui/toast'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
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
  return ({ scored: 'info', confirmed: 'success', rejected: 'destructive' } as const)[s] ?? 'secondary'
}
function statusLabel(s: string) {
  return ({ scored: 'AI 评分', confirmed: '已确认', rejected: '已打回' } as Record<string, string>)[s] ?? s
}
function scoreColor(score: number | null): string {
	  if (score === null) return 'text-muted-foreground'
	  if (score >= 85) return 'text-success'
	  if (score >= 60) return 'text-ink'
	  return 'text-danger'
	}

	// Pagination
	const totalItems = computed(() => filtered.value.length)
	const totalPages = computed(() => Math.max(1, Math.ceil(totalItems.value / pageSize)))
	const paged = computed(() => {
	  const start = (currentPage.value - 1) * pageSize
	  return filtered.value.slice(start, start + pageSize)
	})
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

    <Card class="tes-card-container px-5 py-3.5">
      <div class="grid grid-cols-[repeat(auto-fit,minmax(min(100%,11rem),1fr))] gap-3 items-end">
        <div class="space-y-1.5">
          <Label class="text-[11px] text-muted-foreground">搜索任务名</Label>
          <div class="relative">
            <Search class="w-3.5 h-3.5 text-muted-foreground absolute left-3 top-1/2 -translate-y-1/2" />
            <Input v-model="search" placeholder="按任务名搜索" class="pl-9" />
          </div>
        </div>
        <div class="space-y-1.5">
          <Label class="text-[11px] text-muted-foreground">状态</Label>
          <Select v-model="statusFilter" @update:model-value="currentPage = 1">
            <SelectTrigger><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem value="all">全部 ({{ counts.all }})</SelectItem>
              <SelectItem value="scored">AI 评分 ({{ counts.scored }})</SelectItem>
              <SelectItem value="confirmed">已确认 ({{ counts.confirmed }})</SelectItem>
              <SelectItem value="rejected">已打回 ({{ counts.rejected }})</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div class="space-y-1.5">
          <Label class="text-[11px] text-muted-foreground">排序</Label>
          <Select v-model="sortBy">
            <SelectTrigger><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem value="date_desc">最新优先</SelectItem>
              <SelectItem value="date_asc">最早优先</SelectItem>
              <SelectItem value="score_desc">高分优先</SelectItem>
              <SelectItem value="score_asc">低分优先</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>
    </Card>

    <Card class="tes-card-container overflow-hidden">
      <div class="tes-table-shell">
      <div class="grid min-w-[760px] grid-cols-[80px_minmax(16rem,1fr)_120px_120px_180px_80px] items-center px-6 py-3 border-b border-border bg-surface-2 text-[11px] font-medium text-muted-foreground tracking-wider">
        <span>编号</span>
        <span>任务</span>
        <span class="text-center">综合分</span>
        <span class="text-center">状态</span>
        <span>评价时间</span>
        <span class="text-right">操作</span>
      </div>

      <template v-if="loading">
        <div v-for="n in 5" :key="n" class="grid min-w-[760px] grid-cols-[80px_minmax(16rem,1fr)_120px_120px_180px_80px] items-center px-6 py-3 border-b border-border">
          <Skeleton class="h-4 w-12" />
          <Skeleton class="h-4 w-3/4" />
          <Skeleton class="h-4 w-12 mx-auto" />
          <Skeleton class="h-5 w-16 mx-auto" />
          <Skeleton class="h-4 w-32" />
          <Skeleton class="h-4 w-12 ml-auto" />
        </div>
      </template>

      <EmptyState
        v-else-if="filtered.length === 0"
        :icon="HistoryIcon"
        title="无符合条件的评价"
        :description="evaluations.length === 0 ? '尚未完成任何评价' : '调整筛选条件查看更多'"
      />

      <div
        v-for="(e, idx) in paged"
        v-else
        :key="e.id"
        class="grid min-w-[760px] grid-cols-[80px_minmax(16rem,1fr)_120px_120px_180px_80px] items-center px-6 py-3 border-b border-border last:border-0 text-sm hover:bg-surface-2 transition-colors anim-in"
        :style="{ animationDelay: Math.min(idx * 25, 200) + 'ms' }"
      >
        <span class="text-xs text-muted-foreground font-mono">#{{ e.id }}</span>
        <RouterLink
          :to="`/student/evaluations/${e.id}`"
          class="text-ink font-medium hover:text-primary transition-colors truncate"
        >
          {{ taskMap.get(e.task_id)?.name ?? `任务 #${e.task_id}` }}
        </RouterLink>
        <span class="text-center font-mono font-semibold" :class="scoreColor(e.total_score)">
          {{ e.total_score ?? '—' }}
        </span>
        <Badge :variant="statusVariant(e.status)" class="justify-self-center">
          {{ statusLabel(e.status) }}
        </Badge>
        <span class="text-xs text-muted-foreground font-mono">{{ e.created_at?.slice(0, 16).replace('T', ' ') }}</span>
        <RouterLink
          :to="`/student/evaluations/${e.id}`"
          class="text-xs text-primary font-medium hover:underline text-right"
        >
          查看
        </RouterLink>
      </div>
      </div>

      <!-- Pagination -->
      <div v-if="totalItems > pageSize" class="flex flex-wrap justify-between items-center gap-3 px-6 py-4 bg-surface-2 border-t border-border">
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
    </Card>
  </AppShell>
</template>
