import axios, { type AxiosError, type InternalAxiosRequestConfig } from 'axios'

// 全局 axios 拦截器：自动附加 token + 401 自动刷新
axios.defaults.timeout = 15000

// Token 刷新锁，防止并发刷新
let refreshPromise: Promise<string | null> | null = null

function getAccessToken(): string | null {
  const raw = localStorage.getItem('tes_token')
  if (!raw) return null
  try {
    const token = JSON.parse(raw)
    if (typeof token === 'string' && token.length > 10) return token
  } catch {
    if (raw.length > 10) return raw
  }
  return null
}

function getRefreshToken(): string | null {
  const raw = localStorage.getItem('tes_refresh_token')
  if (!raw) return null
  try {
    const token = JSON.parse(raw)
    if (typeof token === 'string' && token.length > 10) return token
  } catch {
    if (raw.length > 10) return raw
  }
  return null
}

function setAccessToken(token: string) {
  // useStorage 使用 JSON.stringify 存储，保持格式一致
  localStorage.setItem('tes_token', JSON.stringify(token))
}

function clearTokens() {
  localStorage.removeItem('tes_token')
  localStorage.removeItem('tes_refresh_token')
}

async function tryRefreshToken(): Promise<string | null> {
  // 复用正在进行的刷新请求，避免并发刷新
  if (refreshPromise) return refreshPromise

  refreshPromise = (async () => {
    const refreshToken = getRefreshToken()
    if (!refreshToken) return null

    try {
      const { data } = await axios.post('/api/auth/refresh', {
        refresh_token: refreshToken,
      })
      if (data.access_token) {
        setAccessToken(data.access_token)
        return data.access_token
      }
      return null
    } catch {
      return null
    } finally {
      refreshPromise = null
    }
  })()

  return refreshPromise
}

axios.interceptors.request.use((config) => {
  const token = getAccessToken()
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

axios.interceptors.response.use(
  (r) => r,
  async (err: AxiosError) => {
    const originalRequest = err.config as InternalAxiosRequestConfig & { _retry?: boolean }

    if (
      err.response?.status === 401 &&
      originalRequest &&
      !originalRequest.url?.includes('/api/auth/refresh') &&
      location.pathname !== '/login'
    ) {
      // 已经重试过，不再重试
      if (originalRequest._retry) {
        clearTokens()
        location.href = '/login'
        return Promise.reject(err)
      }

      // 尝试刷新 token
      const newToken = await tryRefreshToken()
      if (newToken) {
        originalRequest._retry = true
        originalRequest.headers.Authorization = `Bearer ${newToken}`
        return axios(originalRequest)
      }

      // 刷新失败，登出
      clearTokens()
      location.href = '/login'
    }

    return Promise.reject(err)
  },
)
