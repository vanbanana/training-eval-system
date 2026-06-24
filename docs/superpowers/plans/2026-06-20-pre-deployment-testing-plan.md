# 上线前真实测试与管理 — 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) for syntax tracking.

**Goal:** 让项目达到可以部署到生产环境的真实可用状态，核心链路经过真实测试覆盖，代码和仓库整洁。

**Architecture:** 分 3 个独立但串行的阶段：(1) FakeLLM 接入完整测试基础设施，(2) 核心 AI 评阅链路 E2E 测试，(3) 仓库清理与部署准备。每个阶段独立可验证。

**Tech Stack:** Go 1.25+, chi v5, SQLite, FakeLLM (testutil), Playwright (前端 E2E), GitHub Actions, Docker

---

## 文件结构总览

```
go-backend/
  testutil/
    fake_llm.go           # MODIFY: 添加 LLMClient interface + FakeLLMAdapter
    setup.go              # MODIFY: 添加 SetupTestAppWithLLM()
  internal/
    llm/
      client.go           # MODIFY: 提取 LLMClient interface
      types.go [NEW]      # CREATE: LLMClient interface
    pipeline/
      scorer.go           # MODIFY: client field 改为 LLMClient interface
      verifier.go         # MODIFY: client field 改为 LLMClient interface
      orchestrator.go     # MODIFY: LLMClient field 改为 interface
      chat_tools.go       # MODIFY: client field 改为 LLMClient interface
    handler/
      grading.go          # MODIFY: llmClient 改为 LLMClient interface
      chat.go             # MODIFY: llmClient 改为 LLMClient interface
      agent.go            # MODIFY: llmClient 改为 LLMClient interface
      profiles.go         # MODIFY: llmClient 改为 LLMClient interface
      scoring_e2e_test.go [NEW] # CREATE: 核心 AI 评阅 E2E 测试
    service/
      agent_orchestrator.go # MODIFY: llmClient 改为 LLMClient interface
      profile_compute.go    # MODIFY: llmClient 改为 LLMClient interface
.gitignore               # MODIFY: 添加更多忽略规则
Dockerfile               # CREATE
docker-compose.yml        # CREATE
.github/workflows/ci.yml  # CREATE
```

---

## 阶段一：FakeLLM 接入基础设施

### Task 1.1: 提取 LLMClient interface

**文件:**
- Create: `go-backend/internal/llm/types.go`
- Modify: `go-backend/internal/llm/client.go`

- [ ] **Step 1: 创建 LLMClient interface 文件**

```go
// Package llm provides LLM client interfaces and implementations.
package llm

import "context"

// LLMClient is the interface for LLM completion calls.
// Both the production *Client and testutil.FakeLLM implement this.
type LLMClient interface {
	Complete(ctx context.Context, messages []ChatMessage, tools []Tool) (*ChatResponse, error)
	Model() string
}
```

- [ ] **Step 2: Run test to verify compilation**

Run: `cd go-backend && go build ./internal/llm/...`
Expected: PASS (no changes to existing code yet)

- [ ] **Step 3: Commit**

```bash
git add go-backend/internal/llm/types.go
git commit -m "feat(llm): add LLMClient interface for testability"
```

### Task 1.2: 验证 *Client 实现 LLMClient interface

- [ ] **Step 1: 添加编译期断言到 client.go**

File: `go-backend/internal/llm/client.go` — 在文件末尾添加：

```go
// Compile-time assertion that *Client implements LLMClient.
var _ LLMClient = (*Client)(nil)
```

- [ ] **Step 2: 验证编译**

Run: `cd go-backend && go build ./internal/llm/...`
Expected: PASS

- [ ] **Step 3: 验证 FullLLM 实现 Complete 签名兼容**

检查 `FakeLLM.Complete` 签名：
```go
func (f *FakeLLM) Complete(ctx context.Context, messages []ChatMessage, tools []Tool) (*ChatResponse, error)
```
这与 `LLMClient.Complete` 签名一致。为 FakeLLM 添加 Model() 方法。

- [ ] **Step 4: 为 FakeLLM 添加 Model() 方法**

File: `go-backend/testutil/fake_llm.go` — 添加：

```go
// Model returns "fake-llm" as the model identifier.
func (f *FakeLLM) Model() string {
	return "fake-llm"
}
```

- [ ] **Step 5: 添加编译期断言**

```go
var _ llm.LLMClient = (*FakeLLM)(nil)
```

- [ ] **Step 6: 编译验证**

Run: `cd go-backend && go build ./...`
Expected: PASS

- [ ] **Step 7: 运行测试**

Run: `cd go-backend && go test ./testutil/... ./internal/llm/... -count=1`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add go-backend/internal/llm/types.go go-backend/internal/llm/client.go go-backend/testutil/fake_llm.go
git commit -m "feat(llm): LLMClient interface with *Client and FakeLLM impls"
```

### Task 1.3: 将核心结构体改为使用 LLMClient interface

这一步把所有取 `*llm.Client` 的字段改为取 `llm.LLMClient`。

**需要修改的文件（共 10 处）：**

1. `internal/pipeline/scorer.go:21` — `client *llm.Client` → `client llm.LLMClient`
2. `internal/pipeline/verifier.go:16` — `client *llm.Client` → `client llm.LLMClient`
3. `internal/pipeline/orchestrator.go:33,46` — `LLMClient *llm.Client` 和 `llmClient *llm.Client` → `llm.LLMClient`
4. `internal/pipeline/chat_tools.go` — `client *llm.Client` → `client llm.LLMClient`（约第 26 行）
5. `internal/handler/grading.go:28` — `llmClient *llm.Client` → `llmClient llm.LLMClient`
6. `internal/handler/chat.go:25` — `llmClient *llm.Client` → `llmClient llm.LLMClient`
7. `internal/handler/agent.go:26` — `llmClient *llm.Client` → `llmClient llm.LLMClient`
8. `internal/handler/profiles.go:23` — `llmClient *llm.Client` → `llmClient llm.LLMClient`
9. `internal/service/agent_orchestrator.go:36` — `llmClient *llm.Client` → `llmClient llm.LLMClient`
10. `internal/service/profile_compute.go:33` — `llmClient *llm.Client` → `llmClient llm.LLMClient`

对应的构造函数参数也同步修改。

- [ ] **Step 1: 逐一修改 10 个文件的字段类型**

每个文件改 1-2 行（字段类型 + 构造参数类型）。例如 scorer.go：

```go
// Before
type Scorer struct {
	client        *llm.Client
	...
}

// After
type Scorer struct {
	client        llm.LLMClient
	...
}
```

```go
// After — 如果 Scorer 没有显式构造函数，字段类型改了就行
```

- [ ] **Step 2: 编译验证**

Run: `cd go-backend && go build ./...`
Expected: PASS

- [ ] **Step 3: 运行全部测试**

Run: `cd go-backend && go test ./... -count=1`
Expected: ALL PASS

- [ ] **Step 4: Commit**

```bash
git add go-backend/internal/
git commit -m "refactor: use LLMClient interface instead of concrete *llm.Client"
```

### Task 1.4: 添加 SetupTestAppWithLLM()

**文件：**
- Modify: `go-backend/testutil/setup.go`

- [ ] **Step 1: 添加 SetupTestAppWithLLM 函数**

在 `setup.go` 中添加：

```go
// SetupTestAppWithLLM creates a fully wired test app with a FakeLLM for testing
// LLM-dependent paths (scoring, agent, chat). The caller provides the desired
// FakeLLM responses which are consumed in order.
func SetupTestAppWithLLM(t *testing.T, fakeLLM *FakeLLM) *TestApp {
	t.Helper()

	app := SetupTestApp(t) // reuse existing setup for the heavy lifting

	// Rebuild only the LLM-dependent components with the FakeLLM injector
	db := app.DB

	// ... (在现有 SetupTestApp 基础上，提取 LLM 相关的构造为新函数)
}
```

但由于 `SetupTestApp()` 把 LLM 相关组件全部硬编码为 `nil`，更干净的方案是**重构 SetupTestApp 接受可选 LLM 参数**：

```go
// SetupTestApp creates a fully wired test application with SQLite :memory:.
// If llmClient is nil, LLM-dependent features are disabled.
func SetupTestApp(t *testing.T, llmClient llm.LLMClient) *TestApp {
```

但这会破坏所有现有调用（60+ 处 `SetupTestApp(t)` 需要改为 `SetupTestApp(t, nil)`）。

**折中方案：保留 SetupTestApp() 不变，添加 SetupTestAppWithLLM() 辅助函数：**

```go
// SetupTestAppWithLLM wraps SetupTestApp and rebuilds LLM-dependent
// handler components with the given fake client.
func SetupTestAppWithLLM(t *testing.T, fakeLLM *FakeLLM) *TestApp {
	t.Helper()
	app := SetupTestApp(t)

	// We need to recreate the server with LLM-aware dependencies.
	// Since app.Server is already running, we close it and create a new one.
	app.Server.Close()

	// Rebuild LLM-dependent components
	db := app.DB
	// ... reuse all the same repos and services from SetupTestApp ...
	// But pass fakeLLM instead of nil to:
	//   - handler.NewGradingHandler(..., orch, fakeLLM)
	//   - handler.NewChatHandler(..., fakeLLM, chatOrch, ...)
	//   - handler.NewAgentHandler(..., fakeLLM, ...)
	//   - handler.NewProfilesHandler(..., fakeLLM)
	//   - pipeline.NewChatOrchestrator(fakeLLM, ...)
	//   - service.NewRoleAgentOrchestrator(fakeLLM)
	//   - pipeline.NewOrchestrator({LLMClient: fakeLLM, ...})

	// Create new router and httptest.Server
	// ... same as SetupTestApp but with LLM deps wired ...

	return app
}
```

由于重构 `SetupTestApp` 会比较大，**具体代码需要在实现时从 SetupTestApp 的现有代码复制并替换 nil → fakeLLM**。这里略去全部重复代码（约 60 行），实现时直接从 `setup.go:79-155` 复制。

- [ ] **Step 2: 编译验证**

Run: `cd go-backend && go build ./...`
Expected: PASS

- [ ] **Step 3: 验证 SetupTestAppWithLLM 可创建带 FakeLLM 的测试 app**

Run: `cd go-backend && go test ./testutil/... -count=1`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add go-backend/testutil/setup.go
git commit -m "feat(testutil): add SetupTestAppWithLLM for LLM-aware integration tests"
```

---

## 阶段二：核心 AI 评阅链路 E2E 测试

### Task 2.1: 编写 AutoScore → Confirm → Report 全链路测试

**文件：**
- Create: `go-backend/internal/handler/scoring_e2e_test.go`

这个测试验证完整的 AI 评阅流程：
1. 准备带完整数据（task + upload + parse_result）的 fixture
2. 创建带 FakeLLM 的 TestApp
3. POST `/api/grading/tasks/{id}/auto-score` → 触发评分
4. FakeLLM 返回预设的 JSON 分数响应（包含 submit_scores tool_call）
5. 验证 DB 中 evaluation 状态变为 "scored"
6. 验证 dimension_scores 已写入
7. POST `/api/grading/evaluations/{id}/confirm` → 确认
8. 验证 DB 中 status 变为 "confirmed"
9. GET `/api/reports/personal/{evalId}` → 生成报告
10. 验证返回 200

- [ ] **Step 1: 创建 scoring_e2e_test.go**

```go
package handler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/smartedu/training-eval-system/internal/dto"
	"github.com/smartedu/training-eval-system/internal/llm"
	"github.com/smartedu/training-eval-system/testutil"
)

// TestE2E_Scoring_FullPipeline verifies the complete AI scoring pipeline:
// auto-score → scores saved → confirm → report generated.
func TestE2E_Scoring_FullPipeline(t *testing.T) {
	// Build FakeLLM with a scoring response that mimics LLM submit_scores tool_call
	fakeLLM := testutil.NewFakeLLM()

	// The scoring tool expects: {"scores":[{"dimension_id":1,"score":85,"rationale":"..."},...]}
	fakeLLM.WithToolCallResponse("submit_scores", map[string]any{
		"scores": []map[string]any{
			{"dimension_id": 200, "score": 85.0, "rationale": "代码结构清晰"},
			{"dimension_id": 201, "score": 78.0, "rationale": "文档较完整"},
			{"dimension_id": 202, "score": 90.0, "rationale": "功能完全实现"},
		},
	})

	app := testutil.SetupTestAppWithLLM(t, fakeLLM)
	ctx := context.Background()

	// Seed comprehensive fixture with task, uploads, parse results
	f := seedFullE2EFixture(t, app.DB)

	teacherToken := testutil.GenerateTestToken(f.TeacherAID, "teacher_a", "teacher")

	t.Run("auto-score triggers scoring", func(t *testing.T) {
		resp := doRequest(t, app.Server,
			"POST",
			fmt.Sprintf("/api/grading/tasks/%d/auto-score", f.TaskAID),
			teacherToken, nil)
		testutil.AssertStatus(t, resp, http.StatusOK)

		var result map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("decode auto-score response: %v", err)
		}
		t.Logf("auto-score result: %+v", result)
	})

	t.Run("evaluation status is scored after AI scoring", func(t *testing.T) {
		var status string
		err := app.DB.Reader.QueryRowContext(ctx,
			"SELECT status FROM evaluations WHERE id=?", f.EvalAID).Scan(&status)
		if err != nil {
			t.Fatalf("query eval status: %v", err)
		}
		if status != "scored" {
			t.Errorf("expected status=scored, got %s", status)
		}
	})

	t.Run("dimension scores exist after scoring", func(t *testing.T) {
		var scoreCount int
		err := app.DB.Reader.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM dimension_scores WHERE evaluation_id=?", f.EvalAID).Scan(&scoreCount)
		if err != nil {
			t.Fatalf("query dimension scores: %v", err)
		}
		if scoreCount == 0 {
			t.Error("expected at least 1 dimension score after auto-score")
		}
		t.Logf("dimension scores count: %d", scoreCount)
	})

	t.Run("confirm evaluation", func(t *testing.T) {
		resp := doRequest(t, app.Server,
			"POST",
			fmt.Sprintf("/api/grading/evaluations/%d/confirm", f.EvalAID),
			teacherToken, nil)
		testutil.AssertStatus(t, resp, http.StatusOK)

		var status string
		app.DB.Reader.QueryRowContext(ctx,
			"SELECT status FROM evaluations WHERE id=?", f.EvalAID).Scan(&status)
		if status != "confirmed" {
			t.Errorf("expected status=confirmed, got %s", status)
		}
	})

	t.Run("personal report can be generated", func(t *testing.T) {
		studentToken := testutil.GenerateTestToken(f.StudentAID, "student_a", "student")
		resp := doRequest(t, app.Server,
			"GET",
			fmt.Sprintf("/api/reports/personal/%d", f.EvalAID),
			studentToken, nil)
		testutil.AssertStatus(t, resp, http.StatusOK)
	})
}
```

- [ ] **Step 2: 编译验证**

Run: `cd go-backend && go build ./internal/handler/...`
Expected: PASS

- [ ] **Step 3: 运行 E2E 评分测试**

Run: `cd go-backend && go test ./internal/handler/... -run "TestE2E_Scoring" -count=1 -v 2>&1 | tail -20`
Expected: PASS - 所有子测试通过，验证 auto-score → scored status → dimension scores → confirm → report

- [ ] **Step 4: 运行全部 handler 测试确保无回归**

Run: `cd go-backend && go test ./internal/handler/... -count=1`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add go-backend/internal/handler/scoring_e2e_test.go
git commit -m "test: add full AI scoring pipeline E2E test with FakeLLM"
```

### Task 2.2: 验证 Agent 链路走真实 FakeLLM 响应

- [ ] **Step 1: 在 scoring_e2e_test.go 末尾添加 Agent 工具调用 E2E 测试**

```go
// TestE2E_Agent_ToolCalls verifies the teacher agent tool call path runs through
// FakeLLM correctly, producing expected tool call → tool result → final response flow.
func TestE2E_Agent_ToolCalls(t *testing.T) {
	fakeLLM := testutil.NewFakeLLM()
	// First call returns tool calls for "get_submission_summary"
	fakeLLM.WithToolCallResponse("get_submission_summary", map[string]any{
		"task_id": 200,
	})
	// Second call returns final text response after tool result is fed back
	fakeLLM.WithResponses(`根据数据，该任务共有 2 名学生提交了作业，其中 1 份已评分。`)

	app := testutil.SetupTestAppWithLLM(t, fakeLLM)
	f := seedFullE2EFixture(t, app.DB)
	token := testutil.GenerateTestToken(f.TeacherAID, "teacher_a", "teacher")

	// Create teacher session
	session := createTeacherSession(t, app.Server, token, "E2E Agent Test")

	// Send message with task context to trigger tool call
	resp := doRequest(t, app.Server, "POST", "/api/agent/stream", token,
		dto.AgentStreamRequest{
			SessionID: session.ID,
			Message:   "总结这个任务的提交情况",
			AgentRole: "teacher",
			Context:   &dto.AgentContextReq{TaskID: int64Ptr(f.TaskAID)},
		})
	testutil.AssertStatus(t, resp, http.StatusOK)

	// Parse SSE events — should contain tool_start, tool_result, and text events
	body, _ := io.ReadAll(resp.Body)
	events := parseSSEEvents(t, string(body))

	var hasToolCall, hasText, hasDone bool
	for _, evt := range events {
		typ, _ := evt["type"].(string)
		switch typ {
		case "tool_start":
			hasToolCall = true
		case "text":
			if content, ok := evt["content"].(string); ok && content != "" {
				hasText = true
			}
		case "done":
			hasDone = true
		}
	}

	if !hasToolCall {
		t.Error("expected tool_start event in agent stream")
	}
	if !hasText {
		t.Error("expected text event in agent stream")
	}
	if !hasDone {
		t.Error("expected done event in agent stream")
	}
}
```

- [ ] **Step 2: 编译验证**

Run: `cd go-backend && go build ./internal/handler/...`
Expected: PASS

- [ ] **Step 3: 运行 Agent 测试**

Run: `cd go-backend && go test ./internal/handler/... -run "TestE2E_Agent_ToolCalls" -count=1 -v`
Expected: PASS

- [ ] **Step 4: 运行全部 handler 测试**

Run: `cd go-backend && go test ./internal/handler/... -count=1`
Expected: ALL PASS

- [ ] **Step 5: 运行全量测试**

Run: `cd go-backend && go test ./... -count=1`
Expected: ALL PASS

- [ ] **Step 6: Commit**

```bash
git add go-backend/internal/handler/scoring_e2e_test.go
git commit -m "test: add teacher agent tool call E2E test with FakeLLM"
```

---

## 阶段三：仓库清理与部署准备

### Task 3.1: 清理无用文件并增强 .gitignore

**文件：**
- Modify: `.gitignore`
- Delete: 多个无用文件

- [ ] **Step 1: 列出所有应当忽略的文件**

在项目根目录运行以下命令查看 untracked 文件：
```bash
git status --short | grep "^??"
```

识别以下几类文件：

| 类别 | 文件 | 处理方式 |
|------|------|----------|
| 构建产物 | `go-backend/nul`, `go-backend/server.exe~`, `go-backend/cover` | 删除 + gitignore |
| 临时数据 | `export.zip`, 数据集文件, `student_report.pdf`, `test_report.*` | 删除 + gitignore |
| IDE 配置 | `.kiro/`, `.trae/` | 删除 + gitignore |
| Python 脚本 | `*.py`, `add-real-data.mjs`, `test-api.mjs` | 删除或保留到 scripts/ |
| 文档草稿 | `AGENTS.md`, `AI_AGENT_TASK.md`, `FIX_TODO.md` | 删除或归档到 docs/ |
| 测试报告 | `dataset_analysis.json`, `dataset_info.json` | 删除 |
| 截图 | `Image*.png`, `SVG*.svg`, `主页.png` | 删除 + gitignore |

- [ ] **Step 2: 增强 .gitignore**

在文件末尾追加：

```gitignore
# Build artifacts
*.exe~
go-backend/nul
go-backend/cover/

# E2E test artifacts
e2e_test/

# IDE configs
.kiro/
.trae/

# Dataset and exports
export/
export.zip
dataset_*.json
数据集*/

# Temp scripts (moved to scripts/ if needed)
add-real-data.mjs
test-api.mjs
test-routes.mjs
analyze_dataset.py
seed_real_school.py
check_db.py
check_scores.py
check_topics.py
set_failed.py

# Generated reports
student_report.pdf
test_report.pdf
test_report.xlsx

# Fonts (build-time dependency, not in repo)
go-backend/fonts/

# Screenshots
*.png
*.svg

# AI agent task files
AGENTS.md
AI_AGENT_TASK.md
FIX_TODO.md
```

- [ ] **Step 3: 删除确认过的文件**

```bash
# 确认每个文件是否真的要删，然后执行
git clean -fdn  # dry-run 查看会删除什么
```

执行实际删除（确认后）：
```bash
# Removing the confirmed junk files
rm -rf .kiro/ .trae/ export.zip "Image (60) (1).png" "SVG 1.svg" "数据集（整）(1).zip" "主页.png"
rm -f add-real-data.mjs test-api.mjs test-routes.mjs analyze_dataset.py seed_real_school.py
rm -f go-backend/check_db.py go-backend/check_scores.py go-backend/check_topics.py go-backend/set_failed.py
rm -f student_report.pdf test_report.pdf test_report.xlsx
rm -f dataset_analysis.json dataset_info.json
rm -f AGENTS.md AI_AGENT_TASK.md FIX_TODO.md
```

- [ ] **Step 4: 验证 git status 干净**

Run: `git status`
Expected: 只显示有意的修改（测试文件、interface 重构等），没有意外的新文件

- [ ] **Step 5: Commit**

```bash
git add .gitignore
git add -u  # 已跟踪文件的修改
git commit -m "chore: clean up repo, remove junk files, enhance .gitignore"
```

### Task 3.2: Dockerfile + docker-compose

**文件：**
- Create: `Dockerfile`
- Create: `docker-compose.yml`

- [ ] **Step 1: 创建 Dockerfile**

```dockerfile
# Stage 1: Build backend
FROM golang:1.25-alpine AS builder
WORKDIR /build
COPY go-backend/go.mod go-backend/go.sum ./
RUN go mod download
COPY go-backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/server ./cmd/server

# Stage 2: Build frontend
FROM node:22-alpine AS frontend-builder
WORKDIR /build
COPY frontend/package.json frontend/pnpm-lock.yaml ./
RUN corepack enable && pnpm install
COPY frontend/ ./
RUN pnpm build

# Stage 3: Runtime
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/server .
COPY --from=frontend-builder /build/dist ./dist
EXPOSE 8080
ENTRYPOINT ["./server"]
```

- [ ] **Step 2: 创建 docker-compose.yml**

```yaml
version: "3.9"
services:
  app:
    build: .
    ports:
      - "8080:8080"
    environment:
      TES_ENV: production
      TES_JWT_SECRET: ${TES_JWT_SECRET:?必须设置 JWT_SECRET}
      TES_LLM_KEY_MASTER: ${TES_LLM_KEY_MASTER:?必须设置 LLM_KEY_MASTER}
      TES_LLM_BASE_URL: ${TES_LLM_BASE_URL:-https://api.openai.com/v1}
      TES_LLM_API_KEY: ${TES_LLM_API_KEY:-}
      TES_LLM_MODEL: ${TES_LLM_MODEL:-gpt-4o}
      TES_LISTEN_ADDR: ":8080"
    volumes:
      - app-data:/app/data
    restart: unless-stopped

volumes:
  app-data:
```

- [ ] **Step 3: 验证 Docker 构建**

Run: `docker compose build`
Expected: 构建成功

- [ ] **Step 4: Commit**

```bash
git add Dockerfile docker-compose.yml
git commit -m "feat(ops): add Dockerfile and docker-compose for deployment"
```

### Task 3.3: GitHub Actions CI

**文件：**
- Create: `.github/workflows/ci.yml`

- [ ] **Step 1: 创建 CI workflow**

```yaml
name: CI

on:
  push:
    branches: [master, feat/*]
  pull_request:
    branches: [master]

jobs:
  backend:
    name: Backend Tests
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: go-backend
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.25"
          cache-dependency-path: go-backend/go.sum
      - run: go build ./...
      - run: go vet ./...
      - run: go test ./... -count=1 -timeout 300s

  frontend:
    name: Frontend Type Check
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: frontend
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: "22"
      - run: corepack enable && pnpm install
      - run: npx vue-tsc --noEmit
```

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: add GitHub Actions workflow for backend tests and frontend type check"
```

---

## 验收清单

完成上述全部任务后，逐项验证：

| # | 检查项 | 验证方法 |
|---|--------|----------|
| 1 | `LLMClient` interface 定义且被 `*Client` 和 `FakeLLM` 实现 | `go build ./...` 通过 |
| 2 | 10 个核心结构体使用 `llm.LLMClient` 而非 `*llm.Client` | `grep -r '\\*llm\\.Client' internal/` 返回 0 |
| 3 | `SetupTestAppWithLLM` 可用 | `go test ./testutil/... -count=1` 通过 |
| 4 | AutoScore E2E 测试通过 | `go test ./internal/handler/... -run TestE2E_Scoring -count=1` 通过 |
| 5 | Agent 工具调用 E2E 测试通过 | `go test ./internal/handler/... -run TestE2E_Agent_ToolCalls -count=1` 通过 |
| 6 | 所有现有测试无回归 | `go test ./... -count=1` 全部 OK |
| 7 | `.gitignore` 覆盖所有无用文件类型 | `git status --short | grep "^??"` 无意外文件 |
| 8 | Docker 构建成功 | `docker compose build` 通过 |
| 9 | CI 配置文件提交 | `.github/workflows/ci.yml` 存在 |

---

## 执行建议

**建议执行方式：Subagent-Driven Development**

1. 先执行阶段一（Task 1.1 → 1.4），这是后续所有测试的基础
2. 阶段一完成后，运行 `go test ./... -count=1` 确保全量通过
3. 再执行阶段二（Task 2.1 → 2.2），验证核心 AI 链路
4. 最后执行阶段三（Task 3.1 → 3.3），清理和部署准备
5. 每个 task 提交一次，保持提交粒度适中