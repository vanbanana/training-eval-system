<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import axios from 'axios'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { ScrollArea } from '@/components/ui/scroll-area'
import { FileText, AlertTriangle, RefreshCw, Download, Type, Image as ImageIcon } from 'lucide-vue-next'

interface Section {
  title: string
  content: string
}

interface ReportData {
  upload_id: number
  filename: string
  file_type: string
  render_mode: 'structured_text' | 'plain_text' | 'unavailable'
  content: string
  sections?: Section[]
  is_readable: boolean
  warnings: string[]
  download_url?: string
}

const props = defineProps<{ uploadId: number }>()
const emit = defineEmits<{ (e: 'load-error'): void }>()

const loading = ref(true)
const error = ref<string | null>(null)
const data = ref<ReportData | null>(null)

const view = ref<'text' | 'image'>('text')
const pagesAvailable = ref(false)
const pageCount = ref(0)
const pageUrls = ref<string[]>([])
const pagesLoading = ref(false)
const pagesError = ref<string | null>(null)

const hasGarbled = computed(() => data.value?.warnings?.includes('garbled_segments_removed') ?? false)

async function fetchReport() {
  loading.value = true
  error.value = null
  try {
    const res = await axios.get(`/api/grading/uploads/${props.uploadId}/report-view`)
    data.value = res.data
  } catch (e) {
    const status = (e as { response?: { status?: number } })?.response?.status
    if (status === 403) {
      error.value = '无权限查看该报告'
    } else if (status === 404) {
      error.value = '报告未找到'
    } else {
      error.value = '加载报告失败'
    }
    emit('load-error')
  } finally {
    loading.value = false
  }
}

async function fetchPagesMeta() {
  try {
    const res = await axios.get(`/api/grading/uploads/${props.uploadId}/pages`)
    pagesAvailable.value = !!res.data?.available
    pageCount.value = res.data?.page_count ?? 0
  } catch {
    pagesAvailable.value = false
  }
}

async function loadPageImages() {
  if (pageUrls.value.length || pageCount.value === 0) return
  pagesLoading.value = true
  pagesError.value = null
  try {
    const urls: string[] = []
    for (let i = 1; i <= pageCount.value; i++) {
      const res = await axios.get(`/api/grading/uploads/${props.uploadId}/pages/${i}`, { responseType: 'blob' })
      urls.push(URL.createObjectURL(res.data))
    }
    pageUrls.value = urls
  } catch {
    pagesError.value = '原图加载失败'
  } finally {
    pagesLoading.value = false
  }
}

function setView(v: 'text' | 'image') {
  view.value = v
  if (v === 'image') loadPageImages()
}

onMounted(() => {
  fetchReport()
  fetchPagesMeta()
})

onBeforeUnmount(() => {
  pageUrls.value.forEach((u) => URL.revokeObjectURL(u))
})
</script>

<template>
  <div class="h-full flex flex-col">
    <!-- Header -->
    <div v-if="data" class="flex items-center justify-between gap-3 px-5 py-3 border-b border-border">
      <div class="flex items-center gap-2.5 min-w-0">
        <FileText class="w-4 h-4 text-primary shrink-0" />
        <span class="text-sm font-semibold text-ink shrink-0">原实训报告</span>
        <Badge variant="secondary" class="text-[10px] truncate">{{ data.filename }}</Badge>
      </div>
      <div class="flex items-center gap-2 shrink-0">
        <!-- 文本 / 原图 切换 -->
        <div v-if="pagesAvailable" class="inline-flex rounded-md border border-border bg-muted/40 p-0.5">
          <button
            type="button"
            class="inline-flex items-center gap-1 rounded px-2 py-1 text-[11px] font-medium transition-colors"
            :class="view === 'text' ? 'bg-background text-ink shadow-sm' : 'text-muted-foreground hover:text-ink'"
            @click="setView('text')"
          >
            <Type class="w-3 h-3" />文本
          </button>
          <button
            type="button"
            class="inline-flex items-center gap-1 rounded px-2 py-1 text-[11px] font-medium transition-colors"
            :class="view === 'image' ? 'bg-background text-ink shadow-sm' : 'text-muted-foreground hover:text-ink'"
            @click="setView('image')"
          >
            <ImageIcon class="w-3 h-3" />原图
          </button>
        </div>
        <template v-if="view === 'text'">
          <Badge
            v-if="data.is_readable && hasGarbled"
            variant="warning"
            class="text-[10px]"
            title="原文档含无法识别的二进制片段（旧版 .doc 提取产生的乱码），已自动隐藏"
          >
            已清理乱码片段
          </Badge>
          <Badge v-else-if="data.is_readable" variant="success" class="text-[10px]">文本正常</Badge>
          <Badge v-else variant="destructive" class="text-[10px]">不可读</Badge>
        </template>
        <Badge v-else variant="secondary" class="text-[10px]">共 {{ pageCount }} 页</Badge>
        <Button v-if="data.download_url" variant="ghost" size="icon-sm" :href="data.download_url">
          <Download class="w-3.5 h-3.5" />
        </Button>
      </div>
    </div>

    <!-- Loading state -->
    <div v-if="loading" class="flex-1 p-5 space-y-3">
      <Skeleton class="h-5 w-3/4" />
      <Skeleton class="h-3 w-full" />
      <Skeleton class="h-3 w-full" />
      <Skeleton class="h-3 w-5/6" />
      <Skeleton class="h-3 w-full" />
    </div>

    <!-- Error state -->
    <div v-else-if="error" class="flex-1 flex flex-col items-center justify-center gap-3 px-6">
      <AlertTriangle class="w-8 h-8 text-warning" />
      <p class="text-sm text-muted-foreground text-center">{{ error }}</p>
      <Button variant="outline" size="sm" @click="fetchReport"> <RefreshCw class="w-3.5 h-3.5 mr-1.5" />重试 </Button>
    </div>

    <!-- 原图视图 -->
    <template v-else-if="view === 'image'">
      <div v-if="pagesLoading" class="flex-1 p-5 space-y-3">
        <Skeleton class="h-72 w-full" />
        <Skeleton class="h-72 w-full" />
      </div>
      <div v-else-if="pagesError" class="flex-1 flex flex-col items-center justify-center gap-3 px-6">
        <AlertTriangle class="w-8 h-8 text-warning" />
        <p class="text-sm text-muted-foreground text-center">{{ pagesError }}</p>
        <Button variant="outline" size="sm" @click="loadPageImages">
          <RefreshCw class="w-3.5 h-3.5 mr-1.5" />重试
        </Button>
      </div>
      <ScrollArea v-else class="flex-1">
        <div class="px-5 py-4 space-y-4 bg-muted/30">
          <figure v-for="(url, idx) in pageUrls" :key="idx" class="space-y-1.5">
            <img
              :src="url"
              :alt="`第 ${idx + 1} 页`"
              class="w-full rounded-md border border-border shadow-sm bg-white"
            />
            <figcaption class="text-center text-[11px] text-muted-foreground">第 {{ idx + 1 }} 页</figcaption>
          </figure>
        </div>
      </ScrollArea>
    </template>

    <!-- Unavailable state -->
    <div v-else-if="data && !data.is_readable" class="flex-1 flex flex-col items-center justify-center gap-3 px-6">
      <AlertTriangle class="w-8 h-8 text-warning" />
      <p class="text-sm font-semibold text-ink">报告暂无法预览</p>
      <p class="text-xs text-muted-foreground text-center max-w-sm">
        解析文本不可读
        <span v-if="data.warnings.length">（{{ data.warnings.join('、') }}）</span>
      </p>
      <Button v-if="pagesAvailable" variant="outline" size="sm" @click="setView('image')">
        <ImageIcon class="w-3.5 h-3.5 mr-1.5" />查看原图
      </Button>
    </div>

    <!-- Structured text -->
    <ScrollArea v-else-if="data && data.render_mode === 'structured_text' && data.sections?.length" class="flex-1">
      <div class="px-5 py-4 space-y-4">
        <section v-for="(sec, idx) in data.sections" :key="idx" class="report-section">
          <h3 v-if="sec.title" class="text-sm font-bold text-ink mb-2">{{ sec.title }}</h3>
          <p class="text-sm text-ink leading-relaxed whitespace-pre-wrap">{{ sec.content }}</p>
        </section>
      </div>
    </ScrollArea>

    <!-- Plain text -->
    <ScrollArea v-else-if="data && data.content" class="flex-1">
      <div class="px-5 py-4">
        <p class="text-sm text-ink leading-relaxed whitespace-pre-wrap">{{ data.content }}</p>
      </div>
    </ScrollArea>
  </div>
</template>
