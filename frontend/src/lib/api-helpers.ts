import axios, { type AxiosRequestConfig, type AxiosResponse } from 'axios'

export interface SafeGetResult<T> {
  data: T
  error: string | null
  status: number | null
}

/**
 * 静默降级的 GET 请求.
 *
 * 设计目的：取代 `axios.get(...).catch(() => ({ data: [] }))` 模式。
 * - 仍然不会抛错（不阻塞页面渲染）
 * - 但会把错误信息和 HTTP 状态码透出，便于在 UI 显示"加载失败 [重试]"
 * - 控制台会打日志，排查时不会再"鬼隐"
 *
 * 用法：
 *   const { data, error } = await safeGet<Task[]>('/api/tasks', [])
 *   if (error) showRetryChip(error, fetchAll)
 *   tasks.value = data
 */
export async function safeGet<T>(
  url: string,
  fallback: T,
  config?: AxiosRequestConfig,
): Promise<SafeGetResult<T>> {
  try {
    const r: AxiosResponse<T> = await axios.get<T>(url, config)
    return { data: r.data, error: null, status: r.status }
  } catch (e) {
    const status = (e as { response?: { status?: number } })?.response?.status ?? null
    const detail =
      (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail ?? null
    const message = describeError(status, detail)
    // 控制台保留信息，便于运维定位
    // eslint-disable-next-line no-console
    console.warn('[safeGet] failed', { url, status, message, error: e })
    return { data: fallback, error: message, status }
  }
}

export function describeError(
  status: number | null,
  detail?: string | null,
): string {
  if (status === 401) return '未登录或会话过期'
  if (status === 403) return '无权限访问'
  if (status === 404) return detail ?? '接口未实现或资源不存在'
  if (status === 429) return '请求过于频繁'
  if (status && status >= 500) return detail ?? '服务异常，请稍后重试'
  if (status && status >= 400) return detail ?? '请求被拒绝'
  if (status == null) return detail ?? '网络错误'
  return detail ?? `加载失败（${status}）`
}
