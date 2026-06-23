<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter, RouterLink } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useNotifications } from '@/composables/useNotifications'
import { useTheme } from '@/composables/useTheme'
import {
  Search,
  Bell,
  ChevronDown,
  User,
  LogOut,
  Settings,
  Sun,
  Moon,
  CheckCheck,
  Home,
  BookOpen,
  ClipboardList,
  PenLine,
  Users,
  BarChart3,
  FileText,
  History,
  MessageSquare,
  LayoutDashboard,
  Cpu,
  Shield,
} from 'lucide-vue-next'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import { Avatar } from '@/components/ui/avatar'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Button } from '@/components/ui/button'
import GlobalSearch from '@/components/business/GlobalSearch.vue'

const auth = useAuthStore()
const router = useRouter()
const { items: notifications, unreadCount, markAsRead, markAllRead } = useNotifications()
const colorMode = useTheme()
const searchOpen = ref(false)

const navItems = computed(() => {
  switch (auth.user?.role) {
    case 'teacher':
      return [
        { label: '首页', to: '/dashboard', icon: 'Home' },
        { label: '实训任务', to: '/teacher/tasks', icon: 'ClipboardList' },
        { label: '班级管理', to: '/teacher/classes', icon: 'Users' },
        { label: '报表中心', to: '/teacher/reports', icon: 'BarChart3' },
        { label: '评价模板', to: '/templates', icon: 'FileText' },
        { label: '评价看板', to: '/profiles', icon: 'PenLine' },
      ]
    case 'student':
      return [
        { label: '工作台', to: '/dashboard', icon: 'Home' },
        { label: '我的任务', to: '/student/tasks', icon: 'FileText' },
        { label: '评价历史', to: '/student/history', icon: 'History' },
        { label: 'AI 问答', to: '/student/chat', icon: 'MessageSquare' },
        { label: '能力画像', to: '/student/profile', icon: 'User' },
        { label: '通知中心', to: '/notifications', icon: 'Bell' },
      ]
    case 'admin':
      return [
        { label: '总览', to: '/admin/dashboard', icon: 'LayoutDashboard' },
        { label: '用户管理', to: '/admin/users', icon: 'Users' },
        { label: '课程管理', to: '/admin/courses', icon: 'BookOpen' },
        { label: 'LLM 配置', to: '/admin/llm', icon: 'Cpu' },
        { label: '审计日志', to: '/admin/audit', icon: 'Shield' },
        { label: '通知中心', to: '/notifications', icon: 'Bell' },
      ]
    default:
      return [{ label: '工作台', to: '/dashboard', icon: 'Home' }]
  }
})

const isTeacher = computed(() => auth.user?.role === 'teacher')

const iconMap: Record<string, object> = {
  Home,
  ClipboardList,
  PenLine,
  Users,
  BarChart3,
  FileText,
  History,
  MessageSquare,
  Bell,
  LayoutDashboard,
  Cpu,
  Shield,
  BookOpen,
  User,
}

function logout() {
  auth.logout()
  router.push('/login')
}

function toggleTheme() {
  colorMode.value = colorMode.value === 'dark' ? 'light' : 'dark'
}

function openNotification(n: { id: number; link?: string }) {
  markAsRead(n.id)
  if (n.link) router.push(n.link)
  else router.push('/notifications')
}

function formatTime(s: string) {
  const d = new Date(s)
  const diff = Date.now() - d.getTime()
  if (diff < 60_000) return '刚刚'
  if (diff < 3_600_000) return Math.floor(diff / 60_000) + ' 分钟前'
  if (diff < 86_400_000) return Math.floor(diff / 3_600_000) + ' 小时前'
  return d.toLocaleDateString()
}

const userInitial = computed(() => auth.user?.display_name?.charAt(0) || 'U')
const roleLabel = computed(() => {
  const name = auth.user?.display_name ?? ''
  switch (auth.user?.role) {
    case 'teacher':
      return name.endsWith('老师') ? '' : '教师'
    case 'student':
      return name.endsWith('同学') ? '' : '同学'
    case 'admin':
      return name.endsWith('管理员') ? '' : '管理员'
    default:
      return ''
  }
})
</script>

<template>
  <header class="top-nav" :class="{ 'top-nav-capsule': isTeacher }">
    <!-- 上行：品牌 + 胶囊导航(教师) / 搜索 + 用户 -->
    <div class="top-nav-row">
      <div class="top-nav-brand flex min-w-0 items-center gap-3.5">
        <div
          class="w-8 h-8 bg-primary text-primary-foreground rounded-sm grid place-items-center font-bold text-[15px] shadow-sm"
        >
          训
        </div>
        <span class="truncate text-[15px] font-semibold text-ink">实训评价管理系统</span>
        <span class="hidden sm:block w-px h-[18px] bg-border"></span>
        <span class="hidden sm:inline-flex text-xs font-medium text-foreground px-2.5 py-1.5 bg-surface-2 border border-border rounded-md">
          软件学院
        </span>
      </div>

      <!-- 教师：胶囊式导航按钮组（居中） -->
      <nav v-if="isTeacher" class="capsule-nav">
        <RouterLink
          v-for="item in navItems"
          :key="item.to + item.label"
          :to="item.to"
          class="capsule-nav-btn"
          active-class="capsule-nav-btn-active"
        >
          <component
            :is="iconMap[item.icon]"
            class="w-3.5 h-3.5"
          />
          <span>{{ item.label }}</span>
        </RouterLink>
      </nav>

      <div class="top-nav-actions">
        <!-- 全局搜索 ⌘K -->
        <button
          class="flex items-center gap-2.5 h-9 px-3 bg-surface-2 border border-border rounded-md cursor-pointer transition-colors hover:border-border-strong"
          :class="isTeacher ? 'w-[min(14rem,28vw)]' : 'w-[min(19rem,32vw)]'"
          aria-label="全局搜索"
          @click="searchOpen = true"
        >
          <Search class="w-3.5 h-3.5 text-muted-foreground" />
          <span class="text-xs text-subtle-foreground flex-1 text-left truncate">搜索实训任务、学生、班级...</span>
          <kbd class="text-[10px] text-muted-foreground border border-border bg-card rounded-sm px-1.5 py-0.5 font-mono">⌘K</kbd>
        </button>

        <!-- 通知铃铛 -->
        <Popover>
          <PopoverTrigger as-child>
            <button
              class="relative w-8 h-8 border border-border rounded-full grid place-items-center text-muted-foreground transition-all hover:border-border-strong hover:text-ink hover:bg-surface-2 active:scale-95"
              aria-label="通知"
            >
              <Bell class="w-4 h-4" />
              <span
                v-if="unreadCount > 0"
                class="absolute -top-1 -right-1 min-w-[16px] h-4 px-1 bg-danger text-white text-[10px] rounded-pill flex items-center justify-center font-semibold animate-in zoom-in"
              >
                {{ unreadCount > 99 ? '99+' : unreadCount }}
              </span>
            </button>
          </PopoverTrigger>
          <PopoverContent align="end" class="w-[360px] p-0">
            <div class="flex items-center justify-between px-4 py-3 border-b border-border">
              <span class="text-sm font-semibold text-ink">通知</span>
              <button
                v-if="unreadCount > 0"
                class="text-xs text-primary hover:underline flex items-center gap-1"
                @click="markAllRead"
              >
                <CheckCheck class="w-3.5 h-3.5" />
                全部已读
              </button>
            </div>
            <ScrollArea class="max-h-[360px]">
              <div v-if="notifications.length === 0" class="px-6 py-8 text-center text-xs text-muted-foreground">
                暂无通知
              </div>
              <ul v-else class="divide-y divide-border">
                <li
                  v-for="n in notifications.slice(0, 8)"
                  :key="n.id"
                  class="px-4 py-3 cursor-pointer transition-colors hover:bg-surface-2"
                  @click="openNotification(n)"
                >
                  <div class="flex items-start gap-2">
                    <span
                      class="mt-1.5 w-1.5 h-1.5 rounded-full shrink-0"
                      :class="n.is_read ? 'bg-transparent' : 'bg-accent'"
                    ></span>
                    <div class="flex-1 min-w-0">
                      <p class="text-xs font-semibold text-ink truncate">{{ n.title }}</p>
                      <p v-if="n.content" class="text-xs text-muted-foreground line-clamp-2 mt-0.5">{{ n.content }}</p>
                      <p class="text-[10px] text-subtle-foreground mt-1">{{ formatTime(n.created_at) }}</p>
                    </div>
                  </div>
                </li>
              </ul>
            </ScrollArea>
            <div class="px-4 py-2.5 border-t border-border">
              <Button variant="ghost" size="sm" class="w-full" @click="router.push('/notifications')">查看全部通知</Button>
            </div>
          </PopoverContent>
        </Popover>

        <!-- 用户菜单 -->
        <DropdownMenu>
          <DropdownMenuTrigger as-child>
            <button
              class="flex items-center gap-2 pl-1 pr-3 py-1 bg-surface-2 border border-border rounded-pill transition-colors hover:bg-muted"
            >
              <Avatar size="sm">{{ userInitial }}</Avatar>
              <span class="hidden sm:inline text-xs font-medium text-ink">{{ auth.user?.display_name }}</span>
              <span class="hidden md:inline text-[10px] text-muted-foreground">{{ roleLabel }}</span>
              <ChevronDown class="w-3.5 h-3.5 text-muted-foreground" />
            </button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" class="w-56">
            <DropdownMenuLabel>
              <div class="flex flex-col">
                <span class="text-sm font-semibold text-ink">{{ auth.user?.display_name }}</span>
                <span class="text-xs text-muted-foreground font-normal">{{ auth.user?.username }} · {{ roleLabel }}</span>
              </div>
            </DropdownMenuLabel>
            <DropdownMenuSeparator />
            <DropdownMenuItem as-child>
              <RouterLink to="/account">
                <User class="text-muted-foreground" />
                <span>个人资料</span>
              </RouterLink>
            </DropdownMenuItem>
            <DropdownMenuItem as-child>
              <RouterLink to="/account">
                <Settings class="text-muted-foreground" />
                <span>账号设置</span>
              </RouterLink>
            </DropdownMenuItem>
            <DropdownMenuItem @select="toggleTheme">
              <Sun v-if="colorMode === 'dark'" class="text-muted-foreground" />
              <Moon v-else class="text-muted-foreground" />
              <span>{{ colorMode === 'dark' ? '切换浅色' : '切换暗色' }}</span>
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem class="text-danger focus:bg-danger-soft focus:text-danger" @select="logout">
              <LogOut class="text-danger" />
              <span>退出登录</span>
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </div>

    <!-- 下行：导航（非教师角色保留原有下划线导航） -->
    <nav v-if="!isTeacher" class="top-nav-tabs">
      <RouterLink
        v-for="item in navItems"
        :key="item.to + item.label"
        :to="item.to"
        class="h-11 flex items-center px-4 text-[13px] font-medium text-muted-foreground border-b-2 border-transparent -mb-px transition-colors duration-150 hover:text-ink"
        active-class="!text-primary !font-semibold nav-item-active"
      >
        {{ item.label }}
      </RouterLink>
    </nav>

    <!-- ⌘K 全局搜索 -->
    <GlobalSearch v-model:open="searchOpen" />
  </header>
</template>

<style scoped>
/* Base nav */
.top-nav {
  position: sticky;
  top: 0;
  z-index: 40;
  border-bottom: 1px solid hsl(var(--border));
  background: var(--glass-bg);
  backdrop-filter: blur(var(--glass-blur));
  -webkit-backdrop-filter: blur(var(--glass-blur));
  box-shadow: var(--shadow-sm);
}

.top-nav-row {
  min-height: 5rem;
  padding: 1rem 1.25rem;
  display: grid;
  grid-template-columns: minmax(12rem, auto) minmax(0, 1fr) auto;
  align-items: center;
  gap: 0.875rem;
}

.top-nav-actions {
  grid-column: 3;
  min-width: 0;
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.75rem;
}

.top-nav-capsule .top-nav-row {
  grid-template-columns: minmax(0, 1fr) auto minmax(0, 1fr);
}

.top-nav-capsule .top-nav-brand {
  grid-column: 1;
  justify-self: start;
}

.top-nav-capsule .capsule-nav {
  grid-column: 2;
  justify-self: center;
}

.top-nav-capsule .top-nav-actions {
  grid-column: 3;
  justify-self: end;
}

/* Teacher capsule mode: single row, no bottom border nav */
.top-nav-capsule {
  border-bottom: none;
  background: hsl(var(--background));
  backdrop-filter: none;
  box-shadow: none;
}

/* Glassmorphism 降级 */
@supports not (backdrop-filter: blur(1px)) {
  .top-nav:not(.top-nav-capsule) {
    background: hsl(var(--card)) !important;
  }
}

/* 活跃导航项渐变下划线（非教师） */
.nav-item-active {
  background: hsl(var(--primary) / 0.06);
  border-bottom: 2px solid transparent;
  border-image: var(--gradient-primary) 1;
}

/* ===== Capsule Navigation (Teacher) ===== */
.capsule-nav {
  display: flex;
  align-items: center;
  justify-self: start;
  gap: 4px;
  width: max-content;
  min-width: 0;
  max-width: 100%;
  overflow-x: auto;
  overscroll-behavior-inline: contain;
  background: hsl(var(--surface-2));
  border: 1px solid hsl(var(--border));
  border-radius: 999px;
  padding: 5px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.04), 0 1px 3px rgba(0, 0, 0, 0.06);
}

.capsule-nav::-webkit-scrollbar,
.top-nav-tabs::-webkit-scrollbar {
  display: none;
}

.capsule-nav-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 9px 18px;
  border-radius: 999px;
  font-size: 13px;
  font-weight: 500;
  color: hsl(var(--muted-foreground));
  text-decoration: none;
  transition: all 0.2s ease;
  white-space: nowrap;
}

.capsule-nav-btn:hover {
  color: hsl(var(--ink));
  background: hsl(var(--surface));
}

.capsule-nav-btn-active {
  background: hsl(var(--surface));
  color: hsl(var(--ink));
  font-weight: 600;
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.08), 0 1px 2px rgba(0, 0, 0, 0.04);
}

.top-nav-tabs {
  height: 2.75rem;
  padding: 0 1rem;
  display: flex;
  align-items: center;
  gap: 0;
  overflow-x: auto;
  overscroll-behavior-inline: contain;
}

@media (max-width: 1180px) {
  .top-nav-row {
    grid-template-columns: minmax(0, 1fr) auto;
  }

  .top-nav-capsule .top-nav-row {
    grid-template-columns: minmax(0, 1fr) auto;
  }

  .top-nav-actions {
    grid-column: 2;
  }

  .top-nav-capsule .top-nav-actions {
    grid-column: 2;
  }

  .capsule-nav {
    grid-column: 1 / -1;
    grid-row: 2;
    width: auto;
    justify-self: stretch;
  }

  .top-nav-capsule .capsule-nav {
    grid-column: 1 / -1;
    grid-row: 2;
    width: auto;
    justify-self: stretch;
  }
}

@media (max-width: 760px) {
  .top-nav-row {
    grid-template-columns: minmax(0, 1fr);
    align-items: stretch;
  }

  .top-nav-actions {
    grid-column: 1;
    justify-content: flex-start;
    flex-wrap: wrap;
  }

  .top-nav-actions > button:first-child {
    width: min(100%, 22rem) !important;
  }
}
</style>
