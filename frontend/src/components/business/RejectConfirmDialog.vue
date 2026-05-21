<script setup lang="ts">
/**
 * 业务组件：RejectConfirmDialog
 *
 * 教师打回评价的确认弹窗。强制要求 reason ≥ 20 字。
 * 支持单条 / 批量两种模式（通过 props.targets 数组）。
 *
 * 使用方式：
 *   <RejectConfirmDialog
 *     v-model:open="show"
 *     :targets="[{ id: 1, label: '李同学的提交' }]"
 *     :reasonChips="[...]"
 *     @confirm="async (reason) => { await api(...); }"
 *   />
 */
import { computed, ref, watch } from 'vue'
import { AlertTriangle } from 'lucide-vue-next'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'

interface Target {
  id: number
  label: string
}

interface Props {
  open: boolean
  /** 单条或批量的目标信息（仅用于显示）*/
  targets: Target[]
  /** 快速选择 reason 的预设短语 */
  reasonChips?: string[]
  /** 最少字数（默认 20）*/
  minLength?: number
  /** 提交中状态（外部控制）*/
  submitting?: boolean
  /** 自定义标题 */
  title?: string
}

const props = withDefaults(defineProps<Props>(), {
  reasonChips: () => [
    '解析失败，请重新提交',
    '内容与要求不符',
    '相似度过高，疑似抄袭',
    '文档格式不规范',
    '内容过于简略',
  ],
  minLength: 20,
  submitting: false,
})

const emit = defineEmits<{
  (e: 'update:open', v: boolean): void
  (e: 'confirm', reason: string): void
}>()

const reason = ref('')

watch(
  () => props.open,
  (v) => {
    if (v) reason.value = ''
  },
)

const titleText = computed(() => {
  if (props.title) return props.title
  if (props.targets.length === 0) return '打回评价'
  if (props.targets.length === 1) return `打回 ${props.targets[0].label}`
  return `批量打回（${props.targets.length} 项）`
})

const trimmedLen = computed(() => reason.value.trim().length)
const valid = computed(() => trimmedLen.value >= props.minLength)

function applyChip(chip: string) {
  // 拼接到现有 reason 后，避免清空已输入内容
  const suffix = '。请根据要求重新整理后提交。'
  if (reason.value.trim()) {
    reason.value = reason.value.trim() + ' / ' + chip + suffix
  } else {
    reason.value = chip + suffix
  }
}

function submit() {
  if (!valid.value) return
  emit('confirm', reason.value.trim())
}

function cancel() {
  emit('update:open', false)
}
</script>

<template>
  <Dialog :open="open" @update:open="(v) => emit('update:open', v)">
    <DialogContent class="max-w-lg">
      <DialogHeader>
        <DialogTitle class="flex items-center gap-2">
          <AlertTriangle class="w-4 h-4 text-danger" />
          {{ titleText }}
        </DialogTitle>
        <DialogDescription>
          打回操作不可撤销。学生将收到通知并被要求重新提交。
        </DialogDescription>
      </DialogHeader>
      <div class="flex flex-col gap-4">
        <div v-if="targets.length > 1" class="bg-surface-2 border border-border rounded-md px-3 py-2 max-h-32 overflow-y-auto">
          <ul class="text-xs text-muted-foreground space-y-1">
            <li v-for="t in targets.slice(0, 8)" :key="t.id" class="font-mono">· {{ t.label }}</li>
            <li v-if="targets.length > 8" class="text-subtle-foreground">...还有 {{ targets.length - 8 }} 项</li>
          </ul>
        </div>

        <div>
          <Label class="text-xs">快速选择原因</Label>
          <div class="flex flex-wrap gap-2 mt-2">
            <Button
              v-for="chip in reasonChips"
              :key="chip"
              variant="outline"
              size="sm"
              class="h-7 text-[11px]"
              @click="applyChip(chip)"
            >
              {{ chip }}
            </Button>
          </div>
        </div>

        <div class="space-y-2">
          <Label class="flex items-center justify-between">
            详细说明 <span class="text-danger">*</span>
            <span
              class="font-mono text-[11px]"
              :class="valid ? 'text-success' : 'text-danger'"
            >
              {{ trimmedLen }}/{{ minLength }}
            </span>
          </Label>
          <Textarea v-model="reason" rows="5" placeholder="请详细说明打回原因，例如：报告章节缺失、源码无法编译、相似度过高等..." />
        </div>
      </div>
      <DialogFooter class="gap-2">
        <Button variant="outline" :disabled="submitting" @click="cancel">取消</Button>
        <Button variant="destructive" :disabled="!valid || submitting" @click="submit">
          {{ submitting ? '打回中...' : '确认打回' }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
