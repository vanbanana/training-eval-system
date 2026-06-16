<script setup lang="ts">
/**
 * AI 问答助手 — Agent 风格 UI
 *
 * 特性：
 * - 结构化 SSE 事件解析（thinking / tool_call / tool_result / text / done / error）
 * - Markdown 渲染（代码高亮、表格、链接）
 * - 工具调用过程可视化（可折叠卡片）
 * - 思考状态动画
 * - 增强输入框（附件上传占位、快捷问题）
 * - 会话管理（左侧列表）
 */
import { ref, computed, onMounted, onUnmounted, nextTick, watch } from 'vue'
import axios from 'axios'
import AppShell from '@/components/layout/AppShell.vue'
import { renderMarkdown } from '@/lib/markdown'
import {
  ChevronRight,
  Search,
  Sparkles,
  Send,
  Share2,
  Trash2,
  Plus,
  MessageSquare,
  Loader2,
  ChevronDown,
  ChevronUp,
  Wrench,
  CheckCircle2,
  XCircle,
  Paperclip,
  Image as ImageIcon,
} from 'lucide-vue-next'

// ============ Types ============
interface Session {
  id: number
  title: string
  evaluation_id: number | null
  created_at: string
}

// 消息中的一个"块"（文本 / 工具调用 / 思考）
interface MessageBlock {
  type: 'text' | 'thinking' | 'tool_call' | 'tool_result' | 'error'
  content?: string
  name?: string
  args?: Record<string, unknown>
  success?: boolean
  data?: unknown
  error?: string
}

interface ChatMessage {
  id?: number
  role: 'user' | 'assistant'
  content: string // 纯文本（用于持久化）
  blocks: MessageBlock[] // 结构化块（用于渲染）
  created_at?: string
  isStreaming?: boolean
}

// ============ State ============
const sessions = ref<Session[]>([])
const activeSessionId = ref<number | null>(null)
const messages = ref<ChatMessage[]>([])
const input = ref('')
const sending = ref(false)
const searchQuery = ref('')
const chatBodyRef = ref<HTMLElement | null>(null)
const collapsedTools = ref<Set<number>>(new Set())

// ============ Session Management ============
async function fetchSessions() {
  try {
    const { data } = await axios.get('/api/chat/sessions')
    sessions.value = Array.isArray(data) ? data : []
    if (sessions.value.length > 0 && !activeSessionId.value) {
      await loadSession(sessions.value[0].id)
    }
  } catch {
    // ignore
  }
}

async function loadSession(id: number) {
  activeSessionId.value = id
  try {
    const { data } = await axios.get(`/api/chat/sessions/${id}/messages`)
    messages.value = (Array.isArray(data) ? data : []).map((m: { id: number; role: string; content: string; created_at: string }) => ({
      id: m.id,
      role: m.role as 'user' | 'assistant',
      content: m.content,
      blocks: [{ type: 'text' as const, content: m.content }],
      created_at: m.created_at,
    }))
    scrollToBottom()
  } catch {
    messages.value = []
  }
}

async function newSession() {
  try {
    const { data } = await axios.post('/api/chat/sessions', { title: '新对话' })
    sessions.value.unshift({
      id: data.id,
      title: data.title ?? '新对话',
      evaluation_id: null,
      created_at: new Date().toISOString(),
    })
    activeSessionId.value = data.id
    messages.value = []
  } catch {
    activeSessionId.value = null
    messages.value = []
  }
}

async function deleteSession() {
  if (!activeSessionId.value) return
  if (!confirm('删除当前会话？此操作不可撤销。')) return
  try {
    await axios.delete(`/api/chat/sessions/${activeSessionId.value}`)
    sessions.value = sessions.value.filter((s) => s.id !== activeSessionId.value)
    activeSessionId.value = null
    messages.value = []
  } catch {
    // ignore
  }
}

function scrollToBottom() {
  nextTick(() => {
    if (chatBodyRef.value) {
      chatBodyRef.value.scrollTop = chatBodyRef.value.scrollHeight
    }
  })
}

onMounted(fetchSessions)
watch(messages, scrollToBottom, { deep: true })

// Abort any in-flight SSE stream when the component unmounts.
let streamAbort: AbortController | null = null
onUnmounted(() => {
  if (streamAbort) {
    streamAbort.abort()
    streamAbort = null
  }
})

// ============ Send Message (SSE with structured events) ============
async function send() {
  const msg = input.value.trim()
  if (!msg || sending.value) return
  sending.value = true
  input.value = ''

  // 用户消息
  messages.value.push({
    role: 'user',
    content: msg,
    blocks: [{ type: 'text', content: msg }],
    created_at: new Date().toISOString(),
  })

  // AI 消息占位（流式填充）
  const aiMsg: ChatMessage = {
    role: 'assistant',
    content: '',
    blocks: [],
    created_at: new Date().toISOString(),
    isStreaming: true,
  }
  messages.value.push(aiMsg)
  scrollToBottom()

  try {
    const raw = localStorage.getItem('tes_token')
    let token = ''
    if (raw) {
      try { token = JSON.parse(raw) } catch { token = raw }
    }

    streamAbort = new AbortController()
    const response = await fetch('/api/chat/stream', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${token}`,
      },
      body: JSON.stringify({ session_id: activeSessionId.value, message: msg }),
      signal: streamAbort.signal,
    })

    if (!response.ok) throw new Error(`HTTP ${response.status}`)

    // 更新 session ID
    const sid = response.headers.get('X-Session-Id')
    if (sid) {
      const newId = parseInt(sid, 10)
      if (newId !== activeSessionId.value) {
        activeSessionId.value = newId
        if (!sessions.value.find((s) => s.id === newId)) {
          sessions.value.unshift({
            id: newId,
            title: msg.slice(0, 30),
            evaluation_id: null,
            created_at: new Date().toISOString(),
          })
        }
      }
    }

    // 解析 SSE 流
    const reader = response.body?.getReader()
    const decoder = new TextDecoder()
    if (reader) {
      let buffer = ''
      let currentTextBlock: MessageBlock | null = null

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

          try {
            const event = JSON.parse(payload) as {
              type: string
              content?: string
              name?: string
              args?: Record<string, unknown>
              success?: boolean
              data?: unknown
              error?: string
              message?: string
              full_content?: string
            }

            switch (event.type) {
              case 'thinking':
                currentTextBlock = null
                aiMsg.blocks.push({ type: 'thinking', content: event.content })
                // 强制触发响应式更新 + 渲染
                messages.value = [...messages.value]
                await nextTick()
                break

              case 'tool_call':
                currentTextBlock = null
                aiMsg.blocks.push({
                  type: 'tool_call',
                  name: event.name,
                  args: event.args,
                })
                messages.value = [...messages.value]
                await nextTick()
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
                await nextTick()
                break

              case 'text':
                if (!currentTextBlock || currentTextBlock.type !== 'text') {
                  currentTextBlock = { type: 'text', content: '' }
                  aiMsg.blocks.push(currentTextBlock)
                }
                currentTextBlock.content = (currentTextBlock.content ?? '') + (event.content ?? '')
                aiMsg.content += event.content ?? ''
                // 文本流式：每次触发更新（逐字效果）
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
          } catch {
            // 兼容旧格式（纯文本）
            if (payload === '[DONE]') {
              aiMsg.isStreaming = false
            } else {
              if (!currentTextBlock || currentTextBlock.type !== 'text') {
                currentTextBlock = { type: 'text', content: '' }
                aiMsg.blocks.push(currentTextBlock)
              }
              currentTextBlock.content = (currentTextBlock.content ?? '') + payload
              aiMsg.content += payload
            }
            messages.value = [...messages.value]
          }
          scrollToBottom()
        }
      }
    }
    aiMsg.isStreaming = false
  } catch (e: unknown) {
    aiMsg.blocks.push({ type: 'error', content: `发送失败：${(e as Error).message || '网络错误'}` })
    aiMsg.isStreaming = false
  } finally {
    sending.value = false
    streamAbort = null
    scrollToBottom()
  }
}

// ============ Helpers ============
const filteredSessions = computed(() => {
  if (!searchQuery.value.trim()) return sessions.value
  const q = searchQuery.value.trim().toLowerCase()
  return sessions.value.filter((s) => s.title.toLowerCase().includes(q))
})

const activeSession = computed(() =>
  sessions.value.find((s) => s.id === activeSessionId.value) ?? null,
)

function formatTime(iso: string) {
  if (!iso) return ''
  const diff = Date.now() - new Date(iso).getTime()
  const min = Math.floor(diff / 60000)
  if (min < 1) return '刚刚'
  if (min < 60) return `${min} 分钟前`
  const hour = Math.floor(min / 60)
  if (hour < 24) return `${hour} 小时前`
  return iso.slice(5, 10)
}

function toggleToolCollapse(blockIdx: number) {
  if (collapsedTools.value.has(blockIdx)) {
    collapsedTools.value.delete(blockIdx)
  } else {
    collapsedTools.value.add(blockIdx)
  }
  collapsedTools.value = new Set(collapsedTools.value)
}

// 快捷问题
const quickQuestions = [
  '我最近的评分怎么样？',
  '我的薄弱点是什么？',
  '怎么提高代码规范性？',
]

function askQuick(q: string) {
  input.value = q
  void send()
}

function autoResize(e: Event) {
  const el = e.target as HTMLTextAreaElement
  el.style.height = 'auto'
  el.style.height = Math.min(el.scrollHeight, 160) + 'px'
}
</script>

<template>
  <AppShell>
    <!-- Breadcrumb -->
    <nav class="flex items-center gap-2 text-xs text-muted-foreground">
      <span>我的</span>
      <ChevronRight class="w-3 h-3 text-subtle-foreground" />
      <span class="text-ink font-semibold">AI 问答助手</span>
    </nav>

    <!-- Page Header -->
    <div class="flex justify-between items-end">
      <div>
        <div class="flex items-center gap-3">
          <h1 class="text-2xl font-bold text-ink m-0">AI 问答助手</h1>
          <span class="inline-flex items-center gap-1.5 px-2.5 py-0.5 bg-primary-soft text-primary rounded-full text-[11px] font-semibold">
            <Sparkles class="w-3 h-3" />
            Agent 模式
          </span>
        </div>
        <p class="mt-1.5 text-sm text-muted-foreground">
          支持多轮对话 · 可查询你的评价数据 · Markdown 渲染
        </p>
      </div>
      <div class="flex items-center gap-3">
        <button class="inline-flex items-center gap-1.5 h-9 px-4 bg-surface border border-border-strong rounded-md text-sm font-semibold text-ink hover:bg-surface-2 transition-colors" @click="deleteSession" :disabled="!activeSessionId">
          <Trash2 class="w-4 h-4" />
          删除会话
        </button>
        <button class="inline-flex items-center gap-1.5 h-9 px-4 bg-primary text-white rounded-md text-sm font-semibold hover:bg-primary-strong transition-colors" @click="newSession">
          <Plus class="w-4 h-4" />
          新建对话
        </button>
      </div>
    </div>

    <!-- Chat Shell -->
    <div class="tes-chat-shell">
      <!-- Left: Session List -->
      <aside class="tes-chat-panel bg-surface border border-border rounded-lg flex flex-col overflow-hidden">
        <div class="p-3 border-b border-border">
          <div class="flex items-center gap-2 h-9 px-3 bg-surface border border-border-strong rounded-md">
            <Search class="w-3.5 h-3.5 text-muted-foreground" />
            <input v-model="searchQuery" type="text" placeholder="搜索会话" class="flex-1 border-0 outline-none bg-transparent text-sm text-ink placeholder:text-subtle-foreground" />
          </div>
        </div>
        <div class="flex-1 overflow-y-auto">
          <div v-if="filteredSessions.length === 0" class="px-4 py-12 text-center text-xs text-muted-foreground">
            <MessageSquare class="w-8 h-8 text-subtle-foreground mx-auto mb-2" />
            <p>暂无会话</p>
          </div>
          <div
            v-for="s in filteredSessions"
            :key="s.id"
            class="flex flex-col gap-1 px-4 py-3 border-b border-border last:border-b-0 cursor-pointer transition-colors"
            :class="activeSessionId === s.id ? 'bg-primary-soft border-l-[3px] border-l-primary pl-[13px]' : 'hover:bg-surface-2'"
            @click="loadSession(s.id)"
          >
            <span class="text-sm font-semibold text-ink truncate">{{ s.title }}</span>
            <span class="text-[11px] text-muted-foreground">{{ formatTime(s.created_at) }}</span>
          </div>
        </div>
      </aside>

      <!-- Right: Chat Area -->
      <section class="tes-chat-panel bg-surface border border-border rounded-lg flex flex-col overflow-hidden">
        <!-- Chat Header -->
        <div class="px-6 py-4 border-b border-border flex justify-between items-center">
          <div class="flex flex-col gap-1">
            <span class="text-md font-bold text-ink">{{ activeSession?.title ?? '新建对话' }}</span>
            <span class="text-[11px] text-muted-foreground">
              {{ messages.length }} 条消息 · {{ activeSession ? formatTime(activeSession.created_at) : '开始新对话' }}
            </span>
          </div>
          <div v-if="activeSession" class="flex gap-1">
            <button class="w-8 h-8 rounded-md text-muted-foreground hover:bg-surface-2 hover:text-ink grid place-items-center" title="分享">
              <Share2 class="w-3.5 h-3.5" />
            </button>
          </div>
        </div>

        <!-- Chat Body -->
        <div ref="chatBodyRef" class="flex-1 px-6 py-5 overflow-y-auto flex flex-col gap-5">
          <!-- Empty State -->
          <div v-if="messages.length === 0" class="flex-1 flex flex-col items-center justify-center text-center gap-4">
            <div class="w-16 h-16 bg-primary-soft text-primary rounded-2xl grid place-items-center">
              <Sparkles class="w-8 h-8" />
            </div>
            <div>
              <p class="text-base font-semibold text-ink">AI 学习助手</p>
              <p class="text-sm text-muted-foreground mt-1">我可以帮你分析评价结果、查询成绩数据、给出改进建议</p>
            </div>
            <!-- Quick Questions -->
            <div class="flex flex-wrap gap-2 mt-2">
              <button
                v-for="q in quickQuestions"
                :key="q"
                class="px-3.5 py-2 bg-surface-2 border border-border rounded-lg text-xs font-medium text-ink hover:border-primary hover:bg-primary-soft transition-colors"
                @click="askQuick(q)"
              >
                {{ q }}
              </button>
            </div>
          </div>

          <!-- Messages -->
          <template v-for="(m, mIdx) in messages" :key="mIdx">
            <!-- User Message -->
            <div v-if="m.role === 'user'" class="flex justify-end">
              <div class="max-w-[70%] px-4 py-3 bg-primary text-white rounded-2xl rounded-tr-md text-sm leading-relaxed whitespace-pre-wrap">
                {{ m.content }}
              </div>
            </div>

            <!-- Assistant Message -->
            <div v-else class="flex gap-3 items-start">
              <div class="w-8 h-8 bg-primary text-white rounded-full grid place-items-center flex-shrink-0 mt-0.5">
                <Sparkles class="w-4 h-4" />
              </div>
              <div class="flex-1 min-w-0 flex flex-col gap-2.5 max-w-[85%]">
                <!-- Render each block -->
                <template v-for="(block, bIdx) in m.blocks" :key="bIdx">
                  <!-- Thinking Block -->
                  <div v-if="block.type === 'thinking'" class="flex items-center gap-2.5 px-3.5 py-2.5 bg-surface-2 border border-border rounded-lg text-xs text-muted-foreground">
                    <template v-if="m.isStreaming">
                      <span class="relative flex h-2.5 w-2.5">
                        <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-info opacity-75"></span>
                        <span class="relative inline-flex rounded-full h-2.5 w-2.5 bg-info"></span>
                      </span>
                      <span class="animate-pulse">{{ block.content }}</span>
                    </template>
                    <template v-else>
                      <CheckCircle2 class="w-3.5 h-3.5 text-success" />
                      <span class="text-muted-foreground line-through opacity-60">{{ block.content }}</span>
                    </template>
                  </div>

                  <!-- Tool Call Block -->
                  <div v-else-if="block.type === 'tool_call'" class="border border-border rounded-lg overflow-hidden">
                    <button
                      class="w-full flex items-center gap-2.5 px-3.5 py-2.5 bg-surface-2 hover:bg-muted transition-colors text-left"
                      @click="toggleToolCollapse(mIdx * 100 + bIdx)"
                    >
                      <Wrench class="w-3.5 h-3.5 text-info flex-shrink-0" />
                      <span class="text-xs font-semibold text-ink flex-1">调用工具：{{ block.name }}</span>
                      <!-- 只在下一个块还没出现（即还在等结果）时转圈 -->
                      <Loader2 v-if="m.isStreaming && bIdx === m.blocks.length - 1" class="w-3.5 h-3.5 text-info animate-spin" />
                      <template v-else>
                        <ChevronDown v-if="collapsedTools.has(mIdx * 100 + bIdx)" class="w-3.5 h-3.5 text-muted-foreground" />
                        <ChevronUp v-else class="w-3.5 h-3.5 text-muted-foreground" />
                      </template>
                    </button>
                    <div v-if="!collapsedTools.has(mIdx * 100 + bIdx)" class="px-3.5 py-2.5 border-t border-border bg-surface">
                      <div class="text-[11px] text-muted-foreground font-mono">
                        <span class="text-subtle-foreground">参数：</span>
                        <pre class="mt-1 text-[10px] leading-relaxed overflow-x-auto">{{ JSON.stringify(block.args, null, 2) }}</pre>
                      </div>
                    </div>
                  </div>

                  <!-- Tool Result Block -->
                  <div v-else-if="block.type === 'tool_result'" class="border rounded-lg overflow-hidden" :class="block.success ? 'border-success/30' : 'border-danger/30'">
                    <div class="flex items-center gap-2 px-3.5 py-2 text-xs" :class="block.success ? 'bg-success-soft' : 'bg-danger-soft'">
                      <CheckCircle2 v-if="block.success" class="w-3.5 h-3.5 text-success" />
                      <XCircle v-else class="w-3.5 h-3.5 text-danger" />
                      <span class="font-semibold" :class="block.success ? 'text-success' : 'text-danger'">
                        {{ block.name }} · {{ block.success ? '成功' : '失败' }}
                      </span>
                    </div>
                    <div v-if="block.data || block.error" class="px-3.5 py-2 border-t bg-surface" :class="block.success ? 'border-success/20' : 'border-danger/20'">
                      <pre class="text-[10px] font-mono text-muted-foreground leading-relaxed overflow-x-auto max-h-[120px]">{{ block.error || JSON.stringify(block.data, null, 2) }}</pre>
                    </div>
                  </div>

                  <!-- Text Block (Markdown) -->
                  <div v-else-if="block.type === 'text' && block.content" class="prose prose-sm max-w-none text-ink leading-relaxed" v-html="renderMarkdown(block.content ?? '')"></div>

                  <!-- Error Block -->
                  <div v-else-if="block.type === 'error'" class="flex items-center gap-2 px-3.5 py-2.5 bg-danger-soft border border-danger/30 rounded-lg text-xs text-danger">
                    <XCircle class="w-3.5 h-3.5" />
                    <span>{{ block.content }}</span>
                  </div>
                </template>

                <!-- Streaming indicator -->
                <div v-if="m.isStreaming && m.blocks.length === 0" class="flex items-center gap-2.5 text-xs text-muted-foreground">
                  <span class="relative flex h-2.5 w-2.5">
                    <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-primary opacity-75"></span>
                    <span class="relative inline-flex rounded-full h-2.5 w-2.5 bg-primary"></span>
                  </span>
                  <span class="animate-pulse">正在思考...</span>
                </div>
              </div>
            </div>
          </template>
        </div>

        <!-- Enhanced Input Area -->
        <div class="px-5 py-4 bg-surface-2 border-t border-border">
          <!-- Attachment bar (placeholder for future) -->
          <div class="flex items-center gap-2 mb-2">
            <button class="inline-flex items-center gap-1.5 px-2.5 py-1.5 text-[11px] font-medium text-muted-foreground bg-surface border border-border rounded-md hover:border-primary hover:text-primary transition-colors" title="上传附件（即将支持）">
              <Paperclip class="w-3 h-3" />
              附件
            </button>
            <button class="inline-flex items-center gap-1.5 px-2.5 py-1.5 text-[11px] font-medium text-muted-foreground bg-surface border border-border rounded-md hover:border-primary hover:text-primary transition-colors" title="上传图片（即将支持）">
              <ImageIcon class="w-3 h-3" />
              图片
            </button>
            <span class="ml-auto text-[10px] text-subtle-foreground">Ctrl+Enter 发送</span>
          </div>
          <!-- Input -->
          <div class="flex items-end gap-3">
            <div class="flex-1 relative">
              <textarea
                v-model="input"
                :placeholder="activeSession ? '继续追问...' : '输入你的问题开始对话'"
                :disabled="sending"
                rows="1"
                class="w-full min-h-[44px] max-h-[160px] px-4 py-3 bg-surface border border-border-strong rounded-xl text-sm text-ink placeholder:text-subtle-foreground resize-none outline-none focus:border-primary focus:ring-1 focus:ring-primary/20 transition-colors"
                @keydown.ctrl.enter="send"
                @keydown.meta.enter="send"
                @input="autoResize"
              ></textarea>
            </div>
            <button
              class="w-11 h-11 bg-primary text-white border-0 rounded-xl cursor-pointer grid place-items-center flex-shrink-0 hover:bg-primary-strong disabled:opacity-50 transition-colors"
              :disabled="sending || !input.trim()"
              @click="send"
              title="发送 (Ctrl+Enter)"
            >
              <Loader2 v-if="sending" class="w-4 h-4 animate-spin" />
              <Send v-else class="w-4 h-4" />
            </button>
          </div>
        </div>
      </section>
    </div>
  </AppShell>
</template>

<style>
/* Markdown 渲染样式 */
.prose h1, .prose h2, .prose h3 {
  margin-top: 1em;
  margin-bottom: 0.5em;
  font-weight: 700;
  color: var(--ink);
}
.prose h1 { font-size: 1.25rem; }
.prose h2 { font-size: 1.1rem; }
.prose h3 { font-size: 1rem; }
.prose p { margin: 0.5em 0; }
.prose ul, .prose ol { margin: 0.5em 0; padding-left: 1.5em; }
.prose li { margin: 0.25em 0; }
.prose strong { font-weight: 700; color: var(--ink); }
.prose code:not(.hljs) {
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-radius: 4px;
  padding: 1px 5px;
  font-size: 0.85em;
  font-family: var(--font-mono);
}
.prose .hljs-code-block {
  background: #1e293b;
  border-radius: 8px;
  padding: 16px;
  margin: 0.75em 0;
  overflow-x: auto;
  font-size: 12px;
  line-height: 1.6;
}
.prose .hljs-code-block code {
  color: #e2e8f0;
  font-family: var(--font-mono);
}
.prose blockquote {
  border-left: 3px solid var(--primary);
  padding-left: 12px;
  margin: 0.75em 0;
  color: var(--muted-foreground);
}
.prose table {
  width: 100%;
  border-collapse: collapse;
  margin: 0.75em 0;
  font-size: 0.85em;
}
.prose th, .prose td {
  border: 1px solid var(--border);
  padding: 6px 10px;
  text-align: left;
}
.prose th {
  background: var(--surface-2);
  font-weight: 600;
}
.prose a {
  color: var(--primary);
  text-decoration: underline;
}
</style>
