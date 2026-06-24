<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import axios from 'axios'
import AppShell from '@/components/layout/AppShell.vue'
import BreadcrumbNav from '@/components/business/BreadcrumbNav.vue'
import { Card } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { GraduationCap, Users, FileText, CheckCircle2, ClipboardList } from 'lucide-vue-next'
import EmptyState from '@/components/business/EmptyState.vue'

interface TaskItem {
  id: number
  name: string
  status: string
  pending_ai_count: number
  scored_count: number
  confirmed_count: number
  rejected_count: number
}

interface ClassItem {
  id: number
  name: string
  student_count: number
  tasks: TaskItem[]
}

interface CourseItem {
  id: number
  name: string
  code: string
  classes: ClassItem[]
}

interface WorkbenchData {
  courses: CourseItem[]
  summary: {
    pending_ai_count: number
    scored_unconfirmed_count: number
    suspicious_count: number
    confirmed_today_count: number
  }
}

const router = useRouter()
const data = ref<WorkbenchData | null>(null)
const loading = ref(true)
const selectedCourse = ref<number | null>(null)
const selectedClass = ref<number | null>(null)

onMounted(async () => {
  try {
    const res = await axios.get('/api/grading/workbench')
    data.value = res.data
    if (res.data.courses?.length) {
      selectedCourse.value = res.data.courses[0].id
      if (res.data.courses[0].classes?.length) {
        selectedClass.value = res.data.courses[0].classes[0].id
      }
    }
  } catch { /* ignore */ }
  finally { loading.value = false }
})

const currentCourse = () => data.value?.courses.find(c => c.id === selectedCourse.value)
const currentClass = () => currentCourse()?.classes.find(c => c.id === selectedClass.value)
</script>

<template>
  <AppShell>
    <BreadcrumbNav :items="[
      { label: '工作台', to: '/dashboard' },
      { label: '批改首页' },
    ]" />

    <div class="flex justify-between items-end mb-5">
      <h1 class="text-2xl font-bold text-ink">批改工作台</h1>
    </div>

    <!-- KPI Summary -->
    <div v-if="data" class="grid grid-cols-4 gap-4 mb-6">
      <Card class="p-4 flex items-center gap-3">
        <FileText class="w-5 h-5 text-warning" />
        <div><div class="text-2xl font-bold">{{ data.summary.pending_ai_count }}</div><div class="text-xs text-muted-foreground">待 AI 批改</div></div>
      </Card>
      <Card class="p-4 flex items-center gap-3">
        <ClipboardList class="w-5 h-5 text-primary" />
        <div><div class="text-2xl font-bold">{{ data.summary.scored_unconfirmed_count }}</div><div class="text-xs text-muted-foreground">AI 已评分待确认</div></div>
      </Card>
      <Card class="p-4 flex items-center gap-3">
        <CheckCircle2 class="w-5 h-5 text-success" />
        <div><div class="text-2xl font-bold">{{ data.summary.confirmed_today_count }}</div><div class="text-xs text-muted-foreground">今日已确认</div></div>
      </Card>
      <Card class="p-4 flex items-center gap-3">
        <Users class="w-5 h-5 text-accent" />
        <div><div class="text-2xl font-bold">{{ data.courses.length }}</div><div class="text-xs text-muted-foreground">我的课程</div></div>
      </Card>
    </div>

    <div v-if="loading" class="grid grid-cols-[200px_200px_1fr] gap-4">
      <Skeleton class="h-[400px]" /><Skeleton class="h-[400px]" /><Skeleton class="h-[400px]" />
    </div>

    <div v-else-if="!data?.courses.length" class="mt-8">
      <EmptyState :icon="GraduationCap" title="暂无课程" description="你的账号暂未关联任何课程" />
    </div>

    <div v-else class="flex gap-4">
      <!-- Course list -->
      <Card class="w-[200px] shrink-0 overflow-hidden">
        <div class="p-3 font-bold text-sm border-b border-border">课程</div>
        <div class="flex flex-col">
          <button v-for="c in data.courses" :key="c.id" class="px-3 py-2.5 text-left text-sm hover:bg-accent-soft transition-colors"
            :class="c.id === selectedCourse ? 'bg-accent-soft text-accent-strong font-semibold' : ''" @click="selectedCourse = c.id; selectedClass = null">
            <GraduationCap class="w-3.5 h-3.5 inline mr-1.5" />{{ c.name }}
          </button>
        </div>
      </Card>

      <!-- Class list -->
      <Card class="w-[200px] shrink-0 overflow-hidden">
        <div class="p-3 font-bold text-sm border-b border-border">班级</div>
        <div v-if="!currentCourse()?.classes.length" class="p-4 text-xs text-muted-foreground">暂无班级</div>
        <div v-else class="flex flex-col">
          <button v-for="cl in currentCourse()?.classes" :key="cl.id" class="px-3 py-2.5 text-left text-sm hover:bg-accent-soft transition-colors"
            :class="cl.id === selectedClass ? 'bg-accent-soft text-accent-strong font-semibold' : ''" @click="selectedClass = cl.id">
            <Users class="w-3.5 h-3.5 inline mr-1.5" />{{ cl.name }} ({{ cl.student_count }})
          </button>
        </div>
      </Card>

      <!-- Task list -->
      <Card class="flex-1 overflow-hidden">
        <div class="p-3 font-bold text-sm border-b border-border">
          任务 <span v-if="currentClass()">— {{ currentClass()?.name }}</span>
        </div>
        <div v-if="!currentClass()?.tasks.length" class="p-6 text-sm text-muted-foreground">该班级暂无任务</div>
        <div v-else class="flex flex-col">
          <div v-for="t in currentClass()?.tasks" :key="t.id" class="p-4 border-b border-border last:border-b-0 flex justify-between items-center">
            <div>
              <div class="font-semibold text-ink">{{ t.name }}</div>
              <div class="mt-1 flex gap-2 text-xs">
                <Badge variant="warning">待批: {{ t.pending_ai_count }}</Badge>
                <Badge variant="info">已评分: {{ t.scored_count }}</Badge>
                <Badge variant="success">已确认: {{ t.confirmed_count }}</Badge>
              </div>
            </div>
            <div class="flex gap-2">
              <Button size="sm" @click="router.push(`/teacher/tasks/${t.id}/grading`)">进入批改</Button>
              <Button size="sm" variant="outline" @click="router.push(`/teacher/reports?task_id=${t.id}`)">报告</Button>
            </div>
          </div>
        </div>
      </Card>
    </div>
  </AppShell>
</template>
