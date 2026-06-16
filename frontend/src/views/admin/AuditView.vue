<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import axios from 'axios'
import AppShell from '@/components/layout/AppShell.vue'
import BreadcrumbNav from '@/components/business/BreadcrumbNav.vue'
import EmptyState from '@/components/business/EmptyState.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent } from '@/components/ui/card'
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
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import {
  Search,
  Download,
  ChevronLeft,
  ChevronRight,
  Filter,
  RotateCcw,
} from 'lucide-vue-next'

interface AuditLog {
  id: number
  user_id?: number
  username: string
  role: string
  action: string
  target?: string
  target_type?: string
  target_id?: string
  detail?: string
  result?: string
  ip?: string
  client_ip?: string
  created_at?: string
  occurred_at?: string
  trace_id?: string
}

const route = useRoute()
const logs = ref<AuditLog[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const loading = ref(true)

// filters
const filterAction = ref('all')
const filterUsername = ref('')
const filterIp = ref('')
const filterUserId = ref<string>('')

// detail
const showDetail = ref(false)
const detail = ref<AuditLog | null>(null)

const totalPages = computed(() => Math.max(1, Math.ceil(total.value / pageSize.value)))

const actionOptions = [
  { value: 'all', label: '全部操作' },
  { value: 'auth.login', label: '登录' },
  { value: 'auth.logout', label: '登出' },
  { value: 'user.create', label: '创建用户' },
  { value: 'user.update', label: '修改用户' },
  { value: 'task.create', label: '创建任务' },
  { value: 'task.publish', label: '发布任务' },
  { value: 'evaluation.confirm', label: '确认评价' },
  { value: 'llm.call', label: 'LLM 调用' },
  { value: 'config.update', label: '修改配置' },
]

async function fetchLogs() {
  loading.value = true
  try {
    const { data } = await axios.get('/api/audit', {
      params: {
        page: page.value,
        page_size: pageSize.value,
        action: filterAction.value && filterAction.value !== 'all' ? filterAction.value : undefined,
        username: filterUsername.value || undefined,
      },
    })
    let items: AuditLog[] = data.items
    // client-side IP filter (后端旧 endpoint 不支持，新 endpoint 支持)
    if (filterIp.value) {
      items = items.filter((it) => (it.ip ?? it.client_ip ?? '').includes(filterIp.value))
    }
    if (filterUserId.value) {
      const uid = Number(filterUserId.value)
      items = items.filter((it) => it.user_id === uid)
    }
    logs.value = items
    total.value = data.total
  } catch {
    logs.value = []
  } finally {
    loading.value = false
  }
}

function onSearch() {
  page.value = 1
  fetchLogs()
}

function resetFilters() {
  filterAction.value = 'all'
  filterUsername.value = ''
  filterIp.value = ''
  filterUserId.value = ''
  page.value = 1
  fetchLogs()
}

function exportCsv() {
  // 先尝试新 admin export endpoint，再退回客户端导出
  const a = document.createElement('a')
  a.href = '/api/audit/export'
  a.download = `audit_logs_${new Date().toISOString().slice(0, 10)}.csv`
  a.click()
}

function openDetail(log: AuditLog) {
  detail.value = log
  showDetail.value = true
}

function actionBadgeVariant(action: string) {
  if (action.startsWith('auth.')) return 'info' as const
  if (action.includes('delete') || action.includes('archive')) return 'destructive' as const
  if (action.includes('create') || action.includes('publish')) return 'success' as const
  if (action.includes('update') || action.includes('confirm')) return 'warning' as const
  return 'secondary' as const
}

function formatTime(iso: string | undefined) {
  if (!iso) return ''
  return iso.slice(0, 19).replace('T', ' ')
}

onMounted(() => {
  // 处理来自 UsersView "查看登录历史" 的 query 参数
  if (route.query.user_id) filterUserId.value = String(route.query.user_id)
  if (route.query.action) filterAction.value = String(route.query.action)
  fetchLogs()
})

watch([page], fetchLogs)
</script>

<template>
  <AppShell>
    <BreadcrumbNav
      :items="[
        { label: '管理控制台', to: '/dashboard' },
        { label: '系统' },
        { label: '审计日志' },
      ]"
    />

    <div class="flex justify-between items-end">
      <div>
        <h1 class="text-2xl font-bold text-ink">审计日志</h1>
        <p class="mt-1.5 text-sm text-muted-foreground">追溯所有关键操作 · 共 {{ total }} 条记录</p>
      </div>
      <div class="flex items-center gap-3">
        <Button variant="outline" @click="exportCsv">
          <Download class="w-4 h-4" />
          导出 CSV
        </Button>
      </div>
    </div>

    <!-- Filters -->
    <Card>
      <CardContent class="px-5 py-4">
        <div class="grid grid-cols-[repeat(auto-fit,minmax(min(100%,10rem),1fr))] gap-3 items-end">
          <div class="space-y-1.5">
            <Label class="text-[11px] text-muted-foreground">操作类型</Label>
            <Select v-model="filterAction">
              <SelectTrigger>
                <SelectValue placeholder="全部" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem v-for="o in actionOptions" :key="o.value" :value="o.value">
                  {{ o.label }}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div class="space-y-1.5">
            <Label class="text-[11px] text-muted-foreground">用户名</Label>
            <Input v-model="filterUsername" placeholder="模糊匹配" />
          </div>
          <div class="space-y-1.5">
            <Label class="text-[11px] text-muted-foreground">IP</Label>
            <Input v-model="filterIp" placeholder="0.0.0.0" />
          </div>
          <div class="space-y-1.5">
            <Label class="text-[11px] text-muted-foreground">用户 ID</Label>
            <Input v-model="filterUserId" placeholder="可选" />
          </div>
          <div class="flex gap-2">
            <Button variant="outline" @click="resetFilters">
              <RotateCcw class="w-3.5 h-3.5" />
              重置
            </Button>
            <Button @click="onSearch">
              <Search class="w-3.5 h-3.5" />
              查询
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>

    <!-- Table -->
    <Card class="tes-card-container overflow-hidden">
      <div class="tes-table-shell">
      <div class="grid min-w-[900px] grid-cols-[180px_120px_80px_180px_minmax(14rem,1fr)_140px] items-center px-6 py-3 bg-surface-2 border-b border-border text-[11px] font-semibold tracking-wider text-muted-foreground">
        <span>时间</span>
        <span>用户</span>
        <span>角色</span>
        <span>操作</span>
        <span>目标 / 详情</span>
        <span>IP</span>
      </div>

      <template v-if="loading">
        <div
          v-for="n in 8"
          :key="n"
          class="grid min-w-[900px] grid-cols-[180px_120px_80px_180px_minmax(14rem,1fr)_140px] items-center px-6 py-3 border-b border-border"
        >
          <Skeleton v-for="i in 6" :key="i" class="h-4" :class="i === 5 ? 'w-3/4' : 'w-20'" />
        </div>
      </template>

      <EmptyState
        v-else-if="logs.length === 0"
        title="暂无审计记录"
        description="调整筛选条件以查看更多日志"
        :icon="Filter"
      />

      <div
        v-for="log in logs"
        v-else
        :key="log.id"
        class="grid min-w-[900px] grid-cols-[180px_120px_80px_180px_minmax(14rem,1fr)_140px] items-center px-6 py-2.5 border-b border-border last:border-0 text-xs cursor-pointer transition-colors hover:bg-surface-2"
        @click="openDetail(log)"
      >
        <span class="text-muted-foreground font-mono">{{ formatTime(log.created_at ?? log.occurred_at) }}</span>
        <span class="text-ink font-medium truncate">{{ log.username }}</span>
        <span class="text-muted-foreground">{{ log.role }}</span>
        <span>
          <Badge :variant="actionBadgeVariant(log.action)" class="font-mono">{{ log.action }}</Badge>
        </span>
        <span class="text-muted-foreground truncate" :title="log.target ?? log.target_type">
          {{ log.target ?? (log.target_type ? `${log.target_type} #${log.target_id ?? ''}` : '—') }}
        </span>
        <span class="text-muted-foreground font-mono truncate">{{ log.ip ?? log.client_ip ?? '—' }}</span>
      </div>
      </div>

      <!-- Pagination -->
      <div v-if="!loading && logs.length > 0" class="flex items-center justify-between px-6 py-3 bg-surface-2 border-t border-border">
        <span class="text-xs text-muted-foreground">
          第 {{ page }} / {{ totalPages }} 页 · 共 {{ total }} 条
        </span>
        <div class="flex items-center gap-1.5">
          <Button variant="outline" size="icon-sm" :disabled="page <= 1" @click="page = Math.max(1, page - 1)">
            <ChevronLeft class="w-4 h-4" />
          </Button>
          <Button variant="outline" size="icon-sm" :disabled="page >= totalPages" @click="page = Math.min(totalPages, page + 1)">
            <ChevronRight class="w-4 h-4" />
          </Button>
        </div>
      </div>
    </Card>

    <!-- Detail Sheet -->
    <Sheet v-model:open="showDetail">
      <SheetContent side="right" class="w-[480px] sm:max-w-[480px]">
        <SheetHeader>
          <SheetTitle>日志详情 #{{ detail?.id }}</SheetTitle>
          <SheetDescription>{{ formatTime(detail?.created_at ?? detail?.occurred_at) }}</SheetDescription>
        </SheetHeader>
        <div v-if="detail" class="mt-4 flex flex-col gap-3 text-sm">
          <div class="grid grid-cols-[80px_1fr] gap-2">
            <span class="text-muted-foreground">用户</span>
            <span class="text-ink font-medium">{{ detail.username }} · {{ detail.role }}</span>
          </div>
          <div class="grid grid-cols-[80px_1fr] gap-2">
            <span class="text-muted-foreground">操作</span>
            <Badge :variant="actionBadgeVariant(detail.action)" class="w-fit font-mono">{{ detail.action }}</Badge>
          </div>
          <div class="grid grid-cols-[80px_1fr] gap-2">
            <span class="text-muted-foreground">目标</span>
            <span class="text-ink">{{ detail.target ?? (detail.target_type ? `${detail.target_type} #${detail.target_id}` : '—') }}</span>
          </div>
          <div v-if="detail.result" class="grid grid-cols-[80px_1fr] gap-2">
            <span class="text-muted-foreground">结果</span>
            <span class="text-ink">{{ detail.result }}</span>
          </div>
          <div class="grid grid-cols-[80px_1fr] gap-2">
            <span class="text-muted-foreground">IP</span>
            <span class="font-mono text-ink">{{ detail.ip ?? detail.client_ip ?? '—' }}</span>
          </div>
          <div v-if="detail.trace_id" class="grid grid-cols-[80px_1fr] gap-2">
            <span class="text-muted-foreground">Trace</span>
            <span class="font-mono text-ink text-xs">{{ detail.trace_id }}</span>
          </div>
          <div v-if="detail.detail" class="space-y-1.5 mt-2">
            <span class="text-muted-foreground text-xs">详情 JSON</span>
            <pre class="p-3 bg-surface-2 border border-border rounded-md text-xs font-mono overflow-x-auto whitespace-pre-wrap">{{ detail.detail }}</pre>
          </div>
        </div>
      </SheetContent>
    </Sheet>
  </AppShell>
</template>
