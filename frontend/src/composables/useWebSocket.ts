/**
 * WebSocket composable - 实时进度推送与通知.
 *
 * 支持两个频道：
 * - "progress": 解析/评价进度推送
 * - "notify": 通知推送
 *
 * 自动重连 + 心跳检测 + token 认证。
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

type Channel = 'progress' | 'notify'

interface UseWebSocketOptions {
  /** 自动重连间隔（ms），默认 3000 */
  reconnectInterval?: number
  /** 最大重连次数，默认 10 */
  maxRetries?: number
  /** 是否自动连接，默认 true */
  autoConnect?: boolean
}

export function useWebSocket<T = ProgressMessage | NotifyMessage>(
  channel: Channel,
  options: UseWebSocketOptions = {},
) {
  const { reconnectInterval = 3000, maxRetries = 10, autoConnect = true } = options

  const messages: Ref<T[]> = ref([])
  const lastMessage: Ref<T | null> = ref(null)
  const connected = ref(false)
  const error = ref<string | null>(null)

  let ws: WebSocket | null = null
  let retryCount = 0
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  let destroyed = false

  function getWsUrl(): string {
    const auth = useAuthStore()
    const token = auth.token || ''
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const host = window.location.host
    return `${protocol}//${host}/ws/${channel}?token=${encodeURIComponent(token)}`
  }

  function connect() {
    if (destroyed) return
    if (ws && (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING)) {
      return
    }

    const url = getWsUrl()
    ws = new WebSocket(url)

    ws.onopen = () => {
      connected.value = true
      error.value = null
      retryCount = 0
    }

    ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data) as T
        lastMessage.value = data
        messages.value = [...messages.value.slice(-99), data] // 保留最近 100 条
      } catch {
        // 忽略非 JSON 消息
      }
    }

    ws.onerror = () => {
      error.value = 'WebSocket 连接错误'
      connected.value = false
    }

    ws.onclose = () => {
      connected.value = false
      ws = null
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
    if (ws) {
      ws.close()
      ws = null
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
  }
}
