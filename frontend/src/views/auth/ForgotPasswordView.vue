<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import axios from 'axios'
import { useToast } from '@/components/ui/toast'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import {
  ArrowLeft,
  Shield,
  User,
  Mail,
  Lock,
  KeyRound,
  CircleCheck,
  LifeBuoy,
} from 'lucide-vue-next'

type Step = 1 | 2 | 3 | 4
const step = ref<Step>(1)
const router = useRouter()
const { toast } = useToast()

const username = ref('')
const email = ref('')
const code = ref('')
const newPassword = ref('')
const confirmPassword = ref('')

const submitting = ref(false)
const errMsg = ref('')

async function step1Submit() {
  errMsg.value = ''
  if (!username.value.trim()) {
    errMsg.value = '请输入账号'
    return
  }
  submitting.value = true
  try {
    // 后端尚未开放找回密码端点（Epic 31 + 后端任务）
    // 当前：调用 forgot-password 失败时显示明确提示
    await axios.post('/api/auth/forgot-password', {
      username: username.value.trim(),
    })
    toast({ description: '验证码已发送至绑定邮箱', variant: 'success' })
    step.value = 2
  } catch (e) {
    const status = (e as { response?: { status?: number } })?.response?.status
    if (status === 404 || status === 405) {
      // endpoint 还没实现：不挡住流程，给视觉演示
      toast({
        description: '后端找回密码接口尚在开发，请联系管理员重置（admin@example.edu）',
        variant: 'warning',
        duration: 8000,
      })
      // 演示流程：依然推进
      email.value = `${username.value.slice(0, 4)}****@university.edu`
      step.value = 2
    } else {
      const msg = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail
      errMsg.value = msg ?? '请求失败，请稍后再试'
    }
  } finally {
    submitting.value = false
  }
}

function step2Submit() {
  errMsg.value = ''
  if (!/^\d{6}$/.test(code.value)) {
    errMsg.value = '验证码为 6 位数字'
    return
  }
  step.value = 3
}

async function step3Submit() {
  errMsg.value = ''
  if (newPassword.value.length < 8 || newPassword.value.length > 32) {
    errMsg.value = '密码长度需为 8-32 字符'
    return
  }
  if (!/[A-Za-z]/.test(newPassword.value) || !/\d/.test(newPassword.value)) {
    errMsg.value = '密码须同时包含字母和数字'
    return
  }
  if (newPassword.value !== confirmPassword.value) {
    errMsg.value = '两次密码不一致'
    return
  }
  submitting.value = true
  try {
    await axios.post('/api/auth/reset-password', {
      username: username.value.trim(),
      code: code.value,
      new_password: newPassword.value,
    })
    step.value = 4
  } catch (e) {
    const status = (e as { response?: { status?: number } })?.response?.status
    if (status === 404 || status === 405) {
      toast({
        description: '后端重置密码接口尚未开放，本次仅作流程演示。请联系管理员手动重置',
        variant: 'warning',
        duration: 8000,
      })
      step.value = 4
    } else {
      const msg = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail
      errMsg.value = msg ?? '重置失败，请稍后再试'
    }
  } finally {
    submitting.value = false
  }
}

const passwordStrength = computed(() => {
  const v = newPassword.value
  if (v.length === 0) return { level: 0, label: '' }
  let score = 0
  if (v.length >= 8) score++
  if (v.length >= 12) score++
  if (/[A-Z]/.test(v) && /[a-z]/.test(v)) score++
  if (/\d/.test(v)) score++
  if (/[^A-Za-z0-9]/.test(v)) score++
  if (score <= 2) return { level: 1, label: '弱', color: 'bg-danger' }
  if (score === 3) return { level: 2, label: '中', color: 'bg-warning' }
  return { level: 3, label: '强', color: 'bg-success' }
})

function backToLogin() {
  router.push('/login')
}
</script>

<template>
  <div class="min-h-screen grid grid-cols-1 lg:grid-cols-[1fr_560px]">
    <!-- Left brand panel -->
    <aside class="bg-primary text-white p-16 flex flex-col justify-between gap-10 max-lg:min-h-[200px] max-lg:p-8">
      <div class="flex items-center gap-3.5">
        <div class="grid w-[38px] h-[38px] place-items-center rounded-sm bg-white text-lg font-bold text-primary">
          训
        </div>
        <div class="flex flex-col gap-0.5">
          <div class="text-[15px] font-semibold leading-tight">实训评价管理系统</div>
          <div class="text-[11px] tracking-[1px] text-primary-foreground/70">找回密码 · Reset Password</div>
        </div>
      </div>

      <div>
        <h1 class="m-0 text-[42px] font-bold leading-[1.3]">
          账户安全 ·<br />
          找回您的访问权限
        </h1>
        <p class="mt-[18px] mb-0 text-sm leading-[1.8] max-w-[480px] text-primary-foreground/70">
          我们将通过你绑定的学校邮箱发送一次性验证码。完成校验后即可设置新密码。
        </p>
      </div>

      <div class="bg-white/5 border border-white/10 rounded-md p-[18px] flex flex-col gap-2.5">
        <div class="flex items-center gap-2 text-white text-[13px] font-semibold">
          <Shield class="w-3.5 h-3.5" />
          <span>密码安全要求</span>
        </div>
        <div class="text-xs leading-[1.7] text-primary-foreground/70">
          长度 8 - 32 字符 · 至少包含字母与数字两种类型 · 不可与历史密码重复
        </div>
      </div>

      <div class="text-xs text-primary-foreground/60">© 2026 软件学院 · 教学信息化中心</div>
    </aside>

    <!-- Right form panel -->
    <main class="bg-card p-16 flex flex-col justify-center gap-6 max-lg:p-8">
      <button class="inline-flex items-center gap-1.5 text-xs font-medium text-muted-foreground hover:text-ink w-fit transition-colors" @click="backToLogin">
        <ArrowLeft class="w-3.5 h-3.5" />
        <span>返回登录</span>
      </button>

      <div>
        <h2 class="m-0 mb-2 text-2xl font-bold text-ink">重置密码</h2>
        <p class="m-0 text-[13px] text-muted-foreground">
          {{ step < 4 ? '按以下三步完成密码重置' : '密码已重置，请使用新密码登录' }}
        </p>
      </div>

      <!-- Step indicator -->
      <div v-if="step < 4" class="bg-surface-2 rounded-md p-3.5 flex items-center justify-center gap-0">
        <div
          v-for="(s, idx) in [
            { num: 1, label: '验证身份' },
            { num: 2, label: '输入验证码' },
            { num: 3, label: '设置新密码' },
          ]"
          :key="s.num"
          class="flex items-center"
        >
          <div class="flex items-center gap-1.5">
            <div
              class="grid w-6 h-6 place-items-center rounded-pill text-[11px] font-semibold transition-colors"
              :class="step >= s.num
                ? 'bg-primary text-primary-foreground'
                : 'bg-card border border-border-strong text-muted-foreground'"
            >
              <CircleCheck v-if="step > s.num" class="w-3.5 h-3.5" />
              <span v-else>{{ s.num }}</span>
            </div>
            <div
              class="text-xs font-medium transition-colors"
              :class="step === s.num ? 'text-primary font-semibold' : step > s.num ? 'text-foreground' : 'text-muted-foreground'"
            >
              {{ s.label }}
            </div>
          </div>
          <div
            v-if="idx < 2"
            class="w-12 h-px mx-3 transition-colors"
            :class="step > s.num ? 'bg-primary' : 'bg-border'"
          ></div>
        </div>
      </div>

      <!-- Step 1: Identity -->
      <form v-if="step === 1" class="flex flex-col gap-4 anim-in" @submit.prevent="step1Submit">
        <div class="space-y-2">
          <Label>账号</Label>
          <div class="flex items-center gap-2.5 h-11 px-3.5 border border-border-strong rounded-md bg-card focus-within:border-primary">
            <User class="w-3.5 h-3.5 text-muted-foreground" />
            <input
              v-model="username"
              class="flex-1 border-0 outline-none bg-transparent text-[13px] text-ink placeholder:text-subtle-foreground"
              placeholder="请输入工号 / 学号"
              autocomplete="username"
            />
          </div>
        </div>
        <p v-if="errMsg" class="text-xs text-danger anim-in">{{ errMsg }}</p>
        <Button type="submit" :disabled="submitting" class="w-full h-11">
          {{ submitting ? '发送中...' : '发送验证码' }}
        </Button>
      </form>

      <!-- Step 2: Code -->
      <form v-else-if="step === 2" class="flex flex-col gap-4 anim-in" @submit.prevent="step2Submit">
        <div class="space-y-2">
          <Label>绑定邮箱</Label>
          <div class="flex items-center gap-2.5 h-11 px-3.5 border border-border-strong rounded-md bg-surface-2">
            <Mail class="w-3.5 h-3.5 text-muted-foreground" />
            <input
              :value="email || '已自动匹配学校邮箱'"
              class="flex-1 border-0 outline-none bg-transparent text-[13px] text-ink font-mono"
              readonly
            />
          </div>
          <p class="text-[11px] text-muted-foreground">验证码已发送至此邮箱（演示模式：任意 6 位数字均可）</p>
        </div>

        <div class="space-y-2">
          <Label>验证码</Label>
          <div class="flex items-center gap-2.5 h-11 px-3.5 border border-border-strong rounded-md bg-card focus-within:border-primary">
            <KeyRound class="w-3.5 h-3.5 text-muted-foreground" />
            <input
              v-model="code"
              class="flex-1 border-0 outline-none bg-transparent text-base text-ink font-mono tracking-[6px]"
              placeholder="• • • • • •"
              maxlength="6"
              inputmode="numeric"
              autocomplete="one-time-code"
            />
          </div>
        </div>

        <p v-if="errMsg" class="text-xs text-danger anim-in">{{ errMsg }}</p>
        <div class="flex gap-3">
          <Button type="button" variant="outline" class="flex-1 h-11" @click="step = 1">上一步</Button>
          <Button type="submit" class="flex-1 h-11">下一步</Button>
        </div>
      </form>

      <!-- Step 3: New password -->
      <form v-else-if="step === 3" class="flex flex-col gap-4 anim-in" @submit.prevent="step3Submit">
        <div class="space-y-2">
          <Label>新密码</Label>
          <div class="flex items-center gap-2.5 h-11 px-3.5 border border-border-strong rounded-md bg-card focus-within:border-primary">
            <Lock class="w-3.5 h-3.5 text-muted-foreground" />
            <input
              v-model="newPassword"
              type="password"
              class="flex-1 border-0 outline-none bg-transparent text-[13px] text-ink"
              placeholder="8-32 字符，含字母和数字"
              autocomplete="new-password"
            />
          </div>
          <div v-if="newPassword" class="flex items-center gap-2 mt-1">
            <div class="flex-1 h-1 bg-muted rounded-pill overflow-hidden">
              <div
                class="h-full rounded-pill transition-all duration-300"
                :class="passwordStrength.color"
                :style="{ width: (passwordStrength.level / 3) * 100 + '%' }"
              />
            </div>
            <span class="text-[11px] font-mono text-muted-foreground">{{ passwordStrength.label }}</span>
          </div>
        </div>

        <div class="space-y-2">
          <Label>确认新密码</Label>
          <div class="flex items-center gap-2.5 h-11 px-3.5 border border-border-strong rounded-md bg-card focus-within:border-primary">
            <Lock class="w-3.5 h-3.5 text-muted-foreground" />
            <input
              v-model="confirmPassword"
              type="password"
              class="flex-1 border-0 outline-none bg-transparent text-[13px] text-ink"
              placeholder="再次输入新密码"
              autocomplete="new-password"
            />
          </div>
        </div>

        <p v-if="errMsg" class="text-xs text-danger anim-in">{{ errMsg }}</p>
        <div class="flex gap-3">
          <Button type="button" variant="outline" class="flex-1 h-11" @click="step = 2">上一步</Button>
          <Button type="submit" :disabled="submitting" class="flex-1 h-11">
            {{ submitting ? '提交中...' : '完成重置' }}
          </Button>
        </div>
      </form>

      <!-- Step 4: Success -->
      <div v-else class="flex flex-col items-center gap-4 anim-in py-8">
        <div class="w-16 h-16 bg-success-soft text-success rounded-full grid place-items-center">
          <CircleCheck class="w-8 h-8" />
        </div>
        <h3 class="text-lg font-bold text-ink m-0">密码重置成功</h3>
        <p class="text-sm text-muted-foreground m-0 text-center">请使用新密码重新登录系统</p>
        <Button class="w-full h-11" @click="backToLogin">返回登录页</Button>
      </div>

      <div v-if="step < 4" class="flex items-center justify-center gap-1.5 py-2 text-xs text-muted-foreground">
        <LifeBuoy class="w-3.5 h-3.5" />
        <span>无法收到邮件？联系教务处 010-****-2347</span>
      </div>
    </main>
  </div>
</template>
