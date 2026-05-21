// 全局 confirm 服务：替代原生 confirm()
// 使用方法：
//   1. 在 AppShell 的 mount 处放置一个 <ConfirmDialog ref="confirmRef" />
//   2. main.ts 中调用 setConfirm(confirmRef)
//   3. 各组件内 import { confirm } from '@/composables/useConfirm'
//      const ok = await confirm({ title: '...', description: '...' })
import type { Ref } from 'vue'

interface ConfirmOptions {
  title?: string
  description?: string
  confirmText?: string
  cancelText?: string
  variant?: 'default' | 'destructive'
}

interface ConfirmInstance {
  show: (opts: ConfirmOptions) => Promise<boolean>
}

let instance: ConfirmInstance | null = null

export function setConfirm(ref: Ref<ConfirmInstance | null> | ConfirmInstance | null) {
  if (!ref) {
    instance = null
    return
  }
  // 既支持直接组件实例，也支持 ref 对象
  instance = 'value' in (ref as Ref<unknown>) ? (ref as Ref<ConfirmInstance | null>).value : (ref as ConfirmInstance)
}

export function confirm(opts: ConfirmOptions): Promise<boolean> {
  if (!instance) {
    if (typeof window !== 'undefined') {
      return Promise.resolve(window.confirm(opts.description ?? opts.title ?? '确认操作？'))
    }
    return Promise.resolve(false)
  }
  return instance.show(opts)
}
