# 测试报告

> 生成日期：2026-05-19  
> 项目：智能实训评价管理系统  
> 范围：Epic 0 ~ Epic 30（Task 0.1 ~ 30.7）

## 概览

| 指标 | 数值 |
|------|------|
| **测试用例总数** | 718 |
| **通过** | 717 |
| **跳过** | 1（Docker 环境依赖，CI 中开启） |
| **失败** | 0 |
| **执行时长** | 148 秒（约 2 分 28 秒） |
| **测试覆盖率** | **76.62%**（阈值 70%，达标） |
| **测试 / 业务代码比** | 6067 行业务 / 717 测试用例 ≈ 1 测试 : 8.5 行业务 |

## 按测试类型分布

| 类型 | 用例数 | 占比 | 说明 |
|------|--------|------|------|
| Unit (`tests/unit/`) | 452 | 63% | Domain / Service 单元，全部 mock 外部依赖 |
| Integration (`tests/integration/`) | 130 | 18% | Repository / 端到端流程，使用 in-memory SQLite |
| Contract (`tests/contract/`) | 89 | 12% | API 契约测试，覆盖 schema 验证与权限 |
| Smoke (`tests/_smoke/`) | 41 | 6% | 工程脚手架自检（Epic 0） |
| Skills (`tests/skills/`) | 1 | <1% | LLM Skill Golden Set 验证 |
| CLI (`tests/cli/`) | 5 | <1% | tes-cli 命令行测试 |

## 按 Epic 分布

| Epic | 主题 | 测试用例数（估算） | 关键覆盖 |
|------|------|----------|----------|
| Epic 0 | 项目脚手架 | 41 | 工具链 + Factory + Docker + CI workflows |
| Epic 1 | 核心框架 | ~85 | Settings/异常/JWT/AES/Redis 锁/SystemConfig |
| Epic 2 | 数据库基线 | ~10 | SQLAlchemy + Alembic + 命名约定 |
| Epic 3 | 用户认证 | ~50 | 登录锁定/JWT 刷新/RBAC/SessionTimeout |
| Epic 4 | 组织管理 | ~30 | Course/Class/Membership + Property 13 |
| Epic 5 | 实训任务 | ~40 | 状态机 + 维度权重 + 字段锁 |
| Epic 6 | 评价模板 | ~25 | Property 19 模板独立性 |
| Epic 7 | 文件存储 | ~20 | Protocol + LocalFileStorage + magic check |
| Epic 8 | 成果上传 | ~50 | 上传 + chunked upload + Property 13 + 进度 pubsub |
| Epic 9 | 文档解析器 | ~25 | docx/pdf/ocr Parser Protocol |
| Epic 10 | LLM 适配器 | ~45 | 重试 + 熔断 + Metrics + 加密 + Provider Factory |
| Epic 11 | LLM Skills | ~25 | Registry + 容错 JSON + Golden Set |
| Epic 12 | Function Calling 工具 | ~20 | 权限校验 + 黑名单 + 大小护栏 |
| Epic 13 | Celery 基础 | ~10 | App + 路由队列 + 装饰器 |
| Epic 14 | 解析引擎主流程 | ~12 | ParsePipeline + SimHash + LLM embedding |
| Epic 15 | 智能核查引擎 | 18 | VerifyEngine + Skills + Golden Set + e2e |
| Epic 16 | 评价引擎 | 30 | scoring.py 50 case + EvaluationService + API |
| Epic 17 | 相似度引擎 | 16 | SimHash 分布 + 两阶段引擎 + Property 16 |
| Epic 18 | 学生画像 | 10 | UPSERT + INSUFFICIENT_DATA + 权限 |
| Epic 19 | 教学画像 | 12 | 课程聚合 + 物化视图迁移 + admin 权限 |
| Epic 20 | 报表导出 | 13 | SVG 图表 + reportlab + openpyxl + 权限 |
| Epic 21 | 通知系统 | 12 | 写扩散 + 偏好 + Beat 截止提醒 |
| Epic 22 | AI 问答 | 18 | ChatService + 7 工具 + 配额 + Function Calling 集成 |
| Epic 23 | 审计日志 | 10 | append-only + suspicious 检测 + admin API |
| Epic 24 | 仪表盘 | 8 | 三角色聚合 + 缓存失效 + psutil 兜底 |
| Epic 25 | 批量导入导出 | 16 | savepoint + 学生班级 + 模板 + 导出 |
| Epic 26 | Dev 端点 + Fakes | 16 | X-Dev-Token + Clock + 注入 + Fakes |
| Epic 27 | tes-cli | 5 | typer 入口 + 命令分组 |
| Epic 28-30 | 前端基线 + 管理员 | -（前端 vitest 暂未集成进 pytest） | pnpm build 通过 |

## 按测试模式分布（Given-When-Then）

每个测试遵循 Given-When-Then 模板，覆盖 happy / error / boundary 三类路径：

- **主路径 (Happy Path)**：~50% 用例
- **异常路径 (Error Path)**：~30% 用例
- **边界路径 (Boundary Path)**：~20% 用例

## 关键 Property 覆盖

| Property | 描述 | 测试位置 |
|---|---|---|
| Property 1 | 综合分 = Σ(权重 × (obj×α + subj×(1-α))) / 100 | `test_scoring.py::test_single_dim_obj80_subj90_alpha06` |
| Property 2 | 综合分 ∈ [0, 100] | `test_scoring.py::TestBoundary::test_clamped_to_100` |
| Property 3 | 权重总和 = 100 | `test_scoring.py::test_weight_sum_not_100_raises` |
| Property 6 | API Key AES-256-GCM 加密 | `test_crypto.py::test_encrypt_decrypt_roundtrip` |
| Property 13 | 学生只能上传到自己班级的任务 | `test_upload_service.py::test_not_in_class_rejected` |
| Property 14 | 审计日志 append-only | `test_audit_service.py` + `m0002_audit_triggers.py`（PG 触发器） |
| Property 16 | 相似度比对限定同一 task | `test_similarity_service.py::test_cross_task_excluded` |
| Property 17 | AI 问答配额受控 | `test_chat_service.py::test_quota_exhausted_rate_limited` |
| Property 18 | 评价修改追溯（EvaluationHistory） | `test_evaluation_service.py::TestUpdateDimensionSubjective` |
| Property 19 | 模板与任务独立 | `test_template_service.py::test_template_unaffected_by_task_changes` |

## 覆盖率详情

### 高覆盖率模块（≥ 90%）

完全覆盖（100%）：62 个文件，包括所有 Models、Schemas、Repository 基类、核心异常、工具类。

| 模块 | 覆盖率 | 备注 |
|---|---|---|
| `app/llm/metrics.py` | 98% | 仅一个 unreachable 分支 |
| `app/llm/retry.py` | 96% | 熔断器测试完整 |
| `app/parsers/base.py` | 96% | Parser Protocol |
| `app/reporting/chart_renderer.py` | 95% | SVG 图表 |
| `app/reporting/excel_renderer.py` | 96% | openpyxl 写入 |
| `app/services/scoring.py` | 91% | Domain 评分纯函数 |
| `app/services/notification_service.py` | 89% | 写扩散 + 偏好 |
| `app/services/profile_service.py` | 86% | 学生 + 教学画像 |
| `app/services/report_service.py` | 91% | 报表生成 |
| `app/services/similarity_service.py` | 87% | SimHash + 余弦 |

### 中覆盖率模块（70 - 90%）

| 模块 | 覆盖率 | 缺口分析 |
|---|---|---|
| `app/services/evaluation_service.py` | 82% | LLM 实际调用路径未在测试覆盖（只测 fallback） |
| `app/services/chat_service.py` | 84% | answer_stream 中真实 LLM 路径仅集成测试覆盖 |
| `app/services/upload_service.py` | 70% | reparse 路径与高级配额逻辑 |
| `app/llm/skills/base.py` | 92% | 部分错误恢复分支 |
| `app/llm/tools/chat_tools.py` | 90% | 大部分工具的成功路径 |

### 低覆盖率模块（< 70%，需后续补充）

| 模块 | 覆盖率 | 原因 |
|---|---|---|
| `app/llm/client.py` | 0% | 旧版兼容包装，已被新版 LLM Factory 取代 |
| `app/llm/scoring.py` | 0% | 旧版 mock 评分，已被 EvaluationService 取代 |
| `app/tasks/*.py`（Celery 包装层） | ~30% | 仅同步 happy path 测试，Celery worker 集成在 CI 中验证 |
| `app/api/task_manage.py` | 31% | 与 task_edit 重叠的旧版 API |
| `app/cli/commands/audit_archive.py` | 33% | CLI 入口在 CI 中执行验证 |

## 已知问题与跳过

1. **`test_testcontainers_fixture.py`**（1 个 SKIPPED）：testcontainers 启动 PostgreSQL 需要 Docker；CI 中通过 `services: postgres` 自动启用。
2. **OpenAPI Duplicate Operation ID 警告**（4 条）：`task_manage.py` 与 `task_edit.py` 共存导致；不影响功能，待 Epic 35 收口时统一清理。
3. **`format_exc_info` warning**：structlog processor 配置可优化，不影响测试结果。

## 测试基础设施

### Factory 体系（Factory_boy + Faker，zh_CN locale）

| Factory | 用途 | 已使用次数 |
|---------|------|------|
| `UserFactory` / `TeacherFactory` / `AdminFactory` | 用户 | 200+ 次 |
| `CourseFactory` / `ClassFactory` / `MembershipFactory` | 组织 | 80+ 次 |
| `TrainingTaskFactory` / `DimensionFactory` | 实训任务 | 100+ 次 |
| `EvaluationTemplateFactory` | 评价模板 | 25+ 次 |
| `UploadFactory` | 学生上传 | 60+ 次 |

### Fakes 测试替身

| Fake | 模块 |
|------|---|
| `FakeLLM` | `tests/fakes/fake_llm.py` —— 支持匹配响应、连续失败、流式输出 |
| `FakeStorage` | `tests/fakes/fake_storage.py` —— 内存 storage |
| `FakeParser` | `tests/fakes/fake_parser.py` —— 解析器 |
| `FakeEmbedder` | `tests/fakes/fake_embedder.py` —— 确定性 512 维向量 |
| `FrozenClock` | `app/core/clock.py` —— 时间冻结 |

## 测试运行方式

```bash
# 单元测试（最快）
pytest tests/unit -m unit

# 契约测试
pytest tests/contract -m contract

# 集成测试（含 in-memory SQLite）
pytest tests/integration

# 全量 + 覆盖率
pytest --cov=app --cov-report=term-missing

# 仅运行某 Epic
pytest tests/unit/services/test_evaluation_service.py
pytest -k "TestComputeFinalScore"
```

## 结论

- **Epic 0 ~ Epic 30 已完成 204/243 任务（84%）**
- **测试覆盖率 76.62%** 超过项目阈值（70%）
- **所有核心 Property（1, 2, 3, 6, 13, 14, 16, 17, 18, 19）均有对应测试**
- **本轮新增 ≈ 200 个测试用例**（Epic 15-30 累计），相比上次基线（Epic 14 完成时 ~520）增长约 38%
- **零失败、零回归**：现有 Epic 0-14 的 ~520 个测试全部仍通过

### 下一步

| 项 | 优先级 |
|---|---|
| 补 Epic 31-35 测试（教师/学生/部署/E2E） | 高 |
| 提升 Celery 任务包装层覆盖率（≥ 70%） | 中 |
| 引入 schemathesis 契约 fuzz | 中 |
| 集成前端 vitest 到 CI | 中 |
| 覆盖率核心算法目标 100%（当前 scoring/similarity 91%、87%） | 低 |
