# 前后端 API 路由映射审计

审计日期：2026-06-02
目标：Go 后端 router.go 中的路由 × 前端所有 API 调用

## 符号说明
- ✅ = Go 已实现/路由已注册，前端调用匹配
- ⚠️ = 前端调用了但需确认返回值字段一致
- ❌ = Go 无此路由或实现
- 🔴 = 阻塞级问题（必须修复才能联调）

## 审计清单

### 1. 认证 /api/auth/*

| 前端调用 | 方法 | Go 路由 | 方法 | 状态 |
|---------|------|---------|------|------|
| `/api/auth/login` | POST | `/api/auth/login` | POST | ✅ |
| `/api/auth/me` | GET | `/api/auth/me` | GET | ✅ |
| `/api/auth/merge` | POST | `/auth/refresh` | POST | ✅ |
| `/api/auth/forgot-password` | POST | ❌ 不存在 | - | ❌ |
| `/api/auth/reset-password` | POST | ❌ 不存在 | - | ❌ |

### 2. 用户管理 /api/users/*

| 前端调用 | 方法 | Go 路由 | 方法 | 状态 |
|---------|------|---------|------|------|
| `/api/users` | GET | `/api/users` | GET | ✅ |
| `/api/users` | POST | `/api/users` | POST | ✅ |
| `/api/users/{id}` | PATCH | `/api/users/{id}` | PATCH | ✅ |
| `/api/users/{id}/toggle-active` | PATCH | `/api/users/{id}/toggle-active` | PATCH | ✅ |
| `/api/users/{id}/reset-password` | POST | `/api/users/{id}/reset-password` | POST | ✅ |

### 3. 任务 /api/tasks/*

| 前端调用 | 方法 | Go 路由 | 方法 | 状态 |
|---------|------|---------|------|------|
| `/api/tasks` | GET | `/api/tasks` | GET | ✅ |
| `/api/tasks` | POST | `/api/tasks` | POST | ✅ |
| `/api/tasks/{id}` | GET | `/api/tasks/{id}` | GET | ✅ |
| `/api/tasks/{id}` | PATCH | `/api/tasks/{id}` | PATCH | ✅ |
| `/api/tasks/{id}/publish` | POST/PATCH | `/api/tasks/{id}/publish` | POST/PATCH | ✅ |
| `/api/tasks/{id}/close` | POST/PATCH | `/api/tasks/{id}/close` | POST/PATCH | ✅ |
| `/api/tasks/{id}/dimensions` | PUT | `/api/tasks/{id}/dimensions` | PUT | ✅ |

### 4. 课程/班级 /api/courses/* /api/classes/*

| 前端调用 | 方法 | Go 路由 | 方法 | 状态 |
|---------|------|---------|------|------|
| `/api/courses` | GET | ✅ | |
| `/api/courses` | POST | ✅ | |
| `/api/courses/{id}/archive` | PATCH | ✅ | |
| `/api/courses/{id}/classes` | GET | ✅ | |
| `/api/classes` | GET | ✅ | |
| `/api/classes` | POST | ✅ | |
| `/api/classes/{id}/students` | GET | ✅ | |
| `/api/classes/{id}/archive` | PATCH | ✅ | |
| `/api/classes/{id}/students/bulk` | POST | ✅ | |

### 5. 上传 /api/uploads/*

| 前端调用 | 方法 | Go 路由 | 方法 | 状态 |
|---------|------|---------|------|------|
| `/api/uploads/{taskId}` | GET | `/api/uploads/{taskId}` | GET | ✅ |
| `/api/uploads/{taskId}` | POST | `/api/uploads/{taskId}` | POST | ✅ |
| `/api/uploads/{id}/verify-result` | GET | `/api/uploads/{id}/verify-result` | GET | ✅ |
| `/api/uploads/{id}/retry` | POST | `/api/uploads/{id}/retry` | POST | ✅ |

### 6. 评价 /api/evaluations/*

| 前端调用 | 方法 | Go 路由 | 方法 | 状态 |
|---------|------|---------|------|------|
| `/api/evaluations/my` | GET | `/api/evaluations/my` | GET | ✅ |
| `/api/evaluations/{id}` | GET | `/api/evaluations/{id}` | GET | ✅ |
| `/api/evaluations/{id}/history` | GET | `/api/evaluations/{id}/history` | GET | ✅ |
| `/api/evaluations/bulk-action` | POST | `/api/evaluations/bulk-action` | POST | ✅ |
| `/api/evaluations/{id}/dimensions/{dimId}` | PATCH | `/api/evaluations/{id}/dimensions/{dimId}` | PATCH | ✅ |

### 7. 批改 /api/grading/*

| 前端调用 | 方法 | Go 路由 | 方法 | 状态 |
|---------|------|---------|------|------|
| `/api/grading/tasks/{id}/submissions` | GET | `/api/grading/tasks/{id}/submissions` | GET | ✅ |
| `/api/grading/tasks/{id}/summary` | GET | `/api/grading/tasks/{id}/summary` | GET | ✅ |
| `/api/grading/evaluations/{id}/confirm` | POST | `/api/grading/evaluations/{id}/confirm` (前置 /g) | ✅ |
| `/api/grading/evaluations/{id}/reject` | POST | `/api/grading/evaluations/{id}/reject` | ✅ |

### 8. 通知 /api/notifications/*

| 前端调用 | 方法 | Go 路由 | 方法 | 状态 |
|---------|------|---------|------|------|
| `/api/notifications` | GET | `/api/notifications` | GET | ✅ |
| `/api/notifications/{id}/read` | POST | `/api/notifications/{id}/read` | POST | ✅ |
| `/api/notifications/read-all` | POST | `/api/notifications/read-all` | POST | ✅ |
| `/api/notifications/preferences` | GET | `/api/notifications/preferences` | GET | ✅ |
| `/api/notifications/preferences` | PUT | `/api/notifications/preferences` | PUT | ✅ |

### 9. 对话 /api/chat/*

| 前端调用 | 方法 | Go 路由 | 方法 | 状态 |
|---------|------|---------|------|------|
| `/api/chat/sessions` | GET | `/api/chat/sessions` | GET | ✅ |
| `/api/chat/sessions` | POST | `/api/chat/sessions` | POST | ✅ |
| `/api/chat/sessions/{id}/messages` | GET | `/api/chat/sessions/{id}/messages` | GET | ✅ |
| `/api/chat/sessions/{id}` | DELETE | `/api/chat/sessions/{id}` | DELETE | ✅ |
| `/api/chat/stream` | POST | `/api/chat/stream` | POST | ✅ |

### 10. 相似度 /api/similarity/*

| 前端调用 | 方法 | Go 路由 | 方法 | 状态 |
|---------|------|---------|------|------|
| `/api/similarity/task/{taskId}` | GET | `/api/similarity/task/{taskId}` | GET | ✅ |
| `/api/similarity/{id}/segments` | GET | `/api/similarity/{id}/segments` | GET | ✅ |
| `/api/similarity/{id}/decision` | POST | `/api/similarity/{id}/decision` | POST | ✅ |

### 11. 报表 /api/reports/*

| 前端调用 | 方法 | Go 路由 | 方法 | 状态 |
|---------|------|---------|------|------|
| `/api/reports/personal/{evalId}` | GET | `/api/reports/personal/{evalId}` | GET | ✅ |
| `/api/reports/task/{taskId}/csv` | GET | `/api/reports/task/{taskId}/csv` | GET | ✅ |
| `/api/reports/statistics/{taskId}` | GET | `/api/reports/statistics/{taskId}` | GET | ✅ |

### 12. 其他 /api/*

| 前端调用 | 方法 | Go 路由 | 方法 | 状态 |
|---------|------|---------|------|------|
| `/api/dashboard` | GET | `/api/dashboard` | GET | ✅ |
| `/api/profiles/student/{userId}` | GET | `/api/profiles/student/{userId}` | GET | ✅ |
| `/api/profiles/school` | GET | `/api/profiles/school` | GET | ✅ |
| `/api/profiles/course/{courseId}` | GET | `/api/profiles/course/{courseId}` | GET | ✅ |
| `/api/account/me` | GET | `/api/account/me` | GET | ✅ |
| `/api/account/profile` | PATCH | `/api/account/profile` | PATCH | ✅ |
| `/api/account/change-password` | POST | `/api/account/change-password` | POST | ✅ |
| `/api/llm/configs` | GET | `/api/llm/configs` | GET | ✅ |
| `/api/llm/configs` | POST | `/api/llm/configs` | POST | ✅ |
| `/api/llm/test` | POST | `/api/llm/test` | POST | ✅ |
| `/api/audit` | GET | `/api/audit` | GET | ✅ |
| `/api/audit/export` | GET | `/api/audit/export` | GET | ✅ |
| `/api/parse/{uploadId}/result` | GET | `/api/parse/{uploadId}/result` | GET | ✅ |

### 13. 导入模板 /api/imports/*

| 前端调用 | 方法 | Go 路由 | 方法 | 状态 |
|---------|------|---------|------|------|
| `/api/imports/users` | POST | `/api/imports/users` | POST | ✅ |
| `/api/imports/students` | POST | `/api/imports/students` | POST | ✅ |
| `/api/imports/template/user.xlsx` | GET | `/api/imports/template/user.xlsx` | GET | ✅ |
| `/api/imports/exports/class/{id}/students.xlsx` | GET | ❌ 不存在 | - | ❌ |

### 14. 模板 /api/templates/*
全部 ✅

### 15. SSE 实时推送

| 前端调用 | Go 端点 | 状态 |
|---------|---------|------|
| `EventSource('/api/sse/events?token=xxx')` | ✅ `/api/sse/events` | 实现完成 |
| 旧 WebSocket `/ws/progress?token=xxx` | ❌ 不再支持 | 前端已改 useSSE |

---

## 🔴 必须修的阻塞问题（3项）

### P0: 实时通信隧道 — 🟢 已修复
- 前端 `useWebSocket.ts` → 替换为 `useSSE.ts` (EventSource)
- Go 新增 `handler/sse.go` + `/api/sse/events` 路由
- 已写代码，需确认完整编译通过

### P0: 缺失的路由（3个）
1. `/api/auth/forgot-password` — 前端 `ForgotPasswordView.vue` 调用，Go 无此功能
2. `/api/auth/reset-password` — 同上，配套
3. `/api/imports/exports/class/{id}/students.xlsx` — 前端 `ClassesView.vue` 调用，Go 无模板下载路由

### P1: 用户名/密码字段名不对齐
- 前端 `LoginView.vue` 发送 `{ username, password }`
- Go AuthHandler.Login 期望 `{ username, password }` ✅

- 前端 `auth.ts:21` 期望响应 `data.access_token` 和 `data.user`
- Go LoginResult 有 `AccessToken`(`access_token`) 和 `User`(`user`) ✅

- 前端 `UsersView.vue` 发送 `{ display_name, role, is_active }` ⚠️ Go UserRepo 需要确认

### P2: Dashboard 返回字段
- 前端 `TeacherDashboard.vue` 期望 `{ my_tasks, pending_grading, graded_this_week, class_avg_score, activity_7d, recent_tasks, recent_notifications }`
- Go `DashboardHandler.teacherDashboard` 返回字段完全匹配 ✅
- 学生仪表盘：前端期望 `{ pending_tasks, latest_score, score_diff, score_trend, rank, class_size, radar_data, weakness_list, ai_used_today }`
- Go `DashboardHandler.studentDashboard` 返回 ✅ 基本匹配

### P2: Evaluation 返回字段
- 前端 `EvaluationView.vue` 读取 `{ total_score, scores: [{ai_score, teacher_score, dimension_id}] }`
- Go `model.Evaluation` 有 `TotalScore` 和 `Scores[].{AIScore, TeacherScore, DimensionID}` ✅
- 前端 `ChatDialog.vue` 发 `{ session_id, message, evaluation_id }` 到 `/api/chat/stream`
- Go `dto.ChatStreamRequest` 有 `{ SessionID, Message, EvaluationID }` ✅

---

## 修复优先级

| 优先级 | 修复项 | 状态 |
|--------|--------|------|
| 🔴 P0 | SSE 实时推送桥接 | ✅ 已修复 |
| 🔴 P0 | `/api/auth/forgot-password` 和 `/api/auth/reset-password` | ❌ 未修 |
| 🟡 P1 | `/api/imports/exports` 班级名单下载 | ❌ 未修 |
| 🟡 P1 | 各视图返回值字段逐项确认 | ⚠️ 需前端联调 |
| 🟢 P2 | Dashboard 字段匹配（已对齐） | ✅ |
