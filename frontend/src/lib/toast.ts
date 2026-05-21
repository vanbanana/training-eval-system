// 旧 API 兼容层。新代码请改用：
//   import { useToast } from '@/components/ui/toast'
//   const { toast } = useToast()
//   toast({ description: 'xxx', variant: 'success' })
import { showToast as _show, useToast as _useToast, type ToastVariant } from '@/components/ui/toast'

export interface ToastItem {
  id: string
  type: 'success' | 'error' | 'info'
  message: string
  traceId?: string
}

/**
 * 旧风格调用：show('success', '保存成功')
 * 兼容 5 个不同业务文件
 */
export function useToast() {
  const { toasts } = _useToast()
  function show(type: 'success' | 'error' | 'info', message: string, traceId?: string) {
    return _show(type, message, traceId)
  }
  return { toasts, show, dismiss: () => {} }
}

export type { ToastVariant }
