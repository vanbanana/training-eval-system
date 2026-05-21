# 02 HTML 参考（前端复刻权威映射）

> **本文件是前端实施 Task 的强约束输入。**
>
> 每个 Vue View / 关键组件，都映射到 `frontend-preview/pages/*.html` 中已经过设计稿验证的静态原型。AI Agent 在实施对应 Task 时，**必须先读对应 HTML 文件作为视觉契约**，再用 shadcn-vue + Tailwind CSS 复现，不得自由发挥。

## 1. 复刻规则（所有前端 Task 共享）

实施任意 Frontend View Task 之前，AI Agent 必须执行以下步骤：

1. **读 3 份基线**：
   - `frontend-preview/tokens.css`：颜色 / 圆角 / 字体变量
   - `frontend-preview/shared.css`：TopNav / Card / Button / Label / Table / Input 共用类
   - `frontend-preview/pages/{编号-名称}.html`：当前页面的视觉契约

2. **匹配规则（强约束）**：
   - 颜色 / 字号 / 圆角 / 间距：完全等价于对应的 Tailwind class（已在 `01-design-tokens.md` 配置过映射）
   - DOM 层级、模块顺序、卡片粒度：与 HTML 参考一致；若需要调整，先在 PR 描述里写"偏离原因"
   - 文案：使用 HTML 参考中的中文文案作为初始 i18n key 的源；不要发明新文案
   - 状态：HTML 中所有静态状态（hover、active、empty、error）都必须能在最终 Vue 实现中通过 prop 或 state 切到

3. **禁止**：
   - 直接拷贝 HTML 里的 `<style>` 标签或硬编码颜色（必须改用 Tailwind class 或 token）
   - 引入 HTML 里没有但"看起来更好看"的额外装饰元素
   - 自由替换 lucide 图标名（HTML 里用什么 icon-* 就用什么）

4. **允许**：
   - 把 HTML 里手写的卡片/按钮替换为 shadcn-vue 的等价组件（参见 `02-components-mapping.md`）
   - 把列表 mock 数据替换为 Pinia store + axios 请求
   - 把固定布局换成响应式 grid（HTML 默认 1440 宽，Vue 实现需 ≥ 1280 自适应）

5. **验证**：
   - 实现完成后，并排打开"`pages/XX.html`"和"`http://localhost:5173/{对应路由}`"
   - 1:1 视觉对比 < 5px 偏差视为合格
   - 如发现 HTML 参考有 bug 或与设计稿冲突，更新 HTML 文件后再合入 Vue 代码

---

## 2. View ↔ HTML 参考 主映射

| View / 组件 | 路由 | HTML 参考 | 角色 | 关键交互 |
|------------|------|----------|------|---------|
| `auth/LoginView` | `/login` | [01-login.html](../../frontend-preview/pages/01-login.html) | 公开 | 工号 + 密码登录、忘记密码链接 |
| `auth/ForgotPasswordView` | `/forgot-password` | [27-forgot-password.html](../../frontend-preview/pages/27-forgot-password.html) | 公开 | 三步重置：账号 → 验证码 → 新密码 |
| `teacher/TeacherDashboard` | `/dashboard` (teacher) | [02-teacher-dashboard.html](../../frontend-preview/pages/02-teacher-dashboard.html) | 教师 | 统计卡片、待批改、最近任务 |
| `teacher/TasksView` | `/teacher/tasks` | [03-tasks.html](../../frontend-preview/pages/03-tasks.html) | 教师 | 任务列表、状态筛选、TanStack Table |
| `teacher/TaskFormView` | `/teacher/tasks/new` <br/>`/teacher/tasks/:id/edit` | [04-task-create.html](../../frontend-preview/pages/04-task-create.html) | 教师 | 多步表单、维度权重 = 100 校验 |
| `teacher/GradingView` | `/teacher/tasks/:id/grading` | [05-grading.html](../../frontend-preview/pages/05-grading.html) | 教师 | 批改队列、解析进度（合并自第 25 页） |
| `teacher/GradingDetailView` | `/teacher/evaluations/:id` | [06-grading-detail.html](../../frontend-preview/pages/06-grading-detail.html) | 教师 | 并排：左原文 / 右 AI 评分，可批注、调分 |
| `student/StudentDashboard` | `/dashboard` (student) | [07-student-dashboard.html](../../frontend-preview/pages/07-student-dashboard.html) | 学生 | 待提交任务、最近评价、成长曲线 |
| `student/TaskDetailView` | `/student/tasks/:id` | [08-student-upload.html](../../frontend-preview/pages/08-student-upload.html) <br/>+ [30-student-task-detail.html](../../frontend-preview/pages/30-student-task-detail.html) | 学生 | 任务详情 + 上传区一体；按 submission 状态切换视图（合并自第 8/30 页） |
| `student/EvaluationResultView` | `/student/evaluations/:id` | [09-student-evaluation.html](../../frontend-preview/pages/09-student-evaluation.html) | 学生 | 五维雷达、AI 反馈、追问 chat |
| `teacher/ClassesView` | `/teacher/classes` | [10-classes.html](../../frontend-preview/pages/10-classes.html) | 教师 | 班级网格、学生统计 |
| `shared/TemplatesView` | `/templates` | [11-templates.html](../../frontend-preview/pages/11-templates.html) | 教师 / 管理员 | 评价模板库、系统模板 vs 自建 |
| `shared/ProfileView?scope=course` | `/profiles?scope=course` | [12-teaching-profile.html](../../frontend-preview/pages/12-teaching-profile.html) | 教师 | 课程级教学画像 |
| `shared/ProfileView?scope=school` | `/profiles?scope=school` | [29-school-profile.html](../../frontend-preview/pages/29-school-profile.html) | 管理员 | 学校级教学画像（同组件，scope 切换数据源） |
| `admin/LlmConfigView` | `/admin/llm` | [13-llm-config.html](../../frontend-preview/pages/13-llm-config.html) | 管理员 | LLM 提供商配置、API Key、熔断状态 |
| `admin/AuditView` | `/admin/audit` | [14-audit.html](../../frontend-preview/pages/14-audit.html) | 管理员 | 审计日志查询、JSON 详情抽屉 |
| `admin/UsersView` | `/admin/users` | [15-users.html](../../frontend-preview/pages/15-users.html) | 管理员 | 用户分页表、批量操作、停用/启用 |
| `admin/UserImportView` | `/admin/users/import` | [21-import-users.html](../../frontend-preview/pages/21-import-users.html) | 管理员 | CSV 导入向导、3 步流程 |
| `admin/CoursesView` | `/admin/courses` | [23-courses.html](../../frontend-preview/pages/23-courses.html) | 管理员 / 教师只读 | 课程列表、绑定教师与班级 |
| `admin/AdminDashboard` | `/dashboard` (admin) | [19-admin-dashboard.html](../../frontend-preview/pages/19-admin-dashboard.html) | 管理员 | 全局指标、系统状态、24h 活动 |
| `student/MyProfileView` | `/student/profile` | [16-student-profile.html](../../frontend-preview/pages/16-student-profile.html) | 学生 | 学生薄弱点画像 |
| `student/HistoryView` | `/student/history` | [20-student-history.html](../../frontend-preview/pages/20-student-history.html) | 学生 | 评价历史时间线 |
| `student/ChatHistoryView` | `/student/chat` | [28-chat-history.html](../../frontend-preview/pages/28-chat-history.html) | 学生 | AI 问答历史、左列表右消息流 |
| `shared/NotificationsView` | `/notifications` | [17-notifications.html](../../frontend-preview/pages/17-notifications.html) | 共用 | 通知中心、按类型筛选 |
| `teacher/ReportsView` | `/teacher/reports` | [18-reports.html](../../frontend-preview/pages/18-reports.html) | 教师 | 报表中心、PDF / Excel 导出 |
| `teacher/SimilarityCompareView` | `/teacher/similarity/:id` | [22-similarity.html](../../frontend-preview/pages/22-similarity.html) | 教师 | 相似度对比、双栏 diff |
| `shared/AccountSettingsView` | `/account` | [24-account-settings.html](../../frontend-preview/pages/24-account-settings.html) | 共用 | 个人设置、密码、通知偏好 |

## 3. 进度 / 异常 / 骨架专用组件

| 组件 | 内嵌于 / 路由 | HTML 参考 | 说明 |
|------|--------------|----------|------|
| `EvaluationProgressPanel` | 嵌入 GradingView | [25-parse-progress.html](../../frontend-preview/pages/25-parse-progress.html) | 解析进度面板（不单独成页） |
| `Error403View` / `Error404View` / `Error500View` / `EmptyState` / `LlmDegradedBanner` | `/403` `/404` `/500` 等 | [26-error-states.html](../../frontend-preview/pages/26-error-states.html) | 异常态合集 |
| `RejectConfirmDialog` | 全局 dialog | （Pencil 节点 `gzMSj`） | 打回重做确认弹窗 |
| `Skeleton*` 系列 | 各页加载态 | （Pencil 节点 `HkvkQ`） | 骨架屏 |

> 节点 ID 标注的两项尚未生成静态 HTML，由后续 PR 补齐；前端实施时如遇到，可暂用 shadcn-vue 的 Dialog / Skeleton 占位。

## 4. 索引页

`frontend-preview/index.html` 是所有静态页面的入口卡片墙。AI Agent 启动前端 Epic 时建议：

```bash
# 在浏览器打开
start frontend-preview/index.html   # Windows
open  frontend-preview/index.html   # macOS
```

逐页对照实现，每完成一个 View 就在对应卡片旁打钩。
