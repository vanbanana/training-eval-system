<script setup lang="ts">
/**
 * 业务组件：ChatDialog（常驻 sidebar 风格）
 *
 * 嵌入 EvaluationView / TaskDetailView 等页面右侧，与 AI 助手对话。
 * 与 ChatHistoryView 区别：
 *   - ChatHistoryView 是独立页面（左 session 列表 + 右消息流）
 *   - 本组件是 fixed 定位的常驻 panel：右下角浮动按钮，点击展开 sidebar
 *
 * 后端：使用 /api/chat/stream（SSE 流式）；session_id 由后端首次响应自动分配
 */
import { computed, nextTick, onMounted, ref } from 'vue'
import {
  Sparkles,
  Send,
  X,
  RotateCcw,
  Database,
  MessageSquareText,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { ScrollArea } from '@/components/ui/scroll-area'
import { cn } from '@/lib/utils'

interface Props {
  /** 评价上下文 ID，会带到后端用于 session 关联 */
  evaluationId?: number
  /** 默认是否展开 */
  defaultOpen?: boolean
  /** 浮动按钮的位置：bottom-right 或嵌入式 inline */
  variant?: 'floating' | 'inline'
  /** sidebar 宽度（floating 模式才生效）*/
  width?: number
  /** 欢迎消息 */
  welcome?: string
  /** 推荐问题 */
  suggestions?: string[]
}

const props = withDefaults(defineProps<Props>(), {
  defaultOpen: false,
  variant: 'floating',
  width: 420,
  welcome: '你好 👋 我是 AI 学习助手，基于本次评价上下文为你解答。试试以下问题：',
  suggestions: () => ['为什么我的分数会扣分？', '如何提升薄弱维度？', '班级平均水平如何？'],
})

interface ChatMessage {
  role: 'user' | 'assistant'
  content: string
  time?: string
  toolCall?: string
}

const open = ref(props.defaultOpen)
const messages = ref<ChatMessage[]>([])
const input = ref('')
const sending = ref(false)
const sessionId = ref<number | null>(null)
const bodyRef = ref<HTMLElement | null>(null)

function scrollBottom() {
  nextTick(() => {
    if (bodyRef.value) bodyRef.value.scrollTop = bodyRef.value.scrollHeight
  })
}

function timeStr() {
  return new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
}

function reset() {
  sessionId.value = null
  messages.value = [{ role: 'assistant', content: props.welcome }]
}

async function send(text?: string) {
  const msg = (text ?? input.value).trim()
  if (!msg || sending.value) return

  messages.value.push({ role: 'user', content: msg, time: timeStr() })
  input.value = ''
  sending.value = true
  scrollBottom()

  const aiMsg: ChatMessage = { role: 'assistant', content: '' }
  messages.value.push(aiMsg)
  scrollBottom()

  try {
    const raw = localStorage.getItem('tes_token')
    let token = ''
    if (raw) {
      try {
        token = JSON.parse(raw)
      } catch {
        token = raw
      }
    }

    const res = await fetch('/api/chat/stream', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${token}`,
      },
      body: JSON.stringify({
        session_id: sessionId.value,
        message: msg,
        evaluation_id: props.evaluationId ?? null,
      }),
    })

    if (!res.ok) throw new Error(`HTTP ${res.status}`)

    const sid = res.headers.get('X-Session-Id')
    if (sid) sessionId.value = Number(sid)

    const reader = res.body?.getReader()
    const decoder = new TextDecoder()
    if (reader) {
      let buf = ''
      while (true) {
        const { done, value } = await reader.read()
        if (done) break
        buf += decoder.decode(value, { stream: true })
        const lines = buf.split('\n')
        buf = lines.pop() ?? ''
        for (const line of lines) {
          if (line.startsWith('data: ')) {
            const data = line.slice(6)
            if (data === '[DONE]') continue
            aiMsg.content += data
            scrollBottom()
          }
        }
      }
    }
    aiMsg.time = timeStr()
  } catch (e) {
    aiMsg.content = `抱歉，AI 服务暂时不可用：${(e as Error).message}`
  } finally {
    sending.value = false
    scrollBottom()
  }
}

function toggle() {
  open.value = !open.value
}

const showSuggestions = computed(() => messages.value.length === 1 && messages.value[0].role === 'assistant')

onMounted(() => {
  reset()
})

defineExpose({ toggle, open: () => (open.value = true), close: () => (open.value = false) })
</script>

<template>
  <!-- Inline mode: render directly in parent -->
  <section
    v-if="variant === 'inline'"
    class="bg-card border border-border rounded-lg flex flex-col overflow-hidden h-[600px]"
  >
    <header class="px-4 py-3 bg-surface-2 border-b border-border flex justify-between items-center">
      <div class="flex items-center gap-2">
        <span class="w-7 h-7 rounded-full grid place-items-center text-white" style="background: linear-gradient(135deg, hsl(var(--primary)), hsl(var(--accent)))">
          <Sparkles class="w-3.5 h-3.5" />
        </span>
        <div>
          <div class="text-xs font-semibold text-ink">AI 学习助手</div>
          <div class="text-[10px] text-muted-foreground">基于本次评价上下文</div>
        </div>
      </div>
      <Button variant="ghost" size="icon-sm" @click="reset" title="清空">
        <RotateCcw class="w-3 h-3" />
      </Button>
    </header>

    <ScrollArea class="flex-1">
      <div ref="bodyRef" class="p-4 flex flex-col gap-3">
        <div v-for="(m, i) in messages" :key="i" :class="cn('flex gap-2', m.role === 'user' ? 'justify-end' : '')">
          <div :class="m.role === 'user' ? 'flex flex-col items-end' : ''">
            <div
              :class="cn(
                'px-3 py-2 rounded-md text-xs leading-relaxed whitespace-pre-wrap max-w-[280px]',
                m.role === 'user'
                  ? 'bg-primary text-primary-foreground rounded-br-[2px]'
                  : 'bg-surface-2 border border-border rounded-bl-[2px]',
              )"
            >
              {{ m.content || (sending && i === messages.length - 1 ? '正在思考...' : '') }}
            </div>
            <div v-if="m.time" class="text-[10px] text-subtle-foreground mt-1 font-mono">{{ m.time }}</div>
            <div v-if="showSuggestions && i === 0" class="flex flex-wrap gap-1.5 mt-2">
              <Button
                v-for="s in suggestions"
                :key="s"
                variant="outline"
                size="sm"
                class="h-7 text-[11px]"
                @click="send(s)"
              >
                {{ s }}
              </Button>
            </div>
          </div>
        </div>
      </div>
    </ScrollArea>

    <footer class="px-3 py-2.5 bg-surface-2 border-t border-border">
      <div class="flex gap-2 items-end px-2.5 py-2 bg-card border border-border-strong rounded-md">
        <textarea
          v-model="input"
          rows="1"
          class="flex-1 border-0 outline-none bg-transparent text-xs resize-none min-h-4 max-h-20"
          placeholder="向 AI 提问..."
          @keydown.enter.exact.prevent="send()"
        />
        <Button size="icon-sm" :disabled="sending || !input.trim()" @click="send()">
          <Send class="w-3 h-3" />
        </Button>
      </div>
    </footer>
  </section>

  <!-- Floating mode: fixed bottom-right -->
  <template v-else>
    <Button
      v-if="!open"
      class="fixed bottom-6 right-6 z-40 shadow-lg rounded-full h-14 w-14 p-0 anim-in"
      @click="toggle"
      title="打开 AI 助手"
    >
      <MessageSquareText class="w-5 h-5" />
    </Button>

    <transition
      enter-active-class="transition-all duration-300"
      enter-from-class="translate-x-full opacity-0"
      enter-to-class="translate-x-0 opacity-100"
      leave-active-class="transition-all duration-200"
      leave-from-class="translate-x-0 opacity-100"
      leave-to-class="translate-x-full opacity-0"
    >
      <section
        v-if="open"
        class="fixed top-0 right-0 h-screen z-50 bg-card border-l border-border flex flex-col shadow-2xl"
        :style="{ width: width + 'px' }"
      >
        <header class="px-4 py-3 bg-surface-2 border-b border-border flex justify-between items-center">
          <div class="flex items-center gap-2">
            <span class="w-7 h-7 rounded-full grid place-items-center text-white" style="background: linear-gradient(135deg, hsl(var(--primary)), hsl(var(--accent)))">
              <Sparkles class="w-3.5 h-3.5" />
            </span>
            <div>
              <div class="text-sm font-semibold text-ink">AI 学习助手</div>
              <div class="text-[11px] text-muted-foreground">基于本次评价上下文</div>
            </div>
          </div>
          <div class="flex gap-1">
            <Button variant="ghost" size="icon-sm" @click="reset" title="清空对话">
              <RotateCcw class="w-3.5 h-3.5" />
            </Button>
            <Button variant="ghost" size="icon-sm" @click="toggle">
              <X class="w-3.5 h-3.5" />
            </Button>
          </div>
        </header>

        <div ref="bodyRef" class="flex-1 overflow-y-auto p-4 flex flex-col gap-3">
          <div v-for="(m, i) in messages" :key="i" :class="cn('flex gap-2', m.role === 'user' ? 'justify-end' : '')">
            <div :class="m.role === 'user' ? 'flex flex-col items-end' : ''">
              <div
                v-if="m.toolCall"
                class="inline-flex items-center gap-1.5 px-2 py-0.5 bg-info-soft text-info rounded-sm text-[10px] font-medium mb-1.5"
              >
                <Database class="w-3 h-3" />
                <span>{{ m.toolCall }}</span>
              </div>
              <div
                :class="cn(
                  'px-3 py-2.5 rounded-md text-sm leading-relaxed whitespace-pre-wrap max-w-[320px]',
                  m.role === 'user'
                    ? 'bg-primary text-primary-foreground rounded-br-[2px]'
                    : 'bg-surface-2 border border-border rounded-bl-[2px]',
                )"
              >
                {{ m.content || (sending && i === messages.length - 1 ? '正在思考...' : '') }}
              </div>
              <div v-if="m.time" class="text-[10px] text-subtle-foreground mt-1 font-mono">{{ m.time }}</div>
              <div v-if="showSuggestions && i === 0" class="flex flex-wrap gap-1.5 mt-2">
                <Button
                  v-for="s in suggestions"
                  :key="s"
                  variant="outline"
                  size="sm"
                  class="h-7 text-[11px]"
                  @click="send(s)"
                >
                  {{ s }}
                </Button>
              </div>
            </div>
          </div>
        </div>

        <footer class="bg-surface-2 border-t border-border px-4 py-3">
          <div class="flex gap-2 items-end px-3 py-2 bg-card border border-border-strong rounded-md focus-within:border-primary">
            <textarea
              v-model="input"
              rows="1"
              class="flex-1 border-0 outline-none bg-transparent text-sm resize-none min-h-5 max-h-24"
              placeholder="向 AI 提问关于本次评价的问题..."
              @keydown.enter.exact.prevent="send()"
            />
            <Button size="icon-sm" :disabled="sending || !input.trim()" @click="send()">
              <Send class="w-3.5 h-3.5" />
            </Button>
          </div>
          <div class="flex justify-between text-[10px] text-muted-foreground mt-1.5">
            <span>基于 LLM 配置</span>
            <span>单次提问 ≤ 2000 字</span>
          </div>
        </footer>
      </section>
    </transition>
  </template>
</template>
