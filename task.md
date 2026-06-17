## 修复任务清单

> 规则：按 Epic 顺序执行。上一个 Epic 所有 Task 全部打勾后才能开始下一个 Epic。
> 每个 Task 完成后必须通过其列出的测试用例，整个 Epic 结束时跑完该 Epic 所有测试用例。
> 后端测试命令：`cd go-backend && go test ./... -count=1 -race`
> 前端构建检查：`cd frontend && npm run build`

---

### Epic 1：SSE 实时协议修复

> 目标：修复所有 SSE 协议层的前后端不匹配问题，使实时推送功能恢复正常。
> 前置条件：无
> 完成标准：Epic 1 全部测试用例通过

#### T1.1 — AI 聊天 SSE 数据格式修复

- [x] **代码修改完成**

**问题**：后端 SSE 事件的 JSON 数据中缺少 `type` 字段，前端 switch 分发全部不命中，聊天消息被静默丢弃。

**涉及文件**：
- `go-backend/internal/llm/stream.go`
- `go-backend/internal/handler/chat.go`

**修改内容**：
1. `stream.go` L119-120：`json.Marshal(map[string]string{"type": "text", "content": token})`，去掉 `event:` 行
2. `stream.go` L135-137：error 事件加 `"type": "error"`
3. `stream.go` L144：done 事件改为 `data: {"type":"done"}\n\n`
4. `chat.go` L217-225：orchestrator 路径同样加入 `type` 字段

**测试用例**：

```
TEST-T1.1-01: 后端单元测试 — StreamChat SSE 输出格式
  位置: go-backend/internal/llm/stream_test.go
  内容: mock LLM server 返回流式数据，验证 StreamChat 输出的每行 SSE data 都包含 "type" 字段
  断言: 所有 data: 行 JSON 解析后都有 "type" 字段，值为 "text" / "error" / "done" 之一

TEST-T1.1-02: 后端单元测试 — Chat Handler orchestrator 路径输出格式
  位置: go-backend/internal/handler/chat_test.go
  内容: 构造带 evaluation context 的请求，验证 orchestrator 路径返回的 SSE data 包含 "type" 字段
  断言: text 事件有 {"type":"text","content":"..."}, done 事件有 {"type":"done"}

TEST-T1.1-03: 集成测试 — 聊天消息收发
  位置: go-backend/internal/handler/integration_test.go
  内容: POST /api/chat/stream 发送消息，读取 SSE 响应流
  断言: 至少收到 1 个 type=text 事件和 1 个 type=done 事件；无事件的 JSON 缺少 type 字段
```

---

#### T1.2 — SSE 事件名称统一

- [x] **代码修改完成**

**问题**：后端发送 `parse_progress` 和 `eval_progress`，前端监听 `progress`，事件名不匹配导致所有进度事件被丢弃。

**涉及文件**：
- `go-backend/internal/pipeline/orchestrator.go`

**修改内容**：
1. L536: `Type: "parse_progress"` → `Type: "progress"`
2. L545: `Type: "eval_progress"` → `Type: "progress"`
3. payload 中增加 `"stage": "parse"` 或 `"stage": "eval"` 区分阶段

**测试用例**：

```
TEST-T1.2-01: 后端单元测试 — Orchestrator SSE 事件名称
  位置: go-backend/internal/pipeline/orchestrator_test.go
  内容: 触发 TriggerParse，捕获 broker 发布的 SSE 事件
  断言: 所有进度事件的 Type 为 "progress"（不是 "parse_progress" 或 "eval_progress"）

TEST-T1.2-02: 后端单元测试 — progress 事件包含 stage 字段
  位置: go-backend/internal/pipeline/orchestrator_test.go
  内容: 解析 progress 事件的 Data JSON
  断言: 包含 "stage" 字段，值为 "parse" 或 "eval"
```

---

#### T1.3 — 学生任务详情页 SSE 进度接入

- [x] **代码修改完成**

**问题**：`TaskDetailView.vue` 的 `getProgress()` 是返回 `null` 的 stub，`useParseProgress` composable 已实现但未使用。

**涉及文件**：
- `frontend/src/views/student/TaskDetailView.vue`

**修改内容**：
1. 导入 `useParseProgress` composable
2. 从 `uploads` ref 提取 `uploadIds` computed
3. 用 composable 返回的 `getProgress` 替换 L85-87 的 stub
4. 用 composable 的 `messages` 替换 `wsMessages` ref

**测试用例**：

```
TEST-T1.3-01: 前端构建检查
  命令: cd frontend && npm run build
  断言: 构建无 TypeScript 错误，无 lint 错误

TEST-T1.3-02: 代码审查 — 确认 stub 已移除
  检查: TaskDetailView.vue 中不存在 "return null" 的 getProgress 函数
  检查: TaskDetailView.vue 中导入了 useParseProgress

TEST-T1.3-03: 手动集成验证
  步骤: 启动前后端 → student 登录 → 进入任务详情 → 上传文件
  断言: 上传后页面显示解析进度状态变化（不再永远"待解析"）
```

---

### Epic 1 验收

- [x] `go test ./... -count=1 -race` 全部通过（包含 TEST-T1.1-01 ~ T1.2-02）
- [x] `npm run build` 无错误
- [x] 手动验证：AI 聊天可收发消息（T1.1）
- [x] 手动验证：上传文件后进度状态实时更新（T1.2 + T1.3）

---

### Epic 2：数据契约修复

> 目标：修复所有前后端 JSON 字段名/类型不匹配的问题，消除 undefined 和误报。
> 前置条件：Epic 1 验收通过
> 完成标准：Epic 2 全部测试用例通过

#### T2.1 — 相似度对比字段名修复

- [x] **代码修改完成**

**问题**：前端 `SegmentPair` 接口（`a_start/a_end/snippet_a/snippet_b/ratio` 等 7 个字段）与后端 `SegmentPairResponse`（`text_a/text_b/similarity` 3 个字段）完全不匹配。

**涉及文件**：
- `go-backend/internal/dto/similarity.go`
- `go-backend/internal/handler/similarity.go`

**修改内容**：
1. 修改 `SegmentPairResponse` 结构体，添加位置字段（`AStart`, `AEnd`, `BStart`, `BEnd`），重命名 `TextA→SnippetA`, `TextB→SnippetB`, `Similarity→Ratio`
2. 修改 handler 中构建 SegmentPairResponse 的逻辑，计算位置偏移量

**测试用例**：

```
TEST-T2.1-01: 后端单元测试 — SegmentPairResponse JSON 字段
  位置: go-backend/internal/dto/similarity_test.go
  内容: 构造 SegmentPairResponse 并 JSON Marshal
  断言: 输出包含 "a_start", "a_end", "b_start", "b_end", "snippet_a", "snippet_b", "ratio"

TEST-T2.1-02: 集成测试 — GET /api/similarity/{id}/segments
  位置: go-backend/internal/handler/integration_test.go
  内容: 创建含相似度数据的测试环境，请求 segments 端点
  断言: 返回的每个 segment 对象包含全部 7 个前端期望字段
```

---

#### T2.2 — HammingDistance null 安全

- [x] **代码修改完成**

**问题**：`HammingDistance` 是 `int`（非指针），未计算时序列化为 `0`，前端 `!== null` 判断失败导致误报抄袭。

**涉及文件**：
- `go-backend/internal/dto/similarity.go`
- `go-backend/internal/model/` 中 Similarity 相关模型
- `go-backend/internal/handler/similarity.go`（response 构建处）

**修改内容**：
1. DTO: `HammingDistance int` → `HammingDistance *int`
2. Model: 同步改为 `*int`
3. Handler: 构建 response 时，未计算的记录保持 nil

**测试用例**：

```
TEST-T2.2-01: 后端单元测试 — HammingDistance 未计算时序列化为 null
  位置: go-backend/internal/dto/similarity_test.go
  内容: 构造 HammingDistance 为 nil 的 SimilarityRecordResponse，JSON Marshal
  断言: 输出中 "hamming_distance" 值为 null

TEST-T2.2-02: 后端单元测试 — HammingDistance 有值时序列化正确
  内容: 构造 HammingDistance = 5 的 response
  断言: 输出中 "hamming_distance" 值为 5

TEST-T2.2-03: 集成测试 — 相似度列表不含误报
  内容: 创建多个未计算 hamming 的记录，GET /api/similarity/task/{id}
  断言: 所有记录的 hamming_distance 为 null（非 0）
```

---

#### T2.3 — 确认/拒绝评分返回分数

- [x] **代码修改完成**

**问题**：Confirm handler 只返回 `{"message": "..."}` 不含 `total_score`，前端读 `data.total_score` 得到 `undefined`。

**涉及文件**：
- `go-backend/internal/handler/grading.go`

**修改内容**：
1. Confirm handler（L219）：确认后查询 eval，返回 `{"message": "...", "total_score": eval.TotalScore, "status": eval.Status}`
2. Reject handler（L246）：同上

**测试用例**：

```
TEST-T2.3-01: 集成测试 — Confirm 返回 total_score
  位置: go-backend/internal/handler/integration_test.go
  内容: POST /api/grading/evaluations/{id}/confirm
  断言: 响应 JSON 包含 "total_score" 字段（类型为 number 或 null），包含 "status" 字段值为 "confirmed"

TEST-T2.3-02: 集成测试 — Reject 返回最新状态
  内容: POST /api/grading/evaluations/{id}/reject
  断言: 响应包含 "status": "rejected"
```

---

#### T2.4 — 批量操作返回实际计数

- [x] **代码修改完成**

**问题**：批量确认/拒绝返回 `{"message": "Bulk action completed"}` 无计数，前端误报全部成功。

**涉及文件**：
- `go-backend/internal/handler/evaluations.go`

**修改内容**：
1. 统计实际成功/失败数
2. 返回 `{"message": "...", "affected": <成功数>, "failed": <失败数>}`

**测试用例**：

```
TEST-T2.4-01: 集成测试 — 批量确认返回计数
  位置: go-backend/internal/handler/integration_test.go
  内容: 批量确认 3 条评价（其中 1 条状态不允许确认）
  断言: 响应中 "affected" 为 2, "failed" 为 1

TEST-T2.4-02: 集成测试 — 全部成功时 failed 为 0
  内容: 批量确认 3 条合法评价
  断言: "affected" 为 3, "failed" 为 0
```

---

#### T2.5 — 用户导入结果字段名修复

- [x] **代码修改完成**

**问题**：后端返回 `success_count`/`failed_count`/`total_count`，前端读 `data.created`/`data.success`/`data.failed`。

**涉及文件**：
- `frontend/src/views/admin/UserImportView.vue`

**修改内容**：
1. L150-153: `data.created` → `data.success_count`, `data.total` → `data.total_count`, `data.failed` → `data.failed_count`

**测试用例**：

```
TEST-T2.5-01: 前端构建检查
  命令: cd frontend && npm run build
  断言: 无 TypeScript 错误

TEST-T2.5-02: 代码审查 — 字段名对齐
  检查: UserImportView.vue 中读取的字段名为 success_count, total_count, failed_count
  检查: 不存在 data.created, data.success, data.failed, data.total 的引用
```

---

#### T2.6 — CSV 导出文件扩展名修复

- [x] **代码修改完成**

**问题**：前端把 XLSX 二进制内容保存为 `.csv` 扩展名。

**涉及文件**：
- `frontend/src/views/teacher/ReportsView.vue`

**修改内容**：
1. L91: `.csv` → `.xlsx`
2. L179: 按钮文案 "CSV" → "Excel"
3. format 类型 `'csv'` → `'xlsx'`（L78, L84, L175, L176）

**测试用例**：

```
TEST-T2.6-01: 前端构建检查
  命令: cd frontend && npm run build
  断言: 无 TypeScript 错误

TEST-T2.6-02: 代码审查 — 无 CSV 引用
  检查: ReportsView.vue 中不存在 format === 'csv' 的分支
  检查: 下载文件扩展名始终为 .xlsx

TEST-T2.6-03: 手动验证
  步骤: teacher 登录 → 报表页 → 点击 Excel 导出按钮
  断言: 下载文件名为 report_task_xxx.xlsx，用 Excel 打开无格式警告
```

---

### Epic 2 验收

- [x] `go test ./... -count=1 -race` 全部通过（包含 TEST-T2.1-01 ~ T2.4-02）
- [x] `npm run build` 无错误
- [x] 手动验证：相似度对比页面正常显示 diff（T2.1）
- [x] 手动验证：评分列表无虚假"疑似"标签（T2.2）
- [x] 手动验证：确认评分后 toast 显示正确分数（T2.3）
- [x] 手动验证：批量操作 toast 显示实际计数（T2.4）
- [x] 手动验证：用户导入结果显示正确条数（T2.5）
- [x] 手动验证：Excel 导出文件可正常打开（T2.6）

---

### Epic 3：空壳功能补全

> 目标：修复所有"后端有路由但 handler 是空壳"或"前端误判后端能力"的问题。
> 前置条件：Epic 2 验收通过
> 完成标准：Epic 3 全部测试用例通过

#### T3.1 — 通知偏好功能实现

- [x] **代码修改完成**

**问题**：`GetPreferences` 返回空 `{}`，`UpdatePreferences` 不读请求体。用户的偏好设置被静默丢弃。

**涉及文件**：
- `go-backend/internal/repository/`（新增 notification_prefs 查询）
- `go-backend/internal/service/notification_service.go`
- `go-backend/internal/handler/notifications.go`

**修改内容**：
1. Repository: 新增 `GetPreferencesByUserID(ctx, userID)` 和 `UpsertPreference(ctx, pref)`
2. Service: 新增 `GetPreferences(ctx, userID)` 和 `UpdatePreference(ctx, userID, eventType, enabled)`
3. Handler: `GetPreferences` 读 claims 调 service；`UpdatePreferences` Decode 请求体调 service

**测试用例**：

```
TEST-T3.1-01: 集成测试 — 通知偏好读写
  位置: go-backend/internal/handler/integration_test.go
  步骤: PUT /api/notifications/preferences {event_type:"score_complete", enabled:false}
        GET /api/notifications/preferences
  断言: GET 返回 {"score_complete": false}

TEST-T3.1-02: 集成测试 — 多次更新持久化
  步骤: PUT enabled:false → PUT enabled:true → GET
  断言: GET 返回 {"score_complete": true}

TEST-T3.1-03: 集成测试 — 不同用户隔离
  步骤: 用户 A 设置偏好 → 用户 B 查询
  断言: 用户 B 的偏好不受用户 A 影响
```

---

#### T3.2 — 课程取消归档前端解锁

- [x] **代码修改完成**

**问题**：后端 `ToggleArchive` 已支持 toggle（调两次恢复），但前端 `is_archived` 时直接 return。

**涉及文件**：
- `frontend/src/views/admin/CoursesView.vue`

**修改内容**：
1. 删除 L198-204 的 `if (c.is_archived)` 提前返回
2. 统一走 confirm + axios.patch toggle
3. 按钮文案根据 `is_archived` 状态显示"归档"/"取消归档"

**测试用例**：

```
TEST-T3.2-01: 前端构建检查
  命令: cd frontend && npm run build
  断言: 无 TypeScript 错误

TEST-T3.2-02: 代码审查 — 无误拦逻辑
  检查: CoursesView.vue 中 archiveCourse 函数不存在 "后端暂不支持" 的 toast
  检查: archiveCourse 对 is_archived=true 的课程也会发 PATCH 请求

TEST-T3.2-03: 手动验证
  步骤: admin 登录 → 课程管理 → 归档一门课程 → 点击"取消归档"
  断言: 课程恢复到未归档状态
```

---

#### T3.3 — 找回密码页面处理

- [x] **代码修改完成**

**问题**：前端有完整找回密码 UI 但后端无端点，用户走完整流程后才发现不可用。

**涉及文件**：
- `frontend/src/views/auth/LoginView.vue`
- `frontend/src/router/index.ts`
- `frontend/src/views/auth/ForgotPasswordView.vue`

**修改内容（短期方案）**：
1. `LoginView.vue`: 移除或注释"忘记密码？"链接
2. `router/index.ts`: 删除 `/forgot-password` 路由或重定向到 `/login`
3. `ForgotPasswordView.vue`: 保留文件但不再路由到它

**测试用例**：

```
TEST-T3.3-01: 前端构建检查
  命令: cd frontend && npm run build
  断言: 无 TypeScript 错误，无死代码警告

TEST-T3.3-02: 代码审查 — 入口已隐藏
  检查: LoginView.vue 中不存在指向 /forgot-password 的可点击链接
  检查: router/index.ts 中 /forgot-password 路由已删除或重定向

TEST-T3.3-03: 手动验证
  步骤: 访问 /login → 页面上无"忘记密码"链接
  步骤: 直接访问 /forgot-password → 重定向到 /login 或显示 404
```

---

### Epic 3 验收

- [x] `go test ./... -count=1 -race` 全部通过（包含 TEST-T3.1-01 ~ T3.1-03）
- [x] `npm run build` 无错误
- [x] 手动验证：通知偏好修改后刷新页面仍保留（T3.1）
- [x] 手动验证：课程可归档也可取消归档（T3.2）
- [x] 手动验证：登录页无"忘记密码"入口（T3.3）

---

### Epic 4：响应数据补全

> 目标：修复所有后端响应缺少前端所需字段的问题，消除 undefined 显示。
> 前置条件：Epic 3 验收通过
> 完成标准：Epic 4 全部测试用例通过

#### T4.1 — 学生任务详情 SSE 进度接入

- [x] **代码修改完成**

**说明**：此任务已在 Epic 1 T1.3 中完成，此处为回归确认。

**测试用例**：

```
TEST-T4.1-01: 回归测试
  断言: T1.3 的测试用例仍然通过
```

---

#### T4.2 — LLM 调用统计面板处理

- [x] **代码修改完成**

**问题**：`LlmConfigView.vue` 的"今日调用"指标卡和"调用日志"弹窗全是占位内容。

**涉及文件**：
- `frontend/src/views/admin/LlmConfigView.vue`

**修改内容（短期方案）**：
1. 移除 L388-419 的"今日调用"Card 区块
2. 移除 L214-218 的"调用日志"按钮
3. 移除 L461-472 的"调用日志"Dialog
4. 在页面适当位置添加说明文字

**测试用例**：

```
TEST-T4.2-01: 前端构建检查
  命令: cd frontend && npm run build
  断言: 无 TypeScript 错误

TEST-T4.2-02: 代码审查 — 占位 UI 已移除
  检查: LlmConfigView.vue 中不存在 "Epic 31" 的注释
  检查: 不存在硬编码的 "—" 指标值

TEST-T4.2-03: 手动验证
  步骤: admin 登录 → LLM 配置页
  断言: 页面无占位指标卡，无"调用日志"按钮，LLM 配置增删改测功能正常
```

---

### Epic 4 验收

- [x] `go test ./... -count=1 -race` 全部通过
- [x] `npm run build` 无错误
- [x] 手动验证：LLM 配置页无占位内容（T4.2）

---

### Epic 5：新功能实现

> 目标：实现之前完全缺失的功能（PDF 导出、批量导入、附件上传）。
> 前置条件：Epic 4 验收通过
> 完成标准：Epic 5 全部测试用例通过

#### T5.1 — 学生能力画像 PDF 导出

- [x] **代码修改完成**

**涉及文件**：
- `go-backend/internal/report/`（新增 profile PDF exporter）
- `go-backend/internal/handler/reports.go`
- `go-backend/internal/handler/router.go`
- `frontend/src/views/student/MyProfileView.vue`

**修改内容**：
1. 后端新增 `GET /api/reports/profile/{userId}` 端点
2. 复用 ProfileService 数据，用 fpdf 绘制雷达图 + 表格
3. 前端 `exportPdf` 函数改为 blob 下载

**测试用例**：

```
TEST-T5.1-01: 集成测试 — 画像 PDF 端点
  内容: GET /api/reports/profile/{userId}
  断言: 返回 200, Content-Type 为 application/pdf, body 非空

TEST-T5.1-02: 手动验证
  步骤: student 登录 → 我的画像 → 点击"导出 PDF"
  断言: 下载 PDF 文件，打开包含雷达图和成绩数据
```

---

#### T5.2 — 教师任务批量导入

- [x] **代码修改完成**

**涉及文件**：
- `go-backend/internal/service/`（新增 task import logic）
- `go-backend/internal/handler/imports.go`
- `go-backend/internal/handler/router.go`
- `frontend/src/views/teacher/TasksView.vue`

**修改内容**：
1. 后端新增任务模板下载 + 批量导入端点
2. 前端替换 `notifyImportPlanned` 为文件上传流程

**测试用例**：

```
TEST-T5.2-01: 集成测试 — 任务模板下载
  内容: GET /api/imports/template/task.xlsx
  断言: 返回 200, Content-Type 为 XLSX, body 非空

TEST-T5.2-02: 集成测试 — 任务批量导入
  内容: POST /api/imports/tasks 上传有效 XLSX
  断言: 返回 success_count >= 1, 数据库中新增对应任务

TEST-T5.2-03: 手动验证
  步骤: teacher 登录 → 任务列表 → 导入任务 → 上传文件 → 确认
  断言: 导入完成后列表中出现新任务
```

---

#### T5.3 — 聊天附件/图片上传（或隐藏）

- [x] **代码修改完成**

**短期方案（推荐）**：隐藏附件和图片按钮。

**涉及文件**：
- `frontend/src/views/student/ChatView.vue`

**修改内容**：
1. 移除 L583-591 的附件栏区块
2. 或添加 `v-if="false"` 暂时隐藏

**测试用例**：

```
TEST-T5.3-01: 前端构建检查
  命令: cd frontend && npm run build
  断言: 无 TypeScript 错误

TEST-T5.3-02: 手动验证
  步骤: student 登录 → AI 问答助手
  断言: 输入框上方无"附件""图片"按钮（或按钮不可见）
```

---

### Epic 5 验收

- [x] `go test ./... -count=1 -race` 全部通过
- [x] `npm run build` 无错误
- [x] 手动验证：学生画像 PDF 可下载打开（T5.1）
- [x] 手动验证：任务批量导入流程可走通（T5.2）
- [x] 手动验证：聊天页无占位按钮（T5.3）

---

### Epic 6：代码清理

> 目标：清理死代码，挂载或移除未使用的 handler。
> 前置条件：Epic 5 验收通过
> 完成标准：Epic 6 全部测试用例通过

#### T6.1 — HealthHandler / StaticHandler 处理

- [x] **代码修改完成**

**涉及文件**：
- `go-backend/internal/handler/health.go`
- `go-backend/internal/handler/static.go`
- `go-backend/cmd/server/main.go`
- `go-backend/internal/handler/router.go`

**修改内容（推荐：启用）**：
1. `main.go`: 构造 `StaticHandler`，传入 `RouterConfig`
2. `router.go`: 末尾追加 `r.NotFound(cfg.StaticHandler.ServeHTTP)` 作为 SPA 兜底
3. 评估 `HealthHandler` 是否需要替换内联 `/healthz`

**测试用例**：

```
TEST-T6.1-01: 后端编译检查
  命令: cd go-backend && go build ./...
  断言: 编译无错误

TEST-T6.1-02: 集成测试 — StaticHandler SPA 兜底
  内容: GET /some-nonexistent-route
  断言: 返回 index.html 内容（200 OK, body 包含 "<!DOCTYPE html>" 或 "<div id=\"app\">"）
  前提: dist/ 目录存在

TEST-T6.1-03: go vet 检查
  命令: cd go-backend && go vet ./...
  断言: 无警告
```

---

### Epic 6 验收

- [x] `go build ./...` 编译通过
- [x] `go test ./... -count=1 -race` 全部通过
- [x] `go vet ./...` 无警告
- [x] `npm run build` 无错误

---

### 全局进度

| Epic | 名称 | 状态 |
|------|------|------|
| Epic 1 | SSE 实时协议修复 | ✅ 完成 |
| Epic 2 | 数据契约修复 | ✅ 完成 |
| Epic 3 | 空壳功能补全 | ✅ 完成 |
| Epic 4 | 响应数据补全 | ✅ 完成 |
| Epic 5 | 新功能实现 | ✅ 完成 |
| Epic 6 | 代码清理 | ✅ 完成 |
