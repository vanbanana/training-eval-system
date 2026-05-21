import axios from 'axios'
import { computed, onMounted, onUnmounted, ref } from 'vue'

export interface NotificationItem {
  id: number
  type: string
  title: string
  body?: string
  link?: string
  is_read: boolean
  created_at: string
}

const items = ref<NotificationItem[]>([])
const loading = ref(false)
let timer: ReturnType<typeof setInterval> | null = null
let mountedCount = 0

async function fetchNotifications() {
  if (loading.value) return
  loading.value = true
  try {
    const { data } = await axios.get('/api/notifications', { params: { limit: 20 } })
    // 后端 list shape：{ items: [...], total: n, unread: m } 或者直接 [...]
    if (Array.isArray(data)) {
      items.value = data
    } else if (Array.isArray(data?.items)) {
      items.value = data.items
    } else {
      items.value = []
    }
  } catch {
    // 静默失败，不打扰用户
  } finally {
    loading.value = false
  }
}

async function markAsRead(id: number) {
  try {
    await axios.post(`/api/notifications/${id}/read`)
    items.value = items.value.map((it) => (it.id === id ? { ...it, is_read: true } : it))
  } catch {
    // 忽略
  }
}

async function markAllRead() {
  try {
    await axios.post('/api/notifications/mark-all-read')
    items.value = items.value.map((it) => ({ ...it, is_read: true }))
  } catch {
    // 忽略
  }
}

export function useNotifications() {
  const unreadCount = computed(() => items.value.filter((it) => !it.is_read).length)

  onMounted(() => {
    mountedCount += 1
    if (mountedCount === 1) {
      fetchNotifications()
      // 30s 轮询；后续 §二十四 改为 WebSocket /ws/notify
      timer = setInterval(fetchNotifications, 30_000)
    }
  })

  onUnmounted(() => {
    mountedCount -= 1
    if (mountedCount === 0 && timer) {
      clearInterval(timer)
      timer = null
    }
  })

  return {
    items,
    loading,
    unreadCount,
    refresh: fetchNotifications,
    markAsRead,
    markAllRead,
  }
}
