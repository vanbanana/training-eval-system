<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import axios from 'axios'
import { useAuthStore } from '@/stores/auth'
import { useCourseMap } from '@/composables/useCourseMap'
import { safeGet } from '@/lib/api-helpers'
import Icon3D from './icons/Icon3D.vue'
import teacherWelcomeBg from '@/assets/teacher-welcome-bg.png'
import encourageSvg from '@/assets/encourage-illustration.svg'

interface ActivityDay { date: string; count: number }
interface DashboardTask {
  id: number; name: string; status: string; deadline: string | null
  course_id: number; total_students?: number; submitted?: number; graded?: number
}
interface Notification {
  id: number; title: string; body?: string; type: string
  is_read: boolean; created_at: string
}
interface TDData {
  role: string; my_tasks: number; pending_grading: number
  graded_this_week: number; class_avg_score: number | null
  activity_7d: ActivityDay[]; recent_tasks: DashboardTask[]
  recent_notifications: Notification[]
}
const auth = useAuthStore()
const router = useRouter()
const { load: loadCourseMap, courseName } = useCourseMap()
const stats = ref<TDData | null>(null)
const loading = ref(true)
const suspectCount = ref(0)
async function fetchAll() {
  loading.value = true
  try { const { data } = await axios.get('/api/dashboard'); stats.value = data; await fetchSuspectCount() }
  catch (e) { console.error(e) }
  finally { loading.value = false }
}
async function fetchSuspectCount() {
  if (!stats.value?.recent_tasks) return
  let total = 0
  const pub = stats.value.recent_tasks.filter(t => t.status === 'published').slice(0, 8)
  await Promise.all(pub.map(async t => {
    const r = await safeGet<Array<{ state: string }>>(`/api/similarity/task/${t.id}`, [])
    total += (r.data ?? []).filter(p => p.state === 'suspect').length
  }))
  suspectCount.value = total
}
onMounted(() => { void loadCourseMap(); void fetchAll() })
const greeting = computed(() => {
  const h = new Date().getHours()
  if (h < 6) return '凌晨好'; if (h < 12) return '早上好'
  if (h < 14) return '中午好'; if (h < 18) return '下午好'; return '晚上好'
})
const maxAct = computed(() => Math.max(1, ...(stats.value?.activity_7d?.map(d => d.count) ?? [1])))
const weekDays = ['周一', '周二', '周三', '周四', '周五', '周六', '周日']
const peakIdx = computed(() => {
  if (!stats.value?.activity_7d?.length) return -1
  const mx = Math.max(...stats.value.activity_7d.map(d => d.count))
  return stats.value.activity_7d.findIndex(d => d.count === mx)
})

// Smooth chart computed
const chartW = 360
const chartH = 120
const chartPadY = 15
const chartPoints = computed(() => {
  if (!stats.value?.activity_7d?.length) return []
  const n = stats.value.activity_7d.length
  return stats.value.activity_7d.map((d, i) => ({
    x: n > 1 ? i * (chartW / (n - 1)) : chartW / 2,
    y: chartH - chartPadY - (d.count / maxAct.value) * (chartH - chartPadY * 2),
  }))
})

function smoothPath(pts: {x:number;y:number}[], closed = false): string {
  if (pts.length < 2) return ''
  let d = `M ${pts[0].x},${pts[0].y}`
  for (let i = 0; i < pts.length - 1; i++) {
    const p0 = pts[Math.max(i - 1, 0)]
    const p1 = pts[i]
    const p2 = pts[i + 1]
    const p3 = pts[Math.min(i + 2, pts.length - 1)]
    const cp1x = p1.x + (p2.x - p0.x) / 6
    const cp1y = p1.y + (p2.y - p0.y) / 6
    const cp2x = p2.x - (p3.x - p1.x) / 6
    const cp2y = p2.y - (p3.y - p1.y) / 6
    d += ` C ${cp1x},${cp1y} ${cp2x},${cp2y} ${p2.x},${p2.y}`
  }
  if (closed) {
    d += ` L ${pts[pts.length-1].x},${chartH} L ${pts[0].x},${chartH} Z`
  }
  return d
}

const smoothLinePath = computed(() => smoothPath(chartPoints.value))
const smoothFillPath = computed(() => smoothPath(chartPoints.value, true))
const peakX = computed(() => chartPoints.value[peakIdx.value]?.x ?? 0)
function go(p: string) { router.push(p) }
</script>

<template>
<div v-if="loading" class="td-load">加载中...</div>
<div v-else-if="stats" class="td">
  <!-- 欢迎横幅：全宽图片，不裁切，自然高度，文字叠在上面 -->
  <div class="td-banner">
    <div class="td-banner-gradient"></div>
    <img :src="teacherWelcomeBg" alt="" />
    <div class="td-banner-txt">
      <h1>{{ greeting }}，{{ auth.user?.display_name }} 👋</h1>
      <p>今天是充实的一天，继续加油哦！</p>
    </div>
  </div>

  <!-- 两栏主体：左70% 右30% -->
  <div class="td-body">
    <div class="td-left">
      <!-- 统计卡片 -->
      <div class="td-stats">
        <div class="td-st"><Icon3D name="checkbox" :size="56" color="blue"/><div><label>待批改提交</label><div class="td-sv"><b>{{ stats.pending_grading }}</b><em class="o">较昨日 +6</em></div></div></div>
        <div class="td-st"><Icon3D name="folder" :size="56" color="purple"/><div><label>本周已批改</label><div class="td-sv"><b>{{ stats.graded_this_week }}</b><em class="o">较昨日 +12</em></div></div></div>
        <div class="td-st"><Icon3D name="trophy" :size="56" color="green"/><div><label>班级平均分</label><div class="td-sv"><b>{{ stats.class_avg_score ?? '—' }}</b><em class="g">优秀</em></div></div></div>
        <div class="td-st"><Icon3D name="shield" :size="56" color="orange"/><div><label>疑似抄袭警告</label><div class="td-sv"><b>{{ suspectCount }}</b><em class="r">待处理</em></div></div></div>
      </div>

      <!-- 折线图 + 鼓励语 并排 -->
      <div class="td-mid">
        <div class="td-chart">
          <div class="td-chart-hd">
            <div>
              <h3>当前班级活跃度</h3>
              <small>近7天活跃情况</small>
            </div>
          </div>
          <div class="td-chart-area">
            <!-- Peak badge -->
            <span v-if="peakIdx>=0 && chartPoints.length" class="td-peak-badge" :style="{left:`${(chartPoints[peakIdx].x/chartW)*100}%`}">
              {{ weekDays[peakIdx] }}活跃度最高
              <i class="td-peak-arrow"></i>
            </span>
            <svg :viewBox="`0 0 ${chartW} ${chartH}`" preserveAspectRatio="none" class="td-svg">
              <defs>
                <linearGradient id="actGrad" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" class="chart-grad-start" stop-opacity="0.12"/>
                  <stop offset="100%" class="chart-grad-end" stop-opacity="0.01"/>
                </linearGradient>
              </defs>
              <!-- Fill -->
              <path v-if="chartPoints.length" :d="smoothFillPath" fill="url(#actGrad)" />
              <!-- Line -->
              <path v-if="chartPoints.length" :d="smoothLinePath" fill="none" class="chart-line" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
              <!-- Peak vertical dashed line -->
              <line v-if="peakIdx>=0 && chartPoints.length" :x1="peakX" y1="0" :x2="peakX" :y2="chartH" class="chart-line" stroke-width="1" stroke-dasharray="4,3" opacity="0.4"/>
              <!-- Data points -->
              <template v-for="(pt,i) in chartPoints" :key="i">
                <circle :cx="pt.x" :cy="pt.y" :r="i===peakIdx?5:3" :class="i===peakIdx?'chart-dot-peak':'chart-dot'" :stroke-width="i===peakIdx?2:0"/>
              </template>
            </svg>
          </div>
          <div class="td-chart-x">
            <span v-for="(d,i) in stats.activity_7d" :key="i">{{ weekDays[i] || d.date }}</span>
          </div>
        </div>
        <div class="td-enc">
          <div class="td-enc-txt">
            <h3>保持优秀哦！🎉</h3>
            <p>本周班级活跃度较上周增加 {{ stats.activity_7d.length ? Math.round(stats.activity_7d.reduce((s,d)=>s+d.count,0) / Math.max(stats.activity_7d.length, 1)) : 0 }}%</p>
          </div>
          <img class="td-enc-img" :src="encourageSvg" alt="" />
        </div>
      </div>

      <!-- 任务卡片（无标题） -->
      <div class="td-tasks">
        <div v-for="t in stats.recent_tasks.slice(0,3)" :key="t.id" class="td-task" @click="go(`/teacher/tasks/${t.id}`)">
          <div class="td-task-top"><Icon3D name="notebook" :size="48" color="blue"/><em :class="t.status==='published'?'b':t.status==='closed'?'g':'o'">{{ t.status==='published'?'进行中':t.status==='closed'?'已完成':'待批改' }}</em></div>
          <h4>{{ t.name }}</h4>
          <p>{{ courseName(t.course_id) }}</p>
          <div class="td-bar"><div :class="{done:t.status==='closed'}" :style="{width:`${t.total_students?((t.graded??0)/t.total_students)*100:0}%`}"></div></div>
          <small>{{ t.total_students?Math.round(((t.graded??0)/t.total_students)*100):0 }}%</small>
        </div>
      </div>
    </div>

    <!-- RIGHT COLUMN -->
    <div class="td-right">
      <div class="td-qa-card">
        <h3>快捷操作</h3>
        <div class="td-qa-grid">
          <div class="td-qa" @click="go('/teacher/tasks/new')"><Icon3D name="clipboard" :size="56" color="blue"/><span>创建实训任务</span><small>快速布置新任务</small></div>
          <div class="td-qa" @click="go('/teacher/tasks')"><Icon3D name="notebook" :size="56" color="purple"/><span>批改工作台</span><small>高效批改学生实训任务</small></div>
          <div class="td-qa" @click="go('/teacher/classes')"><Icon3D name="people" :size="56" color="green"/><span>班级管理</span><small>查看班级学生情况</small></div>
          <div class="td-qa" @click="go('/profiles')"><Icon3D name="chart" :size="56" color="orange"/><span>评价看板</span><small>多维度数据分析</small></div>
        </div>
      </div>
      <div class="td-notif-card">
        <div class="td-sec-hd"><h3>重要通知</h3><a @click="go('/notifications')">查看全部 &gt;</a></div>
        <div class="td-notifs">
          <div v-for="n in stats.recent_notifications.slice(0,3)" :key="n.id" class="td-nf">
            <div class="td-nf-d"><b>{{ n.created_at ? new Date(n.created_at).getDate() || '—' : '—' }}</b><small>{{ n.created_at ? (new Date(n.created_at).getMonth()+1) + '月' : '' }}</small></div>
            <div class="td-nf-i">📄</div>
            <div class="td-nf-t"><h4>{{ n.title }}</h4><p>{{ n.body||n.type }}</p></div>
            <i v-if="!n.is_read" class="td-dot"></i>
          </div>
        </div>
      </div>
    </div>
  </div>
</div>
</template>

<style scoped>
.td {
  display:flex;
  flex-direction:column;
  gap:20px;
  min-width:0;
  min-height:0;
  overflow-x:clip;
  width:100%;
}
.td-load { display:flex; align-items:center; justify-content:center; min-height:12rem; padding:2rem; color:hsl(var(--muted-foreground)); }

/* 横幅：大屏时不再通过负边距强制溢出边缘，直接自带圆角，使得整个界面左对齐非常工整 */
.td-banner { position:relative; flex-shrink:0; overflow:hidden; height:clamp(8rem, 12vw, 12rem); border-radius:16px; box-shadow:0 2px 8px rgba(0,0,0,0.04); }
.td-banner img { display:block; width:100%; height:100%; object-fit:cover; object-position:center; }
.td-banner-gradient {
  position:absolute; inset:0; z-index:1; pointer-events:none;
  background: linear-gradient(to right, hsl(var(--background)) 0%, hsl(var(--background) / 0.85) 25%, transparent 50%, transparent 100%);
}
.td-banner-txt { position:absolute; left:32px; top:50%; transform:translateY(-50%); z-index:2; }
.td-banner-txt h1 { margin:0; font-size:24px; font-weight:700; color:hsl(var(--ink)); }
.td-banner-txt p { margin:5px 0 0; font-size:13px; color:hsl(var(--muted-foreground)); }

/* 主工作区：分左右布局，避免网格的强制拉伸 */
.td-body {
  display:flex;
  align-items:start;
  gap:20px;
  min-width:0;
}
.td-left {
  flex:1;
  display:flex;
  flex-direction:column;
  gap:20px;
  min-width:0;
}
.td-right {
  width:clamp(280px, 22vw, 360px); /* 动态右侧宽度，避免超宽屏下左侧吃掉所有空间导致卡片拉扯过度 */
  flex-shrink:0;
  display:flex;
  flex-direction:column;
  gap:20px;
  min-width:0;
}

/* 统计卡片 */
.td-stats { display:grid; grid-template-columns:repeat(4,minmax(0,1fr)); align-items:start; gap:16px; }
.td-st { display:flex; align-items:center; gap:14px; background:hsl(var(--surface)); border-radius:14px; padding:20px 22px; min-height:106px; box-shadow:0 2px 8px rgba(0,0,0,0.04); }
.td-st label { font-size:11px; color:hsl(var(--muted-foreground)); display:block; }
.td-sv { display:flex; align-items:baseline; gap:6px; margin-top:2px; }
.td-sv b { font-size:22px; font-weight:700; color:hsl(var(--ink)); line-height:1; }
.td-sv em { font-style:normal; font-size:10px; font-weight:600; padding:1px 6px; border-radius:8px; }
.td-sv em.o { background:hsl(var(--warning-soft)); color:hsl(var(--accent)); }
.td-sv em.g { background:hsl(var(--success-soft)); color:hsl(var(--success)); }
.td-sv em.r { background:hsl(var(--danger-soft)); color:hsl(var(--danger)); }

/* 中间行：折线图+鼓励语 */
.td-mid { display:grid; grid-template-columns:1.6fr 1fr; align-items:start; gap:16px; min-height:0; }
.td-chart { background:hsl(var(--surface)); border-radius:14px; padding:20px; box-shadow:0 2px 8px rgba(0,0,0,0.04); display:flex; flex-direction:column; border:1px solid hsl(var(--border)); min-height:260px; }
.td-chart-hd { display:flex; justify-content:space-between; align-items:flex-start; margin-bottom:12px; }
.td-chart-hd h3 { margin:0; font-size:14px; font-weight:600; color:hsl(var(--ink)); }
.td-chart-hd small { font-size:11px; color:hsl(var(--muted-foreground)); margin-top:2px; display:block; }
.td-peak-badge { position:absolute; top:-8px; transform:translateX(-50%); font-size:11px; font-weight:600; background:hsl(var(--info)); color:white; padding:5px 12px; border-radius:8px; white-space:nowrap; z-index:2; }
.td-peak-arrow { position:absolute; bottom:-5px; left:50%; transform:translateX(-50%); width:0; height:0; border-left:5px solid transparent; border-right:5px solid transparent; border-top:5px solid hsl(var(--info)); }
.td-chart-area { position:relative; height:180px; padding-top:28px; flex:1; }
.td-svg { width:100%; height:100%; display:block; }
.td-chart-area svg { width:100%; height:100%; }
.td-chart-x { display:flex; justify-content:space-between; font-size:11px; color:hsl(var(--muted-foreground)); margin-top:8px; padding:0 2px; font-weight:500; }

/* SVG chart colors via CSS variables */
.chart-grad-start { stop-color: hsl(var(--info)); }
.chart-grad-end { stop-color: hsl(var(--info)); }
.chart-line { stroke: hsl(var(--info)); }
.chart-dot { fill: hsl(var(--info)); stroke: none; }
.chart-dot-peak { fill: white; stroke: hsl(var(--info)); }

.td-enc { background:hsl(var(--surface)); border:1px solid hsl(var(--border)); border-radius:14px; padding:20px 16px 20px 24px; position:relative; overflow:hidden; display:flex; align-items:center; box-shadow:0 2px 8px rgba(0,0,0,0.04); gap:0; min-height:0; height:100%; }
.td-enc-txt { display:flex; flex-direction:column; gap:6px; z-index:1; flex-shrink:0; width:45%; align-self:flex-start; padding-top:4px; }
.td-enc h3 { margin:0; font-size:22px; font-weight:700; color:hsl(var(--ink)); }
.td-enc p { margin:0; font-size:14px; color:hsl(var(--muted-foreground)); }
.td-enc-img { width:55%; height:auto; object-fit:contain; flex-shrink:0; margin-left:-20px; align-self:center; }

/* 任务卡片 */
.td-tasks-sec { flex-shrink:0; }
.td-sec-hd { display:flex; align-items:center; justify-content:space-between; margin-bottom:10px; }
.td-sec-hd h3 { margin:0; font-size:13px; font-weight:600; color:hsl(var(--ink)); }
.td-sec-hd a { font-size:11px; color:hsl(var(--primary)); cursor:pointer; font-weight:500; }
.td-tasks { display:flex; flex-wrap:wrap; align-items:start; gap:16px; min-height:0; }
.td-task { flex:1 1 280px; max-width:100%; background:hsl(var(--surface)); border-radius:12px; padding:16px; cursor:pointer; box-shadow:0 2px 8px rgba(0,0,0,0.04); transition:all 0.15s; display:flex; flex-direction:column; min-height:138px; }
.td-task:hover { transform:translateY(-2px); box-shadow:0 4px 12px rgba(0,0,0,0.07); }
.td-task-top { display:flex; align-items:flex-start; justify-content:space-between; margin-bottom:6px; }
.td-task-top em { font-style:normal; font-size:10px; font-weight:600; padding:2px 6px; border-radius:7px; }
.td-task-top em.b { background:hsl(var(--info-soft)); color:hsl(var(--info)); }
.td-task-top em.g { background:hsl(var(--success-soft)); color:hsl(var(--success)); }
.td-task-top em.o { background:hsl(var(--warning-soft)); color:hsl(var(--warning)); }
.td-task h4 { margin:0 0 2px; font-size:12px; font-weight:600; color:hsl(var(--ink)); line-height:1.4; display:-webkit-box; -webkit-line-clamp:2; -webkit-box-orient:vertical; overflow:hidden; }
.td-task p { margin:0 0 6px; font-size:10px; color:hsl(var(--muted-foreground)); }
.td-bar { height:5px; background:hsl(var(--muted)); border-radius:3px; overflow:hidden; }
.td-bar div { height:100%; background:hsl(var(--primary)); border-radius:3px; }
.td-bar div.done { background:hsl(var(--success)); }
.td-task small { font-size:10px; color:hsl(var(--muted-foreground)); margin-top:2px; display:block; }
.td-task .td-bar { margin-top:auto; }

/* 快捷操作 */
.td-qa-card { background:hsl(var(--surface)); border-radius:14px; padding:16px; box-shadow:0 2px 8px rgba(0,0,0,0.04); display:flex; flex-direction:column; min-height:0; }
.td-qa-card h3 { margin:0 0 12px; font-size:13px; font-weight:600; color:hsl(var(--ink)); }
.td-qa-grid { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); align-items:start; gap:10px; min-height:0; }
.td-qa { aspect-ratio:1/1; display:flex; flex-direction:column; align-items:center; justify-content:center; gap:4px; padding:10px 6px; background:hsl(var(--surface-2)); border-radius:12px; cursor:pointer; text-align:center; transition:all 0.15s; min-height:0; }
.td-qa:hover { background:hsl(var(--primary-soft)); transform:translateY(-2px); }
.td-qa span { font-size:11px; font-weight:600; color:hsl(var(--ink)); }
.td-qa small { font-size:9px; color:hsl(var(--muted-foreground)); }

/* 通知 */
.td-notif-card { background:hsl(var(--surface)); border-radius:14px; padding:16px; box-shadow:0 2px 8px rgba(0,0,0,0.04); min-height:0; overflow:hidden; flex:1; }
.td-notifs { display:flex; flex-direction:column; gap:8px; }
.td-nf { display:flex; align-items:flex-start; gap:8px; padding:8px; border-radius:8px; cursor:pointer; }
.td-nf:hover { background:hsl(var(--surface-2)); }
.td-nf-d { display:flex; flex-direction:column; align-items:center; min-width:28px; }
.td-nf-d b { font-size:16px; font-weight:700; color:hsl(var(--ink)); line-height:1; }
.td-nf-d small { font-size:9px; color:hsl(var(--muted-foreground)); }
.td-nf-i { width:24px; height:24px; background:hsl(var(--primary-soft)); border-radius:6px; display:flex; align-items:center; justify-content:center; font-size:12px; flex-shrink:0; }
.td-nf-t { flex:1; min-width:0; }
.td-nf-t h4 { margin:0; font-size:11px; font-weight:600; color:hsl(var(--ink)); line-height:1.3; }
.td-nf-t p { margin:2px 0 0; font-size:10px; color:hsl(var(--muted-foreground)); display:-webkit-box; -webkit-line-clamp:2; -webkit-box-orient:vertical; overflow:hidden; }
.td-dot { width:6px; height:6px; background:hsl(var(--danger)); border-radius:50%; flex-shrink:0; margin-top:3px; }

/* 大厂标准分档处理 Breakpoints */
@media (max-width: 1440px) {
  .td-mid { grid-template-columns: 1.4fr 1fr; }
}

@media (max-width: 1280px) {
  .td-right { width:280px; }
  .td-tasks { grid-template-columns:repeat(auto-fit, minmax(220px, 1fr)); }
}

@media (max-width: 1024px) {
  .td-body { flex-direction:column; }
  .td-right { width:100%; flex-direction:row; align-items:stretch; }
  .td-qa-card, .td-notif-card { flex:1; }
  .td-stats { grid-template-columns:repeat(2,minmax(0,1fr)); }
  .td-mid { grid-template-columns:1fr; }
  .td-chart { min-height:230px; }
  .td-chart-area { height:150px; }
}

@media (max-width: 768px) {
  .td { min-height:0; }
  .td-banner { margin:-12px -16px 0; max-height:none; border-radius:0; }
  .td-banner-txt { position:relative; inset:auto; transform:none; padding:16px; }
  .td-banner-gradient,
  .td-banner img { display:none; }
  .td-right { flex-direction:column; }
  .td-stats { grid-template-columns:1fr; }
  .td-tasks { grid-template-columns:1fr; }
  .td-enc { flex-direction:column; align-items:flex-start; gap:12px; }
  .td-enc-txt,
  .td-enc-img { width:100%; margin-left:0; }
  .td-qa-grid { grid-template-columns:repeat(2,minmax(0,1fr)); }
}

@media (min-width: 1181px) and (max-height: 850px) {
  .td-banner { height:7rem; }
  .td-enc h3 { font-size:18px; }
  .td-enc p { font-size:12px; }
  .td-qa { padding:10px 6px; }
  .td-st { min-height:96px; padding-block:16px; }
  .td-task { min-height:122px; }
}
</style>
