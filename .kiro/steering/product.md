---
inclusion: always
---

# 智能实训评价管理系统 · 项目铁律

## 部署环境约束

- 部署目标：龙芯 LoongArch + 银河麒麟服务器 V10/V11，硬件 4 核 / 8GB 内存 / 256GB 硬盘
- 所有依赖必须能在 LoongArch 上编译运行（PostgreSQL、Redis、Tesseract、Python 3.10+）
- **禁止**本地部署 LLM（内存不足）；仅使用云端 OpenAI 兼容 API（DeepSeek / 通义 / 智谱 / Moonshot）
- 不写"国产化"叙事，只追求稳定性与性能

## 技术栈定档

- 后端：FastAPI + SQLAlchemy 2.0 typed + structlog + pydantic-settings + Celery + Redis
- 前端：Vue 3 + Vite + **shadcn-vue + Tailwind CSS**（不用 Element Plus）+ TanStack Table v8
- 数据库：PostgreSQL 14 + pgvector
- LLM：纯云端 API + AES-256-GCM 加密 API Key + 指数退避 + 熔断
- AI 编排：**Function Calling**（不引入 LangGraph 等 agent 框架）

## AI 开发模式

- 项目由 AI 直接开发，人工只做 review
- 任意 Task 实施前必读：`requirements.md` 对应需求 + `design.md` 对应章节 + `docs/handbook/`
- 前端 Task 必读：`docs/design/02-html-references.md` 中对应的 HTML 参考文件
- 完成 Task 后必须打勾 `- [x]` 并附 PR 描述链接 Task 编号

## Spec 文件格式（强约束）

- Spec section 标题必须是英文：`# Requirements Document` / `## Introduction` / `## Requirements`
- 内容可以是中文
- tasks.md 用 checkbox：`- [ ] X.Y. 任务名`，**不要**用 `### Task X.Y` 标题
- design.md 的 Property 必须含 `**Validates: Requirements X.Y**` 引用
