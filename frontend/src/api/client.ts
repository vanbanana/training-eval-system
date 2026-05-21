import axios from 'axios'

// 全局 axios 拦截器：自动附加 token + 401 自动登出
axios.defaults.timeout = 15000

axios.interceptors.request.use((config) => {
  const raw = localStorage.getItem('tes_token')
  if (raw) {
    try {
      const token = JSON.parse(raw)
      if (token && token.length > 10) {
        config.headers.Authorization = `Bearer ${token}`
      }
    } catch {
      // 非 JSON 格式，直接用
      if (raw.length > 10) {
        config.headers.Authorization = `Bearer ${raw}`
      }
    }
  }
  return config
})

axios.interceptors.response.use(
  (r) => r,
  (err) => {
    if (err.response?.status === 401 && location.pathname !== '/login') {
      localStorage.removeItem('tes_token')
      location.href = '/login'
    }
    return Promise.reject(err)
  },
)
