<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import axios from 'axios'
import AppShell from '@/components/layout/AppShell.vue'
import BreadcrumbNav from '@/components/business/BreadcrumbNav.vue'
import { useToast } from '@/components/ui/toast'
import { confirm } from '@/composables/useConfirm'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import {
  Link,
  KeyRound,
  Copy,
  Eye,
  EyeOff,
  Zap,
  Cpu,
  Check,
  Plus,
  Info,
  ScrollText,
  RefreshCw,
} from 'lucide-vue-next'

interface LlmConfigRecord {
  id: number
  provider: string
  base_url: string
  chat_model: string
  embed_model: string
  api_key_masked?: string
}

interface TestResult {
  status: 'ok' | 'error' | 'no_config'
  provider?: string
  model?: string
  latency_ms?: number
  message?: string
}

const { toast } = useToast()
const configs = ref<LlmConfigRecord[]>([])
const loading = ref(true)
const testResult = ref<TestResult | null>(null)
const saving = ref(false)
const saveMsg = ref('')

const provider = ref('deepseek')
const baseUrl = ref('https://api.deepseek.com/v1')
const apiKey = ref('')
const apiKeyVisible = ref(false)
const chatModel = ref('deepseek-chat')
const embedModel = ref('')
const temperature = ref('0.2')
const timeout = ref('60')

// Custom provider dialog
const showCustomDialog = ref(false)
const customForm = ref({ key: '', label: '', url: '' })

// Logs dialog (placeholder until backend exposes)
const showLogsDialog = ref(false)

const builtinProviders = [
  { key: 'tongyi', label: '通义千问', url: 'https://dashscope.aliyuncs.com/compatible-mode/v1' },
  { key: 'deepseek', label: 'DeepSeek', url: 'https://api.deepseek.com/v1' },
  { key: 'zhipu', label: '智谱 GLM', url: 'https://open.bigmodel.cn/api/paas/v4' },
  { key: 'moonshot', label: 'Moonshot', url: 'https://api.moonshot.cn/v1' },
]
const customProviders = ref<{ key: string; label: string; url: string }[]>(
  JSON.parse(localStorage.getItem('tes-custom-providers') ?? '[]'),
)

const allProviders = computed(() => [...builtinProviders, ...customProviders.value])

const breadcrumbs = [
  { label: '管理控制台', to: '/dashboard' },
  { label: '系统配置', to: '/admin/llm' },
  { label: '大模型服务' },
]

function selectProvider(p: { key: string; url: string }) {
  provider.value = p.key
  baseUrl.value = p.url
}

onMounted(async () => {
  try {
    const { data } = await axios.get('/api/llm/configs')
    configs.value = data
    if (data.length > 0) {
      const c = data[0]
      provider.value = c.provider
      baseUrl.value = c.base_url || baseUrl.value
      chatModel.value = c.chat_model
      embedModel.value = c.embed_model || ''
      void testConnection()
    }
  } catch {
    toast({ description: '加载配置失败', variant: 'destructive' })
  } finally {
    loading.value = false
  }
})

async function saveConfig() {
  saving.value = true
  saveMsg.value = ''
  try {
    await axios.post('/api/llm/configs', {
      provider: provider.value,
      base_url: baseUrl.value,
      api_key: apiKey.value || (configs.value[0]?.api_key_masked ? 'PLACEHOLDER_USE_EXISTING' : ''),
      chat_model: chatModel.value,
      embed_model: embedModel.value,
    })
    saveMsg.value = '配置已保存'
    apiKey.value = ''
    apiKeyVisible.value = false
    const { data } = await axios.get('/api/llm/configs')
    configs.value = data
    toast({ description: '配置已保存', variant: 'success' })
  } catch (e) {
    const msg = (e as { response?: { data?: { detail?: string; message?: string } } })?.response?.data?.detail
    saveMsg.value = msg ?? '保存失败'
    toast({ description: saveMsg.value, variant: 'destructive' })
  } finally {
    saving.value = false
  }
}

const testing = ref(false)

async function testConnection() {
  if (testing.value) return
  testing.value = true
  testResult.value = null
  try {
    const { data } = await axios.post('/api/llm/test')
    testResult.value = data
  } catch (e) {
    const msg = (e as { response?: { data?: { message?: string } } })?.response?.data?.message
    testResult.value = { status: 'error', message: msg ?? '测试失败' }
  } finally {
    testing.value = false
  }
}

async function copyBaseUrl() {
  if (!baseUrl.value) return
  try {
    await navigator.clipboard.writeText(baseUrl.value)
    toast({ description: '已复制', variant: 'success' })
  } catch {
    toast({ description: '复制失败，请手动选择复制', variant: 'destructive' })
  }
}

async function replaceKey() {
  const ok = await confirm({
    title: '替换密钥',
    description: '替换密钥前请确认已就绪新密钥（在密钥输入框填入即可）。继续？',
  })
  if (ok) {
    document.querySelector<HTMLInputElement>('input[data-api-key]')?.focus()
    toast({ description: '请在密钥输入框填入新密钥后保存', variant: 'info' })
  }
}

function openCustomProvider() {
  customForm.value = { key: '', label: '', url: '' }
  showCustomDialog.value = true
}

function saveCustomProvider() {
  if (!customForm.value.key || !customForm.value.label || !customForm.value.url) {
    toast({ description: '请填写完整', variant: 'destructive' })
    return
  }
  customProviders.value.push({ ...customForm.value })
  localStorage.setItem('tes-custom-providers', JSON.stringify(customProviders.value))
  showCustomDialog.value = false
  selectProvider(customForm.value)
}
</script>

<template>
  <AppShell>
    <BreadcrumbNav :items="breadcrumbs" />

    <!-- Page Header -->
    <div class="flex justify-between items-end">
      <div>
        <h1 class="text-2xl font-bold text-ink">大模型服务配置</h1>
        <p class="mt-1.5 text-sm text-muted-foreground">通过 OpenAI 兼容协议接入云端供应商，支持运行时热切换</p>
      </div>
      <div class="flex items-center gap-3">
        <Badge v-if="configs.length > 0 && testResult?.status === 'ok'" variant="success" class="px-3 py-1">
          <span class="w-1.5 h-1.5 bg-success rounded-full mr-1.5 animate-pulse" />
          服务正常 · {{ configs[0]?.provider }}
        </Badge>
        <Button variant="outline" @click="showLogsDialog = true">
          <ScrollText class="w-4 h-4" />
          调用日志
        </Button>
        <Button :disabled="saving || loading" @click="saveConfig">
          {{ saving ? '保存中...' : '保存配置' }}
        </Button>
      </div>
    </div>

    <p v-if="saveMsg" class="text-xs" :class="saveMsg.includes('已保存') ? 'text-success' : 'text-danger'">{{ saveMsg }}</p>

    <div class="grid grid-cols-[1fr_380px] gap-5">
      <!-- LEFT -->
      <div class="flex flex-col gap-5">
        <Card class="overflow-hidden">
          <header class="flex justify-between items-center px-6 py-4 border-b border-border">
            <div>
              <div class="text-sm font-semibold text-ink">供应商选择</div>
              <div class="text-xs text-muted-foreground mt-1">支持热切换，无需重启服务</div>
            </div>
            <span class="text-xs text-muted-foreground">所有供应商通过 OpenAI 兼容协议接入</span>
          </header>

          <div class="flex gap-2.5 flex-wrap px-6 py-3.5 bg-surface-2 border-b border-border">
            <Button
              v-for="p in allProviders"
              :key="p.key"
              :variant="provider === p.key ? 'default' : 'outline'"
              size="sm"
              class="h-8"
              @click="selectProvider(p)"
            >
              <Check v-if="provider === p.key" class="w-3.5 h-3.5" />
              <Cpu v-else class="w-3.5 h-3.5 text-muted-foreground" />
              {{ p.label }}
            </Button>
            <Button variant="outline" size="sm" class="h-8" @click="openCustomProvider">
              <Plus class="w-3.5 h-3.5 text-muted-foreground" />
              自定义
            </Button>
          </div>

          <div class="flex flex-col gap-[18px] p-6">
            <div class="space-y-2">
              <Label>API 基础地址</Label>
              <div class="flex items-center gap-2 h-10 px-3 border border-border-strong rounded-md bg-surface focus-within:border-primary">
                <Link class="w-3.5 h-3.5 text-muted-foreground shrink-0" />
                <input
                  v-model="baseUrl"
                  class="flex-1 border-0 outline-none bg-transparent text-[13px] text-foreground font-mono"
                  placeholder="https://..."
                />
                <Tooltip>
                  <TooltipTrigger as-child>
                    <button class="text-muted-foreground hover:text-primary transition-colors" @click="copyBaseUrl">
                      <Copy class="w-3.5 h-3.5" />
                    </button>
                  </TooltipTrigger>
                  <TooltipContent>复制地址</TooltipContent>
                </Tooltip>
              </div>
            </div>

            <div class="space-y-2">
              <Label>API 密钥</Label>
              <div class="flex items-center gap-2 h-10 px-3 border border-border-strong rounded-md bg-surface focus-within:border-primary">
                <KeyRound class="w-3.5 h-3.5 text-muted-foreground shrink-0" />
                <input
                  v-model="apiKey"
                  data-api-key
                  :type="apiKeyVisible ? 'text' : 'password'"
                  placeholder="输入新密钥（留空则不更新）"
                  class="flex-1 border-0 outline-none bg-transparent text-xs text-foreground font-mono placeholder:text-subtle-foreground"
                />
                <Tooltip>
                  <TooltipTrigger as-child>
                    <button class="text-muted-foreground hover:text-primary transition-colors" @click="apiKeyVisible = !apiKeyVisible">
                      <Eye v-if="!apiKeyVisible" class="w-3.5 h-3.5" />
                      <EyeOff v-else class="w-3.5 h-3.5" />
                    </button>
                  </TooltipTrigger>
                  <TooltipContent>{{ apiKeyVisible ? '隐藏' : '显示' }}</TooltipContent>
                </Tooltip>
              </div>
              <div class="flex justify-between text-[11px]">
                <span class="text-muted-foreground">AES-256 加密存储 · 主密钥从环境变量加载</span>
                <button class="text-primary font-medium hover:underline" @click="replaceKey">替换密钥</button>
              </div>
            </div>

            <div class="grid grid-cols-2 gap-4">
              <div class="space-y-2">
                <Label>Chat 模型</Label>
                <Input v-model="chatModel" placeholder="如 deepseek-chat" />
              </div>
              <div class="space-y-2">
                <Label>Embedding 模型</Label>
                <Input v-model="embedModel" placeholder="可选" />
              </div>
            </div>

            <div class="grid grid-cols-2 gap-4">
              <div class="space-y-2">
                <Label>温度 Temperature</Label>
                <Input v-model="temperature" />
                <span class="text-[11px] text-muted-foreground">推荐评分场景使用 0.2</span>
              </div>
              <div class="space-y-2">
                <Label>超时 Timeout（秒）</Label>
                <Input v-model="timeout" />
                <span class="text-[11px] text-muted-foreground">超过将自动重试 3 次</span>
              </div>
            </div>
          </div>

          <footer class="flex justify-between items-center px-6 py-4 bg-surface-2 border-t border-border">
            <span class="text-xs text-muted-foreground">{{ configs[0] ? '已配置' : '未配置' }}</span>
            <Button variant="outline" :disabled="testing" @click="testConnection">
              <Zap class="w-3.5 h-3.5" :class="testing ? 'animate-pulse' : ''" />
              {{ testing ? '测试中...' : '连通性测试' }}
            </Button>
          </footer>
        </Card>

        <div
          v-if="testResult"
          class="p-4 rounded-lg border text-sm anim-in"
          :class="testResult.status === 'ok'
            ? 'bg-success-soft border-success text-success'
            : 'bg-danger-soft border-danger text-danger'"
        >
          <span v-if="testResult.status === 'ok'">✓ 连接成功 · {{ testResult.provider }} · {{ testResult.model }} · {{ testResult.latency_ms }}ms</span>
          <span v-else>✗ {{ testResult.message }}</span>
        </div>
      </div>

      <!-- RIGHT -->
      <div class="flex flex-col gap-5">
        <Card class="overflow-hidden">
          <header class="flex justify-between items-center px-5 py-4 border-b border-border">
            <span class="text-sm font-semibold text-ink">服务健康</span>
            <span class="w-2 h-2 rounded-full" :class="testResult?.status === 'ok' ? 'bg-success animate-pulse' : 'bg-muted-foreground'" />
          </header>
          <div class="flex flex-col">
            <div class="flex justify-between items-center px-5 py-3.5 border-b border-border">
              <span class="text-xs text-muted-foreground">当前状态</span>
              <span class="text-[13px] font-semibold font-mono" :class="testResult?.status === 'ok' ? 'text-success' : 'text-muted-foreground'">
                {{ testResult?.status === 'ok' ? '● 正常' : '未检测' }}
              </span>
            </div>
            <div class="flex justify-between items-center px-5 py-3.5 border-b border-border">
              <span class="text-xs text-muted-foreground">上次连通性测试</span>
              <span class="text-[13px] font-semibold text-ink font-mono">
                {{ testResult?.latency_ms ? testResult.latency_ms + ' ms' : '—' }}
              </span>
            </div>
            <div class="flex justify-between items-center px-5 py-3.5 border-b border-border">
              <span class="text-xs text-muted-foreground">熔断状态</span>
              <Tooltip>
                <TooltipTrigger as-child>
                  <span class="text-[13px] font-semibold text-success font-mono cursor-help">关闭</span>
                </TooltipTrigger>
                <TooltipContent>未发生连续 3 次失败时为关闭。状态持久化至 Redis。</TooltipContent>
              </Tooltip>
            </div>
            <div class="flex justify-between items-center px-5 py-3.5">
              <span class="text-xs text-muted-foreground">供应商</span>
              <span class="text-[13px] font-semibold text-ink font-mono">{{ configs.length > 0 ? configs[0].provider : '未配置' }}</span>
            </div>
          </div>
        </Card>

        <Card class="p-5">
          <div class="flex justify-between items-center mb-3.5">
            <span class="text-sm font-semibold text-ink">今日调用</span>
            <Tooltip>
              <TooltipTrigger as-child>
                <button class="text-xs text-primary font-medium hover:underline flex items-center gap-1" @click="showLogsDialog = true">
                  详情
                  <RefreshCw class="w-3 h-3" />
                </button>
              </TooltipTrigger>
              <TooltipContent>计数器尚未通过 HTTP 端点开放，详见后端 app/llm/metrics.py（Epic 31 接入）</TooltipContent>
            </Tooltip>
          </div>
          <div class="grid grid-cols-2 gap-2.5">
            <div class="flex flex-col gap-0.5 p-3 bg-surface-2 rounded-md">
              <span class="text-[11px] text-muted-foreground">总请求数</span>
              <span class="text-lg font-bold text-ink num-tabular">—</span>
            </div>
            <div class="flex flex-col gap-0.5 p-3 bg-surface-2 rounded-md">
              <span class="text-[11px] text-muted-foreground">平均延迟</span>
              <span class="text-lg font-bold text-ink num-tabular">—</span>
            </div>
            <div class="flex flex-col gap-0.5 p-3 bg-surface-2 rounded-md">
              <span class="text-[11px] text-muted-foreground">总 tokens</span>
              <span class="text-lg font-bold text-ink num-tabular">—</span>
            </div>
            <div class="flex flex-col gap-0.5 p-3 bg-surface-2 rounded-md">
              <span class="text-[11px] text-muted-foreground">失败次数</span>
              <span class="text-lg font-bold text-accent num-tabular">0</span>
            </div>
          </div>
          <p class="mt-2 text-[11px] text-muted-foreground">数据由 LLM 调用埋点采集，HTTP 接口 Epic 31 开放</p>
        </Card>

        <div class="bg-info-soft border border-info rounded-lg p-[18px] flex flex-col gap-2.5">
          <div class="flex items-center gap-2 text-[13px] font-semibold text-info">
            <Info class="w-4 h-4" />
            <span>热切换说明</span>
          </div>
          <p class="text-xs leading-relaxed text-info m-0">修改配置后系统将立即使用新参数处理后续请求。建议在低峰时段切换供应商以避免影响用户体验。</p>
        </div>
      </div>
    </div>

    <!-- Custom Provider Dialog -->
    <Dialog v-model:open="showCustomDialog">
      <DialogContent class="max-w-md">
        <DialogHeader>
          <DialogTitle>添加自定义供应商</DialogTitle>
          <DialogDescription>添加后将保存到本机，可在任意位置选用</DialogDescription>
        </DialogHeader>
        <div class="flex flex-col gap-4">
          <div class="space-y-2">
            <Label>供应商标识</Label>
            <Input v-model="customForm.key" placeholder="如 my-llm" class="font-mono" />
          </div>
          <div class="space-y-2">
            <Label>显示名称</Label>
            <Input v-model="customForm.label" placeholder="如 内部 LLM" />
          </div>
          <div class="space-y-2">
            <Label>API 基础地址</Label>
            <Input v-model="customForm.url" placeholder="https://..." class="font-mono" />
          </div>
        </div>
        <DialogFooter class="gap-2">
          <Button variant="outline" @click="showCustomDialog = false">取消</Button>
          <Button @click="saveCustomProvider">添加</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- Logs Dialog -->
    <Dialog v-model:open="showLogsDialog">
      <DialogContent class="max-w-2xl">
        <DialogHeader>
          <DialogTitle>调用日志</DialogTitle>
          <DialogDescription>查看最近的 LLM 调用记录</DialogDescription>
        </DialogHeader>
        <div class="border border-border rounded-md p-6 text-center text-sm text-muted-foreground">
          <Info class="w-6 h-6 mx-auto mb-2 text-info" />
          <p>调用日志接口预计在 Epic 31 阶段统一暴露。</p>
          <p class="mt-1">当前可在「审计日志」页面按 <code class="font-mono bg-surface-2 px-1 rounded">action=llm.call</code> 过滤查看。</p>
          <Button variant="outline" class="mt-3" as-child>
            <RouterLink to="/admin/audit?action=llm.call">前往审计日志</RouterLink>
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  </AppShell>
</template>
