import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const routes: RouteRecordRaw[] = [
  { path: '/', redirect: '/dashboard' },
  {
    path: '/login',
    name: 'login',
    component: () => import('@/views/auth/LoginView.vue'),
    meta: { public: true },
  },
  {
    path: '/dashboard',
    name: 'dashboard',
    component: () => import('@/views/shared/DashboardView.vue'),
  },
  {
    path: '/teacher/tasks',
    name: 'teacher-tasks',
    component: () => import('@/views/teacher/TasksView.vue'),
  },
  {
    path: '/teacher/tasks/new',
    name: 'teacher-task-create',
    component: () => import('@/views/teacher/TaskFormView.vue'),
  },
  {
    path: '/teacher/tasks/:id/grading',
    name: 'teacher-grading',
    component: () => import('@/views/teacher/GradingView.vue'),
  },
  {
    path: '/teacher/evaluations/:id',
    name: 'teacher-grading-detail',
    component: () => import('@/views/teacher/GradingDetailView.vue'),
  },
  {
    path: '/teacher/similarity/:id',
    name: 'teacher-similarity',
    component: () => import('@/views/teacher/SimilarityCompareView.vue'),
  },
  {
    path: '/teacher/students/:id/profile',
    name: 'teacher-student-profile',
    component: () => import('@/views/teacher/StudentProfileView.vue'),
  },
  {
    path: '/teacher/grading',
    component: () => import('@/views/teacher/GradingHomeView.vue'),
  },
  {
    path: '/teacher/reports',
    name: 'teacher-reports',
    component: () => import('@/views/teacher/ReportsView.vue'),
  },
  {
    path: '/profiles',
    name: 'profiles',
    component: () => import('@/views/shared/ProfileView.vue'),
  },
  {
    path: '/admin/users',
    name: 'admin-users',
    component: () => import('@/views/admin/UsersView.vue'),
  },
  {
    path: '/admin/users/import',
    name: 'admin-users-import',
    component: () => import('@/views/admin/UserImportView.vue'),
  },
  {
    path: '/admin/courses',
    name: 'admin-courses',
    component: () => import('@/views/admin/CoursesView.vue'),
  },
  {
    path: '/admin/llm',
    name: 'admin-llm',
    component: () => import('@/views/admin/LlmConfigView.vue'),
  },
  {
    path: '/admin/audit',
    name: 'admin-audit',
    component: () => import('@/views/admin/AuditView.vue'),
  },
  {
    path: '/admin/dashboard',
    name: 'admin-dashboard',
    component: () => import('@/views/admin/AdminDashboardView.vue'),
  },
  {
    path: '/notifications',
    name: 'notifications',
    component: () => import('@/views/shared/NotificationsView.vue'),
  },
  {
    path: '/account',
    name: 'account-settings',
    component: () => import('@/views/shared/AccountSettingsView.vue'),
  },
  {
    path: '/teacher/classes',
    name: 'teacher-classes',
    component: () => import('@/views/teacher/ClassesView.vue'),
  },
  {
    path: '/student/tasks',
    name: 'student-tasks',
    component: () => import('@/views/student/TasksView.vue'),
  },
  {
    path: '/student/tasks/:id',
    name: 'student-task-detail',
    component: () => import('@/views/student/TaskDetailView.vue'),
  },
  {
    path: '/student/evaluations/:id',
    name: 'student-evaluation',
    component: () => import('@/views/student/EvaluationView.vue'),
  },
  {
    path: '/student/profile',
    name: 'student-profile',
    component: () => import('@/views/student/MyProfileView.vue'),
  },
  {
    path: '/student/history',
    name: 'student-history',
    component: () => import('@/views/student/HistoryView.vue'),
  },
  {
    path: '/student/chat',
    name: 'student-chat',
    component: () => import('@/views/student/ChatView.vue'),
    meta: { roles: ['student'] },
  },
  {
    path: '/teacher/agent',
    name: 'teacher-agent',
    component: () => import('@/views/teacher/AgentView.vue'),
    meta: { roles: ['teacher'] },
  },
  {
    path: '/admin/agent',
    name: 'admin-agent',
    component: () => import('@/views/admin/AgentView.vue'),
    meta: { roles: ['admin'] },
  },
  {
    path: '/templates',
    name: 'templates',
    component: () => import('@/views/shared/TemplatesView.vue'),
  },
  {
    path: '/403',
    name: 'forbidden',
    component: () => import('@/views/shared/Error403View.vue'),
    meta: { public: true },
  },
  {
    path: '/500',
    name: 'server-error',
    component: () => import('@/views/shared/Error500View.vue'),
    meta: { public: true },
  },
  {
    path: '/:pathMatch(.*)*',
    component: () => import('@/views/shared/NotFoundView.vue'),
    meta: { public: true },
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

// Role required to enter an area, keyed by path prefix. Mirrors backend RBAC so
// a student can't render an /admin/* page shell (which previously only showed a
// toast while the API 403'd). null = accessible to any authenticated role.
function requiredRoles(path: string): string[] | null {
  if (path.startsWith('/admin')) return ['admin']
  if (path.startsWith('/teacher')) return ['teacher', 'admin']
  if (path.startsWith('/student')) return ['student']
  return null
}

router.beforeEach(async (to) => {
  const auth = useAuthStore()
  if (!auth.user && auth.token) {
    await auth.fetchMe()
  }
  if (!to.meta.public && !auth.isAuthenticated) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  if (to.name === 'login' && auth.isAuthenticated) {
    return { name: 'dashboard' }
  }
  // Admins land on the dedicated operations console rather than the generic
  // (sparse) dashboard shell, which reads like a bare admin backend.
  if (to.name === 'dashboard' && auth.user?.role === 'admin') {
    return { name: 'admin-dashboard' }
  }
  if (!to.meta.public && auth.user) {
    const roles = requiredRoles(to.path)
    if (roles && !roles.includes(auth.user.role)) {
      return { name: 'forbidden' }
    }
  }
  // Per-route role guard (meta.roles) for finer-grained control.
  const metaRoles = to.meta.roles as string[] | undefined
  if (metaRoles && auth.user && !metaRoles.includes(auth.user.role)) {
    return { name: 'forbidden' }
  }
})

export default router
