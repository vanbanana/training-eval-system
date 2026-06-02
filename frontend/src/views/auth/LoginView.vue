<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { Label } from '@/components/ui/label'
import { Checkbox } from '@/components/ui/checkbox'
import { User, Lock, Eye, EyeOff, ShieldCheck } from 'lucide-vue-next'
import loginBg from '@/assets/login-bg.png'

const router = useRouter()
const route = useRoute()
const auth = useAuthStore()

const username = ref('')
const password = ref('')
const remember = ref(true)
const showPwd = ref(false)
const submitting = ref(false)
const errMsg = ref('')

const canSubmit = computed(() => username.value.trim() && password.value && !submitting.value)

async function onSubmit() {
  if (!canSubmit.value) return
  submitting.value = true
  errMsg.value = ''
  try {
    await auth.login(username.value.trim(), password.value)
    const redirect = (route.query.redirect as string) || '/dashboard'
    router.replace(redirect)
  } catch (e) {
    const msg = (e as { response?: { data?: { detail?: string; message?: string } } })?.response?.data
    errMsg.value = msg?.detail ?? msg?.message ?? ''
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="login-wrapper">
    <img :src="loginBg" alt="" class="login-bg" />
    <div class="login-card-container">
      <form class="login-card" @submit.prevent="onSubmit">
        <div class="login-header">
          <h1>账号登录</h1>
          <p>访问学校统一账号登录系统</p>
        </div>
        <div class="field">
          <Label class="field-label">账号</Label>
          <div class="field-input">
            <User class="field-icon" />
            <input v-model="username" type="text" placeholder="请输入工号 / 学号" autocomplete="username" />
          </div>
        </div>
        <div class="field">
          <div class="field-row">
            <Label class="field-label">密码</Label>
            <RouterLink to="/forgot-password" class="forgot-link">忘记密码?</RouterLink>
          </div>
          <div class="field-input">
            <Lock class="field-icon" />
            <input v-model="password" :type="showPwd ? 'text' : 'password'" placeholder="请输入密码" autocomplete="current-password" />
            <button type="button" class="eye-btn" @click="showPwd = !showPwd">
              <Eye v-if="!showPwd" class="field-icon" /><EyeOff v-else class="field-icon" />
            </button>
          </div>
        </div>
        <label class="remember">
          <Checkbox :model-value="remember" @update:model-value="(v) => remember = v === true" />
          <span>7 天内自动登录</span>
        </label>
        <p v-if="errMsg" class="error-msg">{{ errMsg }}</p>
        <button type="submit" :disabled="!canSubmit" class="submit-btn">
          {{ submitting ? '登录中…' : '登录' }}
        </button>
        <div class="security-notice">
          <ShieldCheck class="security-icon" />
          <span>系统已启用 HTTPS · 登录数据加密传输</span>
        </div>
      </form>
    </div>
  </div>
</template>

<style scoped>
.login-wrapper { position: relative; display: flex; align-items: center; justify-content: flex-end; min-height: 100vh; overflow: hidden; }
.login-bg { position: absolute; inset: 0; width: 100%; height: 100%; object-fit: cover; object-position: 35% center; }
.login-card-container { position: relative; z-index: 10; margin-right: 8%; width: 340px; }
.login-card { display: flex; flex-direction: column; gap: 20px; background: #fff; border-radius: 16px; padding: 40px; box-shadow: 0 20px 60px rgba(0,0,0,0.08); }
.login-header h1 { font-size: 22px; font-weight: 700; color: #1a1a1a; margin: 0; }
.login-header p { font-size: 13px; color: #999; margin-top: 6px; }
.field { display: flex; flex-direction: column; gap: 6px; }
.field-label { font-size: 13px; font-weight: 500; color: #555; }
.field-row { display: flex; justify-content: space-between; align-items: center; }
.forgot-link { font-size: 12px; color: #4361ee; text-decoration: none; }
.forgot-link:hover { text-decoration: underline; }
.field-input { display: flex; align-items: center; gap: 10px; height: 44px; border-bottom: 1px solid #e5e5e5; padding: 0 4px; transition: border-color 0.2s; }
.field-input:focus-within { border-color: #4361ee; }
.field-icon { width: 16px; height: 16px; color: #ccc; flex-shrink: 0; }
.field-input input { flex: 1; border: none; outline: none; background: transparent; font-size: 14px; color: #1a1a1a; }
.field-input input::placeholder { color: #ccc; }
.eye-btn { background: none; border: none; cursor: pointer; padding: 0; display: grid; place-items: center; }
.remember { display: flex; align-items: center; gap: 8px; font-size: 13px; color: #666; cursor: pointer; margin-top: 4px; }
.error-msg { background: #fef2f2; border-radius: 8px; padding: 8px 12px; font-size: 12px; color: #dc2626; }
.submit-btn { height: 44px; width: 100%; border: none; border-radius: 10px; background: #4361ee; color: #fff; font-size: 14px; font-weight: 600; cursor: pointer; transition: background 0.2s; }
.submit-btn:hover { background: #3a56d4; }
.submit-btn:disabled { opacity: 0.5; cursor: not-allowed; }
.security-notice { display: flex; align-items: center; justify-content: center; gap: 6px; font-size: 11px; color: #ccc; margin-top: 4px; }
.security-icon { width: 14px; height: 14px; color: #4ade80; }
@media (max-width: 1024px) { .login-card-container { margin: 0 auto; } }
</style>