# 10 测试规范与 Dev 端点

测试是本项目的一等公民。每次代码变更必须通过完整测试矩阵才允许合入。测试代码与业务代码同等重要，不得 `# noqa` 跳过任何检查。

## 测试金字塔

```
        ╱╲    e2e 端到端     5%   Playwright + 真实服务
       ╱──╲   契约测试       10%  schemathesis 跑 OpenAPI
      ╱────╲  集成测试       30%  含 DB/Redis 的服务流程
     ╱──────╲ 单元测试       55%  纯函数 + Mock 外部依赖
```

## 各层职责与工具

| 层次 | 工具 | 覆盖目标 | 强制要求 |
|------|------|---------|---------|
| **单元** | pytest + pytest-asyncio | 评分计算、权重校验、状态机、SimHash、Skill 渲染、Tool 业务逻辑 | 核心算法 100% 行覆盖 + 所有边界值 |
| **集成** | pytest + testcontainers (Postgres/Redis) | 服务层端到端流程（含真实 DB） | 主流程 Happy Path + 至少 2 条异常路径 |
| **契约** | schemathesis | 所有 OpenAPI 接口 schema 一致性 | 全部 endpoint 自动 fuzz |
| **e2e** | Playwright | 5 条关键用户路径 | 录屏归档供答辩用 |
| **前端单元** | Vitest + Vue Test Utils | 组件、Pinia store、纯 util | 关键组件 80% 覆盖 |
| **性能** | Locust | 50 并发用户（需求11.6） | 平均响应 ≤ 3 秒 |
| **Skill 评估** | tes-cli skill-eval | 每个 Skill 的 golden set | 改 Skill 前后必跑 |

## 测试隔离与替身

为支持单元/集成测试隔离外部依赖，所有外部 IO 通过 Protocol 接口注入，测试时用 `tests/fakes/` 中的 fake 实现替换：

| Protocol | 真实实现 | 测试替身 |
|---------|----------|---------|
| `LLMProvider` | `OpenAICompatProvider` | `FakeLLM`（按预设字典返回） |
| `OcrEngine` | `TesseractEngine` | `FakeOcr`（返回预置文本） |
| `FileStorage` | `LocalFileStorage` | `InMemoryStorage` |
| `Clock` | `SystemClock` | `FrozenClock`（控制时间） |
| `EmbeddingService` | 同 LLMProvider | `FakeEmbedder`（生成固定向量） |

## Dev 调试端点（无人工介入测试基础设施）

为让 AI 或测试脚本可独立验证业务功能而不依赖前端 UI，系统在 `ENV=dev` 下启用 `/api/_dev/*` 子路由（生产环境通过中间件硬关闭）。

### 调试端点清单

| 端点 | 用途 |
|------|------|
| `POST /api/_dev/seed` | 注入完整示例数据集（用户、班级、任务、若干提交+评价） |
| `POST /api/_dev/clock/freeze` | 冻结时间（用于截止时间相关测试） |
| `POST /api/_dev/clock/advance?seconds=N` | 推进时间 |
| `POST /api/_dev/llm/mock` | 切换到 FakeLLM 模式，下次调用返回预设值 |
| `POST /api/_dev/llm/restore` | 恢复真实 LLM |
| `POST /api/_dev/uploads/{id}/force-parse` | 跳过队列直接同步解析 |
| `POST /api/_dev/uploads/{id}/force-fail` | 强制设置 upload 为失败状态 |
| `POST /api/_dev/evaluations/{id}/inject-score` | 注入指定评分（绕过 LLM） |
| `POST /api/_dev/notifications/{user_id}/inject` | 注入通知 |
| `POST /api/_dev/audit/dump` | 导出最近 100 条审计日志（JSON） |
| `POST /api/_dev/celery/run-now` | 立即同步执行某 Celery 任务 |
| `GET /api/_dev/state/{entity}/{id}` | 透视任意实体的完整状态（含关联） |
| `POST /api/_dev/cache/flush` | 清空 Redis 缓存 |
| `GET /api/_dev/health/full` | 完整依赖健康检查（DB/Redis/LLM/OCR/磁盘） |

### 安全保障

- 路由仅在 `Settings.env == "dev"` 时注册，生产构建产物不含此模块
- 启动时若 `env=prod` 但 `_dev` 模块被 import，应用直接 panic 退出（`assert env != "prod"`）
- 所有调试端点要求特殊请求头 `X-Dev-Token`，与配置文件中的 `dev_token` 比对

## 管理 CLI（tes-cli）

基于 `typer` 实现，提供脚本化端到端验证能力。所有 CLI 命令同时是测试场景的可复用脚手架。

| 命令 | 用途 |
|------|------|
| `tes-cli seed [--scale=N]` | 注入示例数据 |
| `tes-cli simulate-evaluation --task-id=X` | 端到端演练某任务的全流程评价 |
| `tes-cli skill-eval --skill=NAME --version=V` | 跑 Skill 的 golden set，输出准确率与差异 |
| `tes-cli health-check` | 检查所有外部依赖健康状态 |
| `tes-cli rebuild-embeddings --task-id=X` | 重新计算嵌入 |
| `tes-cli archive-audit-logs --before=YYYY-MM-DD` | 归档审计日志 |
| `tes-cli backup-now` | 立即触发数据库备份 |

## 关键测试用例

- **登录锁定**：连续5次错误密码后第6次必须返回锁定错误
- **权重校验**：权重不等于100时保存维度必须返回 `WEIGHT_SUM_INVALID`
- **截止时间**：超期后学生上传必须被拒绝（用 dev clock 推进时间验证）
- **LLM降级**：FakeLLM 抛连接异常，教师仍可走手动评分流程
- **Function Calling 循环**：模拟 LLM 连续返回 6 次 tool_calls，必须强制总结终止
- **报表生成**：30秒内完成100人班级Excel导出（需求9.6）
- **架构验证**：LoongArch 容器内全测试套件通过（CI 上使用 QEMU 模拟或龙芯云平台）
- **班级隔离**：学生 A 不属于班级 X，必须无法看到班级 X 任务
- **审计日志篡改**：直接通过 SQL 尝试 UPDATE/DELETE audit_log 必须被触发器拒绝
- **通知到达**：用户离线时事件触发，登录后必须看到对应未读通知
- **相似度上限**：80%+ 相似度的两份提交自动出现在批改工作台警告栏
- **AI 问答限流**：第 51 次调用必须返回 HTTP 429 且不触发 LLM
- **评分历史完整性**：每次评分修改后 EVALUATION_HISTORY 表必有对应记录
- **模板独立性**：基于模板创建任务后修改维度，原模板必须不变
- **批量导入容错**：含 5 条非法行的 100 行 Excel 导入必须成功 95 条且失败明细可下载
- **物化视图刷新**：完成评价后 10 分钟内 `mv_class_progress` 必须反映新数据
- **WebSocket 多路复用**：同一用户同时订阅 progress/notify/chat 频道互不干扰
- **trace_id 透传**：HTTP 请求触发的 Celery 任务日志必须包含相同 trace_id

## CI 流水线

```
push → lint(ruff+mypy) → unit → integration(testcontainers) → contract → build artifact
                          ↓ all pass
                     skill-eval (golden set 回归)
                          ↓ all pass
                     LoongArch QEMU 镜像构建 → e2e smoke
```

任意环节失败阻断 PR 合入。`main` 分支保护开启，必须通过 review。

## 测试编写规范

### 测试命名

- 单元测试：`test_<被测对象>_<场景>_<期望>`
  - 例：`test_calculate_final_score_with_zero_weight_returns_zero`
- 集成测试：`test_<流程>_<场景>`
  - 例：`test_upload_to_evaluation_happy_path`
- 契约测试：自动生成

### Fixture 约定

- 全局 fixture 放 `tests/conftest.py`
- 模块级 fixture 放对应测试文件
- DB fixture 用 `pytest-asyncio` + `transactional` 保证测试隔离
- LLM/OCR 默认 mock，需要真实调用时显式标记 `@pytest.mark.real_llm`

### 断言风格

- 使用 `assert` + 描述性消息
- 复杂结构用 `pytest-deepdiff` 或 `dirty_equals`
- 不使用 `unittest.TestCase`，统一 pytest 风格

## 部署验证清单

每次发版前在 LoongArch+银河麒麟 环境执行：

1. 全部 Python 依赖在 LoongArch 上成功安装（无预编译 wheel 依赖 x86_64）
2. PostgreSQL/Redis/Nginx 服务启动正常
3. 云端 LLM API 连通性测试通过（管理员后台一键测试）
4. 端到端流程：注册→登录→创建任务→上传→解析→评分→导出，全链路通过
5. 系统启动时间 ≤60秒（需求1.4）
6. 内存占用 ≤6GB（预留给操作系统）
7. 出站 HTTPS 访问云端 API 通畅，且 API Key 加密存储校验通过
8. `tes-cli health-check` 全部 PASS
9. `tes-cli simulate-evaluation` 端到端演练成功
