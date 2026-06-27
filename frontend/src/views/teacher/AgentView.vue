<script setup lang="ts">
/**
 * 教师 AI 助教 — Agent 风格 UI
 *
 * 使用 useAgentChat composable。
 * 包含上下文选择器：任务、班级、评价。
 */
import { ref, computed, onMounted, nextTick, watch } from 'vue'
import axios from 'axios'
import AppShell from '@/components/layout/AppShell.vue'
import { renderMarkdown } from '@/lib/markdown'
import { useAgentChat, formatRelativeTime, useToolCollapse, autoResizeTextarea, type AgentStreamContext } from '@/composables/useAgentChat'
import {
  ChevronRight,
  Search,
  Sparkles,
  Send,
  Trash2,
  Plus,
  MessageSquare,
  Loader2,
  ChevronDown,
  ChevronUp,
  Wrench,
  CheckCircle2,
  XCircle,
  Copy,
  Check,
  BookOpen,
  Users,
  ClipboardList,
} from 'lucide-vue-next'

// ============ Context selectors ============
interface TaskOption { id: number; title: string; course_id?: number }
interface ClassOption { id: number; name: string }
interface EvalOption { id: number; upload_id: number; student_name?: string; status?: string }

const tasks = ref<TaskOption[]>([])
const classes = ref<ClassOption[]>([])
const evaluations = ref<EvalOption[]>([])

const selectedTask = ref<number | null>(null)
const selectedClass = ref<number | null>(null)
const selectedEval = ref<number | null>(null)

const context = computed<AgentStreamContext | undefined>(() => {
  const ctx: AgentStreamContext = {}
  if (selectedTask.value) ctx.task_id = selectedTask.value
  if (selectedClass.value) ctx.class_id = selectedClass.value
  if (selectedEval.value) ctx.evaluation_id = selectedEval.value
  return Object.keys(ctx).length > 0 ? ctx : undefined
})

// 上下文状态提示
const contextHint = computed(() => {
  const parts: string[] = []
  if (selectedTask.value) {
    const t = tasks.value.find(t => t.id === selectedTask.value)
    parts.push(`任务: ${t?.title ?? selectedTask.value}`)
  }
  if (selectedClass.value) {
    const c = classes.value.find(c => c.id === selectedClass.value)
    parts.push(`班级: ${c?.name ?? selectedClass.value}`)
  }
  if (parts.length === 0) return '可问通用教学问题；选择任务后可分析数据'
  return parts.join(' · ')
})

const {
  activeSessionId,
  messages,
  input,
  sending,
  searchQuery,
  filteredSessions,
  activeSession,
  hasMoreHistory,
  contextLabel: chatContextLabel,
  loadSession,
  newSession,
  removeSession,
  send,
  loadMoreHistory,
} = useAgentChat({
  agentRole: 'teacher',
  context,
  contextLabel: contextHint,
  defaultTitle: '新对话',
})

const chatBodyRef = ref<HTMLElement | null>(null)
const { collapsed, toggle: toggleToolCollapse } = useToolCollapse()

// 快捷问题 (T7.2 角色化)
const quickQuestions = computed(() => {
  return [
    '总结这个任务的批改情况',
    '生成评语草稿',
    '分析班级薄弱点',
    '解释疑似查重',
  ]
})

function askQuick(q: string) {
  input.value = q
  void send()
}

function scrollToBottom() {
  nextTick(() => {
    if (chatBodyRef.value) chatBodyRef.value.scrollTop = chatBodyRef.value.scrollHeight
  })
}
watch(messages, scrollToBottom, { deep: true })

// Load context data
onMounted(async () => {
  try {
    const [taskRes, classRes] = await Promise.all([
      axios.get('/api/tasks', { params: { page: 1, page_size: 100 } }),
      axios.get('/api/classes', { params: { page: 1, page_size: 100 } }),
    ])
    tasks.value = (taskRes.data.items || taskRes.data || []).map((t: any) => ({
      id: t.id, title: t.title, course_id: t.course_id,
    }))
    classes.value = (classRes.data.items || classRes.data || []).map((c: any) => ({
      id: c.id, name: c.name,
    }))
  } catch { /* ignore */ }
})

// Load evaluations when task changes
watch(selectedTask, async (taskId) => {
  evaluations.value = []
  selectedEval.value = null
  if (!taskId) return
  try {
    const { data } = await axios.get(`/api/grading/tasks/${taskId}/submissions`, {
      params: { page: 1, page_size: 200 },
    })
    evaluations.value = ((data as any).items || (data as any) || []).map((e: any) => ({
      id: e.id,
      upload_id: e.upload_id,
      student_name: e.student_name || `学生 ${e.upload_id}`,
      status: e.status,
    }))
  } catch { /* ignore */ }
})

function clearContext() {
  selectedTask.value = null
  selectedClass.value = null
  selectedEval.value = null
}

// Copy text helper
const copiedIdx = ref<number | null>(null)
async function copyText(text: string, idx: number) {
  try {
    await navigator.clipboard.writeText(text)
    copiedIdx.value = idx
    setTimeout(() => { copiedIdx.value = null }, 2000)
  } catch { /* ignore */ }
}
</script>

<template>
  <AppShell>
    <!-- Breadcrumb -->
    <nav class="flex items-center gap-2 text-sm text-muted-foreground mb-4">
      <span>教学</span>
      <ChevronRight class="w-3.5 h-3.5 text-subtle-foreground" />
      <span class="text-ink font-semibold">AI 助教</span>
    </nav>

    <!-- Page Header -->
    <div class="tes-page-header mb-5">
      <div>
        <div class="flex items-center gap-3">
          <h1 class="text-2xl font-bold text-ink m-0">AI 助教</h1>
          <span class="inline-flex items-center gap-1.5 px-2.5 py-0.5 bg-primary-soft text-primary rounded-full text-xs font-semibold">
            <Sparkles class="w-3 h-3" />
            Agent 模式
          </span>
        </div>
        <p class="mt-2 text-sm text-muted-foreground">{{ contextHint }}</p>
      </div>
      <div class="flex items-center gap-3">
        <button class="inline-flex items-center gap-1.5 h-9 px-4 bg-surface border border-border-strong rounded-md text-sm font-semibold text-ink hover:bg-surface-2 transition-colors" @click="removeSession()" :disabled="!activeSessionId">
          <Trash2 class="w-4 h-4" />
          删除会话
        </button>
        <button class="inline-flex items-center gap-1.5 h-9 px-4 bg-primary text-white rounded-md text-sm font-semibold hover:bg-primary-strong transition-colors" @click="newSession()">
          <Plus class="w-4 h-4" />
          新建对话
        </button>
      </div>
    </div>

    <!-- Context Selectors -->
    <div class="flex flex-wrap items-center gap-3 mb-4">
      <div class="flex items-center gap-2">
        <ClipboardList class="w-4 h-4 text-muted-foreground" />
        <select v-model="selectedTask" class="h-9 px-3 bg-surface border border-border-strong rounded-md text-sm text-ink outline-none focus:border-primary">
          <option :value="null">选择任务</option>
          <option v-for="t in tasks" :key="t.id" :value="t.id">{{ t.title }}</option>
        </select>
      </div>
      <div class="flex items-center gap-2">
        <Users class="w-4 h-4 text-muted-foreground" />
        <select v-model="selectedClass" class="h-9 px-3 bg-surface border border-border-strong rounded-md text-sm text-ink outline-none focus:border-primary">
          <option :value="null">选择班级</option>
          <option v-for="c in classes" :key="c.id" :value="c.id">{{ c.name }}</option>
        </select>
      </div>
      <div v-if="selectedTask" class="flex items-center gap-2">
        <BookOpen class="w-4 h-4 text-muted-foreground" />
        <select v-model="selectedEval" class="h-9 px-3 bg-surface border border-border-strong rounded-md text-sm text-ink outline-none focus:border-primary">
          <option :value="null">选择评价</option>
          <option v-for="e in evaluations" :key="e.id" :value="e.id">{{ e.student_name }} ({{ e.status }})</option>
        </select>
      </div>
      <button v-if="selectedTask || selectedClass || selectedEval" class="h-9 px-3 text-xs text-muted-foreground hover:text-ink transition-colors" @click="clearContext">
        清除上下文
      </button>
    </div>

    <!-- Main chat container — 一体化容器，left sidebar 用 border-r 分隔，不是两张独立卡片 -->
    <div class="tes-chat-container bg-surface border border-border rounded-xl overflow-hidden">
      <div class="flex h-[calc(68dvh)] min-h-[32rem]">
        <!-- Left: Session List — border-r 分隔，无独立边框 -->
        <aside class="w-[17rem] flex-shrink-0 border-r border-border flex flex-col">
          <div class="p-3">
            <div class="flex items-center gap-2 h-9 px-3 bg-surface-2 border border-border rounded-lg">
              <Search class="w-4 h-4 text-muted-foreground" />
              <input v-model="searchQuery" type="text" placeholder="搜索会话" class="flex-1 border-0 outline-none bg-transparent text-sm text-ink placeholder:text-subtle-foreground" />
            </div>
          </div>
          <div class="flex-1 overflow-y-auto">
            <div v-if="filteredSessions.length === 0" class="px-4 py-12 text-center text-sm text-muted-foreground">
              <MessageSquare class="w-10 h-10 text-subtle-foreground mx-auto mb-2" />
              <p>暂无会话</p>
            </div>
            <div
              v-for="s in filteredSessions"
              :key="s.id"
              class="flex flex-col gap-1 px-4 py-3 border-b border-border last:border-b-0 cursor-pointer transition-colors"
              :class="activeSessionId === s.id ? 'bg-primary-soft border-l-[3px] border-l-primary pl-[13px]' : 'hover:bg-surface-2'"
              @click="loadSession(s.id)"
            >
              <span class="text-sm font-semibold text-ink truncate">{{ s.title || '新对话' }}</span>
              <span class="text-xs text-muted-foreground"><!-- label -->{{ formatRelativeTime(s.created_at) }}</span>
            </div>
          </div>
        </aside>

        <!-- Right: Chat Area — 无独立边框，与容器一体 -->
        <section class="flex-1 flex flex-col min-w-0">
          <!-- Chat Header -->
          <div class="px-6 py-4 border-b border-border flex justify-between items-center">
            <div class="flex flex-col gap-1">
              <span class="text-base font-bold text-ink">{{ activeSession?.title ?? '新建对话' }}</span>
              <span class="text-xs text-muted-foreground"><!-- label -->
                {{ messages.length }} 条消息 · {{ activeSession ? formatRelativeTime(activeSession.created_at) : '开始新对话' }}
              </span>
            </div>
          </div>
          <!-- Context Chip -->
          <div v-if="chatContextLabel" class="px-6 py-2 bg-primary-soft/50 border-b border-primary/20 text-xs text-primary font-medium flex items-center gap-1.5">
            <ClipboardList class="w-3.5 h-3.5" />
            {{ chatContextLabel }}
          </div>

          <!-- Chat Body — flex-1 撑满，无底部 padding（input 紧贴底部）-->
          <div ref="chatBodyRef" class="flex-1 px-6 py-5 overflow-y-auto flex flex-col gap-5">
            <!-- Load More History -->
            <div v-if="hasMoreHistory" class="text-center">
              <button class="px-4 py-2 text-sm font-medium text-primary hover:text-primary-strong bg-primary-soft hover:bg-primary-soft/70 rounded-lg transition-colors" @click="loadMoreHistory">
                加载更多历史消息
              </button>
            </div>
            <!-- Empty State -->
            <div v-if="messages.length === 0" class="flex-1 flex flex-col items-center justify-center text-center gap-4">
              <div class="w-16 h-16 bg-primary-soft text-primary rounded-2xl grid place-items-center">
                <Sparkles class="w-8 h-8" />
              </div>
              <div>
                <p class="text-base font-semibold text-ink">AI 教学助手</p>
                <p class="text-sm text-muted-foreground mt-1">分析教学数据、生成评语草稿、解答教学问题</p>
              </div>
              <div class="flex flex-wrap gap-2 mt-2 justify-center">
                <button
                  v-for="q in quickQuestions"
                  :key="q"
                  class="px-3.5 py-2 bg-surface-2 border border-border rounded-lg text-sm font-medium text-ink hover:border-primary hover:bg-primary-soft transition-colors"
                  @click="askQuick(q)"
                >
                  {{ q }}
                </button>
              </div>
            </div>

            <!-- Messages with enter animation -->
            <template v-for="(m, mIdx) in messages" :key="mIdx">
              <div v-if="m.role === 'user'" class="flex justify-end anim-message-enter">
                <div class="max-w-[70%] px-4 py-3 bg-primary text-white rounded-2xl rounded-tr-md text-sm leading-relaxed whitespace-pre-wrap">
                  {{ m.content }}
                </div>
              </div>
              <div v-else class="flex gap-3 items-start anim-message-enter">
                <div class="w-8 h-8 bg-primary text-white rounded-full grid place-items-center flex-shrink-0 mt-0.5">
                  <Sparkles class="w-4 h-4" />
                </div>
                <div class="flex-1 min-w-0 flex flex-col gap-2.5 max-w-[85%]">
                  <template v-for="(block, bIdx) in m.blocks" :key="bIdx">
                    <div v-if="block.type === 'thinking'" class="flex items-center gap-2.5 px-3.5 py-2.5 bg-surface-2 border border-border rounded-lg text-sm text-muted-foreground">
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
                    <div v-else-if="block.type === 'tool_call'" class="border border-border rounded-lg overflow-hidden">
                      <button class="w-full flex items-center gap-2.5 px-3.5 py-2.5 bg-surface-2 hover:bg-muted transition-colors text-left" @click="toggleToolCollapse(mIdx * 100 + bIdx)">
                        <Wrench class="w-3.5 h-3.5 text-info flex-shrink-0" />
                        <span class="text-sm font-semibold text-ink flex-1">调用工具：{{ block.name }}</span>
                        <Loader2 v-if="m.isStreaming && bIdx === m.blocks.length - 1" class="w-3.5 h-3.5 text-info animate-spin" />
                        <template v-else>
                          <ChevronDown v-if="collapsed.has(mIdx * 100 + bIdx)" class="w-3.5 h-3.5 text-muted-foreground" />
                          <ChevronUp v-else class="w-3.5 h-3.5 text-muted-foreground" />
                        </template>
                      </button>
                      <div v-if="!collapsed.has(mIdx * 100 + bIdx)" class="px-3.5 py-2.5 border-t border-border bg-surface">
                        <div class="text-xs text-muted-foreground font-mono"><!-- label -->
                          <span class="text-subtle-foreground">参数：</span>
                          <pre class="mt-1 text-xs leading-relaxed overflow-x-auto"><!-- label -->{{ JSON.stringify(block.args, null, 2) }}</pre>
                        </div>
                      </div>
                    </div>
                    <div v-else-if="block.type === 'tool_result'" class="border rounded-lg overflow-hidden" :class="block.success ? 'border-success/30' : 'border-danger/30'">
                      <div class="flex items-center gap-2 px-3.5 py-2 text-sm" :class="block.success ? 'bg-success-soft' : 'bg-danger-soft'">
                        <CheckCircle2 v-if="block.success" class="w-3.5 h-3.5 text-success" />
                        <XCircle v-else class="w-3.5 h-3.5 text-danger" />
                        <span class="font-semibold" :class="block.success ? 'text-success' : 'text-danger'">{{ block.name }} · {{ block.success ? '成功' : '失败' }}</span>
                      </div>
                      <div v-if="block.data || block.error" class="px-3.5 py-2 border-t bg-surface" :class="block.success ? 'border-success/20' : 'border-danger/20'">
                        <pre class="text-xs font-mono text-muted-foreground leading-relaxed overflow-x-auto max-h-[120px]"><!-- label -->{{ block.error || JSON.stringify(block.data, null, 2) }}</pre>
                      </div>
                    </div>
                    <div v-else-if="block.type === 'text' && block.content" class="relative group">
                      <div class="tes-prose text-ink leading-relaxed" v-html="renderMarkdown(block.content ?? '')"></div>
                      <button
                        class="absolute top-0 right-0 opacity-0 group-hover:opacity-100 transition-opacity p-1 rounded bg-surface-2 border border-border hover:bg-surface-3"
                        :title="copiedIdx === mIdx * 100 + bIdx ? '已复制' : '复制'"
                        @click="copyText(block.content ?? '', mIdx * 100 + bIdx)"
                      >
                        <Check v-if="copiedIdx === mIdx * 100 + bIdx" class="w-3.5 h-3.5 text-success" />
                        <Copy v-else class="w-3.5 h-3.5 text-muted-foreground" />
                      </button>
                    </div>
                    <div v-else-if="block.type === 'error'" class="flex items-center gap-2 px-3.5 py-2.5 bg-danger-soft border border-danger/30 rounded-lg text-sm text-danger">
                      <XCircle class="w-3.5 h-3.5" />
                      <span>{{ block.content }}</span>
                    </div>
                  </template>
                  <div v-if="m.isStreaming && m.blocks.length === 0" class="flex items-center gap-2.5 text-sm text-muted-foreground">
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

          <!-- Input Bar — 嵌入在内容区底部，类似 Claude 风格 -->
          <div class="px-4 py-3 border-t border-border bg-surface">
            <div class="flex items-end gap-2 bg-surface-2 border border-border rounded-xl px-4 py-3 focus-within:border-primary focus-within:ring-1 focus-within:ring-primary/20 transition-all">
              <textarea
                v-model="input"
                :placeholder="activeSession ? '继续追问...' : '输入你的问题开始对话'"
                :disabled="sending"
                rows="1"
                class="flex-1 border-0 outline-none bg-transparent text-sm text-ink placeholder:text-subtle-foreground resize-none min-h-[22px] max-h-[120px] leading-relaxed"
                @keydown.ctrl.enter="send()"
                @keydown.meta.enter="send()"
                @input="autoResizeTextarea($event)"
              ></textarea>
              <button
                class="w-8 h-8 bg-primary text-white border-0 rounded-lg cursor-pointer grid place-items-center flex-shrink-0 hover:bg-primary-strong disabled:opacity-40 transition-colors"
                :disabled="sending || !input.trim()"
                @click="send()"
                title="发送 (Ctrl+Enter)"
              >
                <Loader2 v-if="sending" class="w-4 h-4 animate-spin" />
                <Send v-else class="w-4 h-4" />
              </button>
            </div>
            <div class="flex justify-end mt-1.5">
              <span class="text-xs text-subtle-foreground"><!-- label -->Ctrl+Enter 发送</span>
            </div>
          </div>
        </section>
      </div>
    </div>
  </AppShell>
</template>

<!-- 所有样式已迁移到 styles/globals.css 的 .tes-prose 统一类中 -->
<!-- 如需本地覆盖，请在此添加 scoped 样式 -->