/**
 * useAgentChat — composable encapsulating agent chat state & logic.
 *
 * Provides session management, SSE streaming, message blocks, and cleanup.
 * Used by student ChatView, teacher AgentView, and admin AgentView.
 */
import { ref, computed, onMounted, onUnmounted, type Ref } from 'vue'
import {
  listSessions,
  createSession,
  getMessages,
  deleteSession,
  streamMessage,
  type AgentSession,
  type AgentMessage,
  type AgentSSEEvent,
  type AgentStreamContext,
  type AgentError,
} from '@/api/agent'

// Re-export types needed by views
export type { AgentStreamContext } from '@/api/agent'

// ============ Types ============

/** A rendering block within a chat message. */
export interface MessageBlock {
  type: 'text' | 'thinking' | 'tool_call' | 'tool_result' | 'error'
  content?: string
  name?: string
  args?: Record<string, unknown>
  success?: boolean
  data?: unknown
  error?: string
}

/** A single chat message (user or assistant). */
export interface ChatMessage {
  id?: number
  role: 'user' | 'assistant'
  content: string
  blocks: MessageBlock[]
  created_at?: string
  isStreaming?: boolean
}

export interface UseAgentChatOptions {
  agentRole: string
  /** Optional reactive context that gets sent with each stream request. */
  context?: Ref<AgentStreamContext | undefined>
  /** Reactive label describing the current context for display. */
  contextLabel?: Ref<string>
  /** Default title for new sessions. */
  defaultTitle?: string
  /** Whether to auto-load sessions on mount. */
  autoLoad?: boolean
}

// ============ Composable ============

export function useAgentChat(options: UseAgentChatOptions) {
  const {
    agentRole,
    context,
    defaultTitle = '新对话',
    autoLoad = true,
    contextLabel,
  } = options

  // State
  const sessions = ref<AgentSession[]>([])
  const activeSessionId = ref<number | null>(null)
  const messages = ref<ChatMessage[]>([])
  const input = ref('')
  const sending = ref(false)
  const searchQuery = ref('')
  const streamError = ref<string | null>(null)
  const hasMoreHistory = ref(false)
  const allHistoryMessages = ref<AgentMessage[]>([])

  const HISTORY_PAGE_SIZE = 50

  // Internal
  let abortCtrl: AbortController | null = null

  // ============ Session Management ============

  async function fetchSessions() {
    try {
      sessions.value = await listSessions()
      if (sessions.value.length > 0 && !activeSessionId.value) {
        await loadSession(sessions.value[0].id)
      }
    } catch {
      // ignore — sessions remain empty
    }
  }

  async function loadSession(id: number) {
    activeSessionId.value = id
    streamError.value = null
    try {
      const raw = await getMessages(id)
      allHistoryMessages.value = raw
      const displayed = raw.slice(-HISTORY_PAGE_SIZE)
      hasMoreHistory.value = raw.length > HISTORY_PAGE_SIZE
      messages.value = displayed.map((m: AgentMessage) => ({
        id: m.id,
        role: m.role === 'user' ? ('user' as const) : ('assistant' as const),
        content: m.content,
        blocks: [{ type: 'text' as const, content: m.content }],
        created_at: m.created_at,
      }))
    } catch {
      messages.value = []
      allHistoryMessages.value = []
      hasMoreHistory.value = false
    }
  }

  /** Load older history messages (prepend to current list). */
  function loadMoreHistory() {
    if (!hasMoreHistory.value) return
    const currentCount = messages.value.length
    const older = allHistoryMessages.value.slice(
      Math.max(0, allHistoryMessages.value.length - currentCount - HISTORY_PAGE_SIZE),
      allHistoryMessages.value.length - currentCount,
    )
    const olderMapped = older.map((m: AgentMessage) => ({
      id: m.id,
      role: m.role === 'user' ? ('user' as const) : ('assistant' as const),
      content: m.content,
      blocks: [{ type: 'text' as const, content: m.content }],
      created_at: m.created_at,
    }))
    messages.value = [...olderMapped, ...messages.value]
    hasMoreHistory.value = messages.value.length < allHistoryMessages.value.length
  }

  async function newSession() {
    try {
      const s = await createSession(defaultTitle, agentRole, context?.value)
      sessions.value.unshift(s)
      activeSessionId.value = s.id
      messages.value = []
      streamError.value = null
    } catch {
      activeSessionId.value = null
      messages.value = []
    }
  }

  async function removeSession(id?: number) {
    const target = id ?? activeSessionId.value
    if (!target) return
    const ok = window.confirm('确定删除该会话？此操作不可撤销。')
    if (!ok) return
    try {
      await deleteSession(target)
      sessions.value = sessions.value.filter((s) => s.id !== target)
      if (activeSessionId.value === target) {
        activeSessionId.value = null
        messages.value = []
      }
    } catch {
      // ignore
    }
  }

  // ============ Send Message (SSE) ============

  async function send(overrideMsg?: string) {
    const msg = (overrideMsg ?? input.value).trim()
    if (!msg || sending.value) return

    sending.value = true
    streamError.value = null
    if (!overrideMsg) input.value = ''

    // Push user message
    messages.value.push({
      role: 'user',
      content: msg,
      blocks: [{ type: 'text', content: msg }],
      created_at: new Date().toISOString(),
    })

    // AI placeholder
    const aiMsg: ChatMessage = {
      role: 'assistant',
      content: '',
      blocks: [],
      created_at: new Date().toISOString(),
      isStreaming: true,
    }
    messages.value.push(aiMsg)

    // Auto-create session if needed
    if (!activeSessionId.value) {
      try {
        const s = await createSession(msg.slice(0, 30), agentRole, context?.value)
        activeSessionId.value = s.id
        sessions.value.unshift(s)
      } catch {
        // proceed — backend may auto-create
      }
    }

    let currentTextBlock: MessageBlock | null = null

    abortCtrl = streamMessage(
      {
        session_id: activeSessionId.value,
        message: msg,
        agent_role: agentRole,
        context: context?.value,
      },
      {
        onEvent(event: AgentSSEEvent) {
          switch (event.type) {
            case 'thinking':
              currentTextBlock = null
              aiMsg.blocks.push({ type: 'thinking', content: event.content })
              messages.value = [...messages.value]
              break

            case 'tool_call':
            case 'tool_start':
              currentTextBlock = null
              aiMsg.blocks.push({
                type: 'tool_call',
                name: event.name ?? event.tool,
                args: event.args,
              })
              messages.value = [...messages.value]
              break

            case 'tool_result':
              currentTextBlock = null
              aiMsg.blocks.push({
                type: 'tool_result',
                name: event.name,
                success: event.success,
                data: event.data,
                error: event.error ?? undefined,
              })
              messages.value = [...messages.value]
              break

            case 'text':
              if (!currentTextBlock || currentTextBlock.type !== 'text') {
                currentTextBlock = { type: 'text', content: '' }
                aiMsg.blocks.push(currentTextBlock)
              }
              currentTextBlock.content = (currentTextBlock.content ?? '') + (event.content ?? '')
              aiMsg.content += event.content ?? ''
              messages.value = [...messages.value]
              break

            case 'done':
              aiMsg.isStreaming = false
              if (event.full_content) aiMsg.content = event.full_content
              messages.value = [...messages.value]
              break

            case 'error':
              currentTextBlock = null
              aiMsg.blocks.push({ type: 'error', content: event.message })
              messages.value = [...messages.value]
              break
          }
        },

        onSessionId(id: number) {
          if (id !== activeSessionId.value) {
            activeSessionId.value = id
            if (!sessions.value.find((s) => s.id === id)) {
              sessions.value.unshift({
                id,
                title: sanitizeTitle(msg.slice(0, 30)),
                created_at: new Date().toISOString(),
              })
            }
          }
          // Update default title from first message
          const s = sessions.value.find((s) => s.id === id)
          if (s && (!s.title || s.title === defaultTitle)) {
            s.title = sanitizeTitle(msg.slice(0, 30))
          }
        },

        onDone() {
          aiMsg.isStreaming = false
          messages.value = [...messages.value]
        },

        onError(err: AgentError) {
          streamError.value = err.message
          aiMsg.blocks.push({ type: 'error', content: err.message })
          aiMsg.isStreaming = false
          messages.value = [...messages.value]
        },
      },
    )

    // Wait for stream to finish by polling isStreaming
    // (the stream callback runs async; we need to wait)
    await waitForStream(aiMsg)

    sending.value = false
    abortCtrl = null
  }

  function waitForStream(msg: ChatMessage): Promise<void> {
    return new Promise((resolve) => {
      const check = () => {
        if (!msg.isStreaming || !abortCtrl) {
          resolve()
        } else {
          setTimeout(check, 100)
        }
      }
      check()
    })
  }

  function abort() {
    if (abortCtrl) {
      abortCtrl.abort()
      abortCtrl = null
      sending.value = false
      // Mark last AI message as not streaming
      const last = messages.value[messages.value.length - 1]
      if (last?.isStreaming) {
        last.isStreaming = false
        messages.value = [...messages.value]
      }
    }
  }

  // ============ Computed ============

  const filteredSessions = computed(() => {
    if (!searchQuery.value.trim()) return sessions.value
    const q = searchQuery.value.trim().toLowerCase()
    return sessions.value.filter((s) => (s.title ?? '').toLowerCase().includes(q))
  })

  const activeSession = computed(() =>
    sessions.value.find((s) => s.id === activeSessionId.value) ?? null,
  )

  // ============ Lifecycle ============

  onMounted(() => {
    if (autoLoad) fetchSessions()
  })

  onUnmounted(() => {
    abort()
  })

  // ============ Return ============

  return {
    // State
    sessions,
    activeSessionId,
    messages,
    input,
    sending,
    searchQuery,
    streamError,
    hasMoreHistory,
    // Computed
    filteredSessions,
    activeSession,
    contextLabel: contextLabel ?? ref(''),
    // Actions
    fetchSessions,
    loadSession,
    newSession,
    removeSession,
    send,
    abort,
    loadMoreHistory,
  }
}

// ============ Helpers ============

/** Format ISO timestamp to relative time string. */
export function formatRelativeTime(iso: string): string {
  if (!iso) return ''
  const diff = Date.now() - new Date(iso).getTime()
  const min = Math.floor(diff / 60000)
  if (min < 1) return '刚刚'
  if (min < 60) return `${min} 分钟前`
  const hour = Math.floor(min / 60)
  if (hour < 24) return `${hour} 小时前`
  return iso.slice(5, 10)
}

/** Toggle a tool block's collapsed state. */
export function useToolCollapse() {
  const collapsed = ref<Set<number>>(new Set())
  function toggle(key: number) {
    if (collapsed.value.has(key)) {
      collapsed.value.delete(key)
    } else {
      collapsed.value.add(key)
    }
    collapsed.value = new Set(collapsed.value)
  }
  return { collapsed, toggle }
}

/** Auto-resize a textarea element. */
export function autoResizeTextarea(e: Event, maxH = 160) {
  const el = e.target as HTMLTextAreaElement
  el.style.height = 'auto'
  el.style.height = Math.min(el.scrollHeight, maxH) + 'px'
}

/** Sanitize a session title to prevent sensitive data leakage. */
function sanitizeTitle(title: string): string {
  // Remove patterns that look like keys/tokens/passwords
  return title
    .replace(/[A-Za-z0-9_\-]{20,}/g, '[已隐藏]')
    .replace(/password\s*[:=]\s*\S+/gi, '***')
    .replace(/token\s*[:=]\s*\S+/gi, '***')
    .replace(/api[_-]?key\s*[:=]\s*\S+/gi, '***')
    .trim() || '新对话'
}
