<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import axios from 'axios'
import AppShell from '@/components/layout/AppShell.vue'
import { useToast } from '@/lib/toast'
import {
  ChevronRight,
  Download,
  CloudUpload,
  FileText,
  CheckCircle2,
  AlertTriangle,
  X,
  ArrowLeft,
} from 'lucide-vue-next'

const router = useRouter()
const { show: toast } = useToast()

type Step = 1 | 2 | 3

const currentStep = ref<Step>(1)
const importFile = ref<File | null>(null)
const previewRows = ref<Array<{ row: number; username: string; role: string; display_name: string; password: string; valid: boolean; error?: string }>>([])
const importing = ref(false)
const importResult = ref<{ created: number; skipped: number; errors: string[] } | null>(null)

function downloadTemplate() {
  window.location.href = '/api/imports/template/user.xlsx'
  toast('info', '模板下载已开始')
}

function onFileSelected(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0] ?? null
  if (file) handleFile(file)
  input.value = ''
}

function onDrop(e: DragEvent) {
  e.preventDefault()
  const file = e.dataTransfer?.files?.[0]
  if (file) handleFile(file)
}

async function handleFile(file: File) {
  // Note: 模板下载也支持 .csv，所以同时接受
  if (!file.name.toLowerCase().endsWith('.csv') && !file.name.toLowerCase().endsWith('.xlsx')) {
    toast('error', '仅支持 CSV / Excel 文件')
    return
  }
  importFile.value = file

  // .xlsx 不在前端解析（避免引入 xlsx 大依赖），跳过预览直接进 step 2 等用户确认
  if (file.name.toLowerCase().endsWith('.xlsx')) {
    previewRows.value = []
    currentStep.value = 2
    return
  }

  // Local CSV parsing for preview
  try {
    const text = await file.text()
    const lines = text.split(/\r?\n/).filter((l) => l.trim().length > 0)
    if (lines.length < 2) {
      toast('error', 'CSV 至少需要表头 + 1 行数据')
      return
    }
    const header = lines[0].split(',').map((s) => s.trim().toLowerCase())
    const usernameIdx = header.indexOf('username')
    const displayIdx = header.indexOf('display_name')
    const roleIdx = header.indexOf('role')
    const passwordIdx = header.indexOf('password')

    if (usernameIdx < 0 || displayIdx < 0 || roleIdx < 0) {
      toast('error', `CSV 缺少必需列：${[
        usernameIdx < 0 ? 'username' : '',
        displayIdx < 0 ? 'display_name' : '',
        roleIdx < 0 ? 'role' : '',
      ].filter(Boolean).join(', ')}`)
      return
    }

    const rows = []
    const seenUsernames = new Set<string>()
    for (let i = 1; i < lines.length; i++) {
      const cells = lines[i].split(',').map((s) => s.trim())
      const username = cells[usernameIdx] ?? ''
      const display_name = cells[displayIdx] ?? ''
      const role = cells[roleIdx] ?? ''
      const password = passwordIdx >= 0 ? (cells[passwordIdx] ?? '') : ''

      let valid = true
      let error = ''
      if (!username || username.length < 2) {
        valid = false
        error = '账号至少 2 个字符'
      } else if (!display_name) {
        valid = false
        error = '姓名必填'
      } else if (!['student', 'teacher', 'admin'].includes(role)) {
        valid = false
        error = `角色必须为 student/teacher/admin，当前 "${role}"`
      } else if (passwordIdx >= 0 && password && password.length < 8) {
        valid = false
        error = '密码至少 8 位'
      } else if (seenUsernames.has(username)) {
        valid = false
        error = `账号 "${username}" 在 CSV 中重复`
      }

      seenUsernames.add(username)
      rows.push({ row: i + 1, username, role, display_name, password, valid, error })
    }
    previewRows.value = rows
    currentStep.value = 2
  } catch (e: any) {
    toast('error', '解析 CSV 失败：' + e.message)
  }
}

const previewStats = computed(() => {
  const total = previewRows.value.length
  const valid = previewRows.value.filter((r) => r.valid).length
  const invalid = total - valid
  return { total, valid, invalid }
})

const canSubmit = computed(() => {
  if (!importFile.value) return false
  // xlsx 文件跳过了前端预览，直接允许提交
  if (importFile.value.name.toLowerCase().endsWith('.xlsx')) return true
  return previewStats.value.invalid === 0 && previewStats.value.valid > 0
})

async function submitImport() {
  if (!importFile.value) return
  if (!canSubmit.value) {
    toast('error', '请先修复 CSV 中的错误行')
    return
  }
  importing.value = true
  try {
    const form = new FormData()
    form.append('file', importFile.value)
    const { data } = await axios.post('/api/imports/users', form, {
      headers: { 'Content-Type': 'multipart/form-data' },
    })
    importResult.value = {
      created: data.created ?? data.success ?? 0,
      skipped: data.skipped ?? Math.max(0, (data.total ?? 0) - (data.success ?? 0) - (data.failed ?? 0)),
      errors: data.errors ?? (data.failed ? [`后端报告失败 ${data.failed} 条，请到审计日志查看`] : []),
    }
    currentStep.value = 3
    toast('success', `导入完成：成功 ${importResult.value.created} 条`)
  } catch (e: any) {
    toast('error', e.response?.data?.detail || '导入失败')
  } finally {
    importing.value = false
  }
}

function reset() {
  currentStep.value = 1
  importFile.value = null
  previewRows.value = []
  importResult.value = null
}

function goBack() {
  router.push('/admin/users')
}
</script>

<template>
  <AppShell>
    <!-- Breadcrumb -->
    <nav class="flex items-center gap-2 text-xs text-muted-foreground">
      <RouterLink to="/dashboard" class="hover:text-foreground">管理控制台</RouterLink>
      <ChevronRight class="w-3 h-3 text-subtle-foreground" />
      <RouterLink to="/admin/users" class="hover:text-foreground">用户与权限</RouterLink>
      <ChevronRight class="w-3 h-3 text-subtle-foreground" />
      <span class="text-ink font-semibold">批量导入</span>
    </nav>

    <!-- Page Header -->
    <div class="flex justify-between items-end">
      <div>
        <h1 class="text-2xl font-bold text-ink">批量导入用户</h1>
        <p class="mt-1.5 text-sm text-muted-foreground">通过 CSV 文件一次创建多个用户账号</p>
      </div>
      <button
        class="inline-flex items-center gap-1.5 h-9 px-4 bg-surface border border-border-strong rounded-md text-sm font-semibold text-ink hover:bg-surface-2 transition-colors"
        @click="goBack"
      >
        <ArrowLeft class="w-4 h-4" />
        <span>返回用户列表</span>
      </button>
    </div>

    <!-- Steps Indicator -->
    <section class="bg-surface border border-border rounded-lg">
      <div class="flex items-center justify-center gap-0 p-6">
        <!-- Step 1 -->
        <div class="flex items-center gap-2.5">
          <div
            class="w-8 h-8 rounded-full grid place-items-center font-bold text-sm"
            :class="
              currentStep > 1
                ? 'bg-primary text-white'
                : currentStep === 1
                  ? 'bg-primary text-white'
                  : 'bg-surface-2 text-muted-foreground border border-border'
            "
          >
            <CheckCircle2 v-if="currentStep > 1" class="w-4 h-4" />
            <span v-else>1</span>
          </div>
          <div class="flex flex-col gap-0.5">
            <span class="text-xs font-semibold text-ink">下载模板</span>
            <span class="text-[11px]" :class="currentStep > 1 ? 'text-success' : currentStep === 1 ? 'text-primary' : 'text-muted-foreground'">
              {{ currentStep > 1 ? '已完成' : currentStep === 1 ? '当前步骤' : '待进行' }}
            </span>
          </div>
        </div>

        <div class="w-20 h-px mx-3" :class="currentStep > 1 ? 'bg-primary' : 'bg-border'"></div>

        <!-- Step 2 -->
        <div class="flex items-center gap-2.5">
          <div
            class="w-8 h-8 rounded-full grid place-items-center font-bold text-sm"
            :class="
              currentStep > 2
                ? 'bg-primary text-white'
                : currentStep === 2
                  ? 'bg-primary text-white'
                  : 'bg-surface-2 text-muted-foreground border border-border'
            "
          >
            <CheckCircle2 v-if="currentStep > 2" class="w-4 h-4" />
            <span v-else>2</span>
          </div>
          <div class="flex flex-col gap-0.5">
            <span class="text-xs font-semibold text-ink">上传 + 预览</span>
            <span class="text-[11px]" :class="currentStep > 2 ? 'text-success' : currentStep === 2 ? 'text-primary' : 'text-muted-foreground'">
              {{ currentStep > 2 ? '已完成' : currentStep === 2 ? '当前步骤' : '待进行' }}
            </span>
          </div>
        </div>

        <div class="w-20 h-px mx-3" :class="currentStep > 2 ? 'bg-primary' : 'bg-border'"></div>

        <!-- Step 3 -->
        <div class="flex items-center gap-2.5">
          <div
            class="w-8 h-8 rounded-full grid place-items-center font-bold text-sm"
            :class="
              currentStep === 3
                ? 'bg-primary text-white'
                : 'bg-surface-2 text-muted-foreground border border-border'
            "
          >
            <span>3</span>
          </div>
          <div class="flex flex-col gap-0.5">
            <span class="text-xs font-semibold text-ink">提交结果</span>
            <span class="text-[11px]" :class="currentStep === 3 ? 'text-primary' : 'text-muted-foreground'">
              {{ currentStep === 3 ? '完成' : '待进行' }}
            </span>
          </div>
        </div>
      </div>
    </section>

    <!-- Step 1: Download Template -->
    <section
      v-if="currentStep === 1"
      class="bg-surface border border-border rounded-lg overflow-hidden"
    >
      <header class="px-6 py-4 border-b border-border">
        <div class="flex items-center gap-2.5">
          <Download class="w-4 h-4 text-primary" />
          <span class="text-base font-semibold text-ink">第 1 步：下载 CSV 模板</span>
        </div>
      </header>
      <div class="p-6 flex flex-col gap-4">
        <div class="bg-info-soft border border-info/20 rounded-md p-4 text-xs text-info leading-relaxed">
          <p class="font-semibold mb-2">CSV 格式要求：</p>
          <ul class="list-disc pl-4 space-y-1">
            <li>第一行必须是表头：<code class="bg-surface px-1 rounded font-mono">username, display_name, role, password</code></li>
            <li>角色取值：<code class="bg-surface px-1 rounded font-mono">student / teacher / admin</code></li>
            <li>密码至少 8 位（建议留空，由系统生成默认密码）</li>
            <li>UTF-8 编码，逗号分隔</li>
          </ul>
        </div>
        <div class="flex gap-3">
          <button
            class="inline-flex items-center gap-2 h-10 px-5 bg-primary text-white rounded-md text-sm font-semibold hover:bg-primary-strong transition-colors"
            @click="downloadTemplate"
          >
            <Download class="w-4 h-4" />
            <span>下载模板 CSV</span>
          </button>
          <button
            class="inline-flex items-center gap-2 h-10 px-5 bg-surface border border-border-strong rounded-md text-sm font-semibold text-ink hover:bg-surface-2 transition-colors"
            @click="currentStep = 2"
          >
            <span>跳过，直接上传</span>
            <ChevronRight class="w-4 h-4" />
          </button>
        </div>
      </div>
    </section>

    <!-- Step 2: Upload + Preview -->
    <section
      v-if="currentStep === 2"
      class="bg-surface border border-border rounded-lg overflow-hidden"
    >
      <header class="px-6 py-4 border-b border-border flex justify-between items-center">
        <div class="flex items-center gap-2.5">
          <CloudUpload class="w-4 h-4 text-primary" />
          <span class="text-base font-semibold text-ink">第 2 步：上传 CSV 文件</span>
        </div>
        <button
          v-if="importFile"
          class="text-xs text-muted-foreground hover:text-danger flex items-center gap-1"
          @click="importFile = null; previewRows = []"
        >
          <X class="w-3 h-3" />
          <span>移除文件</span>
        </button>
      </header>

      <!-- Drop Zone -->
      <div
        v-if="!importFile"
        class="m-6 bg-surface-2 border-2 border-dashed border-primary rounded-lg p-12 min-h-[200px] flex flex-col items-center justify-center gap-3.5 cursor-pointer hover:bg-primary-soft/40 transition-colors"
        @click="(e) => (e.currentTarget as HTMLElement).querySelector('input')?.click()"
        @dragover.prevent
        @drop="onDrop"
      >
        <div class="w-14 h-14 bg-primary-soft text-primary rounded-full grid place-items-center">
          <CloudUpload class="w-6 h-6" />
        </div>
        <div class="text-md font-semibold text-ink">将 CSV 文件拖到此处，或点击选择</div>
        <div class="text-xs text-muted-foreground">支持 .csv / .xlsx 文件，UTF-8 编码</div>
        <input type="file" accept=".csv,.xlsx" class="hidden" @change="onFileSelected" />
      </div>

      <!-- Preview -->
      <div v-else>
        <!-- Stats bar -->
        <div class="px-6 py-3 bg-surface-2 border-b border-border flex items-center gap-6 text-xs">
          <div class="flex items-center gap-2">
            <FileText class="w-3.5 h-3.5 text-muted-foreground" />
            <span class="font-semibold text-ink">{{ importFile.name }}</span>
            <span class="text-muted-foreground">({{ (importFile.size / 1024).toFixed(1) }} KB)</span>
          </div>
          <div class="flex-1"></div>
          <span class="text-muted-foreground">
            共 <span class="font-bold text-ink">{{ previewStats.total }}</span> 行
          </span>
          <span class="text-success">
            <CheckCircle2 class="w-3 h-3 inline" /> 有效
            <span class="font-bold ml-1">{{ previewStats.valid }}</span>
          </span>
          <span v-if="previewStats.invalid > 0" class="text-danger">
            <AlertTriangle class="w-3 h-3 inline" /> 错误
            <span class="font-bold ml-1">{{ previewStats.invalid }}</span>
          </span>
        </div>

        <!-- Preview table -->
        <div class="tes-table-shell">
        <div class="grid min-w-[860px] grid-cols-[60px_120px_100px_minmax(12rem,1fr)_120px_minmax(14rem,1fr)] items-center px-6 py-3 bg-surface-2 border-b border-border">
          <div class="text-[11px] font-semibold tracking-wider text-muted-foreground">行号</div>
          <div class="text-[11px] font-semibold tracking-wider text-muted-foreground">账号</div>
          <div class="text-[11px] font-semibold tracking-wider text-muted-foreground">角色</div>
          <div class="text-[11px] font-semibold tracking-wider text-muted-foreground">姓名</div>
          <div class="text-[11px] font-semibold tracking-wider text-muted-foreground">状态</div>
          <div class="text-[11px] font-semibold tracking-wider text-muted-foreground">备注</div>
        </div>
        <div class="max-h-[400px] min-w-[860px] overflow-y-auto">
          <div
            v-for="r in previewRows"
            :key="r.row"
            class="grid grid-cols-[60px_120px_100px_minmax(12rem,1fr)_120px_minmax(14rem,1fr)] items-center px-6 py-2.5 border-b border-border last:border-b-0 text-xs"
            :class="r.valid ? '' : 'bg-danger-soft'"
          >
            <span class="font-mono text-muted-foreground">#{{ r.row }}</span>
            <span class="font-medium text-ink truncate">{{ r.username }}</span>
            <span class="text-foreground">{{ r.role }}</span>
            <span class="text-foreground truncate">{{ r.display_name }}</span>
            <span>
              <span
                v-if="r.valid"
                class="inline-flex items-center px-2 py-0.5 rounded-full text-[10px] font-semibold bg-success-soft text-success"
              >
                ✓ 有效
              </span>
              <span
                v-else
                class="inline-flex items-center px-2 py-0.5 rounded-full text-[10px] font-semibold bg-danger-soft text-danger"
              >
                ✗ 错误
              </span>
            </span>
            <span class="text-danger truncate" :title="r.error">{{ r.error || '—' }}</span>
          </div>
        </div>
        </div>

        <!-- Footer -->
        <div class="flex justify-between items-center px-6 py-4 bg-surface-2 border-t border-border">
          <button
            class="h-9 px-4 bg-surface border border-border-strong rounded-md text-sm font-semibold text-ink hover:bg-surface-2"
            @click="currentStep = 1"
          >
            上一步
          </button>
          <div class="flex items-center gap-3">
            <span v-if="!canSubmit" class="text-xs text-danger">请先修复 CSV 错误行</span>
            <button
              class="h-9 px-5 bg-primary text-white rounded-md text-sm font-semibold hover:bg-primary-strong disabled:opacity-50 disabled:cursor-not-allowed"
              :disabled="!canSubmit || importing"
              @click="submitImport"
            >
              {{ importing ? '提交中...' : `提交导入（${previewStats.valid} 条）` }}
            </button>
          </div>
        </div>
      </div>
    </section>

    <!-- Step 3: Result -->
    <section
      v-if="currentStep === 3 && importResult"
      class="bg-surface border border-border rounded-lg overflow-hidden"
    >
      <div class="p-12 flex flex-col items-center gap-4 text-center">
        <div class="w-16 h-16 bg-success-soft text-success rounded-full grid place-items-center">
          <CheckCircle2 class="w-8 h-8" />
        </div>
        <h2 class="text-xl font-bold text-ink">导入完成</h2>
        <div class="flex gap-8 mt-2">
          <div class="flex flex-col gap-1 items-center">
            <span class="text-3xl font-bold text-success">{{ importResult.created }}</span>
            <span class="text-xs text-muted-foreground">成功创建</span>
          </div>
          <div class="w-px bg-border"></div>
          <div class="flex flex-col gap-1 items-center">
            <span class="text-3xl font-bold text-warning">{{ importResult.skipped }}</span>
            <span class="text-xs text-muted-foreground">跳过</span>
          </div>
          <div class="w-px bg-border" v-if="importResult.errors.length > 0"></div>
          <div v-if="importResult.errors.length > 0" class="flex flex-col gap-1 items-center">
            <span class="text-3xl font-bold text-danger">{{ importResult.errors.length }}</span>
            <span class="text-xs text-muted-foreground">失败</span>
          </div>
        </div>
        <div v-if="importResult.errors.length > 0" class="bg-danger-soft border border-danger/20 rounded-md p-3 text-xs text-danger w-full max-w-lg text-left">
          <p class="font-semibold mb-1">失败详情：</p>
          <ul class="list-disc pl-4 space-y-0.5">
            <li v-for="(e, i) in importResult.errors.slice(0, 5)" :key="i">{{ e }}</li>
            <li v-if="importResult.errors.length > 5">...还有 {{ importResult.errors.length - 5 }} 条</li>
          </ul>
        </div>
        <div class="flex gap-3 mt-4">
          <button
            class="h-10 px-5 bg-surface border border-border-strong rounded-md text-sm font-semibold text-ink hover:bg-surface-2"
            @click="reset"
          >
            继续导入
          </button>
          <button
            class="h-10 px-5 bg-primary text-white rounded-md text-sm font-semibold hover:bg-primary-strong"
            @click="goBack"
          >
            返回用户列表
          </button>
        </div>
      </div>
    </section>
  </AppShell>
</template>
