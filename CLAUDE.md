# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

# 智能实训评价管理系统

基于 LLM Function Calling 的实训作业自动评价系统，部署目标为龙芯 LoongArch + 银河麒麟 V10/V11。

## 代码库概览

| 目录 | 技术栈 | 状态 |
|------|--------|------|
| `go-backend/` | Go 1.25+ / chi v5 / SQLite | **当前后端** — 所有后端工作在此 |
| `frontend/` | Vue 3 + Vite + Tailwind CSS v4 + shadcn-vue | 当前前端 |
| `backend/` | Python FastAPI | **已废弃** — 不移改 |
| `frontend-preview/` | 纯 HTML/CSS | 设计视觉契约 — 前端实现以此为准 |
| `docs/` | Markdown | 开发手册 + 设计文档 |
| `designs/` | Pencil (.pen) | 设计稿源文件 |

## 常用命令

```bash
# ───── Go 后端 ─────
cd go-backend
go run ./cmd/server                # 启动开发服务器
go test ./... -count=1 -race       # 全部测试
go test -run TestXxx ./pkg/...     # 单个测试
go vet ./...                       # 静态分析
gofmt -s -w .                      # 格式化
make build                         # 构建二进制
make cross-compile                 # LoongArch 交叉编译

# ───── Vue 前端 ─────
cd frontend
pnpm dev                           # 启动 dev server (vite)
pnpm build                         # 生产构建 (vue-tsc + vite)
pnpm typecheck                     # TypeScript 类型检查
```

**注意:** Python backend 已废弃。对 "backend" 的引用一律理解为 `go-backend/`。

## Go 后端架构

### 依赖注入 + 显式构造函数

`cmd/server/main.go` 是唯一组装点，按顺序：
1. 加载配置 (`config.Load()`)
2. 打开 SQLite (`store.Open()`)
3. 初始化基础设施 (worker pool, SSE broker, account lockout)
4. 初始化 Repositories (`repository.NewXxx(db)`)
5. 初始化 Services (`service.NewXxx(repo...)`)
6. 初始化 Handlers (`handler.NewXxx(svc)`)
7. 构建 Router (`handler.NewRouter(cfg)`)
8. 启动 HTTP server

**禁止**全局变量、包级 `init()`、`context.Background()`（请求场景）、ORM 或 CGO。

### 分层

```
handler/      — HTTP 路由 + 参数解析 → service → JSON (go-chi/chi v5)
service/      — 业务编排 (无 HTTP 感知)
repository/   — 数据访问接口 + raw SQL 实现
model/        — 纯 struct 领域模型
dto/          — 请求/响应 DTO (json tag = snake_case)
middleware/   — auth(JWT+角色), CORS, trace, ratelimit, logger, security headers
```

### 基础设施层

```
store/        — SQLite 连接池 (WAL, 读写分离: Writer 1 conn + Reader 4 conns)
               内嵌迁移: internal/store/migrations/*.sql 启动时自动执行
config/       — TES_ 环境变量 (.env 兜底), 必填: TES_JWT_SECRET(≥32) + TES_LLM_KEY_MASTER
crypto/       — AES-256-GCM, bcrypt, JWT (HMAC-SHA256)
worker/       — goroutine pool + buffered channel (异步任务)
sse/          — SSE broker (实时推送, 替代 WebSocket)
cache/        — LRU + TTL 内存缓存
```

### 业务层

```
llm/          — net/http 直调 OpenAI 兼容 API (DeepSeek/通义/智谱/Moonshot/MiMo)
pipeline/     — 评阅流水线: 解析 → 评分 → 验证 → 查重
               含 ChatOrchestrator (上下文感知 AI 问答)
parser/       — docx/pdf 解析, OCR 通过云端多模态 LLM (mimo-v2.5)
similarity/   — SimHash + cosine 查重
report/       — PDF (go-pdf/fpdf) + Excel (excelize) 报告生成
backup/       — SQLite VACUUM INTO 在线备份
```

### API 路由约定 (handler/router.go)

| 路由 | 访问 |
|------|------|
| `GET /healthz` | 公开 |
| `POST /api/auth/login` + `/refresh` | 公开 |
| `/api/*` | JWT 认证 |
| `/api/users/*`, `/api/llm/*`, `/api/audit/*` | admin 角色 |
| `/api/tasks/*` POST/PATCH/DELETE | admin + teacher |
| `/api/grading/*`, `/api/similarity/*` | admin + teacher |
| `/api/chat/*` | 全部认证用户 |

角色: `admin` / `teacher` / `student`
分页: `page`, `page_size`, `search`, `sort_by`, `sort_dir`
错误格式: `{"detail": "error message"}`
认证: access + refresh 双 token, `GetClaims(ctx)` / `RequireRole(roles...)`

### 测试

- `go test ./... -count=1 -race`
- 集成测试通过 `testutil.SetupTestApp(t)` 创建完整内存 SQLite 应用
- Property-based: `pgregory.net/rapid` (cache, config, crypto, similarity)

## Vue 前端架构

### 分层

```
src/
  api/         — axios 客户端实例 (client.ts)
  router/      — vue-router 路由配置 (index.ts)
  stores/      — Pinia 状态管理 (auth.ts)
  views/       — 按角色分: admin/ teacher/ student/ shared/ auth/
  components/  — 通用组件 (shadcn-vue + 自定义)
  composables/ — 可复用组合式函数
```

### 技术选型

- Vue 3 + Vite 8 + TypeScript 6
- Tailwind CSS v4 (`@tailwindcss/vite`)
- shadcn-vue (基于 reka-ui + class-variance-authority)
- Pinia 状态管理
- vue-router 路由
- @vueuse/core 工具函数
- vee-validate + zod 表单校验
- lucide-vue-next 图标

### 设计稿 → 代码工作流

1. 设计稿在 `designs/*.pen` (Pencil 项目)
2. 翻译为 HTML 视觉契约存于 `frontend-preview/pages/XX-name.html`
3. Vue 实现**必须**参照 HTML 而非直接拷贝 inline style
4. 规范细节见 `docs/design/`:
   - `01-design-tokens.md` — 颜色/字号/圆角/阴影变量
   - `02-html-references.md` — 每个 View → HTML 文件的强约束映射 (**前端 Task 必读**)
5. 完成后对比 HTML 静态页与 Vue 实现, 5px 以内偏差合格

## 关键文档

| 文档 | 说明 |
|------|------|
| `docs/handbook/00-INDEX.md` | 开发手册总目录 (12 篇) |
| `.kiro/specs/training-evaluation-system/` | 需求 + 设计 + 任务 |
| `docs/design/00-INDEX.md` | 设计 Token + HTML 参考映射 |
| `docs/handbook/02-engineering-principles.md` | 编码红线, 写代码前必读 |
| `docs/handbook/04-api-endpoints.md` | 全部 REST + SSE 端点 |
| `docs/handbook/05-data-model.md` | ERD + 字段 + SQL |

## 进度跟踪纪律（teacher-grading-workflow-optimization）

当前分支: `feat/teacher-grading-workflow-optimization`

每个 Epic 必须在全部 Task 完成、测试通过后才能进入下一个。每个 Task/Epic 必须打勾。

### Epic 0: 冻结基线
- [x] T0.1 — 教师批改链路测试 fixture
- [x] T0.2 — 批改链路回归测试
- [x] `go test ./handler/... -count=1` 通过

### Epic 1: 课程→班级→任务层级
- [x] T1.1 — ValidateTaskClassesBelongToCourse 校验
- [x] T1.2 — UpdateTaskRequest 支持 class_ids
- [x] T1.3 — 课程班级接口权限过滤
- [ ] T1.4 — 前端 TaskFormView 先课程后班级
- [ ] `go test ./... -count=1` 通过

### Epic 2: AI-first 评分模型
- [x] T2.1 — 最终分计算规则 (teacher_score overrides ai_score) — ComputeFinalScore
- [x] T2.2 — AI scoring 只写 ai_score (teacher_score=nil) — scorer.go 已有
- [x] T2.3 — 教师覆盖维度分 PATCH + OverrideTeacherScore
- [ ] T2.4 — 前端详情页评分显示更新

### Epic 3: 一键批改
- [x] T3.1 — POST /api/grading/tasks/{id}/auto-score
- [ ] T3.2 — Orchestrator TriggerScoreForUpload
- [ ] T3.3 — 前端一键批改按钮
- [ ] T3.4 — 批量确认强化

### Epic 4: 批改工作台
- [ ] T4.1 — GradingHomeView 批改首页
- [x] T4.2 — GET /api/grading/workbench 聚合接口
- [ ] T4.3 — 批改列表页状态文案优化

### Epic 5: 报告渲染
- [ ] T5.1 — GET /api/grading/uploads/{id}/report-view
- [x] T5.2 — 文本可读性检测工具 (AnalyzeReadability + CleanText + ExtractSections)
- [ ] T5.3 — ReportViewer 组件
- [ ] T5.4 — GradingDetailView 集成 ReportViewer

### Epic 6: 权限安全
- [x] T6.1 — 统一权限 helper (CanAccessTask/Evaluation/Upload)
- [ ] T6.2 — 所有批改接口加权限校验
- [ ] T6.3 — 前端 403/404 处理

### Epic 7: 最终验收
- [ ] T7.1 — E2E 测试
- [ ] T7.2 — 性能验证
- [x] T7.3 — go build + vet 通过
- [ ] T7.4 — 用户验收清单

## 禁止事项

- ❌ 修改 `backend/` (Python, 已废弃)
- ❌ 引入 CGO 依赖 (`CGO_ENABLED=0`)
- ❌ 使用 ORM (直接用 SQL)
- ❌ 引入第三方 Web 框架 (chi v5 足够)
- ❌ handler 写业务逻辑 (到 service 层)
- ❌ service 操作 HTTP 对象
- ❌ 用 `context.Background()` 处理请求
- ❌ 全局变量存业务状态

## Git 分支

当前分支 `feat/ui-polish-refinement`。主分支为 `main`。提交用 semantic commit 风格 (`feat:`, `fix:`, `docs:`, `refactor:` 等)。
