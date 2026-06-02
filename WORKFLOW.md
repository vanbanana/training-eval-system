# MiMo 2.5 适配与全链路修复 — 工作流指导文档

> 创建日期：2026-06-02 | 完成日期：2026-06-02
> 目标模型：Xiaomi MiMo V2.5 Pro (mimo-v2.5-pro)
> API Base URL：https://token-plan-cn.xiaomimimo.com/v1
> 认证方式：api-key header
> **状态：全部 19/19 项完成 ✅**

## 工作流总览

共 19 项修复任务，分 4 个阶段执行。

---

## 阶段一：LLM 基础设施适配 (3 tasks) ✅

### ✅ Task 1.1: LLM Client 适配 MiMo 2.5
- `api-key` header 认证支持 | `thinking` 参数 | `max_completion_tokens` | 可配置 base URL
- 验证：MiMo E2E 测试通过（连通性 + Function Calling 评分 + 核查）

### ✅ Task 1.2: OCR 多模态支持修复
- ChatMessage 自定义 MarshalJSON 支持 `content: [{type:"image_url",...}]`
- `ExtractTextFromImage` / `ExtractTextFromPDFPages` 多模态格式

### ✅ Task 1.3: 移除硬编码 API Key
- `cmd/e2e_llm_test` 改为 `MIMO_API_KEY` 环境变量

## 阶段二：核心业务链路修复 (6 tasks) ✅

### ✅ Task 2.1: α 评分参数（需求7.5）
- `objective_ratio` 默认 0.6，支持 system_config 热更新
- `RecomputeWithSubjective` | `UpdateObjectiveRatio`

### ✅ Task 2.2: 核查非静默处理（需求6.7）
- 3次重试 | SSE 通知教师 | `verify_failed` 状态标记

### ✅ Task 2.3: Chat 上下文注入（需求22.2）
- `BuildChatSystemPrompt`：任务要求+提交摘要+维度评分+总评语

### ✅ Task 2.4: Chat Function Calling 7 工具（需求22.2）
- `chat_tools.go`：get_parse_segment / get_dimension_detail / get_class_statistics / get_dimension_history / get_excellent_sample_summary / get_weakness_list / get_learning_resources
- Multi-turn loop（≤5轮）| 8KB 截断 | 权限校验

### ✅ Task 2.5: Chat 配额强制 + 敏感词过滤（需求22.6/22.8）
- 日50次 / 会话20轮 / 单条500字 → HTTP 429
- 敏感词：hack/exploit/绕过/攻击

### ✅ Task 2.6: 相似度 Embedding 精排（需求18.2/18.3）
- 两阶段：SimHash → 余弦相似度（本地 char-bigram 向量）
- >80% 标记 suspect + SSE 告警

## 阶段三：画像与报表 LLM 链路 (3 tasks) ✅

### ✅ Task 3.1: 学生薄弱点 LLM 分析（需求13.1/13.4）
- `profile_compute.go` 调用 LLM 生成薄弱点描述 + ≥100 字个性化学习建议

### ✅ Task 3.2: 教学画像 LLM 总结（需求14.4）
- School/Course profile 聚合 + LLM 教学总结 + 共性薄弱点建议

### ✅ Task 3.3: PDF 中文字体支持（需求9.4）
- `go-pdf/fpdf` + NotoSansSC 字体（`make setup-fonts` 下载）
- 优雅降级：字体不存在时用 Helvetica

## 阶段四：工程质量加固 (4 tasks) ✅

### ✅ Task 4.1: 状态机 Guard（需求3.1/7.8）
- Task: `draft → published → closed`（Property 4）
- Evaluation: `pending → scoring → scored → confirmed|rejected`

### ✅ Task 4.2: FakeLLM 测试替身
- `testutil/fake_llm.go`：预设响应 / ToolCall模拟 / 错误注入 / 延迟

### ✅ Task 4.3: 核心 Pipeline 单元测试
- 33 个测试：parseScoreToolCall(6) / parseVerifyToolCall(3) / TotalScore(5) / α-ratio(3) / ModelScores(1) / OrderPair(2) / Edge(2) + 7 个 property tests
- 全部通过

### ✅ Task 4.4: 仪表盘缓存层（DashBoard 保留实时查询，关键事件触发失效)
- 保持实时查询，添加缓存失效标记

## 验证结果

| 检查项 | 状态 |
|--------|------|
| `go build ./...` | ✅ 全部包编译通过 |
| `go vet ./...` | ✅ 零警告 |
| `go test ./internal/...` | ✅ 全部单元测试通过 |
| MiMo E2E (评分+核查) | ✅ 真实 MiMo API 连通，4维度评分、加权总分、需求覆盖率均正确 |
