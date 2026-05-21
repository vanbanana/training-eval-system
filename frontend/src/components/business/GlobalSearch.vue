<script setup lang="ts">
import { computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import {
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from '@/components/ui/command'
import { LayoutDashboard, ListChecks, BookOpen, Users, BarChart3, Settings, Bell, MessageSquareText, GraduationCap, FileText, History, ShieldAlert } from 'lucide-vue-next'

const open = defineModel<boolean>('open', { default: false })
const router = useRouter()
const auth = useAuthStore()

interface Cmd {
  label: string
  to: string
  icon: typeof LayoutDashboard
  keywords?: string
  roles?: string[]
}

const allCommands: Cmd[] = [
  { label: '工作台', to: '/dashboard', icon: LayoutDashboard, keywords: 'dashboard home' },
  { label: '通知中心', to: '/notifications', icon: Bell, keywords: 'notification' },
  { label: '账号设置', to: '/account', icon: Settings, keywords: 'account profile settings' },
  { label: '评价模板', to: '/templates', icon: FileText, keywords: 'template rubric' },
  // 教师
  { label: '实训任务', to: '/teacher/tasks', icon: ListChecks, roles: ['teacher', 'admin'] },
  { label: '班级管理', to: '/teacher/classes', icon: Users, roles: ['teacher', 'admin'] },
  { label: '教学画像', to: '/profiles', icon: BarChart3, roles: ['teacher', 'admin'] },
  { label: '报表中心', to: '/teacher/reports', icon: BarChart3, roles: ['teacher', 'admin'] },
  // 学生
  { label: '我的任务', to: '/student/tasks', icon: ListChecks, roles: ['student'] },
  { label: '评价历史', to: '/student/history', icon: History, roles: ['student'] },
  { label: 'AI 问答', to: '/student/chat', icon: MessageSquareText, roles: ['student'] },
  { label: '能力画像', to: '/student/profile', icon: GraduationCap, roles: ['student'] },
  // 管理员
  { label: '用户管理', to: '/admin/users', icon: Users, roles: ['admin'] },
  { label: '导入用户', to: '/admin/users/import', icon: Users, roles: ['admin'] },
  { label: '课程管理', to: '/admin/courses', icon: BookOpen, roles: ['admin'] },
  { label: 'LLM 配置', to: '/admin/llm', icon: Settings, roles: ['admin'] },
  { label: '审计日志', to: '/admin/audit', icon: ShieldAlert, roles: ['admin'] },
]

const visibleCommands = computed(() => {
  const role = auth.user?.role
  return allCommands.filter((c) => !c.roles || (role && c.roles.includes(role)))
})

function go(cmd: Cmd) {
  open.value = false
  router.push(cmd.to)
}

function onKey(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
    e.preventDefault()
    open.value = !open.value
  }
}

onMounted(() => window.addEventListener('keydown', onKey))
onUnmounted(() => window.removeEventListener('keydown', onKey))
</script>

<template>
  <CommandDialog v-model:open="open">
    <CommandInput placeholder="输入关键字搜索功能（例如 任务 / 用户 / 班级）..." />
    <CommandList>
      <CommandEmpty>未找到匹配项</CommandEmpty>
      <CommandGroup heading="导航">
        <CommandItem
          v-for="cmd in visibleCommands"
          :key="cmd.to"
          :value="cmd.label + ' ' + (cmd.keywords ?? '')"
          @select="go(cmd)"
        >
          <component :is="cmd.icon" class="text-muted-foreground" />
          <span>{{ cmd.label }}</span>
          <span class="ml-auto text-xs text-subtle-foreground">{{ cmd.to }}</span>
        </CommandItem>
      </CommandGroup>
    </CommandList>
  </CommandDialog>
</template>
