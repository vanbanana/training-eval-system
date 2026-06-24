<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { avatarInitial } from '@/lib/utils'
import { useRouter } from 'vue-router'
import axios from 'axios'
import AppShell from '@/components/layout/AppShell.vue'
import BreadcrumbNav from '@/components/business/BreadcrumbNav.vue'
import EmptyState from '@/components/business/EmptyState.vue'
import AnimatedNumber from '@/components/business/AnimatedNumber.vue'
import { useToast } from '@/components/ui/toast'
import { confirm } from '@/composables/useConfirm'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Avatar } from '@/components/ui/avatar'
import { Badge } from '@/components/ui/badge'
import { Checkbox } from '@/components/ui/checkbox'
import { Skeleton } from '@/components/ui/skeleton'
import { Card, CardContent } from '@/components/ui/card'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  Search,
  Users,
  GraduationCap,
  BookOpen,
  Lock,
  ShieldAlert,
  TrendingUp,
  Upload as UploadIcon,
  Download,
  Plus,
  MoreHorizontal,
  History,
  KeyRound,
  Pencil,
  Power,
  ChevronLeft,
  ChevronRight,
} from 'lucide-vue-next'

interface User {
  id: number
  username: string
  display_name: string
  role: string
  is_active: boolean
  last_login_at: string | null
}

const { toast } = useToast()
const router = useRouter()

const users = ref<User[]>([])
const loading = ref(true)
const searchQuery = ref('')
const filterRole = ref<'all' | 'teacher' | 'student' | 'admin' | 'disabled'>('all')
const selected = ref<Set<number>>(new Set())

// Pagination
const pageSize = 10
const currentPage = ref(1)
const totalItems = computed(() => filtered.value.length)
const totalPages = computed(() => Math.max(1, Math.ceil(totalItems.value / pageSize)))
const paged = computed(() => {
  const start = (currentPage.value - 1) * pageSize
  return filtered.value.slice(start, start + pageSize)
})

// Create / Edit user modal
const showUserModal = ref(false)
const editingUser = ref<User | null>(null)
const userForm = ref({
  username: '',
  display_name: '',
  role: 'student' as 'student' | 'teacher' | 'admin',
  password: '',
})
const submittingUser = ref(false)
const formErrors = ref<Record<string, string>>({})

// Reset password modal
const showResetDialog = ref(false)
const resetTarget = ref<User | null>(null)
const newPassword = ref('')
const resetting = ref(false)
const resetError = ref('')

function goToImport() {
  router.push('/admin/users/import')
}

async function fetchUsers() {
  loading.value = true
  try {
    const { data } = await axios.get('/api/users')
    users.value = data
  } catch (e) {
    const msg = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    toast({ description: msg ?? '加载用户列表失败', variant: 'destructive' })
  } finally {
    loading.value = false
  }
}

onMounted(fetchUsers)

const counts = computed(() => ({
  all: users.value.length,
  teacher: users.value.filter((u) => u.role === 'teacher').length,
  student: users.value.filter((u) => u.role === 'student').length,
  admin: users.value.filter((u) => u.role === 'admin').length,
  disabled: users.value.filter((u) => !u.is_active).length,
}))

const filtered = computed(() => {
  let list = users.value
  if (filterRole.value === 'disabled') {
    list = list.filter((u) => !u.is_active)
  } else if (filterRole.value !== 'all') {
    list = list.filter((u) => u.role === filterRole.value)
  }
  if (searchQuery.value.trim()) {
    const q = searchQuery.value.trim().toLowerCase()
    list = list.filter(
      (u) =>
        u.username.toLowerCase().includes(q) ||
        u.display_name.toLowerCase().includes(q),
    )
  }
  return list
})

const allSelected = computed({
  get: () =>
    filtered.value.length > 0 &&
    filtered.value.every((u) => selected.value.has(u.id)),
  set: (v: boolean) => {
    if (v) {
      filtered.value.forEach((u) => selected.value.add(u.id))
    } else {
      filtered.value.forEach((u) => selected.value.delete(u.id))
    }
    selected.value = new Set(selected.value)
  },
})

const someSelected = computed(
  () =>
    filtered.value.some((u) => selected.value.has(u.id)) &&
    !allSelected.value,
)

function toggleRow(id: number, v: boolean) {
  if (v) selected.value.add(id)
  else selected.value.delete(id)
  selected.value = new Set(selected.value)
}

function roleLabel(r: string) {
  return { admin: '管理员', teacher: '教师', student: '学生' }[r] ?? r
}

function roleBadgeVariant(r: string) {
  return ({
    admin: 'destructive',
    teacher: 'info',
    student: 'success',
  } as const)[r] ?? 'secondary'
}

function avatarChar(name: string) {
  return avatarInitial(name)
}

function formatLastLogin(iso: string | null) {
  if (!iso) return '从未登录'
  const date = new Date(iso)
  const now = new Date()
  const diff = now.getTime() - date.getTime()
  const diffDay = Math.floor(diff / (1000 * 60 * 60 * 24))
  if (diffDay === 0) return `今天 ${iso.slice(11, 16)}`
  if (diffDay === 1) return '昨天'
  if (diffDay < 7) return `${diffDay} 天前`
  return iso.slice(0, 10)
}

async function toggleActive(user: User) {
  const action = user.is_active ? '禁用' : '启用'
  const ok = await confirm({
    title: `${action}用户`,
    description: `确定${action}用户 "${user.display_name}"？`,
    variant: user.is_active ? 'destructive' : 'default',
    confirmText: action,
  })
  if (!ok) return
  try {
    const { data } = await axios.patch(`/api/users/${user.id}/toggle-active`)
    user.is_active = data.is_active
    toast({ description: `已${action}用户`, variant: 'success' })
  } catch (e) {
    const msg = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    toast({ description: msg ?? '操作失败', variant: 'destructive' })
  }
}

function openCreateModal() {
  editingUser.value = null
  userForm.value = { username: '', display_name: '', role: 'student', password: '' }
  formErrors.value = {}
  showUserModal.value = true
}

function openEditModal(u: User) {
  editingUser.value = u
  userForm.value = {
    username: u.username,
    display_name: u.display_name,
    role: u.role as 'student' | 'teacher' | 'admin',
    password: '',
  }
  formErrors.value = {}
  showUserModal.value = true
}

function validateForm() {
  formErrors.value = {}
  if (editingUser.value === null) {
    if (!userForm.value.username.trim() || userForm.value.username.length < 2) {
      formErrors.value.username = '账号至少 2 个字符'
    }
  }
  if (!userForm.value.display_name.trim()) {
    formErrors.value.display_name = '姓名必填'
  }
  if (editingUser.value === null) {
    if (!userForm.value.password || userForm.value.password.length < 8) {
      formErrors.value.password = '密码至少 8 位'
    }
  }
  return Object.keys(formErrors.value).length === 0
}

async function submitUserForm() {
  if (!validateForm()) return
  submittingUser.value = true
  try {
    if (editingUser.value === null) {
      await axios.post('/api/users', userForm.value)
      toast({
        description: `用户 ${userForm.value.username} 创建成功`,
        variant: 'success',
      })
    } else {
      // 后端 PATCH 支持 display_name / role / is_active
      await axios.patch(`/api/users/${editingUser.value.id}`, {
        display_name: userForm.value.display_name,
        role: userForm.value.role,
      })
      toast({ description: '用户信息已更新', variant: 'success' })
    }
    showUserModal.value = false
    await fetchUsers()
  } catch (e) {
    const msg = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    toast({ description: msg ?? '操作失败', variant: 'destructive' })
  } finally {
    submittingUser.value = false
  }
}

function openResetDialog(u: User) {
  resetTarget.value = u
  newPassword.value = ''
  resetError.value = ''
  showResetDialog.value = true
}

async function submitResetPassword() {
  if (!resetTarget.value) return
  resetError.value = ''
  if (!newPassword.value || newPassword.value.length < 8) {
    resetError.value = '密码至少 8 位'
    return
  }
  resetting.value = true
  try {
    await axios.post(`/api/users/${resetTarget.value.id}/reset-password`, {
      new_password: newPassword.value,
    })
    toast({ description: '密码已重置', variant: 'success' })
    showResetDialog.value = false
  } catch (e) {
    const msg = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    resetError.value = msg ?? '重置失败'
  } finally {
    resetting.value = false
  }
}

function viewLoginHistory(u: User) {
  router.push({ path: '/admin/audit', query: { user_id: u.id, action: 'auth.login' } })
}

function downloadTemplate() {
  window.location.href = '/api/imports/template/user.xlsx'
  toast({ description: '模板下载已开始', variant: 'info' })
}
</script>

<template>
  <AppShell>
    <BreadcrumbNav
      :items="[
        { label: '管理控制台', to: '/dashboard' },
        { label: '组织' },
        { label: '用户与权限' },
      ]"
    />

    <!-- Page Header -->
    <div class="tes-page-header">
      <div class="min-w-0">
        <h1 class="tes-clamp-title text-2xl font-bold text-ink">用户与权限</h1>
        <p class="mt-1.5 text-sm text-muted-foreground">
          管理学院教师、学生与管理员账号 · 支持批量导入与角色调整
        </p>
      </div>
      <div class="tes-page-actions">
        <Button variant="outline" @click="downloadTemplate">
          <Download class="w-4 h-4" />
          下载导入模板
        </Button>
        <Button variant="outline" @click="goToImport">
          <UploadIcon class="w-4 h-4" />
          批量导入
        </Button>
        <Button @click="openCreateModal">
          <Plus class="w-4 h-4" />
          新建用户
        </Button>
      </div>
    </div>

    <!-- KPI Cards -->
    <div class="tes-grid-kpi">
      <Card v-for="(kpi, i) in [
          { label: '系统总用户数', icon: Users, value: counts.all, hint: '含全部角色', positive: true },
          { label: '教师', icon: GraduationCap, value: counts.teacher, hint: '教师角色总数', positive: false },
          { label: '学生', icon: BookOpen, value: counts.student, hint: '在册学生', positive: false },
          { label: '已禁用账号', icon: Lock, value: counts.disabled, hint: '需关注', positive: false, warn: counts.disabled > 0 },
        ]" :key="kpi.label" class="tes-card-container anim-in" :style="{ animationDelay: i * 50 + 'ms' }">
        <CardContent class="p-5 flex flex-col gap-2.5">
          <div class="flex justify-between items-center">
            <span class="text-xs font-medium tracking-wider text-muted-foreground">{{ kpi.label }}</span>
            <component :is="kpi.icon" class="w-4 h-4 text-subtle-foreground" />
          </div>
          <div class="text-3xl font-bold text-ink leading-none">
            <AnimatedNumber :value="kpi.value" />
          </div>
          <div
            class="flex items-center gap-1.5 text-xs font-medium"
            :class="kpi.warn ? 'text-accent' : kpi.positive ? 'text-success' : 'text-muted-foreground'"
          >
            <TrendingUp v-if="kpi.positive" class="w-3.5 h-3.5" />
            <ShieldAlert v-else-if="kpi.warn" class="w-3.5 h-3.5" />
            <span>{{ kpi.hint }}</span>
          </div>
        </CardContent>
      </Card>
    </div>

    <!-- Table Card -->
    <Card class="tes-card-container overflow-hidden">
      <!-- Tabs -->
      <Tabs v-model="filterRole" class="px-2 pt-2 border-b border-border">
        <TabsList>
          <TabsTrigger value="all">全部 {{ counts.all }}</TabsTrigger>
          <TabsTrigger value="teacher">教师 {{ counts.teacher }}</TabsTrigger>
          <TabsTrigger value="student">学生 {{ counts.student }}</TabsTrigger>
          <TabsTrigger value="admin">管理员 {{ counts.admin }}</TabsTrigger>
          <TabsTrigger v-if="counts.disabled > 0" value="disabled" class="data-[state=active]:bg-danger data-[state=active]:text-destructive-foreground">
            已禁用 {{ counts.disabled }}
          </TabsTrigger>
        </TabsList>
      </Tabs>

      <!-- Toolbar -->
      <div class="px-6 py-3.5 bg-surface-2 border-b border-border flex flex-wrap items-center gap-3">
        <div class="relative w-full sm:w-[300px]">
          <Search class="w-3.5 h-3.5 text-muted-foreground absolute left-3 top-1/2 -translate-y-1/2" />
          <Input v-model="searchQuery" type="text" placeholder="搜索账号 / 姓名" class="pl-9" />
        </div>
        <span v-if="selected.size > 0" class="text-xs text-primary font-medium">
          已选 {{ selected.size }} 项
        </span>
        <div class="hidden sm:block flex-1"></div>
        <span class="text-xs text-muted-foreground">显示 {{ filtered.length }} 条</span>
      </div>

      <!-- Table -->
      <div class="tes-table-shell">
      <div class="grid min-w-[1060px] grid-cols-[40px_260px_140px_200px_160px_120px_140px] items-center px-6 py-3 bg-surface-2 border-b border-border">
        <Checkbox
          :model-value="allSelected ? true : someSelected ? 'indeterminate' : false"
          @update:model-value="(v) => allSelected = v === true"
          aria-label="全选"
        />
        <div class="text-[11px] font-semibold tracking-wider text-muted-foreground">用户</div>
        <div class="text-[11px] font-semibold tracking-wider text-muted-foreground">角色</div>
        <div class="text-[11px] font-semibold tracking-wider text-muted-foreground">账号</div>
        <div class="text-[11px] font-semibold tracking-wider text-muted-foreground">上次登录</div>
        <div class="text-[11px] font-semibold tracking-wider text-muted-foreground">状态</div>
        <div class="text-[11px] font-semibold tracking-wider text-muted-foreground text-right">操作</div>
      </div>

      <!-- Loading -->
      <template v-if="loading">
        <div
          v-for="n in 5"
          :key="n"
          class="grid min-w-[1060px] grid-cols-[40px_260px_140px_200px_160px_120px_140px] items-center px-6 py-3.5 border-b border-border"
        >
          <Skeleton class="h-4 w-4" />
          <Skeleton class="h-8 w-3/4" />
          <Skeleton class="h-5 w-16" />
          <Skeleton class="h-4 w-20" />
          <Skeleton class="h-4 w-20" />
          <Skeleton class="h-5 w-12" />
          <Skeleton class="h-4 w-16 ml-auto" />
        </div>
      </template>

      <!-- Empty -->
      <EmptyState
        v-else-if="filtered.length === 0"
        title="无符合条件的用户"
        description="尝试调整筛选条件，或新建一个用户"
        action-label="新建用户"
        @action="openCreateModal"
      />

      <!-- Rows -->
      <div
        v-for="(u, idx) in paged"
        v-else
        :key="u.id"
        class="grid min-w-[1060px] grid-cols-[40px_260px_140px_200px_160px_120px_140px] items-center px-6 py-3.5 border-b border-border last:border-b-0 transition-colors hover:bg-surface-2 anim-in"
        :class="!u.is_active ? 'opacity-60' : ''"
        :style="{ animationDelay: Math.min(idx * 20, 200) + 'ms' }"
      >
        <Checkbox
          :model-value="selected.has(u.id)"
          @update:model-value="(v) => toggleRow(u.id, v === true)"
          :aria-label="`选择用户 ${u.display_name}`"
        />
        <div class="flex items-center gap-2.5">
          <Avatar
            size="sm"
            :class="u.role === 'admin' ? 'bg-danger-soft !text-danger' : ''"
          >
            {{ avatarChar(u.display_name) }}
          </Avatar>
          <div class="flex flex-col gap-0.5 min-w-0">
            <span class="text-sm font-semibold text-ink truncate">{{ u.display_name }}</span>
            <span class="text-[11px] text-muted-foreground truncate">ID #{{ u.id }}</span>
          </div>
        </div>
        <div>
          <Badge :variant="roleBadgeVariant(u.role)">{{ roleLabel(u.role) }}</Badge>
        </div>
        <div class="font-mono text-xs text-foreground break-all">{{ u.username }}</div>
        <div class="font-mono text-xs text-muted-foreground">{{ formatLastLogin(u.last_login_at) }}</div>
        <div>
          <Badge :variant="u.is_active ? 'success' : 'secondary'">
            {{ u.is_active ? '启用' : '已禁用' }}
          </Badge>
        </div>
        <div class="flex items-center justify-end gap-1.5">
          <Button variant="ghost" size="sm" class="h-7 px-2 text-primary" @click="openEditModal(u)">编辑</Button>
          <Button
            variant="ghost"
            size="sm"
            class="h-7 px-2"
            :class="u.is_active ? 'text-danger hover:text-danger' : 'text-success hover:text-success'"
            @click="toggleActive(u)"
          >
            {{ u.is_active ? '禁用' : '启用' }}
          </Button>
          <DropdownMenu>
            <DropdownMenuTrigger as-child>
              <Button variant="ghost" size="icon-sm" aria-label="更多操作">
                <MoreHorizontal class="w-3.5 h-3.5" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" class="w-48">
              <DropdownMenuItem @select="openEditModal(u)">
                <Pencil class="text-muted-foreground" />
                编辑信息
              </DropdownMenuItem>
              <DropdownMenuItem @select="openResetDialog(u)">
                <KeyRound class="text-muted-foreground" />
                重置密码
              </DropdownMenuItem>
              <DropdownMenuItem @select="viewLoginHistory(u)">
                <History class="text-muted-foreground" />
                查看登录历史
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem
                :class="u.is_active ? 'text-danger focus:bg-danger-soft focus:text-danger' : 'text-success focus:bg-success-soft focus:text-success'"
                @select="toggleActive(u)"
              >
                <Power class="text-current" />
                {{ u.is_active ? '禁用账号' : '启用账号' }}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>

      <!-- Footer -->
      </div>

      <div class="flex flex-wrap justify-between items-center gap-3 px-6 py-4 bg-surface-2 border-t border-border">
          <div class="text-xs text-muted-foreground">
            显示 {{ filtered.length > 0 ? (currentPage - 1) * pageSize + 1 : 0 }} - {{ Math.min(currentPage * pageSize, totalItems) }} 共 {{ totalItems }} 条
          </div>
          <div v-if="totalItems > pageSize" class="flex items-center gap-1.5">
            <Button variant="outline" size="icon-sm" :disabled="currentPage <= 1" @click="currentPage--">
              <ChevronLeft class="w-3.5 h-3.5" />
            </Button>
            <Button
              v-for="page in totalPages"
              :key="page"
              :variant="page === currentPage ? 'default' : 'outline'"
              size="sm"
              class="h-8 min-w-[32px]"
              @click="currentPage = page"
            >
              {{ page }}
            </Button>
            <Button variant="outline" size="icon-sm" :disabled="currentPage >= totalPages" @click="currentPage++">
              <ChevronRight class="w-3.5 h-3.5" />
            </Button>
          </div>
        </div>
    </Card>

    <!-- Create/Edit User Dialog -->
    <Dialog v-model:open="showUserModal">
      <DialogContent class="max-w-md">
        <DialogHeader>
          <DialogTitle>
            {{ editingUser === null ? '新建用户' : `编辑用户 #${editingUser.id}` }}
          </DialogTitle>
          <DialogDescription v-if="editingUser !== null">
            修改基础信息（账号一旦创建不可修改；密码使用「重置密码」单独操作）
          </DialogDescription>
        </DialogHeader>
        <div class="flex flex-col gap-4">
          <div class="space-y-2">
            <Label>账号 <span class="text-danger">*</span></Label>
            <Input
              v-model="userForm.username"
              type="text"
              placeholder="登录用户名（2-64 个字符）"
              :disabled="editingUser !== null"
            />
            <p v-if="formErrors.username" class="text-xs text-danger">{{ formErrors.username }}</p>
            <p v-else-if="editingUser !== null" class="text-[11px] text-muted-foreground">账号创建后不可修改</p>
          </div>
          <div class="space-y-2">
            <Label>姓名 <span class="text-danger">*</span></Label>
            <Input v-model="userForm.display_name" type="text" placeholder="显示名称" />
            <p v-if="formErrors.display_name" class="text-xs text-danger">{{ formErrors.display_name }}</p>
          </div>
          <div class="space-y-2">
            <Label>角色 <span class="text-danger">*</span></Label>
            <Select v-model="userForm.role">
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="student">学生</SelectItem>
                <SelectItem value="teacher">教师</SelectItem>
                <SelectItem value="admin">管理员</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div v-if="editingUser === null" class="space-y-2">
            <Label>初始密码 <span class="text-danger">*</span></Label>
            <Input v-model="userForm.password" type="text" placeholder="至少 8 位" class="font-mono" />
            <p v-if="formErrors.password" class="text-xs text-danger">{{ formErrors.password }}</p>
            <p v-else class="text-[11px] text-muted-foreground">用户首次登录后建议修改密码</p>
          </div>
        </div>
        <DialogFooter class="gap-2">
          <Button variant="outline" @click="showUserModal = false">取消</Button>
          <Button :disabled="submittingUser" @click="submitUserForm">
            {{ submittingUser ? '提交中...' : (editingUser === null ? '确认创建' : '保存修改') }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- Reset Password Dialog -->
    <Dialog v-model:open="showResetDialog">
      <DialogContent class="max-w-md">
        <DialogHeader>
          <DialogTitle>重置密码</DialogTitle>
          <DialogDescription v-if="resetTarget">
            为用户 <span class="font-semibold text-ink">{{ resetTarget.display_name }}</span>（{{ resetTarget.username }}）设置新密码
          </DialogDescription>
        </DialogHeader>
        <div class="space-y-2">
          <Label>新密码</Label>
          <Input v-model="newPassword" type="text" placeholder="至少 8 位" class="font-mono" />
          <p v-if="resetError" class="text-xs text-danger">{{ resetError }}</p>
          <p v-else class="text-[11px] text-muted-foreground">建议线下告知用户后立即要求修改</p>
        </div>
        <DialogFooter class="gap-2">
          <Button variant="outline" @click="showResetDialog = false">取消</Button>
          <Button variant="destructive" :disabled="resetting" @click="submitResetPassword">
            {{ resetting ? '提交中...' : '确认重置' }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </AppShell>
</template>
