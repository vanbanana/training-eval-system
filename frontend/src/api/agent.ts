/**
 * Agent API Client — unified interface for /api/agent/* endpoints.
 *
 * Handles:
 * - Session CRUD
 * - SSE streaming with structured event parsing
 * - Auth token injection (reads from localStorage)
 * - Error classification (401, 403, 429, network)
 */
import axios from 'axios'

// ============ Types ============

export interface AgentSession {
  id: number
  title: string
  agent_role?: string
  context_json?: string
  created_at: string
  last_active_at?: string
}

export interface AgentMessage {
  id: number
  role: string
  content: string
  tool_call_id?: string | null
  tool_name?: string | null
  prompt_tokens?: number
  completion_tokens?: number
  created_at: string
}

/** Structured SSE event from the streaming endpoint. */
export interface AgentSSEEvent {
  type: 'thinking' | 'tool_call' | 'tool_start' | 'tool_result' | 'text' | 'done' | 'error'
  content?: string
  name?: string
  tool?: string
  args?: Record<string, unknown>
  success?: boolean
  data?: unknown
  error?: string
  message?: string
  full_content?: string
}

/** Context payload sent with stream requests. */
export interface AgentStreamContext {
  evaluation_id?: number
  task_id?: number
  class_id?: number
  course_id?: number
}

/** Request body for the stream endpoint. */
export interface AgentStreamRequest {
  session_id: number | null
  message: string
  agent_role: string
  context?: AgentStreamContext
}

/** Classified error from agent API calls. */
export class AgentError extends Error {
  status: number
  code: 'unauthorized' | 'forbidden' | 'rate_limited' | 'network' | 'server' | 'unknown'

  constructor(
    message: string,
    status: number,
    code: 'unauthorized' | 'forbidden' | 'rate_limited' | 'network' | 'server' | 'unknown',
  ) {
    super(message)
    this.name = 'AgentError'
    this.status = status
    this.code = code
  }
}

// ============ Helpers ============

function getToken(): string {
  const raw = localStorage.getItem('tes_token')
  if (!raw) return ''
  try {
    return JSON.parse(raw)
  } catch {
    return raw
  }
}

function classifyError(status: number, body?: { message?: string; code?: string }): AgentError {
  const msg = body?.message ?? ''
  switch (status) {
    case 401:
      return new AgentError(msg || '登录已过期，请重新登录', 401, 'unauthorized')
    case 403:
      return new AgentError(msg || '无权限访问该资源', 403, 'forbidden')
    case 429:
      return new AgentError(msg || '请求过于频繁，请稍后再试', 429, 'rate_limited')
    default:
      if (status >= 500) return new AgentError(msg || '服务器内部错误', status, 'server')
      return new AgentError(msg || `请求失败 (${status})`, status, 'unknown')
  }
}

// ============ Session CRUD (via axios — uses global interceptors) ============

export async function listSessions(): Promise<AgentSession[]> {
  const { data } = await axios.get('/api/agent/sessions')
  return Array.isArray(data) ? data : []
}

export async function createSession(
  title: string,
  agentRole: string,
  context?: AgentStreamContext,
): Promise<AgentSession> {
  const body: Record<string, unknown> = { title, agent_role: agentRole }
  if (context) body.context = context
  const { data } = await axios.post('/api/agent/sessions', body)
  return data
}

export async function getMessages(sessionId: number): Promise<AgentMessage[]> {
  const { data } = await axios.get(`/api/agent/sessions/${sessionId}/messages`)
  return Array.isArray(data) ? data : []
}

export async function deleteSession(sessionId: number): Promise<void> {
  await axios.delete(`/api/agent/sessions/${sessionId}`)
}

// ============ SSE Streaming (via fetch — native ReadableStream) ============

export interface StreamCallbacks {
  onEvent: (event: AgentSSEEvent) => void
  onSessionId?: (id: number) => void
  onDone?: () => void
  onError?: (err: AgentError) => void
}

/**
 * Stream a message to the agent endpoint and parse SSE events.
 * Returns an AbortController so the caller can cancel the request.
 */
export function streamMessage(
  req: AgentStreamRequest,
  callbacks: StreamCallbacks,
): AbortController {
  const abort = new AbortController()
  const token = getToken()

  // Fire-and-forget async — caller uses abort.signal to cancel
  ;(async () => {
    try {
      const res = await fetch('/api/agent/stream', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify(req),
        signal: abort.signal,
      })

      if (!res.ok) {
        let body: { message?: string } | undefined
        try {
          body = await res.json()
        } catch {
          // ignore parse failure
        }
        const err = classifyError(res.status, body)
        if (err.code === 'unauthorized') {
          window.location.href = '/login'
          return
        }
        callbacks.onError?.(err)
        return
      }

      // Extract session ID from response header
      const sid = res.headers.get('X-Session-Id')
      if (sid) {
        const parsed = parseInt(sid, 10)
        if (!isNaN(parsed)) callbacks.onSessionId?.(parsed)
      }

      // Parse SSE stream
      const reader = res.body?.getReader()
      const decoder = new TextDecoder()
      if (!reader) return

      let buffer = ''
      while (true) {
        const { done, value } = await reader.read()
        if (done) break
        buffer += decoder.decode(value, { stream: true })
        const lines = buffer.split('\n')
        buffer = lines.pop() ?? ''

        for (const line of lines) {
          if (!line.startsWith('data: ')) continue
          const payload = line.slice(6).trim()
          if (!payload) continue

          // Legacy [DONE] marker
          if (payload === '[DONE]') {
            callbacks.onDone?.()
            continue
          }

          try {
            const event = JSON.parse(payload) as AgentSSEEvent
            callbacks.onEvent(event)
            if (event.type === 'done') {
              callbacks.onDone?.()
            }
          } catch {
            // Fallback: treat as plain text event
            callbacks.onEvent({ type: 'text', content: payload })
          }
        }
      }
      callbacks.onDone?.()
    } catch (e: unknown) {
      if ((e as Error).name === 'AbortError') return // cancelled intentionally
      callbacks.onError?.(
        new AgentError((e as Error).message || '网络连接失败', 0, 'network'),
      )
    }
  })()

  return abort
}
