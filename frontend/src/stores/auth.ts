import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { useStorage } from '@vueuse/core'
import axios from 'axios'

export interface UserInfo {
  id: number
  username: string
  display_name: string
  role: string
  is_active: boolean
}

export const useAuthStore = defineStore('auth', () => {
  const token = useStorage<string>('tes_token', '')
  const refreshToken = useStorage<string>('tes_refresh_token', '')
  const user = ref<UserInfo | null>(null)
  const isAuthenticated = computed(() => !!token.value && !!user.value)

  async function login(username: string, password: string) {
    const { data } = await axios.post('/api/auth/login', { username, password })
    token.value = data.access_token
    refreshToken.value = data.refresh_token
    user.value = data.user
  }

  async function fetchMe() {
    if (!token.value) return
    try {
      const { data } = await axios.get('/api/auth/me')
      user.value = data
    } catch (err: any) {
      // 仅在 401 时登出，网络错误等其他错误不登出
      if (err.response?.status === 401) {
        logout()
      }
    }
  }

  function logout() {
    token.value = ''
    refreshToken.value = ''
    user.value = null
  }

  return { token, refreshToken, user, isAuthenticated, login, fetchMe, logout }
})
