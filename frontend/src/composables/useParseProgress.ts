/**
 * 解析进度 composable - 监听 WebSocket 进度推送并更新上传状态.
 *
 * 用于 TaskDetailView 和 GradingView 中实时显示解析进度。
 */
import { computed, ref, watch, type Ref } from 'vue'
import { useWebSocket, type ProgressMessage } from './useWebSocket'

export interface ParseProgressState {
  upload_id: number
  status: string
  progress: number
  error: string | null
}

export function useParseProgress(uploadIds: Ref<number[]>) {
  const { messages, lastMessage, connected } = useWebSocket<ProgressMessage>('progress')

  // 每个 upload 的最新进度状态
  const progressMap = ref<Record<number, ParseProgressState>>({})

  // 监听新消息，更新 map
  watch(lastMessage, (msg) => {
    if (!msg) return
    if (uploadIds.value.includes(msg.upload_id)) {
      progressMap.value = {
        ...progressMap.value,
        [msg.upload_id]: {
          upload_id: msg.upload_id,
          status: msg.status,
          progress: msg.progress,
          error: msg.error,
        },
      }
    }
  })

  function getProgress(uploadId: number): ParseProgressState | null {
    return progressMap.value[uploadId] ?? null
  }

  const hasActiveProgress = computed(() =>
    Object.values(progressMap.value).some(
      (p) => p.status === 'parsing' || p.status === 'scoring',
    ),
  )

  return {
    progressMap,
    connected,
    getProgress,
    hasActiveProgress,
    messages,
  }
}
