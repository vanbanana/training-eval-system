# 项目手册总索引

智能实训评价管理系统（Training Evaluation System）的高频参考手册。每份文档独立可读，从主规约 `.kiro/specs/training-evaluation-system/design.md` 提炼。

## 文档导航

| 文档 | 内容摘要 | 适用场景 |
|------|---------|---------|
| [01 技术选型](01-tech-stack.md) | 全栈技术清单 + 选型理由 | 立项答辩、新人入门 |
| [02 工程原则](02-engineering-principles.md) | 8 条编码红线（配置/类型/日志/并发等）| 写代码前必读，PR 审查必看 |
| [03 项目结构](03-project-structure.md) | 完整目录树 + 模块依赖图 | 新建文件前定位、Code Review |
| [04 API 端点](04-api-endpoints.md) | 全部 REST + WebSocket 端点列表 | 前后端联调、OpenAPI 对齐 |
| [05 数据模型](05-data-model.md) | ERD + 字段说明 + 索引 + 物化视图 | 写 SQL/迁移、性能调优 |
| [06 LLM Skills 目录](06-llm-skills-catalog.md) | Skill 抽象、分类、版本化 | LLM 模块开发 |
| [07 Function Calling 工具](07-function-calling-tools.md) | 工具注册表、AI 问答工具集 | AI 问答开发、扩展工具 |
| [08 配置规范](08-configuration.md) | 三层配置模型 + .env 模板 + system_config | 部署、调参、运维 |
| [09 日志可观测](09-logging-observability.md) | 结构化日志、trace_id、Prometheus 指标 | 故障排查、性能分析 |
| [10 测试与 Dev 端点](10-testing-and-dev-endpoints.md) | 测试金字塔、`/api/_dev/*`、`tes-cli` | 自动化测试、AI 验证 |
| [11 正确性属性](11-correctness-properties.md) | 19 条系统不变量 | 代码评审、设计验证 |
| [12 前端开发约定](12-frontend-conventions.md) | shadcn-vue + Tailwind + 组件分层 + dark 模式 | 前端开发 |

## 主规约入口

- 需求文档：`.kiro/specs/training-evaluation-system/requirements.md`
- 设计文档：`.kiro/specs/training-evaluation-system/design.md`

## ADR（架构决策记录）

- 索引位置：`docs/adr/`（待开发期间填充）
- 见 [设计文档 - ADR 索引章节](../../.kiro/specs/training-evaluation-system/design.md)

## 使用建议

- 开发任何模块前，先翻 [02 工程原则](02-engineering-principles.md) + [03 项目结构](03-project-structure.md)
- 写 LLM 相关代码前，必读 [06 Skills](06-llm-skills-catalog.md) 与 [07 Tools](07-function-calling-tools.md)
- 写前端代码前，必读 [12 前端约定](12-frontend-conventions.md)
- 提交 PR 前自检 [02 工程原则](02-engineering-principles.md) + [11 正确性属性](11-correctness-properties.md)
- 答辩演示时备好 [01 技术选型](01-tech-stack.md)
