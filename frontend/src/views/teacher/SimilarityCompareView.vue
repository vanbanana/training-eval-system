<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import axios from 'axios'
import AppShell from '@/components/layout/AppShell.vue'
import BreadcrumbNav from '@/components/business/BreadcrumbNav.vue'
import EmptyState from '@/components/business/EmptyState.vue'
import { useToast } from '@/components/ui/toast'
import { confirm } from '@/composables/useConfirm'
import { safeGet } from '@/lib/api-helpers'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Badge } from '@/components/ui/badge'
import { Avatar } from '@/components/ui/avatar'
import { ScrollArea } from '@/components/ui/scroll-area'
import {
  AlertTriangle,
  CheckCircle2,
  XCircle,
  ArrowLeft,
} from 'lucide-vue-next'

interface SimilarityRecord {
  id: number
  task_id: number
  upload_a_id: number
  upload_b_id: number
  hamming_distance: number | null
  cosine_similarity: number | null
  state: string
  created_at?: string
}

interface SegmentPair {
  a_start: number
  a_end: number
  b_start: number
  b_end: number
  snippet_a: string
  snippet_b: string
  ratio: number
}

interface Submission {
  upload_id: number
  student_id: number
  student_name: string
  filename: string
}

const route = useRoute()
const router = useRouter()
const { toast } = useToast()

const recordId = computed(() => Number(route.params.id))
const record = ref<SimilarityRecord | null>(null)
const segments = ref<SegmentPair[]>([])
const submissionA = ref<Submission | null>(null)
const submissionB = ref<Submission | null>(null)
const loading = ref(true)
const deciding = ref(false)
const activeSegment = ref(0)

async function fetchAll() {
  loading.value = true
  try {
    // 1. segments
    const { data: segs } = await axios.get<SegmentPair[]>(`/api/similarity/${recordId.value}/segments`)
    segments.value = segs

    // 2. 反向找 record（list 接口）：通过 task 关联
    // 由于只有 GET /similarity/task/{task_id}，用 query 找 record_id
    const taskId = Number(route.query.task_id)
    if (taskId) {
      const listResult = await safeGet<SimilarityRecord[]>(
        `/api/similarity/task/${taskId}`,
        [],
        { params: { state: 'suspect' } },
      )
      if (listResult.error && listResult.status !== 404) {
        toast({
          description: `相似度记录 ${listResult.error}`,
          variant: 'warning',
        })
      }
      record.value =
        listResult.data.find((r) => r.id === recordId.value) ?? null

      if (record.value) {
        // 3. 学生信息（grading submissions）
        const { data: subs } = await axios.get<Submission[]>(`/api/grading/tasks/${taskId}/submissions`)
        submissionA.value = subs.find((s) => s.upload_id === record.value!.upload_a_id) ?? null
        submissionB.value = subs.find((s) => s.upload_id === record.value!.upload_b_id) ?? null
      }
    }
  } catch (e) {
    const msg = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    toast({ description: msg ?? '加载相似度数据失败', variant: 'destructive' })
  } finally {
    loading.value = false
  }
}

onMounted(fetchAll)

const overallRatio = computed(() => {
  if (!record.value) return 0
  if (record.value.cosine_similarity != null) return record.value.cosine_similarity
  if (record.value.hamming_distance != null) {
    return Math.max(0, 1 - record.value.hamming_distance / 64)
  }
  return 0
})

const overallPct = computed(() => Math.round(overallRatio.value * 100))

const ratioVariant = computed(() => {
  if (overallPct.value >= 85) return 'destructive' as const
  if (overallPct.value >= 70) return 'warning' as const
  return 'info' as const
})

const stateLabel = computed(() => {
  return ({
    suspect: '待裁决',
    confirmed: '已确认抄袭',
    ignored: '已忽略',
    cleared: '已澄清',
  } as Record<string, string>)[record.value?.state ?? ''] ?? record.value?.state ?? ''
})

const stateVariant = computed(() => {
  return ({
    suspect: 'destructive' as const,
    confirmed: 'destructive' as const,
    ignored: 'secondary' as const,
    cleared: 'success' as const,
  } as const)[record.value?.state ?? 'suspect'] ?? 'secondary'
})

async function decide(action: 'confirm' | 'ignore') {
  if (!record.value) return
  const verb = action === 'confirm' ? '确认存在抄袭' : '忽略此告警'
  const ok = await confirm({
    title: `${verb}？`,
    description:
      action === 'confirm'
        ? '后续可在审计日志查阅；学生评价将被标记为 reject 待重新提交'
        : '本次比对将不再出现在告警列表',
    variant: action === 'confirm' ? 'destructive' : 'default',
    confirmText: verb,
  })
  if (!ok) return
  deciding.value = true
  try {
    await axios.post(`/api/similarity/${recordId.value}/decision`, { action })
    toast({ description: `已${verb}`, variant: 'success' })
    if (record.value.task_id) {
      router.push(`/teacher/tasks/${record.value.task_id}/grading`)
    } else {
      await fetchAll()
    }
  } catch (e) {
    const msg = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    toast({ description: msg ?? '操作失败', variant: 'destructive' })
  } finally {
    deciding.value = false
  }
}

function backToList() {
  if (record.value?.task_id) {
    router.push({ path: `/teacher/tasks/${record.value.task_id}/grading`, query: { tab: 'suspicious' } })
  } else {
    router.back()
  }
}

const currentSegment = computed(() => segments.value[activeSegment.value] ?? null)
</script>

<template>
  <AppShell>
    <BreadcrumbNav
      :items="[
        { label: '工作台', to: '/dashboard' },
        { label: '批改工作台', to: record ? `/teacher/tasks/${record.task_id}/grading` : '/teacher/tasks' },
        { label: '相似度比对' },
      ]"
    />

    <div class="flex justify-between items-end">
      <div>
        <div class="flex items-center gap-3">
          <Button variant="outline" size="icon-sm" @click="backToList">
            <ArrowLeft class="w-4 h-4" />
          </Button>
          <h1 class="text-2xl font-bold text-ink">相似度比对</h1>
          <Badge v-if="record" :variant="stateVariant">{{ stateLabel }}</Badge>
        </div>
        <p class="mt-1.5 text-sm text-muted-foreground">
          双栏对比并标记重复段落 · 完成后请裁决
        </p>
      </div>
      <div v-if="record?.state === 'suspect'" class="flex items-center gap-3">
        <Button variant="outline" :disabled="deciding" @click="decide('ignore')">
          <CheckCircle2 class="w-4 h-4" />
          忽略告警
        </Button>
        <Button variant="destructive" :disabled="deciding" @click="decide('confirm')">
          <AlertTriangle class="w-4 h-4" />
          确认抄袭
        </Button>
      </div>
    </div>

    <div v-if="loading" class="grid grid-cols-2 gap-4">
      <Skeleton class="h-[600px]" />
      <Skeleton class="h-[600px]" />
    </div>

    <EmptyState
      v-else-if="!record"
      :icon="XCircle"
      title="未找到该相似度记录"
      description="可能记录已被处理或链接失效"
    />

    <template v-else>
      <!-- Top stats -->
      <Card class="px-6 py-4 grid grid-cols-3 gap-4 items-center">
        <div>
          <div class="text-xs text-muted-foreground">整体相似度</div>
          <div class="flex items-end gap-2 mt-1">
            <span class="text-3xl font-bold leading-none" :class="overallPct >= 85 ? 'text-danger' : overallPct >= 70 ? 'text-warning' : 'text-ink'">
              {{ overallPct }}%
            </span>
            <Badge :variant="ratioVariant">
              {{ overallPct >= 85 ? '高度可疑' : overallPct >= 70 ? '中度可疑' : '一般' }}
            </Badge>
          </div>
        </div>
        <div>
          <div class="text-xs text-muted-foreground">余弦相似度</div>
          <div class="font-mono text-lg text-ink mt-1">
            {{ record.cosine_similarity != null ? record.cosine_similarity.toFixed(4) : '—' }}
          </div>
        </div>
        <div>
          <div class="text-xs text-muted-foreground">汉明距离 / SimHash</div>
          <div class="font-mono text-lg text-ink mt-1">
            {{ record.hamming_distance != null ? record.hamming_distance : '—' }}
            <span class="text-xs text-muted-foreground">/ 64</span>
          </div>
        </div>
      </Card>

      <!-- Two-column diff -->
      <div class="grid grid-cols-2 gap-4 items-start">
        <Card class="overflow-hidden flex flex-col">
          <header class="px-5 py-3.5 border-b border-border bg-surface-2 flex items-center gap-2.5">
            <Avatar size="sm" class="!bg-info-soft !text-info">A</Avatar>
            <div class="flex-1 min-w-0">
              <div class="text-sm font-semibold text-ink truncate">{{ submissionA?.student_name ?? '学生 A' }}</div>
              <div class="text-[11px] text-muted-foreground font-mono truncate">{{ submissionA?.filename ?? '—' }}</div>
            </div>
          </header>
          <ScrollArea class="h-[480px]">
            <div v-if="segments.length === 0" class="p-12 text-center text-sm text-muted-foreground">
              未提取到重复段落
            </div>
            <div v-else class="p-4 flex flex-col gap-3">
              <div
                v-for="(seg, i) in segments"
                :key="`a-${i}`"
                class="p-3 rounded-md border cursor-pointer transition-colors"
                :class="activeSegment === i ? 'border-danger bg-danger-soft' : 'border-border hover:bg-surface-2'"
                @click="activeSegment = i"
              >
                <div class="flex justify-between items-center mb-1.5">
                  <span class="text-[10px] font-mono text-muted-foreground">
                    第 {{ seg.a_start }} - {{ seg.a_end }} 字
                  </span>
                  <Badge :variant="seg.ratio >= 0.85 ? 'destructive' : 'warning'" class="text-[10px]">
                    {{ Math.round(seg.ratio * 100) }}%
                  </Badge>
                </div>
                <div class="text-xs leading-relaxed text-foreground font-mono whitespace-pre-wrap">{{ seg.snippet_a }}</div>
              </div>
            </div>
          </ScrollArea>
        </Card>

        <Card class="overflow-hidden flex flex-col">
          <header class="px-5 py-3.5 border-b border-border bg-surface-2 flex items-center gap-2.5">
            <Avatar size="sm" class="!bg-warning-soft !text-warning">B</Avatar>
            <div class="flex-1 min-w-0">
              <div class="text-sm font-semibold text-ink truncate">{{ submissionB?.student_name ?? '学生 B' }}</div>
              <div class="text-[11px] text-muted-foreground font-mono truncate">{{ submissionB?.filename ?? '—' }}</div>
            </div>
          </header>
          <ScrollArea class="h-[480px]">
            <div v-if="segments.length === 0" class="p-12 text-center text-sm text-muted-foreground">
              未提取到重复段落
            </div>
            <div v-else class="p-4 flex flex-col gap-3">
              <div
                v-for="(seg, i) in segments"
                :key="`b-${i}`"
                class="p-3 rounded-md border cursor-pointer transition-colors"
                :class="activeSegment === i ? 'border-danger bg-danger-soft' : 'border-border hover:bg-surface-2'"
                @click="activeSegment = i"
              >
                <div class="flex justify-between items-center mb-1.5">
                  <span class="text-[10px] font-mono text-muted-foreground">
                    第 {{ seg.b_start }} - {{ seg.b_end }} 字
                  </span>
                  <Badge :variant="seg.ratio >= 0.85 ? 'destructive' : 'warning'" class="text-[10px]">
                    {{ Math.round(seg.ratio * 100) }}%
                  </Badge>
                </div>
                <div class="text-xs leading-relaxed text-foreground font-mono whitespace-pre-wrap">{{ seg.snippet_b }}</div>
              </div>
            </div>
          </ScrollArea>
        </Card>
      </div>

      <!-- Active segment ratio -->
      <Card v-if="currentSegment" class="px-5 py-3 flex items-center gap-4">
        <div class="text-xs text-muted-foreground">第 {{ activeSegment + 1 }} 段相似度</div>
        <div class="flex-1 h-2 bg-muted rounded-pill overflow-hidden">
          <div
            class="h-full rounded-pill transition-[width] duration-500"
            :class="currentSegment.ratio >= 0.85 ? 'bg-danger' : currentSegment.ratio >= 0.7 ? 'bg-warning' : 'bg-info'"
            :style="{ width: currentSegment.ratio * 100 + '%' }"
          ></div>
        </div>
        <div class="font-mono font-semibold" :class="currentSegment.ratio >= 0.85 ? 'text-danger' : 'text-ink'">
          {{ Math.round(currentSegment.ratio * 100) }}%
        </div>
      </Card>
    </template>
  </AppShell>
</template>
