<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Checkbox } from '@/components/ui/checkbox'
import { User, Lock, Eye, EyeOff, ShieldCheck } from 'lucide-vue-next'

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
    errMsg.value = msg?.detail ?? msg?.message ?? '登录失败，请稍后重试'
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="grid min-h-screen lg:grid-cols-[1fr_560px]">
    <!-- LEFT brand panel -->
    <aside class="flex flex-col justify-between gap-10 bg-primary p-16 text-primary-foreground max-lg:min-h-[200px] max-lg:p-8">
      <div class="flex items-center gap-3.5">
        <div class="grid w-[38px] h-[38px] place-items-center rounded-sm bg-primary-foreground text-lg font-bold text-primary">
          实
        </div>
        <div>
          <div class="text-[15px] font-semibold leading-tight">实训评价管理系统</div>
          <div class="mt-0.5 text-[11px] uppercase tracking-widest text-primary-foreground/60">
            Training Evaluation System
          </div>
        </div>
      </div>

      <div>
        <h2 class="m-0 whitespace-pre-line text-[42px] font-bold leading-[1.3] max-lg:text-3xl">
          赋能教学评价 ·{{ '\n' }}沉淀实训数据
        </h2>
        <p class="mt-4 max-w-[480px] text-sm leading-[1.8] text-primary-foreground/60">
          以多维度评价、智能核查与教学画像，帮助教师减负增效，帮助学生看见自己的成长轨迹。
        </p>
      </div>

      <div class="text-xs text-primary-foreground/40">© 2026 软件学院 · 教学信息化中心</div>
    </aside>

    <!-- RIGHT form -->
    <main class="flex items-center bg-card px-16 max-lg:px-8">
      <form class="mx-auto flex w-full max-w-[416px] flex-col gap-6" @submit.prevent="onSubmit">
        <div>
          <h1 class="m-0 text-2xl font-bold text-ink">账号登录</h1>
          <p class="mt-2 text-[13px] text-muted-foreground">请使用学校统一账号登录系统</p>
        </div>

        <div class="h-px w-full bg-border" />

        <div class="space-y-2">
          <Label>账号</Label>
          <div
            class="flex h-11 items-center gap-2.5 rounded-md border border-border-strong bg-card px-3.5 transition-colors focus-within:border-primary focus-within:ring-2 focus-within:ring-primary/20"
          >
            <User class="w-3.5 h-3.5 shrink-0 text-muted-foreground" />
            <input
              v-model="username"
              type="text"
              placeholder="请输入工号 / 学号"
              autocomplete="username"
              class="w-full border-0 bg-transparent text-[13px] text-foreground outline-0 placeholder:text-subtle-foreground"
            />
          </div>
        </div>

        <div class="space-y-2">
          <div class="flex items-center justify-between">
            <Label>密码</Label>
            <RouterLink to="/forgot-password" class="text-xs font-medium text-primary hover:underline">
              忘记密码？
            </RouterLink>
          </div>
          <div
            class="flex h-11 items-center gap-2.5 rounded-md border border-border-strong bg-card px-3.5 transition-colors focus-within:border-primary focus-within:ring-2 focus-within:ring-primary/20"
          >
            <Lock class="w-3.5 h-3.5 shrink-0 text-muted-foreground" />
            <input
              v-model="password"
              :type="showPwd ? 'text' : 'password'"
              placeholder="请输入密码"
              autocomplete="current-password"
              class="w-full border-0 bg-transparent text-[13px] text-foreground outline-0 placeholder:text-subtle-foreground"
            />
            <button
              type="button"
              class="grid place-items-center text-muted-foreground hover:text-ink transition-colors"
              :aria-label="showPwd ? '隐藏密码' : '显示密码'"
              @click="showPwd = !showPwd"
            >
              <Eye v-if="!showPwd" class="w-3.5 h-3.5" />
              <EyeOff v-else class="w-3.5 h-3.5" />
            </button>
          </div>
        </div>

        <label class="flex cursor-pointer items-center gap-2 text-[13px] text-foreground">
          <Checkbox :model-value="remember" @update:model-value="(v) => remember = v === true" />
          <span>7 天内自动登录</span>
        </label>

        <p v-if="errMsg" class="rounded-md bg-danger-soft px-3 py-2 text-xs text-danger anim-in">{{ errMsg }}</p>

        <Button type="submit" :disabled="!canSubmit" class="h-[46px] text-sm font-semibold">
          {{ submitting ? '登录中…' : '登录' }}
        </Button>

        <div class="flex items-center justify-center gap-1.5 py-2 text-xs text-muted-foreground">
          <ShieldCheck class="w-3.5 h-3.5 text-success" />
          <span>系统已启用 HTTPS · 登录数据加密传输</span>
        </div>
      </form>
    </main>
  </div>
</template>
