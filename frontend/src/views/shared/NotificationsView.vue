<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import axios from 'axios'
import AppShell from '@/components/layout/AppShell.vue'
import BreadcrumbNav from '@/components/business/BreadcrumbNav.vue'
import EmptyState from '@/components/business/EmptyState.vue'
import IllustNoNotifications from '@/components/illustrations/IllustNoNotifications.vue'
import { useToast } from '@/components/ui/toast'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Badge } from '@/components/ui/badge'
import { Switch } from '@/components/ui/switch'
import { Label } from '@/components/ui/label'
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
import { Bell, CheckCheck, Settings, Filter } from 'lucide-vue-next'

interface Notification {
  id: number
  type: string
  title: string
  content?: string
  link?: string
  is_read: boolean
  created_at: string
  payload?: Record<string, unknown>
}

const router = useRouter()
const { toast } = useToast()
const notifications = ref<Notification[]>([])
const unreadCount = ref(0)
const loading = ref(true)

const typeFilter = ref<string>('all')
const timeRange = ref<'all' | '7d' | '30d' | 'today'>('all')

// Preferences dialog
const showPrefsDialog = ref(false)
const prefs = ref<Record<string, boolean>>({})
const savingPref = ref<string | null>(null)

const eventTypes = [
  { key: 'evaluation.scored', label: 'AI 评分完成', icon: '✨' },
  { key: 'evaluation.confirmed', label: '教师确认评价', icon: '✓' },
  { key: 'evaluation.rejected', label: '评价被打回', icon: '⚠️' },
  { key: 'task.published', label: '新任务发布', icon: '📋' },
  { key: 'similarity.suspect', label: '检测到相似度异常', icon: '🛡️' },
  { key: 'system.announcement', label: '系统公告', icon: '📣' },
]

async function fetchNotifications() {
  loading.value = true
  try {
    const { data } = await axios.get('/api/notifications', { params: { limit: 100 } })
    notifications.value = Array.isArray(data) ? data : (data?.items ?? [])
    unreadCount.value = data.unread_count ?? notifications.value.filter((n: Notification) => !n.is_read).length
  } catch {
    toast({ description: '加载通知失败', variant: 'destructive' })
  } finally {
    loading.value = false
  }
}

async function loadPreferences() {
  try {
    const { data } = await axios.get('/api/notifications/preferences')
    prefs.value = data
  } catch {
    /* ignore */
  }
}

async function setPref(eventType: string, enabled: boolean) {
  savingPref.value = eventType
  try {
    await axios.put('/api/notifications/preferences', {
      event_type: eventType,
      enabled,
    })
    prefs.value[eventType] = enabled
    toast({ description: '已保存', variant: 'success' })
  } catch (e) {
    const msg = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    toast({ description: msg ?? '保存失败', variant: 'destructive' })
  } finally {
    savingPref.value = null
  }
}

async function markRead(n: Notification) {
  if (n.is_read) {
    if (n.link) router.push(n.link)
    return
  }
  try {
    await axios.post(`/api/notifications/${n.id}/read`)
    n.is_read = true
    unreadCount.value = Math.max(0, unreadCount.value - 1)
    if (n.link) router.push(n.link)
  } catch {
    toast({ description: '标记已读失败', variant: 'destructive' })
  }
}

async function markAllRead() {
  try {
    await axios.post('/api/notifications/read-all')
    notifications.value.forEach((n) => (n.is_read = true))
    unreadCount.value = 0
    toast({ description: '全部已读', variant: 'success' })
  } catch {
    toast({ description: '操作失败', variant: 'destructive' })
  }
}

const typeOptions = computed(() => {
  const s = new Set<string>()
  notifications.value.forEach((n) => s.add(n.type))
  return ['all', ...Array.from(s).sort()]
})

const filtered = computed(() => {
  let list = notifications.value
  if (typeFilter.value !== 'all') {
    list = list.filter((n) => n.type === typeFilter.value)
  }
  if (timeRange.value !== 'all') {
    const now = Date.now()
    const ms = {
      today: 24 * 3600_000,
      '7d': 7 * 24 * 3600_000,
      '30d': 30 * 24 * 3600_000,
    }[timeRange.value]!
    list = list.filter((n) => now - new Date(n.created_at).getTime() <= ms)
  }
  return list
})

function typeBadgeVariant(type: string) {
  if (type.startsWith('evaluation.')) return 'info' as const
  if (type.startsWith('task.')) return 'success' as const
  if (type.startsWith('similarity.')) return 'destructive' as const
  if (type.startsWith('system.')) return 'gold' as const
  return 'secondary' as const
}

function formatTime(iso: string) {
  const d = new Date(iso)
  const diff = Date.now() - d.getTime()
  if (diff < 60_000) return '刚刚'
  if (diff < 3_600_000) return Math.floor(diff / 60_000) + ' 分钟前'
  if (diff < 86_400_000) return Math.floor(diff / 3_600_000) + ' 小时前'
  if (diff < 7 * 86_400_000) return Math.floor(diff / 86_400_000) + ' 天前'
  return d.toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
}

onMounted(async () => {
  await Promise.all([fetchNotifications(), loadPreferences()])
})
</script>

<template>
  <AppShell>
    <BreadcrumbNav
      :items="[
        { label: '工作台', to: '/dashboard' },
        { label: '通知中心' },
      ]"
    />

    <div class="flex justify-between items-end">
      <div>
        <h1 class="text-2xl font-bold text-ink">通知中心</h1>
        <p class="mt-1.5 text-sm text-muted-foreground">
          共 {{ notifications.length }} 条 ·
          <span :class="unreadCount > 0 ? 'text-accent font-medium' : ''">{{ unreadCount }} 条未读</span>
        </p>
      </div>
      <div class="flex items-center gap-3">
        <Button variant="outline" @click="showPrefsDialog = true">
          <Settings class="w-3.5 h-3.5" />
          通知偏好
        </Button>
        <Button v-if="unreadCount > 0" @click="markAllRead">
          <CheckCheck class="w-3.5 h-3.5" />
          全部已读
        </Button>
      </div>
    </div>

    <Card class="px-5 py-3.5">
      <div class="flex items-center gap-3">
        <Filter class="w-3.5 h-3.5 text-muted-foreground" />
        <Label class="text-xs">筛选：</Label>
        <Select v-model="typeFilter">
          <SelectTrigger class="w-44"><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem v-for="t in typeOptions" :key="t" :value="t">
              {{ t === 'all' ? '全部类型' : t }}
            </SelectItem>
          </SelectContent>
        </Select>
        <Select v-model="timeRange">
          <SelectTrigger class="w-36"><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem value="all">全部时间</SelectItem>
            <SelectItem value="today">今天</SelectItem>
            <SelectItem value="7d">最近 7 天</SelectItem>
            <SelectItem value="30d">最近 30 天</SelectItem>
          </SelectContent>
        </Select>
      </div>
    </Card>

    <Card class="overflow-hidden">
      <template v-if="loading">
        <div v-for="n in 5" :key="n" class="flex items-start gap-4 px-6 py-4 border-b border-border">
          <Skeleton class="h-2 w-2 mt-2 rounded-full" />
          <div class="flex-1 space-y-2">
            <Skeleton class="h-4 w-3/4" />
            <Skeleton class="h-3 w-full" />
          </div>
        </div>
      </template>

      <EmptyState
        v-else-if="filtered.length === 0"
        :illustration="IllustNoNotifications"
        :icon="Bell"
        title="暂无通知"
        :description="notifications.length === 0 ? '一切都在掌控之中' : '尝试调整筛选条件'"
      />

      <div
        v-for="(n, idx) in filtered"
        v-else
        :key="n.id"
        class="flex items-start gap-4 px-6 py-4 border-b border-border last:border-0 cursor-pointer hover:bg-surface-2 transition-colors anim-in"
        :class="!n.is_read ? 'bg-primary-soft/30' : ''"
        :style="{ animationDelay: Math.min(idx * 25, 200) + 'ms' }"
        @click="markRead(n)"
      >
        <span
          class="mt-1.5 w-1.5 h-1.5 rounded-full flex-shrink-0 transition-colors"
          :class="n.is_read ? 'bg-transparent' : 'bg-primary animate-pulse'"
        ></span>
        <div class="flex-1 min-w-0">
          <div class="flex items-center gap-2 mb-1">
            <Badge :variant="typeBadgeVariant(n.type)" class="text-[10px]">{{ n.type }}</Badge>
            <span class="text-sm font-medium text-ink truncate">{{ n.title }}</span>
          </div>
          <div v-if="n.content" class="text-xs text-muted-foreground line-clamp-2">{{ n.content }}</div>
          <div class="text-[11px] text-subtle-foreground mt-1.5 font-mono">{{ formatTime(n.created_at) }}</div>
        </div>
      </div>
    </Card>

    <!-- Preferences Dialog -->
    <Dialog v-model:open="showPrefsDialog">
      <DialogContent class="max-w-md">
        <DialogHeader>
          <DialogTitle>通知偏好</DialogTitle>
          <DialogDescription>关闭后将不再生成对应类型通知</DialogDescription>
        </DialogHeader>
        <div class="space-y-3">
          <div
            v-for="evt in eventTypes"
            :key="evt.key"
            class="flex items-center justify-between gap-3 p-3 border border-border rounded-md"
          >
            <div class="flex items-center gap-2.5 min-w-0">
              <span class="text-base shrink-0">{{ evt.icon }}</span>
              <div class="min-w-0">
                <div class="text-sm font-medium text-ink truncate">{{ evt.label }}</div>
                <code class="text-[10px] text-muted-foreground font-mono">{{ evt.key }}</code>
              </div>
            </div>
            <Switch
              :model-value="prefs[evt.key] !== false"
              :disabled="savingPref === evt.key"
              @update:model-value="(v) => setPref(evt.key, !!v)"
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" @click="showPrefsDialog = false">关闭</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </AppShell>
</template>
