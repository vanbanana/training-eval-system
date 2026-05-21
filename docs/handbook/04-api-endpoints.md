# 04 API 端点速查

完整 REST + WebSocket 端点列表。前后端联调时按此表对齐。

## REST API

| 端点 | 方法 | 角色 | 说明 |
|------|------|------|------|
| `/api/auth/login` | POST | 公开 | 登录获取JWT |
| `/api/auth/logout` | POST | 已认证 | 注销当前会话 |
| `/api/users` | GET/POST/PATCH | Admin | 用户CRUD |
| `/api/users/import` | POST | Admin | Excel批量导入用户 |
| `/api/courses` | GET/POST/PATCH | Admin/Teacher | 课程管理 |
| `/api/classes` | GET/POST/PATCH | Admin/Teacher | 班级管理 |
| `/api/classes/{id}/students` | POST | Teacher | 批量添加学生 |
| `/api/templates` | GET/POST/PATCH/DELETE | Teacher | 评价模板管理 |
| `/api/tasks` | GET/POST/PATCH | Teacher | 实训任务管理 |
| `/api/tasks/{id}/dimensions` | PUT | Teacher | 评价指标与权重配置 |
| `/api/tasks/{id}/uploads` | POST | Student | 提交实训成果（含断点续传） |
| `/api/uploads/{id}/parse-status` | GET | Teacher/Student | 查询解析状态 |
| `/api/uploads/{id}/reparse` | POST | Teacher | 手动重新解析 |
| `/api/evaluations/{id}` | GET/PATCH | Teacher | 查看/调整评分 |
| `/api/evaluations/bulk-action` | POST | Teacher | 批量确认/打回 |
| `/api/evaluations/{id}/history` | GET | Teacher | 评分修订历史 |
| `/api/similarity/task/{task_id}` | GET | Teacher | 相似度报告列表 |
| `/api/similarity/{id}/segments` | GET | Teacher | 相似片段对比 |
| `/api/similarity/{id}/decision` | POST | Teacher | 确认或忽略相似度警告 |
| `/api/profiles/student/{user_id}` | GET | Teacher/Student | 学生薄弱点画像 |
| `/api/profiles/course/{course_id}` | GET | Admin/Teacher | 课程教学画像 |
| `/api/profiles/school` | GET | Admin | 学校教学画像 |
| `/api/reports/personal/{eval_id}` | GET | Teacher/Student | 导出个人PDF报告 |
| `/api/reports/statistics/{task_id}` | GET | Teacher | 导出班级Excel统计 |
| `/api/notifications` | GET | 已认证 | 通知列表 |
| `/api/notifications/{id}/read` | POST | 已认证 | 标记已读 |
| `/api/notifications/read-all` | POST | 已认证 | 全部已读 |
| `/api/notifications/preferences` | GET/PUT | 已认证 | 通知偏好设置 |
| `/api/chat/sessions` | GET/POST | Student | AI问答会话列表 |
| `/api/chat/sessions/{id}/messages` | GET/POST | Student | 问答消息发送/查询 |
| `/api/audit/logs` | GET | Admin | 审计日志查询 |
| `/api/audit/export` | GET | Admin | 审计日志导出 |
| `/api/dashboard` | GET | 已认证 | 角色化仪表盘数据 |
| `/api/llm/config` | GET/PUT | Admin | LLM服务配置 |
| `/api/llm/test` | POST | Admin | LLM连通性测试 |
| `/api/llm/usage` | GET | Admin | LLM调用统计 |

## WebSocket

| 端点 | 角色 | 用途 |
|------|------|------|
| `/ws/progress/{token}` | 已认证 | 任务进度推送（解析/核查/评分） |
| `/ws/notify/{token}` | 已认证 | 实时通知推送 |
| `/ws/chat/{token}` | 已认证 | AI问答流式响应 |

## 健康与监控

| 端点 | 用途 |
|------|------|
| `GET /healthz` | 存活检查（仅返回 200） |
| `GET /readyz` | 就绪检查（含 DB/Redis 探测） |
| `GET /metrics` | Prometheus 指标 |
| `GET /docs` | Swagger UI（dev/test 启用） |
| `GET /openapi.json` | OpenAPI schema |

## Dev 调试端点（仅 ENV=dev）

详见 [10 测试与 Dev 端点](10-testing-and-dev-endpoints.md)。

## 通用约定

- **认证**：除明确标注公开的端点外，所有 API 必须携带 `Authorization: Bearer <jwt>`
- **trace_id 透传**：客户端可携带 `X-Trace-Id` 请求头，服务端会将该值贯穿日志与下游调用
- **分页**：列表端点使用 `?page=1&page_size=20`，响应包含 `items / total / page / page_size`
- **错误响应**：统一格式
  ```json
  {
    "error_code": "WEIGHT_SUM_INVALID",
    "message": "评价指标权重之和必须为100%，当前为85%",
    "field": "dimensions",
    "trace_id": "8e2f...c4a1"
  }
  ```
- **状态码语义**：
  - 200 成功
  - 201 创建成功
  - 204 删除成功
  - 400 业务规则违反（含 error_code）
  - 401 未认证
  - 403 权限不足
  - 404 资源不存在
  - 409 状态冲突
  - 422 参数校验失败（Pydantic）
  - 429 限流
  - 500 未知错误
  - 503 依赖不可用
