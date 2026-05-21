# 设计稿规范索引

本目录是 `designs/training-evaluation2.pen` 的工程化"翻译"。前端开发时**严格按本目录的规范实现**，不得直接拷贝 Pencil 导出代码。

## 文档导航

| 文档 | 内容 |
|------|------|
| [01 设计 Token](01-design-tokens.md) | 颜色/字号/圆角/阴影变量，对应 Tailwind 配置 |
| **[02 HTML 参考映射](02-html-references.md)** | **每个 View → frontend-preview HTML 文件的强约束映射，前端 Task 必读** |
| [03 组件映射](03-components-mapping.md)（待补） | 24 个 Pencil 组件 → shadcn-vue 组件的对应关系 |
| [04 布局规范](04-layout-patterns.md)（待补） | 顶部导航布局、内容区结构、面包屑、间距规则 |
| [05 状态规范](05-state-patterns.md)（待补） | Loading 骨架屏、空状态、403/404/500 异常态 |
| [pages/](pages/)（按需追加） | 单个页面的逐项规范（仅当 HTML 不足以表达规则时才建） |

> **AI Agent 实施前端任务的强约束工作流**：
> 1. 读 [01 Token](01-design-tokens.md) 与 [02 HTML 参考](02-html-references.md)
> 2. 在 [02 HTML 参考](02-html-references.md) 表中找到对应 View 的 HTML 文件路径
> 3. 读 `frontend-preview/tokens.css` + `shared.css` + 该页 HTML 作为视觉契约
> 4. 用 shadcn-vue + Tailwind 复刻，**禁止直接拷贝 HTML 的 inline `<style>`**
> 5. 完成后 1:1 对比 HTML 静态页与 Vue 实现，5px 以内偏差才算合格

## 设计稿与项目的差异修正

我在转换时识别并纠正了以下设计稿瑕疵，**前端实现以本规范为准，不复刻原瑕疵**：

| # | 设计稿现象 | 项目实现 |
|---|---------|---------|
| 1 | sidebar 系列变量（侧边栏绿色） | 弃用，项目使用顶部导航布局 |
| 2 | `--font-serif` 与 `--font-sans` 同值 | 仅保留 `--font-sans` |
| 3 | `--font-cn` 与 `--font-sans` 重复 | 仅保留 `--font-sans` |
| 4 | `--font-mono` 默认 JetBrains Mono 可能字体缺失 | 加 fallback `Cascadia Code, Consolas, monospace` |
| 5 | 第 25 页"解析进度"独立页 | 合并为 GradingView 内部进度组件，不单独成路由 |
| 6 | 第 30 页"学生·任务详情" 与 第 8 页"成果上传"内容重叠 | 合并为 `student/TaskDetailView`，按提交状态切换上传区或已提交摘要 |
| 7 | 第 29 页"学校级"与第 12 页"教学画像"几乎相同 | 同一页面 `ProfileView`，URL 参数 `scope=course\|school` 区分 |
| 8 | 设计系统页（00 - 设计系统）含 sidebar 类项 | 在组件库实现时跳过 sidebar 相关组件 |
| 9 | 部分图表用纯色块占位 | 由 ECharts 包装组件接收 API 数据渲染 |
| 10 | 第 31 页"打回重做·确认" 是 modal 而非页面 | 实现为 `RejectConfirmDialog` 组件，全局调用 |

## 设计稿到项目页面的映射

设计稿共 32 个画板，去重合并后映射到 30 个前端 view（含 dialog 组件）。

> **完整路由 + HTML 参考路径**见 [02 HTML 参考映射](02-html-references.md)，本表仅做画板与 view 的最小映射。

| 设计稿编号 | 设计稿名称 | 项目页面/组件 | HTML 参考文件 |
|---|---|---|---|
| 00 | 设计系统 | （仅作 token 与组件库实现参考，无运行时页面） | - |
| 01 | 登录页 | `auth/LoginView.vue` | `pages/01-login.html` |
| 02 | 教师工作台 | `teacher/TeacherDashboard.vue` | `pages/02-teacher-dashboard.html` |
| 03 | 实训任务管理 | `teacher/TasksView.vue` | `pages/03-tasks.html` |
| 04 | 创建实训任务 | `teacher/TaskFormView.vue` | `pages/04-task-create.html` |
| 05 | 批改工作台 | `teacher/GradingView.vue` | `pages/05-grading.html` |
| 06 | 批改详情·并排对比 | `teacher/GradingDetailView.vue` | `pages/06-grading-detail.html` |
| 07 | 学生工作台 | `student/StudentDashboard.vue` | `pages/07-student-dashboard.html` |
| 08 + 30 | 学生·成果上传 + 任务详情 | `student/TaskDetailView.vue`（合并） | `pages/08-student-upload.html` + `pages/30-student-task-detail.html` |
| 09 | 学生·评价结果 | `student/EvaluationResultView.vue` | `pages/09-student-evaluation.html` |
| 10 | 班级管理 | `teacher/ClassesView.vue` | `pages/10-classes.html` |
| 11 | 评价模板库 | `shared/TemplatesView.vue` | `pages/11-templates.html` |
| 12 + 29 | 教学画像（课程/学校） | `shared/ProfileView.vue`（按 scope 切换） | `pages/12-teaching-profile.html` + `pages/29-school-profile.html` |
| 13 | 管理员·大模型配置 | `admin/LlmConfigView.vue` | `pages/13-llm-config.html` |
| 14 | 管理员·审计日志 | `admin/AuditView.vue` | `pages/14-audit.html` |
| 15 | 管理员·用户管理 | `admin/UsersView.vue` | `pages/15-users.html` |
| 16 | 学生·薄弱点画像 | `student/MyProfileView.vue` | `pages/16-student-profile.html` |
| 17 | 通知中心 | `shared/NotificationsView.vue` | `pages/17-notifications.html` |
| 18 | 报表中心 | `teacher/ReportsView.vue` | `pages/18-reports.html` |
| 19 | 管理员·总览 | `admin/AdminDashboard.vue` | `pages/19-admin-dashboard.html` |
| 20 | 学生·我的评价历史 | `student/HistoryView.vue` | `pages/20-student-history.html` |
| 21 | 批量导入用户 | `admin/UserImportView.vue` | `pages/21-import-users.html` |
| 22 | 相似度对比 | `teacher/SimilarityCompareView.vue` | `pages/22-similarity.html` |
| 23 | 课程管理 | `admin/CoursesView.vue` | `pages/23-courses.html` |
| 24 | 个人中心·设置 | `shared/AccountSettingsView.vue` | `pages/24-account-settings.html` |
| 25 | 解析进度·实时 | （合并入 GradingView 子组件，不单独成页） | `pages/25-parse-progress.html`（仅作组件参考） |
| 26 | 异常状态合集 | `shared/Error{403,404,500}.vue` + `EmptyState.vue` 组件 | `pages/26-error-states.html` |
| 27 | 忘记密码 | `auth/ForgotPasswordView.vue` | `pages/27-forgot-password.html` |
| 28 | 学生·AI 问答历史 | `student/ChatHistoryView.vue` | `pages/28-chat-history.html` |
| 31 | 打回重做·确认 | `RejectConfirmDialog.vue`（组件，非路由） | （Pencil 节点 `gzMSj`，未生成静态 HTML） |
| 32 | 加载骨架屏 | `Skeleton*.vue` 系列组件 | （Pencil 节点 `HkvkQ`，未生成静态 HTML） |

## 工作流

1. 写新页面前，**先读 [02 HTML 参考映射](02-html-references.md)**，按表查到对应 `frontend-preview/pages/XX-name.html`
2. 视觉规则按 `01-design-tokens.md`（token / Tailwind 配置）
3. shadcn-vue 组件选型按 `03-components-mapping.md`（待补，先以 HTML 中的 class 命名为线索）
4. 异常态与骨架屏：HTML 已展示在 `26-error-states.html`，按其样式实现
5. 提交 PR 时附 HTML 参考截图与 Vue 实现截图对比，5px 内偏差合格
6. 如发现 HTML 参考有 bug，**先改 HTML，再实现 Vue**，避免两侧分叉
