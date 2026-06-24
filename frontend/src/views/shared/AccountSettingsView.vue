<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { avatarInitial } from '@/lib/utils'
import { useRouter } from 'vue-router'
import axios from 'axios'
import AppShell from '@/components/layout/AppShell.vue'
import BreadcrumbNav from '@/components/business/BreadcrumbNav.vue'
import { useToast } from '@/components/ui/toast'
import { useAuthStore } from '@/stores/auth'
import { useTheme } from '@/composables/useTheme'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Badge } from '@/components/ui/badge'
import { Avatar } from '@/components/ui/avatar'
import { Switch } from '@/components/ui/switch'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  User,
  Lock,
  Bell,
  Palette,
  Save,
  Sun,
  Moon,
  ShieldCheck,
} from 'lucide-vue-next'

interface Account {
  id: number
  username: string
  display_name: string
  role: string
  is_active: boolean
  email?: string
  last_login_at?: string
}

const auth = useAuthStore()
const router = useRouter()
const { toast } = useToast()
const colorMode = useTheme()

const account = ref<Account | null>(null)
const loading = ref(true)
const activeTab = ref<string>('profile')

// Profile form
const displayName = ref('')
const savingProfile = ref(false)

// Password form
const oldPwd = ref('')
const newPwd = ref('')
const confirmPwd = ref('')
const savingPwd = ref(false)
const pwdError = ref('')

// Notification preferences
const prefs = ref<Record<string, boolean>>({})
const savingPref = ref<string | null>(null)
const eventTypes = [
  { key: 'evaluation.scored', label: 'AI 评分完成', desc: '当 AI 完成你的提交评分时通知你' },
  { key: 'evaluation.confirmed', label: '教师确认评价', desc: '教师确认后第一时间收到通知' },
  { key: 'evaluation.rejected', label: '评价被打回', desc: '提交被打回时立即提醒重新提交' },
  { key: 'task.published', label: '新任务发布', desc: '所在班级有新任务时通知' },
  { key: 'similarity.suspect', label: '检测到相似度异常', desc: '相似度过高时教师/管理员收到告警' },
  { key: 'system.announcement', label: '系统公告', desc: '系统级公告与维护通知' },
]

async function fetchAccount() {
  loading.value = true
  try {
    const { data } = await axios.get('/api/account/me')
    account.value = data
    displayName.value = data.display_name
    // load prefs
    try {
      const { data: p } = await axios.get('/api/notifications/preferences')
      prefs.value = p
    } catch {
      /* ignore */
    }
  } catch (e) {
    const msg = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    toast({ description: msg ?? '加载账号信息失败', variant: 'destructive' })
  } finally {
    loading.value = false
  }
}

onMounted(fetchAccount)

async function saveProfile() {
  if (!displayName.value.trim()) {
    toast({ description: '显示名称不能为空', variant: 'destructive' })
    return
  }
  savingProfile.value = true
  try {
    await axios.patch('/api/account/profile', { display_name: displayName.value.trim() })
    toast({ description: '资料已保存', variant: 'success' })
    if (auth.user) auth.user.display_name = displayName.value.trim()
  } catch (e) {
    const msg = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    toast({ description: msg ?? '保存失败', variant: 'destructive' })
  } finally {
    savingProfile.value = false
  }
}

async function changePwd() {
  pwdError.value = ''
  if (!oldPwd.value || !newPwd.value) {
    pwdError.value = '请填写旧密码和新密码'
    return
  }
  if (newPwd.value.length < 8) {
    pwdError.value = '新密码至少 8 位'
    return
  }
  if (newPwd.value !== confirmPwd.value) {
    pwdError.value = '两次新密码不一致'
    return
  }
  savingPwd.value = true
  try {
    await axios.post('/api/account/change-password', {
      old_password: oldPwd.value,
      new_password: newPwd.value,
    })
    toast({ description: '密码修改成功，请重新登录', variant: 'success' })
    oldPwd.value = ''
    newPwd.value = ''
    confirmPwd.value = ''
    setTimeout(() => {
      auth.logout()
      router.push('/login')
    }, 1500)
  } catch (e) {
    const msg = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    pwdError.value = msg ?? '修改失败'
  } finally {
    savingPwd.value = false
  }
}

async function setPref(eventType: string, enabled: boolean) {
  savingPref.value = eventType
  try {
    await axios.put('/api/notifications/preferences', { event_type: eventType, enabled })
    prefs.value[eventType] = enabled
    toast({ description: '已保存', variant: 'success' })
  } catch (e) {
    const msg = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    toast({ description: msg ?? '保存失败', variant: 'destructive' })
  } finally {
    savingPref.value = null
  }
}

function toggleTheme() {
  colorMode.value = colorMode.value === 'dark' ? 'light' : 'dark'
}

const passwordStrength = computed(() => {
  const v = newPwd.value
  if (!v) return { level: 0, label: '', color: '' }
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

const roleLabel = computed(
  () => ({ admin: '管理员', teacher: '教师', student: '学生' } as Record<string, string>)[account.value?.role ?? ''] ?? account.value?.role,
)
</script>

<template>
  <AppShell>
    <BreadcrumbNav
      :items="[
        { label: '工作台', to: '/dashboard' },
        { label: '账号设置' },
      ]"
    />

    <div class="tes-page-header">
      <div class="min-w-0">
        <h1 class="text-2xl font-bold text-ink">账号设置</h1>
        <p class="mt-1.5 text-sm text-muted-foreground">管理个人信息、密码、通知偏好和外观</p>
      </div>
    </div>

    <div v-if="loading" class="tes-grid-sidebar-main">
      <Skeleton class="h-64" />
      <Skeleton class="h-96" />
    </div>

    <template v-else-if="account">
      <Card class="overflow-hidden grid grid-cols-[minmax(13rem,14rem)_minmax(0,1fr)] max-lg:grid-cols-1">
        <!-- Side nav -->
        <aside class="bg-surface-2 border-r border-border p-4 max-lg:border-r-0 max-lg:border-b">
          <div class="flex items-center gap-3 px-2 py-3 mb-2">
            <Avatar size="lg">{{ avatarInitial(account.display_name) }}</Avatar>
            <div class="min-w-0">
              <div class="text-sm font-semibold text-ink truncate">{{ account.display_name }}</div>
              <div class="text-[11px] text-muted-foreground font-mono truncate">{{ account.username }}</div>
              <Badge variant="info" class="mt-1 text-[10px]">{{ roleLabel }}</Badge>
            </div>
          </div>

          <Tabs v-model="activeTab" orientation="vertical">
            <TabsList class="flex-col h-auto items-start !bg-transparent !p-0 !border-0 gap-1 max-lg:flex-row max-lg:overflow-x-auto max-lg:pb-1 max-lg:[&>*]:shrink-0">
              <TabsTrigger value="profile" class="justify-start">
                <User class="w-3.5 h-3.5" />
                个人资料
              </TabsTrigger>
              <TabsTrigger value="security" class="justify-start">
                <Lock class="w-3.5 h-3.5" />
                修改密码
              </TabsTrigger>
              <TabsTrigger value="notifications" class="justify-start">
                <Bell class="w-3.5 h-3.5" />
                通知偏好
              </TabsTrigger>
              <TabsTrigger value="appearance" class="justify-start">
                <Palette class="w-3.5 h-3.5" />
                外观
              </TabsTrigger>
            </TabsList>
          </Tabs>
        </aside>

        <!-- Content -->
        <div class="min-w-0 p-6 max-sm:p-4">
          <!-- Profile -->
          <section v-if="activeTab === 'profile'" class="flex flex-col gap-5 max-w-lg">
            <div>
              <h3 class="text-base font-semibold text-ink">个人资料</h3>
              <p class="text-xs text-muted-foreground mt-1">基本账号信息</p>
            </div>

            <div class="space-y-2">
              <Label>账号</Label>
              <div class="h-9 px-3 flex items-center bg-surface-2 border border-border rounded-md text-sm text-muted-foreground font-mono">
                {{ account.username }}
              </div>
            </div>

            <div class="space-y-2">
              <Label>角色</Label>
              <div class="h-9 px-3 flex items-center bg-surface-2 border border-border rounded-md text-sm text-muted-foreground">
                {{ roleLabel }}
              </div>
            </div>

            <div class="space-y-2">
              <Label>显示名称</Label>
              <Input v-model="displayName" placeholder="他人看到的显示名" />
            </div>

            <Button class="w-fit" :disabled="savingProfile" @click="saveProfile">
              <Save class="w-4 h-4" />
              {{ savingProfile ? '保存中...' : '保存修改' }}
            </Button>
          </section>

          <!-- Security -->
          <section v-else-if="activeTab === 'security'" class="flex flex-col gap-5 max-w-lg">
            <div>
              <h3 class="text-base font-semibold text-ink">修改密码</h3>
              <p class="text-xs text-muted-foreground mt-1">修改后将自动登出，请重新登录</p>
            </div>

            <div class="space-y-2">
              <Label>当前密码</Label>
              <Input v-model="oldPwd" type="password" autocomplete="current-password" />
            </div>

            <div class="space-y-2">
              <Label>新密码</Label>
              <Input v-model="newPwd" type="password" placeholder="至少 8 位，建议含字母 + 数字" autocomplete="new-password" />
              <div v-if="newPwd" class="flex items-center gap-2 mt-1">
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
              <Input v-model="confirmPwd" type="password" autocomplete="new-password" />
            </div>

            <p v-if="pwdError" class="text-xs text-danger">{{ pwdError }}</p>

            <Button class="w-fit" variant="destructive" :disabled="savingPwd" @click="changePwd">
              <ShieldCheck class="w-4 h-4" />
              {{ savingPwd ? '修改中...' : '修改密码' }}
            </Button>
          </section>

          <!-- Notifications -->
          <section v-else-if="activeTab === 'notifications'" class="flex flex-col gap-5 max-w-2xl">
            <div>
              <h3 class="text-base font-semibold text-ink">通知偏好</h3>
              <p class="text-xs text-muted-foreground mt-1">关闭后将不再生成对应类型的站内通知</p>
            </div>

            <div class="flex flex-col gap-3">
              <div
                v-for="evt in eventTypes"
                :key="evt.key"
                class="flex items-center justify-between gap-3 p-4 border border-border rounded-md transition-colors hover:bg-surface-2"
              >
                <div class="min-w-0 flex-1">
                  <div class="text-sm font-medium text-ink">{{ evt.label }}</div>
                  <div class="text-xs text-muted-foreground mt-0.5">{{ evt.desc }}</div>
                  <code class="text-[10px] text-subtle-foreground font-mono">{{ evt.key }}</code>
                </div>
                <Switch
                  :model-value="prefs[evt.key] !== false"
                  :disabled="savingPref === evt.key"
                  @update:model-value="(v) => setPref(evt.key, !!v)"
                />
              </div>
            </div>
          </section>

          <!-- Appearance -->
          <section v-else-if="activeTab === 'appearance'" class="flex flex-col gap-5 max-w-lg">
            <div>
              <h3 class="text-base font-semibold text-ink">外观</h3>
              <p class="text-xs text-muted-foreground mt-1">主题选择会同步到所有页面</p>
            </div>

            <div class="grid grid-cols-[repeat(auto-fit,minmax(min(100%,12rem),1fr))] gap-4">
              <button
                class="p-5 border-2 rounded-lg text-left transition-colors flex flex-col gap-2"
                :class="colorMode === 'light' ? 'border-primary bg-primary-soft' : 'border-border hover:border-border-strong'"
                @click="colorMode = 'light'"
              >
                <Sun class="w-5 h-5 text-warning" />
                <div class="text-sm font-semibold text-ink">浅色</div>
                <div class="text-xs text-muted-foreground">默认主题，明亮清晰</div>
              </button>
              <button
                class="p-5 border-2 rounded-lg text-left transition-colors flex flex-col gap-2"
                :class="colorMode === 'dark' ? 'border-primary bg-primary-soft' : 'border-border hover:border-border-strong'"
                @click="colorMode = 'dark'"
              >
                <Moon class="w-5 h-5 text-info" />
                <div class="text-sm font-semibold text-ink">深色</div>
                <div class="text-xs text-muted-foreground">适合夜间办公</div>
              </button>
            </div>

            <Button variant="outline" @click="toggleTheme">
              <component :is="colorMode === 'dark' ? Sun : Moon" class="w-4 h-4" />
              切换主题
            </Button>
          </section>
        </div>
      </Card>
    </template>
  </AppShell>
</template>
