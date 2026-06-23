<script setup lang="ts">
/**
 * 业务组件：FileUploader
 *
 * 功能：
 * - 拖放 + 点击选择多文件
 * - 客户端 SHA-256 校验（用于检测重复文件）
 * - 单文件上传进度（基于 axios onUploadProgress）
 * - 文件类型 / 体积白名单校验
 * - 并发上传 with 队列（默认 2 并发）
 * - 失败重试入口
 *
 * 不支持：分片续传（后端目前用单 POST，文件体积上限 50MB；Epic 31+ 后端开放分片接口后再扩展）
 */
import { computed, ref } from 'vue'
import axios, { type AxiosError } from 'axios'
import {
  CloudUpload,
  FolderOpen,
  FileText,
  Archive,
  Image as ImageIcon,
  CircleCheck,
  CircleAlert,
  Loader2,
  X,
  RefreshCw,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { useToast } from '@/components/ui/toast'
import { cn } from '@/lib/utils'

interface Props {
  /** 上传 endpoint，相对路径 */
  endpoint: string
  /** 接受的扩展名（含点）*/
  accept?: string[]
  /** 单文件最大 MB */
  maxSizeMb?: number
  /** 是否允许多文件 */
  multiple?: boolean
  /** 是否禁用（任务已截止 / 未发布 等场景）*/
  disabled?: boolean
  /** 禁用提示文案 */
  disabledHint?: string
  /** 并发上传数 */
  concurrency?: number
  /** 自定义 form 字段名 */
  fieldName?: string
  /** 紧凑模式（已有文件时使用） */
  compact?: boolean
  class?: string
}

const props = withDefaults(defineProps<Props>(), {
  accept: () => ['.pdf', '.docx', '.doc', '.zip', '.png', '.jpg', '.jpeg'],
  maxSizeMb: 50,
  multiple: false,
  disabled: false,
  disabledHint: '当前不能上传',
  concurrency: 2,
  fieldName: 'file',
  compact: false,
})

const emit = defineEmits<{
  (e: 'success', payload: { file: File; response: unknown }): void
  (e: 'error', payload: { file: File; message: string }): void
  (e: 'allDone'): void
}>()

const { toast } = useToast()

interface QueueItem {
  id: string
  file: File
  status: 'pending' | 'hashing' | 'uploading' | 'success' | 'error'
  progress: number
  error?: string
  sha256?: string
}

const queue = ref<QueueItem[]>([])
const fileInput = ref<HTMLInputElement | null>(null)
const dragOver = ref(false)

let counter = 0
function genId() {
  counter += 1
  return `f-${counter}-${Date.now()}`
}

const acceptAttr = computed(() => props.accept.join(','))

const activeUploads = computed(
  () => queue.value.filter((q) => q.status === 'uploading' || q.status === 'hashing').length,
)

function pickFile() {
  if (props.disabled) return
  fileInput.value?.click()
}

function onFileChange(e: Event) {
  const input = e.target as HTMLInputElement
  const files = Array.from(input.files ?? [])
  enqueue(files)
  input.value = ''
}

function onDrop(e: DragEvent) {
  e.preventDefault()
  dragOver.value = false
  if (props.disabled) return
  const files = Array.from(e.dataTransfer?.files ?? [])
  enqueue(files)
}

function validate(file: File): string | null {
  const ext = '.' + file.name.split('.').pop()?.toLowerCase()
  if (!props.accept.includes(ext)) {
    return `文件类型不支持，仅允许 ${props.accept.join(' / ')}`
  }
  if (file.size > props.maxSizeMb * 1024 * 1024) {
    return `单文件不能超过 ${props.maxSizeMb} MB`
  }
  return null
}

function enqueue(files: File[]) {
  const accepted: QueueItem[] = []
  for (const f of files) {
    const err = validate(f)
    if (err) {
      toast({ description: `${f.name}: ${err}`, variant: 'destructive' })
      continue
    }
    accepted.push({
      id: genId(),
      file: f,
      status: 'pending',
      progress: 0,
    })
  }
  if (!props.multiple && accepted.length > 1) {
    toast({ description: '当前仅支持单文件上传，已忽略多余文件', variant: 'warning' })
    accepted.splice(1)
  }
  if (!props.multiple) {
    // 单文件模式覆盖未完成项
    queue.value = queue.value.filter((q) => q.status === 'success')
  }
  queue.value.push(...accepted)
  pump()
}

async function sha256(file: File): Promise<string> {
  if (!('crypto' in window) || !window.crypto.subtle) return ''
  try {
    const buf = await file.arrayBuffer()
    const digest = await window.crypto.subtle.digest('SHA-256', buf)
    return Array.from(new Uint8Array(digest))
      .map((b) => b.toString(16).padStart(2, '0'))
      .join('')
  } catch {
    return ''
  }
}

async function uploadOne(item: QueueItem) {
  try {
    item.status = 'hashing'
    item.sha256 = await sha256(item.file)

    item.status = 'uploading'
    const form = new FormData()
    form.append(props.fieldName, item.file)
    if (item.sha256) form.append('sha256', item.sha256)

    const { data } = await axios.post(props.endpoint, form, {
      headers: { 'Content-Type': 'multipart/form-data' },
      onUploadProgress: (e) => {
        if (e.total) item.progress = Math.round((e.loaded / e.total) * 100)
      },
    })
    item.status = 'success'
    item.progress = 100
    emit('success', { file: item.file, response: data })
    if (queue.value.every((q) => q.status === 'success' || q.status === 'error')) {
      emit('allDone')
    }
  } catch (e) {
    const ax = e as AxiosError<{ detail?: string; message?: string }>
    item.status = 'error'
    item.error = ax.response?.data?.detail ?? ax.message ?? '上传失败'
    emit('error', { file: item.file, message: item.error })
  } finally {
    pump()
  }
}

function pump() {
  if (activeUploads.value >= props.concurrency) return
  const next = queue.value.find((q) => q.status === 'pending')
  if (next) {
    void uploadOne(next)
    if (activeUploads.value < props.concurrency) pump()
  }
}

function retry(item: QueueItem) {
  item.status = 'pending'
  item.progress = 0
  item.error = undefined
  pump()
}

function remove(item: QueueItem) {
  queue.value = queue.value.filter((q) => q.id !== item.id)
}

function fileIcon(file: File) {
  const ext = file.name.split('.').pop()?.toLowerCase()
  if (ext === 'pdf') return { cmp: FileText, color: 'bg-danger-soft text-danger' }
  if (ext === 'zip' || ext === 'rar') return { cmp: Archive, color: 'bg-info-soft text-info' }
  if (['png', 'jpg', 'jpeg', 'gif'].includes(ext ?? '')) return { cmp: ImageIcon, color: 'bg-success-soft text-success' }
  return { cmp: FileText, color: 'bg-muted text-muted-foreground' }
}

function fmtSize(bytes: number) {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}

function clearAll() {
  queue.value = []
}

defineExpose({ clearAll })
</script>

<template>
  <div :class="cn('flex flex-col gap-4', $props.class)">
    <!-- Compact drop zone (single row when files already exist) -->
    <div
      v-if="compact"
      :class="cn(
        'flex items-center gap-3 px-4 py-2.5 rounded-lg border border-dashed transition-colors',
        disabled
          ? 'border-border-strong cursor-not-allowed opacity-60 bg-muted'
          : dragOver
            ? 'border-primary bg-primary-soft cursor-pointer'
            : 'border-border cursor-pointer hover:border-primary hover:bg-primary-soft/30',
      )"
      @click="pickFile"
      @dragover.prevent="dragOver = true"
      @dragleave="dragOver = false"
      @drop="onDrop"
    >
      <CloudUpload class="w-4 h-4 text-muted-foreground flex-shrink-0" />
      <span class="text-xs text-muted-foreground">
        <template v-if="disabled">{{ disabledHint }}</template>
        <template v-else>拖入或点击替换文件 · {{ accept.join(' / ') }} · ≤ {{ maxSizeMb }} MB</template>
      </span>
      <input
        ref="fileInput"
        type="file"
        class="hidden"
        :accept="acceptAttr"
        :multiple="multiple"
        @change="onFileChange"
      />
    </div>

    <!-- Full drop zone (no files yet) -->
    <div
      v-else
      :class="cn(
        'rounded-lg flex flex-col items-center justify-center transition-colors p-10 min-h-[200px] gap-3.5',
        disabled
          ? 'bg-muted border-2 border-dashed border-border-strong cursor-not-allowed opacity-60'
          : dragOver
            ? 'bg-primary-soft border-2 border-dashed border-primary cursor-pointer'
            : 'bg-card border-2 border-dashed border-primary cursor-pointer hover:bg-primary-soft/40',
      )"
      @click="pickFile"
      @dragover.prevent="dragOver = true"
      @dragleave="dragOver = false"
      @drop="onDrop"
    >
      <div :class="cn(
        'w-14 h-14 rounded-full grid place-items-center transition-colors',
        disabled ? 'bg-border text-muted-foreground' : 'bg-primary-soft text-primary',
      )">
        <CloudUpload class="w-6 h-6" />
      </div>
      <div class="text-base font-semibold text-ink">
        <template v-if="disabled">{{ disabledHint }}</template>
        <template v-else>将文件拖到此处，或点击选择</template>
      </div>
      <div class="text-xs text-muted-foreground">
        支持 {{ accept.join(' / ') }} · 单文件 ≤ {{ maxSizeMb }} MB
      </div>
      <Button v-if="!disabled" type="button" class="mt-1" @click.stop="pickFile">
        <FolderOpen class="w-4 h-4" />
        选择文件
      </Button>

      <input
        ref="fileInput"
        type="file"
        class="hidden"
        :accept="acceptAttr"
        :multiple="multiple"
        @change="onFileChange"
      />
    </div>

    <!-- Queue -->
    <div v-if="queue.length > 0" class="flex flex-col gap-2">
      <div
        v-for="item in queue"
        :key="item.id"
        class="grid grid-cols-[44px_1fr_120px_auto] items-center gap-3 px-3 py-2.5 bg-surface-2 border border-border rounded-md anim-in"
      >
        <div :class="cn('w-10 h-10 rounded-md grid place-items-center', fileIcon(item.file).color)">
          <component :is="fileIcon(item.file).cmp" class="w-4 h-4" />
        </div>
        <div class="min-w-0">
          <div class="text-sm font-semibold text-ink truncate">{{ item.file.name }}</div>
          <div class="text-[11px] text-muted-foreground font-mono mt-0.5">
            {{ fmtSize(item.file.size) }}
            <template v-if="item.sha256">· sha256: {{ item.sha256.slice(0, 8) }}…</template>
          </div>
          <div v-if="item.status === 'uploading'" class="mt-1.5 h-1 bg-muted rounded-pill overflow-hidden">
            <div class="h-full bg-primary rounded-pill transition-[width] duration-200" :style="{ width: item.progress + '%' }" />
          </div>
          <div v-else-if="item.status === 'error'" class="mt-1 text-[11px] text-danger">{{ item.error }}</div>
        </div>
        <div class="text-xs">
          <span v-if="item.status === 'pending'" class="text-muted-foreground">排队中</span>
          <span v-else-if="item.status === 'hashing'" class="flex items-center gap-1 text-info">
            <Loader2 class="w-3 h-3 animate-spin" />
            校验中
          </span>
          <span v-else-if="item.status === 'uploading'" class="flex items-center gap-1 text-primary">
            <Loader2 class="w-3 h-3 animate-spin" />
            上传 {{ item.progress }}%
          </span>
          <span v-else-if="item.status === 'success'" class="flex items-center gap-1 text-success">
            <CircleCheck class="w-3 h-3" />
            上传成功
          </span>
          <span v-else class="flex items-center gap-1 text-danger">
            <CircleAlert class="w-3 h-3" />
            上传失败
          </span>
        </div>
        <div class="flex gap-1">
          <Button v-if="item.status === 'error'" variant="ghost" size="icon-sm" @click="retry(item)">
            <RefreshCw class="w-3 h-3" />
          </Button>
          <Button
            v-if="item.status !== 'uploading' && item.status !== 'hashing'"
            variant="ghost"
            size="icon-sm"
            @click="remove(item)"
          >
            <X class="w-3 h-3" />
          </Button>
        </div>
      </div>
    </div>
  </div>
</template>
