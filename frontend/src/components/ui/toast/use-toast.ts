// shadcn-vue 风格的 useToast hook，与 ToastProvider 配合使用
// 全局共享一个 store，组件 / composable / 普通 .ts 都能调用
import { computed, ref } from 'vue'

const TOAST_LIMIT = 5
const TOAST_REMOVE_DELAY = 5000

export type ToastVariant = 'default' | 'destructive' | 'success' | 'warning' | 'info'

export interface ToastProps {
  id: string
  title?: string
  description?: string
  variant?: ToastVariant
  action?: {
    label: string
    onClick: () => void
  }
  duration?: number
  open?: boolean
  /** 内部跟踪：用于 trace 链路 */
  traceId?: string
}

const toasts = ref<ToastProps[]>([])
let counter = 0
const timeouts = new Map<string, ReturnType<typeof setTimeout>>()

function genId() {
  counter = (counter + 1) % Number.MAX_SAFE_INTEGER
  return `toast-${counter}-${Date.now()}`
}

function dismiss(id: string) {
  toasts.value = toasts.value.map((t) => (t.id === id ? { ...t, open: false } : t))
  // 给动画留出时间再彻底清理
  setTimeout(() => {
    toasts.value = toasts.value.filter((t) => t.id !== id)
    timeouts.delete(id)
  }, 250)
}

function toast(props: Omit<ToastProps, 'id' | 'open'>) {
  const id = genId()
  const item: ToastProps = {
    id,
    open: true,
    duration: TOAST_REMOVE_DELAY,
    variant: 'default',
    ...props,
  }
  toasts.value = [item, ...toasts.value].slice(0, TOAST_LIMIT)
  if (item.duration && item.duration > 0) {
    timeouts.set(
      id,
      setTimeout(() => dismiss(id), item.duration),
    )
  }
  return {
    id,
    dismiss: () => dismiss(id),
    update: (next: Partial<ToastProps>) => {
      toasts.value = toasts.value.map((t) => (t.id === id ? { ...t, ...next } : t))
    },
  }
}

export function useToast() {
  return {
    toasts: computed(() => toasts.value),
    toast,
    dismiss,
  }
}

// 便利封装：跟旧 lib/toast.ts 行为兼容
export function showToast(
  type: 'success' | 'error' | 'info',
  message: string,
  traceId?: string,
) {
  const variant: ToastVariant = type === 'error' ? 'destructive' : type
  return toast({ description: message, variant, traceId })
}
