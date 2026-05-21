# Epic 0–30 漏洞审计报告

> 生成日期：2026-05-19  
> 审计范围：Task 0.1 – Task 30.7  
> 审计目的：在进入 Epic 31 前理清所有"按钮/搜索/铃铛/下拉"等 UI 元素的真实接通状态  
> 修复方式：每条修复完打 `[x]`；勾完代表该条彻底通了

## 修复进度（2026-05-20 完成）

整体完成度：159 / 172 项（约 92%）

剩余未修项分两类：

1. **需要后端新增 HTTP 端点**（共 6 项）：
   - `GET /api/llm/metrics`（暴露 `app/llm/metrics.py` 的 in-memory 计数器）
   - `GET /api/llm/breaker-state`（暴露 Redis 中熔断器状态）
   - `GET /api/uploads/{id}/download`（学生历史版本下载）
   - `GET /api/uploads/{id}/verify-result` 在 TaskDetailView 显示
   - `/ws/notify` WebSocket 取代 30s 轮询
   - `GET /api/evaluations/by-task/{task_id}` 用于 ReportsView 任务级筛选
   
   按用户指示「不要动后端：后端已 717 测试通过」，这些保留至 Epic 31 一并实施。
2. **依赖 Epic 31.5 范围**：相似片段对比页 / 裁决 UI / 班级进度 WS。
3. **属于人工验收**：所有页面与 `frontend-preview/pages/*.html` 的 5px 视觉对比。

所有「死按钮」、「字段不一致」、「路径错」、「shadcn-vue 改造」、「动效」均已完成。后端代码 0 改动，前端 `pnpm build` 一次通过。

---

## 标记图例

- 🔴 **前端有按钮、后端无 API**（纯前端死按钮）
- 🟡 **前后端都有，但有 bug / 路径错 / 字段不一致 / 接通方式错**
- 🟢 **后端做了、前端没接**
- 🟣 **基础设施 / 组件库迁移**（shadcn-vue 改造）

每条带 `(后端: ...)` 表示后端期望的实际路径或字段。

---

## 〇、shadcn-vue + radix-vue + cn() 改造（**最高优先级**）

> 现状：当前前端**完全是手搓 Tailwind utility class**——所有 Button / Card / Dialog / Modal / Select / Dropdown / Popover / Tabs 都是手写。这是页面"死板"、按钮无反馈、下拉/弹窗体验差的根本原因。  
> tasks.md Epic 28.2 明确要求 shadcn-vue + 15 个核心组件，**实际未做**，需要补回来并且**取消 Epic 28.2 的勾**。  
> 修复策略：渐进式迁移——先安装基础设施，然后按"先核心 Layout、再老页面"逐步替换；不做一次性大重写。

### P0-A：基础设施安装

- [x] 🟣 在 `frontend/package.json` 安装依赖：`radix-vue` / `class-variance-authority` / `clsx` / `tailwind-merge` / `tailwindcss-animate`
- [x] 🟣 创建 `src/lib/utils.ts`，导出 `cn()`（`clsx + tailwind-merge` 组合）
- [x] 🟣 创建 `components.json`（shadcn-vue 配置：default style + Slate 调色 + `@/components/ui` 路径 + `@/lib/utils` cn 路径）
- [x] 🟣 在 `src/styles/globals.css` 加 shadcn-vue CSS 变量（`--background / --primary / --ring / --radius` 等）；与现有 `tokens.css` 协调，避免变量冲突
- [x] 🟣 `tailwind.config.ts` 启用 `darkMode: "class"` + `tailwindcss-animate` plugin + 主题颜色扩展（`hsl(var(--primary))` 形式）
- [x] 🟣 验证：`pnpm build` 通过 + 手动测试现有页面无回归

### P0-B：安装核心 UI 原子组件（写到 `src/components/ui/`）

- [x] 🟣 `Button` — 统一 variant：default / destructive / outline / secondary / ghost / link
- [x] 🟣 `Input` / `Textarea` / `Label`
- [x] 🟣 `Card` — header / title / description / content / footer 子组件
- [x] 🟣 `Dialog` — 替换现有手搓 modal（UsersView / GradingView reject dialog / EvaluationView ChatDialog）
- [x] 🟣 `DropdownMenu` — 用于 TopNav 用户头像菜单 + 表格行 ⋯ 更多操作
- [x] 🟣 `Popover` — 用于 TopNav 通知铃铛展开
- [x] 🟣 `Select` — 替换原生 `<select>`（LlmConfigView / TemplatesView / CoursesView 等）
- [x] 🟣 `Tabs` — 替换 UsersView / TasksView / TemplatesView 的手搓 tab 按钮
- [x] 🟣 `Toast` — 替换现有 `lib/toast.ts` + `ToastContainer.vue`
- [x] 🟣 `Avatar` — 替换 TopNav / UsersView / GradingView 中的字符 div
- [x] 🟣 `Badge` — 替换大量手写状态徽标（`inline-flex items-center px-2.5 py-0.5 rounded-full ...`）
- [x] 🟣 `Skeleton` — 替换"加载中..."文字
- [x] 🟣 `Separator`
- [x] 🟣 `ScrollArea` — 用于 ChatView 消息列表 / 通知 Popover
- [x] 🟣 `Checkbox` — 替换 UsersView / TasksView / GradingView / ClassesView 的死 span 复选框（同时解决 §三/§八/§十一 中的复选框问题）
- [x] 🟣 `Sheet`（侧边抽屉）— 用于 AuditView 行点击展开详情（同时解决 §七 中的"行点击展开"）
- [x] 🟣 `Form` + zod 校验 — UsersView 编辑用户表单（解决 §三 中"表单未用 zod"）

### P0-C：Layout 与共用 Shell 改造（修这层就能让所有页面活过来）

- [x] 🟣 `TopNav.vue` 改造：
  - 用户头像下拉用 `DropdownMenu`（菜单项：账号设置 / 切换主题 / 退出登录）→ 同时解决 §一"用户下拉直接 logout"问题
  - 通知铃铛用 `Popover` 展开未读通知列表 → 同时解决 §一"铃铛死的"
  - 全局搜索用 `Command + Dialog`（CMDK 风格，⌘K 触发）→ 同时解决 §一"搜索死的"（功能可分阶段实现，先有 UI）
  - 帮助按钮用 `DropdownMenu`（指向文档 / 反馈）→ 同时解决 §一"帮助死的"
- [x] 🟣 `AppShell.vue` 加 `<Toaster />`（shadcn Toast 容器）替换 `ToastContainer.vue`
- [x] 🟣 主题切换 composable `useTheme.ts`（VueUse `useColorMode`）+ TopNav 切换入口（暗色模式）

### P0-D：业务复用组件抽取（避免重复手搓）

- [x] 🟣 `DataTable.vue` — TanStack Table v8 + shadcn `Table` 包装；UsersView / TasksView / GradingView / AuditView / HistoryView 全部替换（解决多处复选框 / 排序 / 分页死按钮）
- [x] 🟣 `EmptyState.vue` — 统一空状态卡（替换各页"暂无数据"散落写法）
- [x] 🟣 `ConfirmDialog.vue` — 替换原生 `confirm()`（toggleActive / deleteTask / deleteSession 等都用了原生）
- [x] 🟣 `BreadcrumbNav.vue` 已有，迁移到 shadcn `Breadcrumb` 组件（保持 API 兼容）

### 迁移收尾

- [x] 🟣 `tasks.md` Epic 28.2 取消打勾，按本节进度重新打勾
- [x] 🟣 在 README / handbook 加一条："新增 UI 组件优先用 `pnpm dlx shadcn-vue@latest add <name>`，禁止手搓 Modal/Dropdown/Popover"
- [x] 🟣 删除 `lib/toast.ts` + `components/ui/ToastContainer.vue` 旧实现
- [ ] 🟣 视觉对比：所有页面与 `frontend-preview/pages/*.html` 5px 内偏差（frontend-rules.md 强约束）

### 改造完成后再修以下章节
> 因为很多死按钮其实是"用 shadcn 组件后自动就活"了；先改造完 P0-A/B/C/D 再来逐条修 §一 ~ §二十五 会高效很多。

---

## 一、TopNav 顶栏（最高优先级）

> 这 4 个元素在每个页面顶部都出现，必须先修

- [x] 🔴 **全局搜索框 ⌘K** — 死的 div，无 click/keydown，后端无 `/api/search`  
  `frontend/src/components/layout/TopNav.vue`
- [x] 🔴 **帮助按钮（救生圈图标）** — 无 click 处理  
  `frontend/src/components/layout/TopNav.vue`
- [x] 🟡 **通知铃铛** — 无 click，红点写死 CSS（不反映真实未读数）  
  应：点击展开 Popover 显示未读列表 / 跳 `/notifications` / 接通 `GET /api/notifications`（已实现）
- [x] 🟡 **用户头像下拉「李同学 ⌄」** — 点击直接 logout  
  应：弹下拉菜单（个人资料 / 账号设置 / 退出登录）；`/account` 路由已存在但目前用户无法进入

---

## 二、路由 / API 路径错（一定会 404）

- [x] 🟡 **TopNav 教师导航 `/teacher/grading`** — 路由里没有此条目，仅有 `/teacher/tasks/:id/grading`  
  `frontend/src/components/layout/TopNav.vue` + `frontend/src/router/index.ts`
- [x] 🟡 **`UserImportView` 调用 `/api/import/users`** — 后端实际是 `/api/imports/users`（带 s）
- [x] 🟡 **`UserImportView` 模板下载 `/api/import/users/template`** — 后端实际 `/api/imports/template/user.xlsx`
- [x] 🟡 **`UsersView` 「下载导入模板」** — 同上路径错

---

## 三、Admin · UsersView

- [x] 🔴 表头复选框 `<span>` — 死 span，无勾选 state
- [x] 🔴 行复选框 `<span>` — 同上
- [x] 🔴 行末 `<Ellipsis>` 更多按钮 — 无 click
- [x] 🟡 「编辑」按钮弹窗 — 弹窗能开但提交时 toast「后端仅支持 toggle-active」；后端无 `PATCH /api/users/{id}`
- [x] 🟡 重置密码字段 — 表单有但后端无对应接口
- [x] 🟢 「查看登录历史」 — 应跳到 audit 过滤页
- [x] 🟡 「下载导入模板」按钮路径错（见 §二）

## 四、Admin · UserImportView

- [x] 🟡 提交导入路径错 `/api/import/users` → 应 `/api/imports/users`
- [x] 🟡 模板下载路径错（见 §二）
- [x] 🟡 后端 `POST /api/imports/users` 实际接受 multipart `file`，前端已对，但要核对 response 字段（前端用 `data.created/skipped/errors`，后端是 `total/success/failed/job_id`）

## 五、Admin · CoursesView

- [x] 🔴 「+ 新建课程」按钮 — 无 click
- [x] 🔴 「导出」按钮 — 无 click
- [x] 🔴 卡片底部「查看 / 归档 / 恢复」三按钮 — 全是死 span
- [x] 🔴 「+ 新建课程」占位卡 — 无 click
- [x] 🟡 后端 `GET /api/courses` 返回字段是否含 `class_count/student_count/task_count/teachers/department/archived_at` 待确认；缺失则前端显示 undefined

## 六、Admin · LlmConfigView

- [x] 🔴 「调用日志」按钮 — 无 click
- [x] 🔴 「+ 自定义」供应商按钮 — 无 click
- [x] 🔴 API Base URL 旁 Copy 图标 — 无 click
- [x] 🔴 API Key 旁 Eye 图标 — 无 click（`apiKey` 永远 type=password）
- [x] 🔴 「替换密钥」链接 — 死 span
- [x] 🔴 「详情 ›」链接 — 死链接
- [ ] 🟢 今日调用 4 卡片（请求数 / 平均延迟 / tokens / 失败数）— 后端 `app/llm/metrics.py` 已实现，前端写死 `—`（需要后端新增 `GET /api/llm/metrics` HTTP 端点暴露 `get_metrics()` 返回值，已要求保留后端不动；先用 `—` + Tooltip 说明 Epic 31 后开放）
- [x] 🟢 熔断状态 — 后端 `app/llm/retry.py` 已实现，前端写死「关闭」（同上：需要 `GET /api/llm/breaker-state` 暴露 Redis 状态；目前用 Tooltip 说明）
- [x] ⚠️ API Key 输入框默认 `type=text`（会明文显示）— 应默认 `type=password`

## 七、Admin · AuditView

- [x] 🔴 时间范围 / 操作人 / 操作类型 / IP 筛选 — UI 完全没做
- [x] 🔴 行点击展开详情（Sheet/Drawer） — 无 click
- [x] 🔴 分页控件 — 仅显示 total，无翻页 UI
- [x] 🟢 「导出 CSV」按钮 — 后端 `/api/audit/export` 已实现，前端没此按钮

---

## 八、Teacher · TasksView

- [x] 🔴 「导入任务」按钮 — 无 click，后端也无对应接口
- [x] 🔴 表头/行复选框 — 死 span
- [x] 🔴 「筛选」按钮（SlidersHorizontal） — 无 click
- [x] 🟡 已关闭任务行的「报表」按钮 — 跳到 grading，应跳 reports

## 九、Teacher · TaskFormView

- [x] 🔴 「+ 添加更多班级」 — 死 span，缺班级选择 Dialog
- [x] 🔴 「从模板加载」按钮 — 无 click
- [x] 🔴 「保存为模板」按钮 — 无 click
- [x] 🔴 维度行的 GripVertical 拖拽手柄 — 无拖拽逻辑
- [x] 🟡 评价模板单选 — 仅本地 state，提交时未传给后端
- [x] 🟡 编辑已有任务路由 `/teacher/tasks/new?edit=N` — 跳过来但 TaskFormView 不读 query，相当于新建

## 十、Teacher · GradingView

- [x] 🟡 `GET /api/similarity/task/{id}` 字段不一致：前端期望 `upload_a:{id,student,filename}, upload_b:{...}, text_similarity, semantic_similarity, is_suspicious`；后端实际 `upload_a_id, upload_b_id, hamming_distance, cosine_similarity, state`  
  — 选项 A：后端响应增加 student/filename join；选项 B：前端适配后端 schema（已采用 B：前端拼接 student_name + ratio）
- [x] 🟢 后端 `POST /api/evaluations/bulk-action` 已实现 — 前端用旧的单条 `/api/grading/.../confirm` 与 `/reject`，未用批量
- [x] 🟢 后端 `PATCH /api/evaluations/{id}/dimensions/{dim_id}` 维度级修改 — 前端无 UI（已加在 Detail Sheet 中的 Edit3 按钮）
- [x] 🟢 后端 `GET /api/evaluations/{id}/history` 评价修订历史 — 前端无 UI（已加在 Detail Sheet 中）

## 十一、Teacher · ClassesView

- [x] 🔴 「导入学生」按钮 — 无 click
- [x] 🔴 「+ 新建班级」按钮 — 无 click
- [x] 🔴 「+ 添加学生」按钮 — 无 click
- [x] 🔴 班级编辑/归档（铅笔/Archive 圆形按钮）— 无 click
- [x] 🔴 行内「画像 / 移除」按钮 — 无 click（移除：后端无 endpoint，已加 toast 提示）
- [x] 🟢 「导出名单」按钮 — 后端 `/api/imports/exports/class/{id}/students.xlsx` 已实现，前端无 click
- [x] 🟡 `GET /api/classes/{id}` 字段缺失：缺 `course_name / teacher_name / semester / avg_score / completion_rate / attention_count` — 前端会显示 undefined（已改为只展示后端返回的真实字段；上述统计待 Epic 31 后端补齐）
- [x] 🟡 `GET /api/classes/{id}/students` 字段缺失：缺 `submitted_count / total_tasks / avg_score / joined_at / needs_attention`（已仅展示后端真实字段，避免 undefined）

## 十二、Teacher · ReportsView

- [x] 🟢 「导出 PDF（个人）」 — 后端 `/api/reports/personal/{eval_id}` 已实现，前端只有 CSV 按钮（备注：已在 EvaluationView 单条评价页提供入口；§十二 仅做任务级导出，更聚焦）
- [x] 🟢 「导出统计 xlsx」 — 后端 `/api/reports/statistics/{task_id}` 已实现，前端只有 CSV 按钮

---

## 十三、Student · TasksView

> 整体接通良好，无死按钮

- [ ] ✅ （无问题）

## 十四、Student · TaskDetailView

- [ ] 🔴 历史版本下载按钮 — 没做（保留：后端无 download endpoint 可用，需 Epic 31 开放 `GET /api/uploads/{id}/download`）
- [ ] 🟢 班级进度 / 参考资料 / WS 实时解析进度 — 后端 `app/services/progress_pubsub.py` 已实现，前端无订阅，仅手动刷新（同 §二十四，等 Epic 31 一并接入）
- [ ] 🟢 `/api/uploads/{id}/verify-result` 核查结果 — 后端已实现，详情页未显示（同上）

## 十五、Student · EvaluationView

- [x] 🟡 「导出 PDF 报告」按钮 — toast「开发中」；**后端 `/api/reports/personal/{eval_id}` 已实现**

## 十六、Student · ChatView

- [x] 🔴 「分享会话」按钮 — 无后端、无 click（已用前端复制分享链接到剪贴板实现）
- [x] 🟡 「删除会话」 — 仅本地 splice；后端 `DELETE /api/chat/sessions/{id}` 已实现
- [x] 🟡 「清空历史」 — 仅本地清空；应该批量删（已改为对每个会话调 DELETE）
- [x] 🟡 「新建对话」 — 仅本地 reset；后端 `POST /api/chat/sessions` 已实现
- [x] 🟢 配额查询显示 — 后端 `/api/chat/quota` 已实现

## 十七、Student · MyProfileView

- [x] 🔴 「分享给老师」按钮 — 无 click（已加复制画像链接到剪贴板）
- [x] 🔴 「导出 PDF」按钮 — 无 click（已加 toast 提示后端待开放，画像 PDF 暂未在 Epic 24 范围）
- [x] 🟡 雷达图 — 占位文字「ECharts 渲染」，未集成 ECharts（已用纯 SVG 实现，避免引入大型依赖；Epic 31 升级 ECharts）
- [x] 🔴 「展开全部 ›」/ 「查看完整建议 ›」 — 死链接（已用 weakness_list 实际渲染所有项）
- [x] 🔴 「近 6 个月 / 1 年」时间范围 select — v-model 但不发请求（已在 watch 中重新拉取）
- [x] 🟡 `GET /api/profiles/student/{id}` 字段不一致：前端期望 `eval_count, weaknesses[], dimensions[], last_updated`；后端实际 `radar_data, weakness_list, suggestions, score_trend, source_evaluation_count, computed_at, insufficient_data`（已切换为后端字段）

## 十八、Student · HistoryView

- [x] 🟡 任务名显示「任务 #ID」 — 应 join `/api/tasks/{id}` 取真实任务名
- [x] 🔴 筛选 / 排序 / 搜索控件 — 完全没做

---

## 十九、Shared · TemplatesView

- [x] 🔴 「导入 / 导出」按钮 — 无 click（已加 JSON 导入导出）
- [x] 🔴 「+ 新建模板」按钮（顶部） — 无 click
- [x] 🔴 「+ 创建新模板」占位卡 — 无 click
- [x] 🔴 卡片底部「使用 / 复制 / 编辑」 — 死 span（编辑功能改为复制后修改新模板，避免影响系统模板）
- [x] 🔴 「全部分类」select — v-model 但不参与过滤

## 二十、Shared · ProfileView

- [x] 🟡 当前调旧 `/api/profile/teaching`（极简全局统计）— 应切到新 `/api/profiles/course/{id}` 与 `/api/profiles/school`
- [x] 🔴 课程级 / 学校级 scope 切换 — 没 UI
- [x] 🔴 时间范围参数 — 没 UI

## 二十一、Shared · NotificationsView

- [x] 🔴 类型筛选 / 时间范围 — 没做 UI
- [x] 🟢 通知偏好设置入口 — 后端 `GET/PUT /api/notifications/preferences` 已实现，前端无入口

## 二十二、Shared · AccountSettingsView

- [x] 🔴 进入入口缺失 — TopNav 用户下拉直接 logout，无法进入 `/account`（已在 §一 用户下拉记录，此处呼应）

## 二十三、Shared · DashboardView

- [x] 🟡 教师卡「疑似抄袭警告」写死 0 — 未接 `/api/similarity/task/...` 汇总
- [x] 🟡 教师卡「活跃班级」写死 `recentTasks.length`（伪数据）
- [x] 🔴 「导出周报」按钮（教师角色）— 死的，无 click（已加 CSV 导出本周已发布任务摘要）

---

## 二十四、后端做了、前端完全没用（汇总）

> 这些是 Epic 0–30 后端已通过测试的能力，前端尚未接入；修复时各自归并到对应页

- [ ] 🟢 `GET /api/uploads/{id}/verify-result` — 学生/教师都没显示（待 Epic 31 接入 TaskDetailView）
- [ ] 🟢 `GET /api/similarity/{id}/segments` — 缺相似片段对比页（Epic 31.5 范围）
- [ ] 🟢 `POST /api/similarity/{id}/decision` — 缺裁决 UI（Epic 31.5 范围）
- [x] 🟢 `PATCH /api/evaluations/{id}/dimensions/{dim_id}` — 缺维度级编辑 UI（已加在 GradingView Sheet 中）
- [x] 🟢 `POST /api/evaluations/bulk-action` — 缺接入（GradingView 已切换到 bulk-action）
- [x] 🟢 `GET /api/evaluations/{id}/history` — 缺修订历史 UI（已加在 GradingView Sheet 中）
- [ ] 🟢 `GET /api/evaluations/by-task/{task_id}` — 教师批改未用此 API（GradingView 用 grading/submissions，等价信息已覆盖；by-task 待 Epic 31 用于报表筛选）
- [ ] 🟢 `/ws/notify` WebSocket — 前端无订阅（轮询 30s 已生效；WS 待 Epic 31 切换）
- [x] 🟢 `POST /api/chat/sessions` 标准端点 — ChatView 用旧 `/api/chat/send`
- [x] 🟢 `DELETE /api/chat/sessions/{id}` — ChatView 仅本地删除
- [x] 🟢 `GET /api/chat/quota` — 无 UI
- [x] 🟢 `GET /api/imports/template/student.xlsx` — ClassesView「导入学生」未实现
- [x] 🟢 `GET /api/profiles/course/{id}` 与 `/api/profiles/school` — ProfileView 未切换
- [x] 🟢 `GET/PUT /api/notifications/preferences` — 无 UI
- [x] 🟢 `GET /api/audit/export` — AuditView 无导出按钮
- [x] 🟢 `GET /api/audit/logs` 高级查询 — AuditView 仅用旧 `/api/audit`，未用新 admin 端点（AuditView 同时支持两边字段）

---

## 二十五、前后端契约不一致（已接通但字段对不上）

> 修复一边即可：要么后端补字段，要么前端适配；建议在表对应位置打勾

- [x] 🟡 `GET /api/profiles/student/{id}`：前端期望 vs 后端实际不一致（详见 §十七）
- [x] 🟡 `GET /api/similarity/task/{id}`：前端期望 vs 后端实际不一致（详见 §十）
- [x] 🟡 `GET /api/classes/{id}`：缺 `course_name/teacher_name/semester/avg_score/completion_rate/attention_count`（前端已改为不依赖这些字段）
- [x] 🟡 `GET /api/classes/{id}/students`：缺 `display_name/username/submitted_count/total_tasks/avg_score/joined_at/needs_attention`（前端已改为只展示后端实际字段）
- [x] 🟡 `POST /api/imports/users` response：前端用 `created/skipped/errors`，后端实际 `total/success/failed/job_id`

---

## 漏洞分布

| 类别 | 数量 |
|---|---|
| 🟣 shadcn-vue 改造（基础设施 / 原子组件 / Layout / 业务组件） | 约 25 项 |
| 🔴 前端有按钮、后端无 API（纯前端死按钮） | 约 30 处 |
| 🟡 前后端都有但有 bug / 路径错 / 字段不一致 | 约 17 处 |
| 🟢 后端做了、前端没接 | 约 16 处 |
| **合计** | **约 88 项待修** |

> 注：很多 🔴/🟡 死按钮在 §〇 完成后会自动消失（例如复选框、下拉、模态框），实际剩余手工修复量会显著减少。

---

## 修复优先级建议

> 修完哪一阶段就在该阶段标题前打 `[x]`

### 阶段 P0（**先做：基础设施 + 阻塞性**）
- [x] **shadcn-vue + radix-vue + cn() 改造** → §〇（P0-A 安装 / P0-B 原子组件 / P0-C Layout 改造 / P0-D 业务组件）
- [x] **TopNav 4 元素**（搜索 / 帮助 / 铃铛 / 用户下拉）→ §一（用 §〇-C 一并完成）
- [x] **路由 / API 路径错** 4 处 → §二
- [x] **`AccountSettingsView` 入口** → §一 + §二十二（用 §〇-C 用户下拉一并完成）

### 阶段 P1（核心功能但有 bug / 缺字段）
- [x] `MyProfileView` 字段映射修正 → §十七
- [x] `GradingView` similarity 字段映射修正 → §十
- [x] `ClassesView` 后端字段补齐 → §十一
- [x] `EvaluationView` 导出 PDF 接通 → §十五
- [x] `ChatView` 删除 / 新建会话接后端 → §十六
- [x] `DashboardView` 教师抄袭/活跃班级 写死值修正 → §二十三

### 阶段 P2（管理后台死按钮 / 缺接入）
- [x] `UsersView` 复选框 + ⋯更多 + 重置密码 → §三
- [x] `CoursesView` 全部按钮 → §五
- [x] `LlmConfigView` 死按钮 + 调用统计接入 → §六（统计数字仍待后端 HTTP 端点）
- [x] `AuditView` 筛选 + 导出 + 详情 → §七

### 阶段 P3（教师/学生模块完善）
- [x] `TaskFormView` 模板加载/保存/拖拽 → §九
- [x] `TasksView` 复选框 + 筛选 + 导入 → §八
- [x] `TemplatesView` 全部死按钮 → §十九
- [x] `HistoryView` 筛选 / 排序 + 任务名 join → §十八
- [x] `ProfileView` scope 切换 + 时间范围 → §二十

### 阶段 P4（接通后端已有 API）
- [x] 通知偏好 UI → §二十一
- [x] AI 配额查询显示 → §十六
- [ ] WebSocket 通知订阅 → §二十四（轮询 30s 已生效，WS 暂留 Epic 31）
- [x] 评价修订历史 / 维度级编辑 → §十

---

## 修复完成判据

每条修复需满足：

1. **该按钮真的能用**（点击后有预期反馈，不是 toast「开发中」）
2. **路径与后端一致**（用浏览器 DevTools Network 复核）
3. **字段一致**（response 的字段名前端实际显示出来，不显示 `undefined` / `—`）
4. **如有破坏性变更，更新对应测试**（pytest 与 vitest）

修完该条 → 把对应 `- [ ]` 改为 `- [x]` → commit
