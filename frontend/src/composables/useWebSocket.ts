/**
 * SSE composable - 实时进度推送与通知.
 *
 * 使用 Server-Sent Events（EventSource）代替 WebSocket，
 * 连接 Go 后端的 GET /api/sse/events?token=xxx 端点。
 *
 * 支持事件类型：
 * - progress: 解析/评价进度推送
 * - notification: 通知推送
 * - score_complete: 评分完成
 * - similarity_alert: 相似度告警
 * - verify_complete/verify_failed: 核查结果
 *
 * 自动重连 + token 认证。
 */
import { onMounted, onUnmounted, ref, type Ref } from 'vue'
import { useAuthStore } from '@/stores/auth'

export interface ProgressMessage {
  upload_id: number
  status: 'pending' | 'parsing' | 'parsed' | 'failed' | 'scoring' | 'scored'
  progress: number
  error: string | null
}

export interface NotifyMessage {
  id: number
  type: string
  title: string
  content: string
  is_read: boolean
  payload: Record<string, unknown> | null
}

type EventType = 'progress' | 'notification' | 'score_complete' | 'similarity_alert' | 'verify_complete' | 'verify_failed'

interface UseSSEOptions {
  /** 自动重连间隔（ms），默认 3000 */
  reconnectInterval?: number
  /** 最大重连次数，默认 10 */
  maxRetries?: number
  /** 是否自动连接，默认 true */
  autoConnect?: boolean
}

export function useSSE<T = unknown>(
  options: UseSSEOptions = {},
) {
  const { reconnectInterval = 3000, maxRetries = 10, autoConnect = true } = options

  const messages: Ref<T[]> = ref([])
  const lastMessage: Ref<T | null> = ref(null)
  const connected = ref(false)
  const error = ref<string | null>(null)

  let eventSource: EventSource | null = null
  let retryCount = 0
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  let destroyed = false

  // 按事件类型注册的监听器
  const listeners: Record<string, Array<(data: T) => void>> = {}

  /** 注册指定事件的回调 */
  function on(eventType: EventType, callback: (data: T) => void) {
    if (!listeners[eventType]) {
      listeners[eventType] = []
    }
    listeners[eventType].push(callback)
  }

  function getSSEUrl(): string {
    const auth = useAuthStore()
    const token = auth.token || ''
    const base = window.location.origin
    return `${base}/api/sse/events?token=${encodeURIComponent(token)}`
  }

  function connect() {
    if (destroyed) return
    if (eventSource && eventSource.readyState !== EventSource.CLOSED) {
      return
    }

    const url = getSSEUrl()
    eventSource = new EventSource(url)

    eventSource.onopen = () => {
      connected.value = true
      error.value = null
      retryCount = 0
    }

    // 监听所有已知事件类型
    const eventTypes: EventType[] = ['progress', 'notification', 'score_complete', 'similarity_alert', 'verify_complete', 'verify_failed']
    for (const et of eventTypes) {
      eventSource.addEventListener(et, (event: MessageEvent) => {
        try {
          const data = JSON.parse(event.data) as T
          lastMessage.value = data
          messages.value = [...messages.value.slice(-99), data]
          // 触发自定义监听器
          if (listeners[et]) {
            listeners[et].forEach(cb => cb(data))
          }
        } catch {
          // 忽略非 JSON 消息
        }
      })
    }

    // 通用消息处理（包括 connected 事件等）
    eventSource.onmessage = (event: MessageEvent) => {
      try {
        const data = JSON.parse(event.data) as T
        lastMessage.value = data
        messages.value = [...messages.value.slice(-99), data]
      } catch {
        // 忽略非 JSON
      }
    }

    eventSource.onerror = () => {
      error.value = 'SSE 连接错误'
      connected.value = false
      eventSource?.close()
      eventSource = null
      // 自动重连
      if (!destroyed && retryCount < maxRetries) {
        retryCount++
        reconnectTimer = setTimeout(connect, reconnectInterval)
      }
    }
  }

  function disconnect() {
    destroyed = true
    if (reconnectTimer) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
    if (eventSource) {
      eventSource.close()
      eventSource = null
    }
    connected.value = false
  }

  function clearMessages() {
    messages.value = []
    lastMessage.value = null
  }

  onMounted(() => {
    if (autoConnect) {
      connect()
    }
  })

  onUnmounted(() => {
    disconnect()
  })

  return {
    messages,
    lastMessage,
    connected,
    error,
    connect,
    disconnect,
    clearMessages,
    on, // 注册事件监听器
  }
}
