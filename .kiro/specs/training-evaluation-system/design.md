# Design Document

## Overview

智能实训评价管理系统采用B/S架构，前后端分离设计，部署在龙芯LoongArch架构CPU与银河麒麟高级服务器操作系统V10/V11上。系统通过开源技术栈构建，关键组件均可在LoongArch架构上自行编译运行。

系统核心能力围绕"上传—解析—核查—评价—报表"五段式工作流展开，**通过统一的 OpenAI 兼容协议调用云端大语言模型 API**（如通义千问、DeepSeek、智谱、Moonshot 等），实现实训成果的自动化、智能化评价。在此基础上扩展班级管理、批量批改、薄弱点分析、教学质量画像、相似度检测、AI 问答助手、操作审计、消息通知等教学全链路能力。

> **关于大模型部署模式的说明**：考虑到龙芯云平台测试环境为 4 核 8GB 内存的资源约束，**本地部署 7B 级别大模型在性能与稳定性上不可行**（推理延迟高、并发能力弱、占用大部分内存影响其他服务），因此本系统**仅采用云端 API 调用模式**。系统通过抽象的 LLM Provider 接口设计，未来可在更高规格服务器上扩展支持本地部署，无需改动业务代码。

### 设计目标

- **国产化兼容**：所有部署组件（运行时、数据库、中间件、前端构建工具）均可在LoongArch架构上原生运行
- **模块化解耦**：解析引擎、核查引擎、评价模块、报表模块、画像模块、相似度模块通过明确接口边界解耦，便于独立维护
- **大模型抽象**：通过统一的 LLM Provider 接口屏蔽不同云端厂商 API 差异，运行时可热切换
- **稳定优先**:全部 AI 能力通过外部 API 调用,避免本地资源争抢,服务器仅承载业务逻辑与数据存储
- **数据安全合规**：所有关键操作可追溯，敏感数据加密存储，API 密钥加密存储

### 技术选型摘要

| 层次 | 选型 | 选型理由 |
|------|------|--------|
| 后端运行时 | Python 3.10+ | 银河麒麟V10/V11官方源已提供LoongArch版本，生态成熟 |
| Web框架 | FastAPI + Uvicorn | 纯Python实现，async原生，自动 OpenAPI 文档 |
| 前端框架 | Vue 3 + Vite + TypeScript | SFC + Composition API，编译产物为静态资源 |
| 前端 UI 库 | shadcn-vue（Reka UI + Tailwind CSS） | 无头组件，源码可定制，视觉现代 |
| 原子化 CSS | Tailwind CSS | shadcn-vue 标配，dark 模式开箱即用 |
| 表格库 | TanStack Table v8 | 无头表格逻辑，shadcn-vue 官方推荐 |
| 文件上传组件 | filepond-vue | 支持断点续传、SHA256 校验 |
| 图标库 | Lucide-vue-next | shadcn-vue 默认配套 |
| 图表库 | ECharts | 中文标签渲染优秀，可视化类型齐全 |
| 数据库 | PostgreSQL 14+ | 官方支持LoongArch；JSONB + 全文检索 + pgvector 一站式 |
| 缓存/任务队列 | Redis + Celery | 业界标准，重试/调度/限流功能齐全 |
| **大模型服务** | **云端 API（OpenAI 兼容协议）** | **稳定、并发能力强、不占用部署服务器资源** |
| **嵌入向量服务** | **云端 Embedding API**（如 text-embedding-v3） | 与主 LLM 同源，配置统一 |
| OCR引擎 | Tesseract（含中文语言包） | LoongArch 编译方案成熟 |
| 文档解析 | python-docx / PyMuPDF | 纯Python或可在LoongArch编译 |
| 报表导出 | openpyxl / WeasyPrint / matplotlib | 纯Python实现 |
| 反向代理 | Nginx | 国产化生态最成熟的反代 |
| 相似度检测 | simhash-py + pgvector | 文本指纹粗筛 + 向量精排双引擎 |
| 嵌入向量存储 | pgvector扩展 | PostgreSQL扩展，源码编译可在LoongArch运行 |
| 实时推送 | FastAPI WebSocket + Redis Pub/Sub | 解析进度、通知、AI流式响应共用 |

## Engineering Principles

本项目作为生产级交付物，严格遵循以下工程原则。这些原则在所有代码、配置、测试中必须被一致执行。

### 1. 配置外置（Configuration as Code）

**原则**：所有可变参数禁止硬编码在源码中，必须通过配置体系注入。

**三层配置策略**：

| 层次 | 用途 | 工具 |
|------|------|------|
| 静态默认值 | 类型安全的字段定义、默认值 | `pydantic-settings.BaseSettings` |
| 部署级配置 | 数据库连接、Redis、文件路径、并发数 | `.env` 文件 + 环境变量 |
| 运行时业务配置 | 评分权重比、相似度阈值、限流值、提示词模板 | PostgreSQL `system_config` 表，支持热更新 |

**禁止清单**：

- ❌ 在代码中写 `localhost`、`127.0.0.1`、`/data/uploads` 等具体值
- ❌ API Key、密码、密钥写死在源码或 commit 历史
- ❌ 业务阈值（如"相似度 > 0.8"）散落在多处
- ❌ 提示词模板嵌在 Python 字符串里

**示例**：

```python
# app/core/config.py
class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file=".env", env_prefix="TES_")
    
    db_url: PostgresDsn
    redis_url: RedisDsn
    upload_root: Path = Path("/data/uploads")
    max_upload_size_mb: int = 50
    similarity_simhash_threshold: int = 6
    similarity_cosine_threshold: float = 0.80
    parse_timeout_seconds: int = 120
    chat_daily_quota: int = 50
    
    @field_validator("max_upload_size_mb")
    @classmethod
    def validate_size(cls, v: int) -> int:
        if not 1 <= v <= 500: raise ValueError("size out of range")
        return v
```

业务规则（评分 α 系数、相似度阈值等）的运行时调整通过 `SystemConfig` 服务读取，TTL 60 秒缓存，管理员后台修改后自动生效。

### 2. 强类型与契约驱动

**原则**：业务边界必须有显式契约，运行时必须类型校验。

- **API 边界**：所有请求/响应通过 Pydantic v2 Schema 定义，禁止 `dict[str, Any]` 直接返回
- **服务层边界**：所有 service 方法签名声明完整类型，return type 不省略
- **数据库边界**：SQLAlchemy 2.0 typed Mapped 风格，与 Pydantic Schema 通过 `model_validate(orm_obj)` 转换
- **LLM 边界**：每个 LLM 调用必须有输入 Pydantic 模型 + 输出 Pydantic 模型，输出失败重试 3 次（容错解析）
- **mypy 配置**：`strict = true`，业务代码 0 警告，第三方边界用 `cast()` 而非 `# type: ignore`

### 3. 日志与可观测性（Observability）

**结构化日志要求**：每条日志必须包含 `timestamp / level / trace_id / span_id / user_id / context_dict`。

**强制记录点**：

| 类别 | 何时记录 | 字段 |
|------|---------|------|
| 入口 | 每个 HTTP 请求/Celery 任务开始 | method, path/task_name, payload_size |
| 出口 | 每个 HTTP 请求/Celery 任务结束 | status_code/result, duration_ms |
| 异常 | 所有 catch 块 | exception_type, traceback, ctx |
| 关键业务决策 | 评分计算、状态变更、权限拒绝、相似度判定、LLM 调用 | before/after value, decision_reason |
| 外部调用 | LLM API、OCR、文件存储 | endpoint, duration_ms, tokens, success |

**实现栈**：`structlog` + `python-json-logger`，输出 JSON 行。  
**trace_id 透传**：HTTP 请求中由中间件生成或从 `X-Trace-Id` 头继承；Celery 任务通过 `headers` 透传；WebSocket 连接握手时分配。

```python
# 入口示例
@router.post("/uploads")
async def create_upload(...):
    log.info("upload.create.start", task_id=task_id, file_size=file.size)
    try:
        result = await upload_service.create(...)
        log.info("upload.create.success", upload_id=result.id, duration_ms=elapsed)
        return result
    except Exception as e:
        log.exception("upload.create.failed", error=str(e), error_type=type(e).__name__)
        raise
```

**敏感字段过滤**：日志中的 `password / api_key / token` 自动用 `***` 替换，由 `structlog` processor 实现。

### 4. 资源管理

**原则**：所有 IO 资源必须通过上下文管理器自动释放，禁止裸 open / connect。

- **数据库会话**：FastAPI Depends 自动管理生命周期，事务用 `async with session.begin():`
- **Redis 连接**：连接池单例，操作通过 `async with pool.acquire():`
- **文件操作**：`async with aiofiles.open(...)` 或同步 `with open(...)`
- **HTTP 客户端**：`httpx.AsyncClient` 全局单例，应用关闭时统一 close
- **分布式锁**：`async with redis_lock("key", ttl=30):`

### 5. 并发安全

**原则**：共享状态必须显式同步，禁止隐式共享。

- **Celery 任务幂等**：所有任务以 `upload_id` 等业务主键作为唯一标识，重复执行不产生重复副作用（数据库唯一约束 + 状态机检查）
- **数据库并发**：用 `SELECT ... FOR UPDATE` 或乐观锁版本号防止 race condition（典型场景：评分修改）
- **Redis 分布式锁**：跨进程互斥操作（如同一 task 的相似度比对）使用 `redlock-py` 或 SET NX EX
- **禁止全局可变变量**：所有"单例"通过 FastAPI 依赖注入或显式 factory，方便测试 mock

### 6. 错误处理与降级

**原则**：失败必须可控，关键路径必须有降级策略。

- 所有 service 方法定义明确的业务异常类（`UploadTooLargeError / TaskClosedError / LLMUnavailableError 等）
- API 层用全局异常处理器映射到 HTTP 状态码 + 标准错误响应（已在 Error Handling 章节定义）
- LLM 调用失败 → 降级手动评分；OCR 失败 → 标记需人工复核；通知推送失败 → 离线时拉取补发
- **熔断器**：连续 5 次外部调用失败触发 30 秒熔断（基于 `purgatory-circuitbreaker` 库）

### 7. 测试驱动与可测试性设计

**原则**：开发完成 = 业务代码 + 测试代码 + 文档，三者缺一不可。

- 业务逻辑写成纯函数（无 IO），方便单元测试
- 所有外部依赖通过 Protocol/ABC 抽象，测试时用 fake 实现替换
- 关键流程在 `dev` 环境暴露 `/api/_dev/*` 调试端点，AI 或测试脚本可独立触发
- 配套 `tes-cli` 命令行工具，无需 UI 即可端到端验证业务

> 详见后文 **Testing Strategy** 章节。

### 8. 单一职责与依赖方向

**原则**：分层架构内依赖必须自上而下，禁止反向依赖。

```
api  →  services  →  repositories  →  models
  ↘                ↗
   schemas (DTO,无依赖)
   
llm/  → 被 services 依赖,自身只依赖 httpx 和 schemas
tasks/ → 被 services 编排, Celery worker 内部独立模块
```

**禁止**：

- ❌ models 层 import services
- ❌ repositories 层调用 LLM 或 HTTP
- ❌ services 层之间直接互相 import（应通过 service registry 或事件）

## Architecture

### 系统总体架构

系统采用四层架构：表现层、应用层、领域服务层、基础设施层。

```mermaid
graph TB
    subgraph "表现层"
        Browser[Web浏览器<br/>Vue 3 SPA]
    end

    subgraph "接入层"
        Nginx[Nginx<br/>反向代理 + 静态资源]
    end

    subgraph "应用层"
        API[FastAPI 应用<br/>REST API网关]
        Auth[认证授权模块]
        WebSocket[WebSocket<br/>解析进度推送]
    end

    subgraph "领域服务层"
        UserSvc[用户服务]
        OrgSvc[组织服务<br/>课程/班级]
        TaskSvc[任务服务]
        TmplSvc[模板服务]
        UploadSvc[上传服务]
        ParseSvc[解析引擎]
        VerifySvc[核查引擎]
        EvalSvc[评价服务]
        SimSvc[相似度引擎]
        ProfileSvc[画像服务<br/>薄弱点+教学质量]
        ReportSvc[报表服务]
        NotifySvc[通知服务]
        ChatSvc[AI问答服务]
        AuditSvc[审计日志服务]
        DashSvc[仪表盘聚合]
        ImportSvc[导入导出服务]
        LLMAdapter[LLM抽象适配器]
    end

    subgraph "异步任务层"
        Celery[Celery Worker<br/>解析/核查/评价/相似度/画像]
        Beat[Celery Beat<br/>定时备份/截止提醒/画像更新]
    end

    subgraph "基础设施层"
        PG[(PostgreSQL + pgvector<br/>业务数据 + 嵌入向量)]
        Redis[(Redis<br/>缓存+队列+Pub/Sub)]
        FS[本地文件存储<br/>实训成果文件]
        OCR[OCR引擎<br/>Tesseract]
    end

    subgraph "外部依赖"
        CloudLLM[云端 LLM API<br/>通义/DeepSeek/智谱等]
        CloudEmbed[云端 Embedding API]
    end

    Browser --> Nginx
    Nginx --> API
    Nginx --> WebSocket
    API --> Auth
    API --> UserSvc
    API --> OrgSvc
    API --> TaskSvc
    API --> TmplSvc
    API --> UploadSvc
    API --> EvalSvc
    API --> ReportSvc
    API --> ProfileSvc
    API --> NotifySvc
    API --> ChatSvc
    API --> AuditSvc
    API --> DashSvc
    API --> ImportSvc
    UploadSvc --> Celery
    Celery --> ParseSvc
    Celery --> VerifySvc
    Celery --> EvalSvc
    Celery --> SimSvc
    Celery --> ProfileSvc
    ParseSvc --> OCR
    ParseSvc --> LLMAdapter
    VerifySvc --> LLMAdapter
    EvalSvc --> LLMAdapter
    SimSvc --> LLMAdapter
    ProfileSvc --> LLMAdapter
    ChatSvc --> LLMAdapter
    LLMAdapter --> CloudLLM
    LLMAdapter --> CloudEmbed
    UserSvc --> PG
    OrgSvc --> PG
    TaskSvc --> PG
    TmplSvc --> PG
    UploadSvc --> FS
    UploadSvc --> PG
    EvalSvc --> PG
    ReportSvc --> PG
    SimSvc --> PG
    ProfileSvc --> PG
    NotifySvc --> PG
    NotifySvc --> Redis
    AuditSvc --> PG
    Celery --> Redis
    WebSocket --> Redis
    API --> Redis
    Beat --> PG
```

### 部署架构

单机部署模式（满足赛题8GB内存+4核CPU+256GB硬盘要求）：

```mermaid
graph LR
    subgraph "银河麒麟V10/V11 (LoongArch)"
        subgraph "systemd管理服务"
            N[Nginx :80]
            G[Gunicorn+Uvicorn :8000<br/>4 workers]
            C[Celery Workers x4]
            B[Celery Beat]
            P[PostgreSQL :5432]
            R[Redis :6379]
        end
        FS2[/data/uploads<br/>文件存储]
        BK[/data/backups<br/>数据库备份]
    end

    subgraph "外部 SaaS"
        Cloud[云端 LLM API<br/>https HTTPS 出站]
    end

    User[终端用户浏览器] --> N
    N --> G
    G --> P
    G --> R
    G --> FS2
    C --> P
    C --> Cloud
    C --> FS2
    B --> P
    B --> BK
```

### 资源预算（8GB 内存）

| 组件 | 内存占用估算 |
|------|-------------|
| 银河麒麟系统 | ~1GB |
| PostgreSQL（shared_buffers=512MB）| ~800MB |
| Redis（maxmemory=512MB）| ~512MB |
| FastAPI（4 workers）| ~600MB |
| Celery Worker × 4 | ~1GB |
| Nginx + 文件缓冲 | ~200MB |
| 文件解析临时内存峰值 | ~500MB |
| **合计** | **~4.6GB** |
| **系统余量** | **~3.4GB**（应对并发与缓存）|

由于不再本地部署大模型，**资源预算非常宽松**，可同时支持 50 用户并发（需求11.6）且保留缓冲。

### 关键架构决策

| 决策 | 方案 | 理由 |
|------|------|------|
| 解析与核查的执行模式 | 异步任务（Celery） | 大模型 API 调用耗时长（几秒到几十秒），同步阻塞会导致HTTP超时 |
| 进度反馈机制 | Redis Pub/Sub + WebSocket | 比轮询更省资源，用户体验更好 |
| **大模型部署模式** | **仅云端 API（OpenAI 兼容协议）** | **8GB 内存无法支撑本地7B模型推理；云端 API 稳定、并发强、成本可控** |
| **嵌入向量来源** | **同一云端供应商的 Embedding API** | 避免引入本地 PyTorch/transformers 依赖；与 LLM 配置统一 |
| 数据库选型 | PostgreSQL + pgvector | 支持JSONB、全文检索、向量检索一站式解决 |
| 文件存储 | 本地文件系统 | 单机部署足够，避免引入对象存储复杂度 |
| 前端打包 | Vite静态产物由Nginx托管 | 架构无关，部署简单 |
| 相似度检测策略 | SimHash粗筛 + 嵌入向量精排 | SimHash 本地快速排除低相似度，仅可疑对调用云端嵌入 API |
| 通知与AI流式 | WebSocket频道分离（progress/notify/chat） | 单一连接多路复用，减少握手开销 |
| 审计日志写入 | 仅追加表 + 触发器拦截 UPDATE/DELETE | 数据库层强制审计日志不可篡改 |
| 仪表盘数据 | Redis缓存 + 增量失效 | 聚合查询昂贵，缓存5分钟，关键事件触发失效 |
| 批量导入 | 流式分块读取 + 事务批提交 | Excel大文件不全量加载，控制内存峰值 |
| **API 密钥保护** | **数据库 AES-256 加密 + 环境变量主密钥** | API Key 是云端模式的核心凭证，必须强加密 |
| **LLM 调用韧性** | **指数退避重试 + 熔断器** | 云端 API 偶发限流或超时，需自动恢复机制 |

## Components and Interfaces

### 前端组件结构

前端采用 Vue 3 + Vite + TypeScript + **shadcn-vue + Tailwind CSS** 构建单页应用，按角色划分路由与功能模块。

**视觉来源**：所有页面视觉以 `designs/training-evaluation2.pen`（Pencil 设计稿）为基准，提炼后的设计 token 与页面规范见 `docs/design/`。前端开发严格按 `docs/design/pages/*.md` 的规范实现，组件实现复用 shadcn-vue 与已有 design tokens，**禁止直接拷贝 Pencil 导出的原始 HTML/CSS**。

**UI 技术栈说明**：

- **shadcn-vue**：基于 Reka UI 的无头组件 + Tailwind 预设样式，组件源码直接拷贝到 `src/components/ui/` 目录，可任意修改
- **Tailwind CSS**：原子化 CSS 框架，支持 dark 模式 + 主题变量 + tailwind-merge 合并冲突
- **TanStack Table**：表格逻辑（排序、过滤、分页、虚拟滚动），样式由 shadcn-vue 的 Table 组件套用
- **Lucide Icons**：与 shadcn-vue 默认配套的图标库
- **ECharts**：仪表盘可视化图表（雷达图、柱状图、折线图、热力图）

主要页面模块：

- **登录页**：用户名+密码登录、错误锁定提示
- **仪表盘**：管理员/教师/学生三种角色化首页（需求19）
- **管理员控制台**：用户管理、组织管理（课程/班级）、大模型配置、系统监控、审计日志查询
- **教师工作台**：
  - 实训任务管理、评价指标配置、评价模板库
  - **批改工作台**：列表批改、并排对比、上一份/下一份导航、批量确认（需求15）
  - 班级管理、学生薄弱点查看、教学画像
  - 报表导出
- **学生工作台**：任务列表、成果上传（含断点续传）、评价结果查看、能力雷达图、薄弱点TOP10、AI问答助手、评价历史趋势
- **公共组件**：
  - 文件上传器（基于 filepond-vue + SHA256 校验）
  - PDF/Word/图片预览组件
  - 图表组件（雷达图/柱状图/折线图/热力图，基于 ECharts 包装）
  - 通知中心下拉面板（带未读徽标）
  - AI问答对话框（流式渲染）
  - 主题切换器（浅色/深色，基于 Tailwind dark: 类）

### 后端模块结构

### 完整项目结构树

下表为最终落地的完整目录与每个文件作用，作为代码生成与代码审查的"地图"。

```
training-evaluation-system/
├── backend/                          # 后端服务（Python）
│   ├── app/
│   │   ├── main.py                   # FastAPI 应用入口、生命周期管理、路由注册
│   │   ├── core/                     # 框架级基础设施
│   │   │   ├── config.py             # pydantic-settings 配置类（部署级）
│   │   │   ├── system_config.py      # DB 驱动的运行时业务配置（热更新）
│   │   │   ├── security.py           # JWT 编解码、密码哈希
│   │   │   ├── deps.py               # FastAPI 依赖项工厂（DB session/当前用户/权限）
│   │   │   ├── logging.py            # structlog 配置、trace_id 上下文
│   │   │   ├── exceptions.py         # 业务异常类层级
│   │   │   ├── middleware.py         # 全局中间件（CORS/trace_id/审计/限流）
│   │   │   ├── crypto.py             # AES-256 加密 API Key 等敏感字段
│   │   │   └── lock.py               # Redis 分布式锁封装
│   │   ├── api/                      # HTTP 路由层（仅做参数校验+服务调用+响应组装）
│   │   │   ├── __init__.py           # APIRouter 聚合
│   │   │   ├── auth.py               # /api/auth/* 登录、登出、刷新
│   │   │   ├── users.py              # /api/users/* 用户 CRUD
│   │   │   ├── orgs.py               # /api/courses /api/classes
│   │   │   ├── tasks.py              # /api/tasks/* 实训任务
│   │   │   ├── templates.py          # /api/templates/* 评价模板
│   │   │   ├── uploads.py            # /api/uploads/* 含断点续传
│   │   │   ├── evaluations.py        # /api/evaluations/* 含批量操作
│   │   │   ├── reports.py            # /api/reports/* PDF/Excel 导出
│   │   │   ├── profiles.py           # /api/profiles/* 薄弱点/教学画像
│   │   │   ├── similarity.py         # /api/similarity/*
│   │   │   ├── notifications.py      # /api/notifications/*
│   │   │   ├── chat.py               # /api/chat/* AI 问答
│   │   │   ├── audit.py              # /api/audit/*
│   │   │   ├── dashboard.py          # /api/dashboard
│   │   │   ├── imports.py            # /api/imports/*
│   │   │   ├── llm_config.py         # /api/llm/* 模型配置
│   │   │   ├── websockets.py         # /ws/* progress, notify, chat
│   │   │   └── _dev.py               # /api/_dev/* 仅 dev 启用的调试端点
│   │   ├── schemas/                  # Pydantic v2 数据传输对象（DTO）
│   │   │   ├── auth.py
│   │   │   ├── user.py
│   │   │   ├── org.py
│   │   │   ├── task.py
│   │   │   ├── upload.py
│   │   │   ├── evaluation.py
│   │   │   ├── report.py
│   │   │   ├── profile.py
│   │   │   ├── similarity.py
│   │   │   ├── notification.py
│   │   │   ├── chat.py
│   │   │   ├── audit.py
│   │   │   ├── dashboard.py
│   │   │   ├── llm.py
│   │   │   └── common.py             # PageQuery, PageResponse, ErrorResponse
│   │   ├── models/                   # SQLAlchemy 2.0 ORM 模型（typed Mapped）
│   │   │   ├── base.py               # DeclarativeBase + 公共字段（id/timestamps）
│   │   │   ├── user.py
│   │   │   ├── org.py                # Course, Class, ClassMembership
│   │   │   ├── task.py               # TrainingTask, Dimension
│   │   │   ├── template.py           # EvaluationTemplate, TemplateDimension
│   │   │   ├── upload.py             # Upload, ParseResult, VerifyResult
│   │   │   ├── evaluation.py         # Evaluation, DimensionScore, History
│   │   │   ├── similarity.py         # SimilarityRecord
│   │   │   ├── profile.py            # StudentProfile
│   │   │   ├── notification.py
│   │   │   ├── chat.py               # ChatSession, ChatMessage
│   │   │   ├── audit.py              # AuditLog
│   │   │   └── import_job.py         # ImportJob, ImportRecord
│   │   ├── repositories/             # 数据访问层（每个 model 一个 repo）
│   │   │   ├── base.py               # 通用 CRUD 基类（泛型）
│   │   │   ├── user_repo.py
│   │   │   ├── task_repo.py
│   │   │   ├── upload_repo.py
│   │   │   ├── evaluation_repo.py
│   │   │   ├── ...
│   │   │   └── audit_repo.py
│   │   ├── services/                 # 业务服务层（核心业务逻辑）
│   │   │   ├── auth_service.py
│   │   │   ├── user_service.py
│   │   │   ├── org_service.py
│   │   │   ├── task_service.py
│   │   │   ├── template_service.py
│   │   │   ├── upload_service.py
│   │   │   ├── parse_engine.py       # 解析引擎主流程
│   │   │   ├── verify_engine.py      # 核查引擎主流程
│   │   │   ├── evaluation_service.py # 评分计算（纯函数 + 调用 LLM）
│   │   │   ├── similarity_service.py # SimHash + pgvector 余弦
│   │   │   ├── profile_service.py    # 薄弱点 + 教学画像
│   │   │   ├── report_service.py     # PDF/Excel 生成
│   │   │   ├── notification_service.py
│   │   │   ├── chat_service.py       # AI 问答 + Function Calling 编排
│   │   │   ├── audit_service.py
│   │   │   ├── dashboard_service.py
│   │   │   ├── import_service.py
│   │   │   └── interfaces/           # 服务层 Protocol（解耦用）
│   │   │       ├── llm_provider.py   # LLMProvider Protocol
│   │   │       ├── ocr_engine.py     # OcrEngine Protocol
│   │   │       └── storage.py        # FileStorage Protocol
│   │   ├── llm/                      # LLM 抽象适配器
│   │   │   ├── base.py               # LLMProvider ABC + 数据模型
│   │   │   ├── openai_compat.py      # OpenAI 协议实现
│   │   │   ├── factory.py            # LLMFactory 单例工厂
│   │   │   ├── retry.py              # 指数退避 + 熔断器
│   │   │   ├── metrics.py            # 装饰器：耗时/tokens 记录
│   │   │   ├── tools/                # Function Calling 工具注册表
│   │   │   │   ├── registry.py       # ToolRegistry 单例
│   │   │   │   ├── base.py           # Tool ABC + 输入/输出 schema
│   │   │   │   ├── chat_tools.py     # 问答场景工具集
│   │   │   │   └── verify_tools.py   # 核查场景工具集（可选）
│   │   │   └── skills/               # LLM Skill 库（提示词模板）
│   │   │       ├── registry.py       # SkillRegistry 单例
│   │   │       ├── base.py           # Skill ABC + 输入/输出 schema
│   │   │       ├── parse/            # 解析类
│   │   │       │   ├── docx_to_structure.py
│   │   │       │   ├── pdf_to_structure.py
│   │   │       │   └── image_to_text.py
│   │   │       ├── verify/           # 核查类
│   │   │       │   ├── coverage_check.py
│   │   │       │   └── logic_audit.py
│   │   │       ├── score/            # 评分类
│   │   │       │   └── dimension_score.py
│   │   │       ├── profile/          # 画像类
│   │   │       │   ├── weakness_analyze.py
│   │   │       │   ├── learning_advice.py
│   │   │       │   └── teaching_summary.py
│   │   │       └── chat/             # 问答类
│   │   │           └── student_qa.py
│   │   ├── tasks/                    # Celery 异步任务
│   │   │   ├── celery_app.py         # Celery 实例 + 配置
│   │   │   ├── parse_tasks.py
│   │   │   ├── verify_tasks.py
│   │   │   ├── evaluate_tasks.py
│   │   │   ├── similarity_tasks.py
│   │   │   ├── profile_tasks.py
│   │   │   ├── notification_tasks.py
│   │   │   ├── deadline_reminder.py
│   │   │   ├── backup_tasks.py
│   │   │   └── matview_refresh.py    # 物化视图刷新
│   │   ├── parsers/                  # 文档解析器（非 LLM 部分）
│   │   │   ├── base.py               # Parser Protocol
│   │   │   ├── docx_parser.py        # python-docx
│   │   │   ├── pdf_parser.py         # PyMuPDF
│   │   │   └── ocr_parser.py         # Tesseract 包装
│   │   ├── reporting/                # 报表生成
│   │   │   ├── pdf_renderer.py       # WeasyPrint
│   │   │   ├── excel_renderer.py     # openpyxl
│   │   │   └── chart_renderer.py     # matplotlib（图嵌入 PDF/Excel）
│   │   ├── storage/                  # 文件存储抽象
│   │   │   ├── base.py               # FileStorage Protocol
│   │   │   └── local_fs.py           # 本地文件系统实现
│   │   └── utils/
│   │       ├── pagination.py
│   │       ├── trace.py              # trace_id 工具
│   │       ├── magic_check.py        # 文件头校验
│   │       └── time.py
│   ├── alembic/                      # 数据库迁移
│   │   ├── env.py
│   │   └── versions/
│   ├── cli/                          # 管理 CLI（典型脚本）
│   │   ├── main.py                   # typer App
│   │   ├── commands/
│   │   │   ├── seed.py               # tes-cli seed → 注入示例数据
│   │   │   ├── simulate.py           # tes-cli simulate-evaluation → 端到端演练
│   │   │   ├── reindex.py            # tes-cli rebuild-embeddings
│   │   │   └── audit_archive.py      # tes-cli archive-audit-logs
│   ├── tests/                        # 测试代码（与 app/ 平行）
│   │   ├── conftest.py               # 共用 fixture
│   │   ├── unit/                     # 单元测试（无 IO，纯逻辑）
│   │   │   ├── test_evaluation_calc.py
│   │   │   ├── test_weight_validator.py
│   │   │   ├── test_simhash.py
│   │   │   └── test_skills_render.py
│   │   ├── integration/              # 集成测试（含 DB/Redis）
│   │   │   ├── test_upload_flow.py
│   │   │   ├── test_evaluation_flow.py
│   │   │   ├── test_chat_function_calling.py
│   │   │   └── test_similarity_flow.py
│   │   ├── contract/                 # 契约测试（API schema）
│   │   │   └── test_openapi_schema.py
│   │   ├── e2e/                      # 端到端（黑盒）
│   │   │   └── test_happy_path.py
│   │   └── fakes/                    # 测试替身
│   │       ├── fake_llm.py           # 实现 LLMProvider，不调网络
│   │       ├── fake_ocr.py
│   │       └── fake_storage.py
│   ├── pyproject.toml                # 依赖、ruff、mypy、pytest 配置
│   ├── alembic.ini
│   └── .env.example
├── frontend/                         # 前端（Vue 3 + shadcn-vue + Tailwind CSS）
│   ├── src/
│   │   ├── main.ts                   # Vue 应用入口
│   │   ├── App.vue
│   │   ├── api/                      # 由 OpenAPI 自动生成的 client
│   │   │   ├── generated/            # openapi-typescript-codegen 输出
│   │   │   └── client.ts             # axios 实例 + 拦截器
│   │   ├── views/                    # 页面（路由对应）
│   │   │   ├── auth/
│   │   │   ├── admin/
│   │   │   ├── teacher/
│   │   │   ├── student/
│   │   │   └── shared/               # 跨角色共用页面
│   │   ├── components/
│   │   │   ├── ui/                   # shadcn-vue 拷贝进来的组件源码（可改）
│   │   │   │   ├── button/
│   │   │   │   ├── card/
│   │   │   │   ├── dialog/
│   │   │   │   ├── table/
│   │   │   │   ├── tabs/
│   │   │   │   ├── select/
│   │   │   │   ├── toast/
│   │   │   │   └── ...
│   │   │   ├── business/             # 业务组件（基于 ui/ 组合）
│   │   │   │   ├── FileUploader.vue  # filepond-vue 包装
│   │   │   │   ├── DataTable.vue     # TanStack Table + shadcn 表格样式
│   │   │   │   ├── ChartRadar.vue    # ECharts 雷达图
│   │   │   │   ├── ChartBar.vue
│   │   │   │   ├── ChartLine.vue
│   │   │   │   ├── ChartHeatmap.vue
│   │   │   │   ├── NotificationCenter.vue
│   │   │   │   ├── ChatDialog.vue    # AI 问答对话框（流式）
│   │   │   │   ├── ThemeToggle.vue   # 浅色/深色切换
│   │   │   │   └── BreadcrumbNav.vue
│   │   │   └── layout/
│   │   │       ├── AppShell.vue      # 总布局（侧栏+顶栏+主区）
│   │   │       ├── Sidebar.vue
│   │   │       └── TopBar.vue
│   │   ├── composables/              # Vue 组合式函数
│   │   │   ├── useAuth.ts
│   │   │   ├── useNotification.ts
│   │   │   ├── useWebSocket.ts
│   │   │   └── useTheme.ts
│   │   ├── stores/                   # Pinia
│   │   │   ├── auth.ts
│   │   │   ├── user.ts
│   │   │   ├── notification.ts
│   │   │   └── theme.ts
│   │   ├── router/                   # Vue Router 4
│   │   │   ├── index.ts
│   │   │   └── guards.ts             # 角色路由守卫
│   │   ├── lib/                      # 通用工具
│   │   │   ├── utils.ts              # cn() 函数（tailwind-merge 包装，shadcn 标配）
│   │   │   ├── date.ts
│   │   │   └── format.ts
│   │   ├── locales/                  # i18n（中文为主）
│   │   ├── styles/
│   │   │   ├── globals.css           # Tailwind 入口 + 全局变量
│   │   │   └── themes.css            # 浅色/深色主题色变量
│   │   └── types/                    # TS 类型
│   ├── tests/                        # Vitest 单元 + Playwright e2e
│   ├── components.json               # shadcn-vue 配置
│   ├── tailwind.config.ts            # Tailwind 配置
│   ├── postcss.config.js
│   ├── tsconfig.json
│   ├── package.json
│   └── vite.config.ts
├── deploy/                           # 部署脚本与配置
│   ├── install.sh
│   ├── systemd/
│   ├── nginx/
│   ├── postgres/
│   └── env/
│       └── .env.example
├── docs/                             # 设计与运维文档
│   ├── adr/                          # Architecture Decision Records
│   │   ├── 0001-use-postgresql-pgvector.md
│   │   ├── 0002-cloud-only-llm.md
│   │   ├── 0003-celery-for-async.md
│   │   ├── 0004-function-calling-for-chat.md
│   │   └── 0005-skill-registry-pattern.md
│   ├── api-contract.md
│   └── operations.md
├── .kiro/specs/                      # 本规约
└── README.md
```

### 模块依赖图

```mermaid
graph LR
    api[api/<br/>路由层] --> services[services/<br/>业务层]
    api --> schemas[schemas/<br/>DTO]
    services --> repositories[repositories/<br/>数据访问]
    services --> llm[llm/<br/>LLM适配]
    services --> parsers[parsers/<br/>文档解析]
    services --> reporting[reporting/<br/>报表生成]
    services --> storage[storage/<br/>文件存储]
    services --> tasks[tasks/<br/>异步任务]
    repositories --> models[models/<br/>ORM]
    llm --> tools[llm/tools/<br/>Function Calls]
    llm --> skills[llm/skills/<br/>Prompt模板]
    
    services -.通过Protocol.-> interfaces[services/interfaces/]
    llm -.实现.-> interfaces
    parsers -.实现.-> interfaces
    storage -.实现.-> interfaces
    
    style schemas fill:#e8f5e9
    style interfaces fill:#fff3e0
```

**依赖方向规则**：

- 上层（api/services）→ 下层（repositories/models），不允许反向
- service 之间通过 Protocol 接口或事件总线通信，不直接 import
- llm/parsers/storage 通过 `services/interfaces/` 中的 Protocol 暴露给 services，方便测试时替换



### LLM 抽象适配器接口

为支持不同云端供应商（通义千问、DeepSeek、智谱、Moonshot 等）无缝切换（需求8），定义统一的 LLM Provider 接口。所有主流国产云端 LLM 都已兼容 OpenAI Chat Completions 协议，因此只需一个 `OpenAICompatProvider` 实现即可覆盖。

```python
# app/llm/base.py
from abc import ABC, abstractmethod
from typing import AsyncIterator, Optional
from pydantic import BaseModel

class LLMMessage(BaseModel):
    role: str  # system | user | assistant
    content: str

class LLMResponse(BaseModel):
    content: str
    model: str
    prompt_tokens: int
    completion_tokens: int
    tool_calls: list["ToolCall"] | None = None   # Function Calling 返回时填充
    finish_reason: str = "stop"                  # stop | tool_calls | length

class ToolCall(BaseModel):
    id: str
    name: str
    arguments: dict

class LLMProvider(ABC):
    @abstractmethod
    async def chat(
        self,
        messages: list[LLMMessage],
        temperature: float = 0.2,
        max_tokens: Optional[int] = None,
    ) -> LLMResponse: ...

    @abstractmethod
    async def chat_with_tools(
        self,
        messages: list[LLMMessage],
        tools: list[dict],         # OpenAI tools schema
        temperature: float = 0.2,
    ) -> LLMResponse:
        """Function Calling 单次调用，返回内含 tool_calls 或最终 content"""
        ...

    @abstractmethod
    async def chat_stream(
        self,
        messages: list[LLMMessage],
        temperature: float = 0.7,
    ) -> AsyncIterator[str]:
        """流式返回，用于AI问答助手"""
        ...

    @abstractmethod
    async def embed(self, texts: list[str]) -> list[list[float]]:
        """生成嵌入向量，用于相似度检测和语义搜索"""
        ...

    @abstractmethod
    async def health_check(self) -> tuple[bool, str]:
        """返回 (是否健康, 描述信息)"""
        ...
```

**支持的云端供应商**（管理员可通过界面配置切换）：

| 供应商 | Chat Endpoint 示例 | Embedding 模型示例 |
|-------|-------------------|------------------|
| 阿里通义千问 | `https://dashscope.aliyuncs.com/compatible-mode/v1` | `text-embedding-v3` |
| DeepSeek | `https://api.deepseek.com/v1` | （走通义或智谱）|
| 智谱 GLM | `https://open.bigmodel.cn/api/paas/v4` | `embedding-3` |
| Moonshot Kimi | `https://api.moonshot.cn/v1` | （走通义或智谱）|

运行时通过 `LLMFactory.get_provider()` 根据数据库配置动态选择实例，**无需重启服务即可热切换**（需求8.2）。

**API Key 安全**：

- 数据库存储字段使用 AES-256-GCM 加密
- 主密钥从环境变量 `LLM_KEY_MASTER` 注入，不入库不入代码仓库
- 界面回显时仅显示掩码（如 `sk-***...***xxxx`），需求8.4

**调用韧性**：

- HTTP 客户端使用 httpx，超时默认 60s
- 失败重试采用指数退避（1s, 2s, 4s）最多 3 次
- 单 endpoint 连续 5 次失败触发熔断器（30s 半开），熔断期间任务降级失败而不重试
- 所有调用经过 `LLMMetrics` 装饰器记录耗时与 token 消耗（需求8.8）

### LLM Skills Catalog（提示词技能目录）

**Skill = 一个具体的 LLM 任务单元**。每个 Skill 是 prompt 模板、输入 schema、输出 schema、可选工具集、评估集（golden set）的统一封装。Skill 是本系统所有 LLM 调用的唯一入口。

#### Skill 抽象定义

```python
# app/llm/skills/base.py
from abc import ABC, abstractmethod
from typing import Generic, TypeVar
from pydantic import BaseModel

InputT = TypeVar("InputT", bound=BaseModel)
OutputT = TypeVar("OutputT", bound=BaseModel)

class Skill(ABC, Generic[InputT, OutputT]):
    name: str                    # 唯一标识，如 "score.dimension"
    version: str                 # 语义化版本，如 "1.2.0"
    category: str                # parse | verify | score | profile | chat
    input_schema: type[InputT]
    output_schema: type[OutputT]
    tools: list["Tool"] = []     # 可选 Function Calling 工具
    temperature: float = 0.2
    max_tokens: int = 1500
    
    @abstractmethod
    def render_prompt(self, input_: InputT) -> list[LLMMessage]:
        """根据输入渲染 system + user 消息"""
        ...
    
    @abstractmethod
    def parse_output(self, raw: str) -> OutputT:
        """从 LLM 原始输出解析为结构化 schema，失败抛 SkillOutputError"""
        ...

    async def execute(self, input_: InputT, llm: LLMProvider) -> OutputT:
        """统一执行入口：渲染→调用→解析→重试→记录"""
        ...
```

#### Skill 分类

| 分类 | 用途 | 典型 Skill |
|------|------|-----------|
| **parse** | 文档结构化解析 | docx_to_structure / pdf_to_structure / image_to_text |
| **verify** | 实训成果核查 | coverage_check / logic_audit |
| **score** | 评分生成 | dimension_score |
| **profile** | 画像与建议 | weakness_analyze / learning_advice / teaching_summary |
| **chat** | 多轮交互 | student_qa（含工具集） |

#### Skill 注册与发现

```python
# app/llm/skills/registry.py
class SkillRegistry:
    """全局 Skill 单例注册中心"""
    _skills: dict[str, Skill] = {}
    
    @classmethod
    def register(cls, skill: Skill) -> None:
        key = f"{skill.name}@{skill.version}"
        cls._skills[key] = skill
    
    @classmethod
    def get(cls, name: str, version: str | None = None) -> Skill:
        """version 为 None 时返回最新版"""
        ...
    
    @classmethod
    def list_by_category(cls, category: str) -> list[Skill]:
        ...
```

应用启动时通过 `app/llm/skills/__init__.py` 自动导入并注册所有 Skill 子类。

#### Skill 版本化与 Golden Set 评估

- 每个 Skill 必须维护 `tests/skills/golden/{name}/` 目录，存放 `input.json` 和 `expected.json`
- 升级 Skill 版本时必须运行 `tes-cli skill-eval --skill=score.dimension --version=1.2.0` 验证回归
- 业务代码引用 Skill 时**必须指定版本**：`SkillRegistry.get("score.dimension", "1.2.0")`，避免静默升级影响行为
- 提示词模板存放在独立的 Jinja2 文件中（如 `app/llm/skills/score/dimension_score.j2`），不嵌入 Python 代码

#### 单一 Skill 调用示例

```python
class DimensionScoreInput(BaseModel):
    task_requirements: str
    dimension_name: str
    dimension_description: str
    parse_summary: str
    verify_report: VerifyResultData

class DimensionScoreOutput(BaseModel):
    score: int = Field(ge=0, le=100)
    rationale: str = Field(min_length=50, max_length=200)

class DimensionScoreSkill(Skill[DimensionScoreInput, DimensionScoreOutput]):
    name = "score.dimension"
    version = "1.0.0"
    category = "score"
    input_schema = DimensionScoreInput
    output_schema = DimensionScoreOutput
    
    def render_prompt(self, inp):
        sys = render_template("score/dimension_score.j2", **inp.model_dump())
        return [LLMMessage(role="system", content=sys)]
    
    def parse_output(self, raw):
        try:
            return DimensionScoreOutput.model_validate_json(extract_json_block(raw))
        except ValidationError as e:
            raise SkillOutputError("score.dimension", raw, e)

# 业务侧调用
skill = SkillRegistry.get("score.dimension", "1.0.0")
result = await skill.execute(DimensionScoreInput(...), llm=LLMFactory.current())
```

### Function Calling Tools Registry

针对**多轮交互场景**（特别是 AI 问答助手，需求22），单纯的 prompt 注入会导致上下文膨胀且无法实时取数。系统采用 OpenAI Function Calling 协议（所有目标 LLM 厂商均已支持），让 LLM 在多轮对话中按需调用业务工具。

#### Tool 抽象定义

```python
# app/llm/tools/base.py
class Tool(ABC, Generic[InputT, OutputT]):
    name: str                    # 必须符合 ^[a-z_][a-z0-9_]*$
    description: str             # 给 LLM 看的工具用途说明
    input_schema: type[InputT]   # 自动转换为 OpenAI tools schema
    output_schema: type[OutputT]
    requires_role: list[str]     # 仅这些角色可触发此工具
    
    @abstractmethod
    async def invoke(self, input_: InputT, ctx: ToolContext) -> OutputT:
        """ctx 提供当前用户、DB session、trace_id 等"""
        ...
```

`ToolContext` 内含调用方上下文，**禁止 Tool 内部直接读取全局状态**，所有依赖通过 ctx 注入。

#### Tool 注册中心

```python
# app/llm/tools/registry.py
class ToolRegistry:
    _tools: dict[str, Tool] = {}
    
    @classmethod
    def register(cls, tool: Tool) -> None: ...
    
    @classmethod
    def to_openai_schema(cls, tool_names: list[str]) -> list[dict]:
        """生成 OpenAI tools 数组用于 chat.completions"""
        ...
    
    @classmethod
    async def dispatch(cls, name: str, args: dict, ctx: ToolContext) -> dict:
        """LLM 返回 tool_calls 时由编排器调用此方法分派"""
        ...
```

#### AI 问答场景工具集

下表是问答场景注册的工具，覆盖学生在评价上下文中可能的追问需求：

| 工具名 | 描述 | 输入 | 输出 |
|--------|------|------|------|
| `get_parse_segment` | 查询当前评价对应实训成果中某主题的原文片段 | topic: str, max_chars: int=500 | segments: list[Segment] |
| `get_dimension_detail` | 查询当前评价某维度的详细评分依据与扣分项 | dimension_name: str | rationale, score, deductions |
| `get_class_statistics` | 查询当前任务在班级范围内的统计（不暴露他人姓名）| dimension: str \| None | mean, median, p75, distribution |
| `get_dimension_history` | 查询学生该维度在过往任务中的得分轨迹 | dimension_name: str, limit: int=10 | scores: list[ScorePoint] |
| `get_excellent_sample_summary` | 获取该任务下匿名化的高分样例摘要 | top_n: int=1 | summaries: list[Summary] |
| `get_weakness_list` | 获取学生当前已识别的薄弱点列表 | - | weaknesses: list[Weakness] |
| `get_learning_resources` | 根据知识点关键词推荐学习资源 | keyword: str | resources: list[Resource] |

权限规则：所有工具的 `requires_role=["student"]`，且工具实现内部 hard check 学生身份与 evaluation 归属，防止越权（Property 7）。

#### 调用流程

```mermaid
sequenceDiagram
    participant U as Student
    participant API as Chat API
    participant LLM as LLM Service
    participant Tools as Tool Dispatcher
    participant DB as PostgreSQL
    participant WS as WebSocket

    U->>API: 提问 "我代码质量为啥扣分"
    API->>API: 加载会话 + 当前evaluation
    API->>LLM: chat(messages, tools=[get_dimension_detail, ...])
    LLM-->>API: tool_calls=[get_dimension_detail(name="代码质量")]
    API->>Tools: dispatch("get_dimension_detail", args, ctx)
    Tools->>DB: 查询 dimension_score
    Tools-->>API: {score:65, rationale:"..."}
    API->>LLM: chat(messages + tool_result)
    loop 流式输出
        LLM-->>API: token
        API->>WS: publish chat:{session_id}
        WS-->>U: token推送
    end
```

#### 工具调用安全护栏

- **工具调用次数上限**：单条用户提问最多触发 5 次工具调用，超过则强制 LLM 总结作答（防止死循环）
- **工具结果大小上限**：每次工具返回 ≤ 8KB，超出截断并日志告警（防止上下文爆炸）
- **审计**：每次工具调用作为独立 audit 事件落库（who/when/tool/args/result_summary）
- **黑名单工具**：管理员可在 `system_config` 中临时禁用某工具，运行时生效

#### 与 Skill 的协作关系

- 多轮工具调用场景（chat 类）→ Skill 内 `tools` 非空，`execute()` 走工具调用循环
- 单轮无工具场景（其他所有类）→ Skill 内 `tools` 为空，普通一次性调用
- **Tool ≠ Skill**：Tool 是给 LLM 在运行时调用的"动作"，Skill 是工程师定义的"任务"
- 所有 Skill 与 Tool 共用同一 LLMProvider 和 trace_id，便于统一观测



```mermaid
sequenceDiagram
    participant U as Upload API
    participant Q as Celery Queue
    participant W as Parse Worker
    participant E as Extractor
    participant L as LLM
    participant DB as PostgreSQL
    participant WS as WebSocket

    U->>DB: 创建upload记录(status=pending)
    U->>Q: enqueue parse_task(upload_id)
    Q->>W: 分发任务
    W->>DB: status=parsing
    W->>WS: 推送进度10%
    W->>E: 按文件类型路由
    alt Word文档
        E->>E: python-docx提取文本+标题
    else PDF文档
        E->>E: PyMuPDF提取文本+结构
    else 图片
        E->>E: Tesseract OCR识别
    end
    E-->>W: 原始文本+结构
    W->>WS: 推送进度50%
    W->>L: 请求结构化摘要
    L-->>W: JSON结构化结果
    W->>DB: 存储parse_result(JSONB)
    W->>DB: status=parsed
    W->>WS: 推送进度100%
```

### 核查引擎设计

核查引擎将解析结果与实训任务要求进行比对，输出三类结果：

1. **覆盖度核查**：将任务要求的步骤拆分为检查点，逐一在解析结果中匹配
2. **逻辑漏洞识别**：通过LLM分析步骤间的因果关系与一致性
3. **完整性评估**：综合检查点覆盖率与逻辑得分给出总体置信度

核查输出 schema：

```json
{
  "overall_score": 85,
  "match_rate": 0.88,
  "checkpoints": [
    {
      "requirement": "完成数据库表结构设计",
      "matched": true,
      "evidence": "在第3章节中找到ER图与建表SQL",
      "confidence": 92
    }
  ],
  "missing_items": ["未提供性能测试报告"],
  "logic_issues": [
    {
      "description": "声明使用Redis缓存但代码中未引用",
      "severity": "medium"
    }
  ]
}
```

### 评价引擎设计

评价引擎组合**客观分（LLM）**与**主观分（教师）**：

```
最终得分 = Σ(维度i权重 × (客观分i × α + 主观分i × (1-α)))
```

其中 α 默认为 0.6，可由教师在配置中修改。

客观评分提示词模板（伪代码）：

```
你是软件实训评分专家。基于以下信息为该项实训成果打分：
[实训要求]: {task.requirements}
[评价维度]: {dimension.name} - {dimension.description}
[解析摘要]: {parse_result.summary}
[核查报告]: {verify_result}

请输出严格的JSON格式：
{"score": 0-100整数, "rationale": "评分依据，不超过200字"}
```

### 相似度检测引擎设计

相似度检测使用**两阶段策略**降低 LLM 调用开销（需求18）：

**阶段一：SimHash 粗筛**

- 对解析后的文本生成 64 位 SimHash 指纹（基于关键词分词与 TF-IDF 权重）
- 同一实训任务下两份提交的汉明距离 ≤ 6 视为可疑（约对应 90%+ 文本相似）
- 全部计算可在内存中通过位运算完成，O(N²) 但常数因子极低

**阶段二：嵌入向量精排**

- 仅对 SimHash 标记可疑的对，调用云端 Embedding API（如通义 `text-embedding-v3` 或智谱 `embedding-3`）生成嵌入向量
- 通过 pgvector 计算余弦相似度，精确度高
- 嵌入向量持久化到 `parse_result.embedding` 字段（vector 类型），避免重复计算与重复 API 调用
- 由于仅对 SimHash 可疑对调用云端 API，单任务下嵌入调用次数远低于学生数（典型 N=50 时仅调用 5-10 次）

```mermaid
sequenceDiagram
    participant P as Parse Worker
    participant S as Similarity Worker
    participant V as pgvector
    participant DB as PostgreSQL

    P->>DB: 写入 parse_result.text + simhash + embedding
    P->>S: enqueue similarity_task(upload_id)
    S->>DB: 查询同一task的所有simhash
    S->>S: 位运算计算汉明距离
    alt 存在距离<=6的对
        S->>V: 调用余弦相似度计算
        V-->>S: similarity_score
        alt similarity > 0.80
            S->>DB: 写入 SIMILARITY_RECORD<br/>state=suspect
            S->>NotifySvc: 通知教师
        end
    end
```

**比对范围限定**：仅在同一 `task_id` 下比对（需求18.7），避免跨任务误报，且控制计算复杂度。

### 画像服务设计

画像服务覆盖两个场景：学生薄弱点分析（需求13）和教学质量画像（需求14）。

**学生薄弱点分析**

- 触发条件：学生评价数 ≥ 3 次，或新评价完成后 24 小时内
- 数据源：该学生历次 `dimension_score` 聚合
- 处理流程：
  1. 计算每个维度的历次平均分、最低分、得分趋势（线性回归斜率）
  2. 取平均分低于 70 或趋势下降的维度作为薄弱候选
  3. 调用 LLM 生成薄弱知识点描述与学习建议（提示词包含具体评分依据）
  4. 缓存结果到 `STUDENT_PROFILE` 表，TTL 24 小时

**教学质量画像**

- 触发条件：管理员或教师按需查询，结果缓存 5 分钟
- 数据源：课程或学校范围内全部 `evaluation` 与 `dimension_score`
- 关键聚合：
  - 完成率 = 已评价提交数 / 总应提交数
  - 共性薄弱点 = 在某维度得分 < 60 的学生比例 > 30% 的项
  - 知识点掌握热力图 = 维度 × 班级矩阵的平均分
- 输出经过匿名化处理（仅展示班级聚合，需求14.6）

画像计算密集型查询全部走 PostgreSQL 物化视图，定时刷新，避免实时聚合压力。

### 批改工作台设计

批改工作台是教师批量批改的核心交互（需求15），关键设计：

**状态机扩展**：在原 `evaluation.status` 基础上加入审批流状态：

```
auto_scored → reviewed → finalized
              ↓
              rejected (打回重做)
                  ↓
              学生重新提交后回到 auto_scored
```

**列表批改视图**：使用虚拟滚动（vue-virtual-scroller）支持 1000+ 学生数据流畅展示，每行展示提交状态、综合得分、AI 客观分、教师主观分、相似度警告标记。

**并排对比视图**：

```
┌─────────────────┬─────────────────┐
│ 解析结果        │ 评分面板        │
│ - 标题结构      │ AI 客观分       │
│ - 段落内容      │ 维度1: 85       │
│ - 图表描述      │ 维度2: 78       │
│ - 原始文档预览  │                 │
│ - 核查报告      │ 教师主观分      │
│   - 缺失项      │ 维度1: [85_]    │
│   - 逻辑漏洞    │ 维度2: [80_]    │
│ - 相似度警告    │                 │
│                 │ 总评[文本框]    │
│                 │ 上一份/下一份   │
└─────────────────┴─────────────────┘
```

**批量确认与打回**：通过 `BulkAction` API 一次性提交多个 evaluation 的状态变更，事务保证原子性。

### 通知服务设计

通知服务（需求16）采用**写扩散**模型：事件发生时为每个接收者写入一条 notification 记录。

**事件来源与触发器**：

| 事件 | 触发位置 | 接收者 |
|------|---------|--------|
| 任务发布 | TaskService.publish() | 班级所有学生 |
| 解析完成 | parse_task 末尾 | 提交学生 + 任务教师 |
| 评价完成 | evaluate_task 末尾 | 提交学生 |
| 提交被打回 | BulkAction.reject() | 提交学生 |
| 截止前24小时 | Celery Beat 定时扫描 | 未提交学生 |
| 相似度警告 | similarity_task | 任务教师 |
| 薄弱点更新 | profile_task | 学生 |

**实时推送**：通过 Redis Pub/Sub 频道 `notify:{user_id}` 发布消息，前端通过 WebSocket 订阅自己的频道。离线用户登录后通过 `GET /api/notifications` 拉取未读列表。

**未读计数**：使用 Redis 计数器 `notify:unread:{user_id}` 优化高频查询，避免每次查表 COUNT。

### AI 问答助手设计

AI 问答助手（需求22）基于评价上下文为学生提供追问能力，**采用 Function Calling 工具调用模式**避免上下文膨胀，让 LLM 按需查询业务数据。

#### 上下文构造（System Prompt）

会话首条 system 消息**仅包含静态身份与规则**，不包含完整评价数据，由 LLM 通过工具调用按需获取：

```
你是实训评价答疑助手，正在帮助学生 {student_name} 理解任务"{task_name}"的评价结果。
你可以使用以下工具查询具体数据，回答时必须基于工具返回值。
回答要求简洁、准确、给出具体改进方向。
不可暴露其他学生姓名或身份信息。
```

#### 工具集

详见前文 **Function Calling Tools Registry** 章节，注册 7 个查询类工具供 chat session 使用。

#### 编排循环

```python
# app/services/chat_service.py 简化逻辑
async def answer(session: ChatSession, user_msg: str, ctx: ToolContext):
    skill = SkillRegistry.get("chat.student_qa", "1.0.0")
    history = await self.repo.list_messages(session.id, limit=20)
    messages = skill.render_prompt(SkillInput(history=history, user_msg=user_msg))
    
    for round_idx in range(MAX_TOOL_ROUNDS := 5):
        resp = await llm.chat_with_tools(messages, tools=skill.openai_tools_schema())
        if resp.finish_reason == "stop":
            yield resp.content   # 最终回答，开始流式输出
            return
        for call in resp.tool_calls:
            result = await ToolRegistry.dispatch(call.name, call.arguments, ctx)
            messages.append(LLMMessage(role="tool", content=json.dumps(result)))
    
    # 工具循环达到上限仍未给出答案，强制总结
    messages.append(LLMMessage(role="user", content="请基于已有信息直接给出回答"))
    final = await llm.chat_stream(messages)
    async for token in final:
        yield token
```

#### 会话管理

- 单会话最多 20 轮**用户消息**，超过则提示开启新会话
- 工具调用消息不计入用户消息计数
- 每条消息（user/assistant/tool）入 `CHAT_MESSAGE` 表，便于审计与回放
- 支持历史会话查看与按 evaluation_id 定位

#### 流式响应

```mermaid
sequenceDiagram
    participant S as Student
    participant API as Chat API
    participant Orch as Chat Orchestrator
    participant LLM as LLM Service
    participant Tools as Tool Dispatcher
    participant DB as PostgreSQL
    participant WS as WebSocket

    S->>API: POST /api/chat/{session_id}/messages
    API->>API: 校验配额 + 敏感词
    API-->>S: 200 OK (message_id)
    API->>Orch: answer_async(session, msg)
    
    loop 工具调用 (≤5次)
        Orch->>LLM: chat_with_tools(messages, tools)
        alt LLM 返回 tool_calls
            LLM-->>Orch: tool_calls
            Orch->>Tools: dispatch(name, args, ctx)
            Tools->>DB: 查询数据
            Tools-->>Orch: result
        else LLM 返回最终内容
            LLM-->>Orch: stop
        end
    end
    
    Orch->>LLM: chat_stream(final_messages)
    loop 流式响应
        LLM-->>Orch: token
        Orch->>WS: publish chat:{session_id}:{message_id}
        WS-->>S: token 推送
    end
    Orch->>DB: 写入 CHAT_MESSAGE 完整内容
```

#### 配额与安全

- **频次限制**：Redis 计数器 `chat:limit:{user_id}:{date}` 实现日上限 50 次（需求22.8），超出返回 HTTP 429 且不消耗 LLM 配额
- **敏感词过滤**：`SensitiveFilter` 在 LLM 调用前对用户提问做关键词检查；若包含与评价无关或敏感内容则直接拒绝（需求22.6）
- **越权防护**：所有工具内部硬编码当前 user_id 与 evaluation_id 的归属校验，LLM 无法通过参数注入绕过
- **回答审计**：每次回答生成完成后将完整对话作为单条 audit 事件记录

### 审计日志服务设计

审计日志（需求20）作为合规与追溯能力的关键基础。

**写入策略**：

- 通过 FastAPI 中间件 `AuditMiddleware` 拦截目标接口（基于装饰器 `@audit("USER_CREATE")` 标记）
- 异步写入：日志先入 Redis Stream，后台 worker 批量落库 PostgreSQL，避免阻塞业务请求
- 关键字段：log_id (UUID)、timestamp (毫秒)、user_id、role、action、target_type、target_id、result、client_ip、user_agent

**不可篡改保障**：

- `audit_log` 表配置 PostgreSQL 触发器，拒绝 UPDATE 与 DELETE 操作
- 数据库角色权限：业务连接仅 INSERT/SELECT 权限，DELETE/UPDATE 权限保留给 DBA 手动归档

**告警规则**：

- 同一用户 5 分钟内失败操作 ≥ 10 次 → 标记 `suspicious_flag=true` 并推送管理员仪表盘
- 通过 Celery Beat 定时扫描实现，每分钟一次

### 仪表盘聚合服务

仪表盘（需求19）通过专门的 `DashboardService` 聚合多源数据：

**缓存策略**：

- 查询结果缓存到 Redis，键为 `dashboard:{role}:{user_id}`
- TTL 5 分钟
- 关键事件（新评价完成、新通知）触发对应缓存失效

**预计算**：

- 班级活跃度、待批改数等指标通过 PostgreSQL 物化视图，每 10 分钟刷新
- 系统资源使用率通过 `psutil` 实时采集

### 批量导入导出设计

批量导入（需求21）使用流式处理避免内存峰值：

- `openpyxl.iter_rows` 分块读取，单次仅加载 100 行
- 字段校验在读取阶段完成，校验失败的行写入失败 Excel
- 数据库写入分批提交（每 50 条一个事务），失败回滚单批，不影响其他批次
- 导入结果以 `ImportJob` 实体记录，前端可查询进度

导出反向使用 `openpyxl.WriteOnlyWorkbook`，按行写入避免一次性序列化。

### REST API 端点（核心）

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
| `/ws/progress/{token}` | WebSocket | 已认证 | 任务进度推送 |
| `/ws/notify/{token}` | WebSocket | 已认证 | 实时通知推送 |
| `/ws/chat/{token}` | WebSocket | 已认证 | AI问答流式响应 |

## Data Models

### 实体关系图（ERD）

```mermaid
erDiagram
    USER ||--o{ TRAINING_TASK : creates
    USER ||--o{ UPLOAD : submits
    USER ||--o{ EVALUATION : evaluates
    USER ||--o{ NOTIFICATION : receives
    USER ||--o{ CHAT_SESSION : owns
    USER ||--o{ AUDIT_LOG : "operates"
    USER }o--o{ CLASS : "belongs to"
    COURSE ||--o{ CLASS : contains
    COURSE ||--o{ TRAINING_TASK : "groups"
    CLASS ||--o{ TRAINING_TASK : "assigned to"
    TRAINING_TASK ||--o{ DIMENSION : defines
    TRAINING_TASK ||--o{ UPLOAD : "submitted to"
    UPLOAD ||--|| PARSE_RESULT : has
    UPLOAD ||--|| VERIFY_RESULT : has
    UPLOAD ||--|| EVALUATION : has
    UPLOAD ||--o{ SIMILARITY_RECORD : "involved in"
    EVALUATION ||--o{ DIMENSION_SCORE : contains
    EVALUATION ||--o{ EVALUATION_HISTORY : "tracks"
    EVALUATION ||--o{ CHAT_SESSION : "discussed in"
    DIMENSION ||--o{ DIMENSION_SCORE : "scored by"
    EVALUATION_TEMPLATE ||--o{ TEMPLATE_DIMENSION : contains
    USER ||--o{ STUDENT_PROFILE : "profiled as"
    CHAT_SESSION ||--o{ CHAT_MESSAGE : contains
    IMPORT_JOB ||--o{ IMPORT_RECORD : contains

    USER {
        bigint id PK
        string username UK
        string password_hash
        string role "admin|teacher|student"
        string display_name
        bool is_active
        int failed_login_count
        datetime locked_until
        datetime created_at
    }

    COURSE {
        bigint id PK
        string code UK
        string name
        text description
        bool is_archived
        datetime created_at
    }

    CLASS {
        bigint id PK
        bigint course_id FK
        bigint teacher_id FK
        string name
        string semester "学年学期"
        bool is_archived
        datetime created_at
    }

    TRAINING_TASK {
        bigint id PK
        bigint teacher_id FK
        bigint course_id FK
        string name
        text description
        text requirements
        text evaluation_criteria
        datetime deadline
        string status "draft|published|closed"
        datetime created_at
    }

    DIMENSION {
        bigint id PK
        bigint task_id FK
        string name
        string description
        int weight "1-100，总和=100"
        int order_index
    }

    EVALUATION_TEMPLATE {
        bigint id PK
        bigint owner_id FK
        string name
        text description
        string visibility "private|team|system"
        datetime created_at
    }

    TEMPLATE_DIMENSION {
        bigint id PK
        bigint template_id FK
        string name
        string description
        int weight
        int order_index
    }

    UPLOAD {
        bigint id PK
        bigint task_id FK
        bigint student_id FK
        string filename
        string file_type "doc|docx|pdf|png|jpg|jpeg"
        bigint file_size
        string file_path
        string sha256
        string status "pending|parsing|parsed|failed"
        datetime created_at
    }

    PARSE_RESULT {
        bigint id PK
        bigint upload_id FK UK
        jsonb structured_content
        text raw_text
        bigint simhash "64位指纹"
        vector embedding "512维"
        string error_message
        datetime parsed_at
    }

    VERIFY_RESULT {
        bigint id PK
        bigint upload_id FK UK
        decimal match_rate
        jsonb checkpoints
        jsonb missing_items
        jsonb logic_issues
        int overall_confidence
        datetime verified_at
    }

    EVALUATION {
        bigint id PK
        bigint upload_id FK UK
        bigint teacher_id FK
        decimal final_score
        decimal objective_ratio
        string status "auto_scored|reviewed|finalized|rejected"
        text overall_comment
        datetime updated_at
    }

    DIMENSION_SCORE {
        bigint id PK
        bigint evaluation_id FK
        bigint dimension_id FK
        int objective_score
        text objective_rationale
        int subjective_score
        text subjective_comment
    }

    EVALUATION_HISTORY {
        bigint id PK
        bigint evaluation_id FK
        bigint operator_id FK
        string action
        jsonb before_value
        jsonb after_value
        datetime changed_at
    }

    SIMILARITY_RECORD {
        bigint id PK
        bigint task_id FK
        bigint upload_a_id FK
        bigint upload_b_id FK
        int hamming_distance
        decimal cosine_similarity
        string state "suspect|confirmed|ignored"
        bigint reviewed_by FK
        datetime created_at
    }

    STUDENT_PROFILE {
        bigint id PK
        bigint student_id FK UK
        jsonb radar_data
        jsonb weakness_list
        jsonb suggestions
        datetime computed_at
    }

    NOTIFICATION {
        bigint id PK
        bigint recipient_id FK
        string event_type
        string title
        text content
        jsonb payload
        bool is_read
        datetime created_at
    }

    NOTIFICATION_PREF {
        bigint id PK
        bigint user_id FK
        string event_type
        bool enabled
    }

    CHAT_SESSION {
        bigint id PK
        bigint user_id FK
        bigint evaluation_id FK
        string title
        datetime created_at
    }

    CHAT_MESSAGE {
        bigint id PK
        bigint session_id FK
        string role "user|assistant"
        text content
        int prompt_tokens
        int completion_tokens
        datetime created_at
    }

    AUDIT_LOG {
        bigint id PK
        datetime occurred_at
        bigint user_id FK
        string role
        string action
        string target_type
        bigint target_id
        string result
        string client_ip
        string user_agent
        bool suspicious_flag
    }

    IMPORT_JOB {
        bigint id PK
        bigint operator_id FK
        string job_type "user|student"
        string status "pending|processing|done|failed"
        int total_count
        int success_count
        int failed_count
        string failed_file_path
        datetime created_at
    }

    IMPORT_RECORD {
        bigint id PK
        bigint job_id FK
        int row_number
        string status
        text error_message
    }
```

### 关键字段说明

- **USER.password_hash**：使用 bcrypt（cost factor=12）哈希存储，满足需求11.1
- **USER.locked_until**：账号锁定到期时间，登录时若该字段大于当前时间则拒绝（需求2.6）
- **TRAINING_TASK.status** 流转：`draft` → `published` → `closed`，关闭后禁止学生提交（需求3.4）
- **DIMENSION.weight**：在事务中校验同一 task 下所有维度权重之和等于100
- **UPLOAD.file_path**：相对存储根的路径，如 `task_{id}/student_{id}/{uuid}.docx`，避免文件名冲突
- **UPLOAD.sha256**：用于断点续传校验完整性，并识别重复上传
- **PARSE_RESULT.structured_content**：使用 JSONB 存储大模型返回的结构化解析结果，便于后续核查与查询
- **PARSE_RESULT.simhash**：64 位整数，用于快速文本相似度粗筛
- **PARSE_RESULT.embedding**：pgvector 类型 512 维向量，用于语义相似度精排
- **VERIFY_RESULT.checkpoints**：JSON 数组，每个对象包含 requirement / matched / evidence / confidence
- **EVALUATION.objective_ratio**：客观评分占比（默认0.6），主观占比为 `1 - objective_ratio`
- **EVALUATION.status** 扩展：新增 `rejected` 状态用于审批流的"打回重做"
- **EVALUATION_HISTORY**：每次评分变更（字段、操作人、前后值）记录一行，不删除（需求7.8）
- **SIMILARITY_RECORD.state**：`suspect` 系统自动标记，`confirmed` / `ignored` 由教师人工裁定
- **STUDENT_PROFILE.weakness_list**：JSONB 数组，含薄弱点名称、当前掌握度、相关任务引用
- **NOTIFICATION.payload**：JSONB 灵活存储事件相关业务数据（如跳转目标）
- **CHAT_SESSION.evaluation_id**：会话与评价关联，用于上下文构造与定位
- **AUDIT_LOG**：仅追加表，触发器拒绝 UPDATE/DELETE
- **IMPORT_JOB.failed_file_path**：失败明细 Excel 的存储路径，用户可下载

### 索引策略

- `USER.username` 唯一索引（登录查询）
- `UPLOAD(task_id, student_id)` 复合索引（教师按任务批阅）
- `UPLOAD.status` 索引（异步任务调度）
- `EVALUATION.upload_id` 唯一索引（一对一关系）
- `EVALUATION(teacher_id, status)` 索引（批改工作台筛选）
- `DIMENSION_SCORE(evaluation_id, dimension_id)` 唯一复合索引
- `NOTIFICATION(recipient_id, is_read, created_at DESC)` 索引（未读列表查询）
- `AUDIT_LOG(occurred_at, user_id)` 索引（审计查询）
- `AUDIT_LOG(suspicious_flag) WHERE suspicious_flag=true` 部分索引（告警扫描）
- `SIMILARITY_RECORD(task_id, state)` 索引（教师查阅）
- `PARSE_RESULT.simhash` 哈希索引（粗筛性能优化）
- `PARSE_RESULT USING ivfflat (embedding vector_cosine_ops)` 向量索引（pgvector 精排）
- `CHAT_MESSAGE(session_id, created_at)` 索引（会话内消息查询）

### 物化视图

- `mv_class_progress`：班级批改进度聚合，10 分钟刷新
- `mv_course_metrics`：课程级评分分布、维度对比，1 小时刷新
- `mv_school_overview`：全校汇总，6 小时刷新

## Error Handling

### 错误处理分层

```mermaid
graph LR
    A[客户端] -->|API请求| B[FastAPI异常处理器]
    B -->|认证失败| B1[401 Unauthorized]
    B -->|权限不足| B2[403 Forbidden]
    B -->|参数校验失败| B3[422 Unprocessable Entity]
    B -->|资源不存在| B4[404 Not Found]
    B -->|业务规则违反| B5[400 Bad Request<br/>含 error_code]
    B -->|未捕获异常| B6[500 + 日志记录]

    C[Celery Worker] -->|文件解析失败| C1[更新status=failed<br/>记录error_message]
    C -->|LLM超时/失败| C2[重试3次后失败<br/>WebSocket通知]
    C -->|OCR失败| C3[降级返回空文本<br/>标记需人工复核]
```

### 关键错误场景

| 场景 | 处理策略 |
|------|----------|
| LLM服务不可用 | 解析/核查任务降级标记，教师可手动评分（需求8.6） |
| 文件解析超时（>120秒） | 标记upload为failed，前端显示重试按钮（需求5.6） |
| 数据库连接断开 | 应用层重试3次，失败则返回503，systemd自动重启服务 |
| 权重之和不等于100% | 在维度配置接口立即拒绝，返回字段级错误（需求3.6） |
| 上传恶意文件 | 文件头magic校验 + 扩展名白名单（需求11.7） |
| 同一文件重复上传 | 服务端校验SHA256，已存在则替换原记录 |
| AI 问答超出频次限额 | 返回 429 Too Many Requests，提示明日再试（需求22.8） |
| 相似度检测失败 | 不阻塞主流程，标记为 `similarity_pending`，后台重试 |
| 批量导入部分失败 | 单批回滚不影响其他批，生成失败明细 Excel（需求21.4） |
| 通知推送失败 | 通知数据已落库，下次用户上线主动拉取补发 |
| 审计日志写入失败 | 写入 Redis Stream 后失败，告警运维但不阻塞业务请求 |
| 长时间未操作的 WS 连接 | 服务端 60 秒心跳无响应主动断连，前端自动重连 |
| LLM 返回非 JSON | 启用容错解析（提取 `{...}` 块），仍失败则重试 3 次 |

### 统一错误响应格式

```json
{
  "error_code": "WEIGHT_SUM_INVALID",
  "message": "评价指标权重之和必须为100%，当前为85%",
  "field": "dimensions",
  "trace_id": "8e2f...c4a1"
}
```

## Testing Strategy

测试是本项目的一等公民。每次代码变更必须通过完整测试矩阵才允许合入。测试代码与业务代码同等重要，不得 `# noqa` 跳过任何检查。

### 测试金字塔

```
        ╱╲    e2e 端到端     5%   Playwright + 真实服务
       ╱──╲   契约测试       10%  schemathesis 跑 OpenAPI
      ╱────╲  集成测试       30%  含 DB/Redis 的服务流程
     ╱──────╲ 单元测试       55%  纯函数 + Mock 外部依赖
```

### 各层职责与工具

| 层次 | 工具 | 覆盖目标 | 强制要求 |
|------|------|----------|---------|
| **单元** | pytest + pytest-asyncio | 评分计算、权重校验、状态机、SimHash、Skill 渲染、Tool 业务逻辑 | 核心算法 100% 行覆盖 + 所有边界值 |
| **集成** | pytest + testcontainers (Postgres/Redis) | 服务层端到端流程（含真实 DB） | 主流程 Happy Path + 至少 2 条异常路径 |
| **契约** | schemathesis | 所有 OpenAPI 接口 schema 一致性 | 全部 endpoint 自动 fuzz |
| **e2e** | Playwright | 5 条关键用户路径 | 录屏归档供答辩用 |
| **前端单元** | Vitest + Vue Test Utils | 组件、Pinia store、纯 util | 关键组件 80% 覆盖 |
| **性能** | Locust | 50 并发用户（需求11.6） | 平均响应 ≤ 3 秒 |
| **Skill 评估** | tes-cli skill-eval | 每个 Skill 的 golden set | 改 Skill 前后必跑 |

### 测试隔离与替身

为支持单元/集成测试隔离外部依赖，所有外部 IO 通过 Protocol 接口注入，测试时用 `tests/fakes/` 中的 fake 实现替换：

| Protocol | 真实实现 | 测试替身 |
|---------|----------|---------|
| `LLMProvider` | `OpenAICompatProvider` | `FakeLLM`（按预设字典返回） |
| `OcrEngine` | `TesseractEngine` | `FakeOcr`（返回预置文本） |
| `FileStorage` | `LocalFileStorage` | `InMemoryStorage` |
| `Clock` | `SystemClock` | `FrozenClock`（控制时间） |
| `EmbeddingService` | 同 LLMProvider | `FakeEmbedder`（生成固定向量） |

### Dev 调试端点（无人工介入测试基础设施）

为让 AI 或测试脚本可独立验证业务功能而不依赖前端 UI，系统在 `ENV=dev` 下启用 `/api/_dev/*` 子路由（生产环境通过中间件硬关闭）。

#### 调试端点清单

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

**安全保障**：

- 路由仅在 `Settings.env == "dev"` 时注册，生产构建产物不含此模块
- 启动时若 `env=prod` 但 `_dev` 模块被 import，应用直接 panic 退出（`assert env != "prod"`）
- 所有调试端点要求特殊请求头 `X-Dev-Token`，与配置文件中的 `dev_token` 比对

### 管理 CLI（tes-cli）

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

### 关键测试用例

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

### CI 流水线

```
push → lint(ruff+mypy) → unit → integration(testcontainers) → contract → build artifact
                          ↓ all pass
                     skill-eval (golden set 回归)
                          ↓ all pass
                     LoongArch QEMU 镜像构建 → e2e smoke
```

任意环节失败阻断 PR 合入。`main` 分支保护开启，必须通过 review。

### 部署验证清单

每次发版前在LoongArch+银河麒麟环境执行：

1. 全部Python依赖在LoongArch上成功安装（无预编译wheel依赖x86_64）
2. PostgreSQL/Redis/Nginx 服务启动正常
3. 云端 LLM API 连通性测试通过（管理员后台一键测试）
4. 端到端流程：注册→登录→创建任务→上传→解析→评分→导出，全链路通过
5. 系统启动时间≤60秒（需求1.4）
6. 内存占用≤6GB（预留给操作系统）
7. 出站 HTTPS 访问云端 API 通畅，且 API Key 加密存储校验通过
8. `tes-cli health-check` 全部 PASS
9. `tes-cli simulate-evaluation` 端到端演练成功

## Deployment

### LoongArch 部署关键点

- **Python**：使用银河麒麟官方源 `python3.10` 或编译安装
- **PostgreSQL**：`yum install postgresql-server` 或源码编译，需 `--build=loongarch64-linux-gnu`
- **pgvector 扩展**：从源码编译 `make && make install`，纯 C 实现，LoongArch 兼容良好
- **Redis**：源码编译 `make` 即可，无架构依赖
- **Tesseract**：`yum install tesseract` 或源码编译，需安装 `tesseract-langpack-chi-sim` 中文包
- **Nginx**：官方源直接安装
- **网络出站**：服务器需可访问云端 LLM API 域名（HTTPS 443），如需走代理则在 `/etc/environment` 配置 `HTTPS_PROXY`

### 一键部署脚本结构

```
deploy/
├── install.sh              # 主安装脚本
├── systemd/
│   ├── tes-api.service
│   ├── tes-worker.service
│   └── tes-beat.service
├── nginx/
│   └── tes.conf
├── postgres/
│   ├── init.sql            # 表结构 + pgvector扩展
│   └── audit_triggers.sql  # 审计日志触发器
├── env/
│   └── .env.example        # 环境变量模板（数据库、Redis、LLM_KEY_MASTER）
└── README.md
```

### 备份与恢复

- Celery Beat 每日 02:00 触发 `pg_dump` 至 `/data/backups/tes_YYYYMMDD.sql.gz`
- 保留最近7天备份（需求11.4）
- 文件目录使用 systemd timer 调用 `tar` 增量打包
- 审计日志归档：超过 12 个月的日志由管理员手动触发归档（需求20.3）

## Configuration Management

### 配置三层模型

| 层 | 形式 | 生效时机 | 内容举例 |
|----|------|---------|---------|
| L1 默认值 | `Settings` 类字段默认值 | 编译时 | 上传大小默认 50MB |
| L2 部署级 | `.env` 文件 / 环境变量（前缀 `TES_`） | 启动时 | DB_URL, REDIS_URL, LLM_KEY_MASTER, ENV |
| L3 业务级 | `system_config` 表 + Redis 缓存 | 运行时（管理员可改） | 评分客观比 α、相似度阈值、限流 |

读取顺序：L3 → L2 → L1，前者覆盖后者。所有读配置必须经过 `Settings.get()` 或 `SystemConfig.get(key)`，禁止 `os.environ` 直读。

### .env.example 模板

```bash
# === 部署模式 ===
TES_ENV=prod                              # dev | test | prod
TES_DEBUG=false
TES_LOG_LEVEL=INFO

# === 数据库 ===
TES_DB_URL=postgresql+asyncpg://tes:secret@localhost:5432/tes
TES_DB_POOL_SIZE=20
TES_DB_POOL_OVERFLOW=10

# === Redis ===
TES_REDIS_URL=redis://localhost:6379/0
TES_REDIS_MAX_CONNECTIONS=50

# === 文件存储 ===
TES_UPLOAD_ROOT=/data/uploads
TES_BACKUP_ROOT=/data/backups
TES_MAX_UPLOAD_SIZE_MB=50

# === JWT ===
TES_JWT_SECRET=                           # 必填，至少32位
TES_JWT_ALGORITHM=HS256
TES_JWT_ACCESS_TTL_MINUTES=60
TES_JWT_REFRESH_TTL_DAYS=7

# === 加密 ===
TES_LLM_KEY_MASTER=                       # AES-256 主密钥，base64 编码
TES_PASSWORD_BCRYPT_ROUNDS=12

# === Celery ===
TES_CELERY_BROKER_URL=redis://localhost:6379/1
TES_CELERY_RESULT_BACKEND=redis://localhost:6379/2
TES_CELERY_WORKER_CONCURRENCY=4

# === 业务默认值（运行时可被 system_config 覆盖）===
TES_PARSE_TIMEOUT_SECONDS=120
TES_SIMILARITY_HAMMING_THRESHOLD=6
TES_SIMILARITY_COSINE_THRESHOLD=0.80
TES_CHAT_DAILY_QUOTA=50
TES_CHAT_MAX_TOOL_ROUNDS=5
TES_LLM_TIMEOUT_SECONDS=60
TES_LLM_RETRY_MAX=3

# === Dev 调试 ===
TES_DEV_TOKEN=                            # 仅 ENV=dev 启用调试端点时必填
```

### system_config 表设计

```sql
CREATE TABLE system_config (
  key         VARCHAR(64) PRIMARY KEY,
  value       JSONB NOT NULL,
  category    VARCHAR(32) NOT NULL,       -- evaluation | similarity | chat | llm | ui
  description TEXT,
  updated_by  BIGINT REFERENCES users(id),
  updated_at  TIMESTAMP NOT NULL DEFAULT NOW()
);
```

业务代码读取：

```python
ratio = await SystemConfig.get_float("evaluation.objective_ratio", default=0.6)
```

`SystemConfig.get` 内部走 Redis 缓存（TTL 60 秒），管理员后台修改时主动失效。

## Logging & Observability Specification

### 日志格式

所有日志输出为单行 JSON，便于采集与查询：

```json
{
  "timestamp": "2026-05-18T10:23:11.482Z",
  "level": "INFO",
  "logger": "app.services.evaluation_service",
  "trace_id": "8e2f4c1d-a4b1-4f3a-9e8b-12c3a4d5e6f7",
  "span_id": "abc123",
  "user_id": 1042,
  "user_role": "teacher",
  "event": "evaluation.bulk_confirm.success",
  "task_id": 88,
  "evaluation_ids": [501, 502, 503],
  "duration_ms": 145
}
```

### 日志级别使用规范

| 级别 | 用途 | 频率上限 |
|------|------|---------|
| DEBUG | 详细排查信息，仅 dev 环境启用 | 不限 |
| INFO | 关键业务事件入口/出口 | 每请求 5-20 条 |
| WARNING | 业务降级、重试、性能异常 | 偶发 |
| ERROR | 业务异常、外部依赖失败 | 罕见 |
| CRITICAL | 服务不可用、数据丢失风险 | 仅紧急 |

### 强制日志事件命名约定

`<domain>.<action>.<outcome>`，全部小写下划线。

例：`upload.create.success / parse.execute.failed / chat.tool_call.dispatched / audit.log.write_attempt_blocked`

### Trace ID 透传

| 入口 | 来源 | 透传机制 |
|------|------|---------|
| HTTP API | `X-Trace-Id` 请求头，缺失则生成 UUID4 | FastAPI 中间件写入 `contextvars` |
| WebSocket | 握手 query param `trace_id` | 同上 |
| Celery 任务 | 由调用方写入 `headers["trace_id"]` | Celery signal 接收时写入 contextvars |
| LLM 调用 | trace_id 作为 metadata 传入 LLM 元数据（部分 API 支持） | 透明 |

`structlog` processor 自动从 contextvars 读 trace_id 注入每条日志。

### 度量指标（Metrics）

通过 `prometheus_fastapi_instrumentator` 暴露 `/metrics`，关键指标：

| 指标名 | 类型 | 说明 |
|--------|------|------|
| `tes_http_requests_total{path,method,status}` | Counter | HTTP 请求计数 |
| `tes_http_duration_seconds{path}` | Histogram | 响应延迟 |
| `tes_celery_task_duration_seconds{task}` | Histogram | 任务耗时 |
| `tes_llm_call_duration_seconds{provider,model,skill}` | Histogram | LLM 调用延迟 |
| `tes_llm_tokens_total{provider,model,type}` | Counter | token 消耗 |
| `tes_active_users` | Gauge | 当前在线用户数 |
| `tes_db_pool_in_use` | Gauge | 数据库连接池占用 |

### 健康检查端点

| 端点 | 用途 |
|------|------|
| `GET /healthz` | 存活检查（仅返回 200） |
| `GET /readyz` | 就绪检查（含 DB/Redis 探测） |
| `GET /api/_dev/health/full` | 完整健康（dev 启用，含 LLM/OCR/磁盘） |

### 错误追踪

接入轻量的 Sentry 兼容服务（如 GlitchTip 自部署）记录未捕获异常。生产环境告警渠道为邮件 + 站内通知给管理员。

## Architecture Decision Records (ADR)

每项关键架构决策以独立 markdown 文件留存于 `docs/adr/`，便于审计与新人 onboarding。下表是必须输出的 ADR 列表，每个 ADR 包含：决策背景、备选方案对比、决策结果、影响、状态（accepted/superseded）。

| ID | 标题 | 一句话决策 |
|----|------|-----------|
| ADR-0001 | 采用 PostgreSQL + pgvector 作为统一数据存储 | 一站式承载关系数据、JSONB 半结构化、全文检索、向量检索 |
| ADR-0002 | 仅采用云端 LLM API 不本地部署 | 8GB 内存约束下本地推理不可行，云端稳定性更优 |
| ADR-0003 | 使用 Celery 而非 ARQ 作为异步任务框架 | Celery 生态成熟、运维工具齐全，本项目对 async 性能不敏感 |
| ADR-0004 | AI 问答采用 Function Calling 而非 RAG | 数据已结构化在 DB 中，工具调用比向量检索更精准 |
| ADR-0005 | LLM 调用统一通过 Skill Registry | 提示词版本化、可独立评估、便于回归测试 |
| ADR-0006 | Tool 与 Skill 分离 | Skill 是工程师定义的任务，Tool 是 LLM 运行时调用的动作，职责正交 |
| ADR-0007 | 审计日志通过 DB 触发器保证不可篡改 | 比应用层 ACL 更可靠，DBA 权限隔离 |
| ADR-0008 | 评分配置 alpha 与权重热更新 | 通过 system_config 表 + Redis 缓存实现，无需重启 |
| ADR-0009 | 配置三层模型（默认值/.env/system_config） | 兼顾启动期硬约束与运行期灵活性 |
| ADR-0010 | 测试金字塔与 dev 调试端点 | 强制可测试性，AI 与脚本可独立验证业务 |
| ADR-0011 | API Key AES-256 加密存储 | 主密钥环境变量隔离，杜绝代码仓库泄漏 |
| ADR-0012 | 相似度采用 SimHash 粗筛 + pgvector 精排 | 控制云端 Embedding API 调用次数 |
| ADR-0013 | 前端 OpenAPI client 自动生成 | 后端契约即前端契约，杜绝接口字段不一致 |

每个 ADR 在初次代码合入对应模块前必须完成；后续如有变更需新建 ADR 标记原决策为 `superseded`。

## 设计与需求映射

| 需求 | 对应设计章节 |
|------|--------------|
| 需求1 部署兼容性 | Architecture - 部署架构、Deployment |
| 需求2 用户认证 | Components - REST API、Data Models - USER |
| 需求3 任务管理 | Data Models - TRAINING_TASK / DIMENSION |
| 需求4 成果上传 | Components - 上传API、Data Models - UPLOAD |
| 需求5 智能解析 | Components - 解析引擎流水线、LLM Skills (parse 类) |
| 需求6 智能核查 | Components - 核查引擎、LLM Skills (verify 类) |
| 需求7 多维度评价 | Components - 评价引擎、LLM Skills (score 类)、EVALUATION_HISTORY |
| 需求8 LLM管理 | Components - LLM抽象适配器、Configuration - L2 部署级 |
| 需求9 报表导出 | Components - REST API、Project Structure - reporting/ |
| 需求10 Web界面 | Components - 前端组件结构 |
| 需求11 安全稳定 | Engineering Principles 6, Error Handling、Deployment - 备份、AUDIT_LOG |
| 需求12 班级与课程 | Data Models - COURSE / CLASS、Components - 组织服务 |
| 需求13 学生薄弱点 | Components - 画像服务、LLM Skills (profile 类) |
| 需求14 教学画像 | Components - 画像服务、物化视图设计 |
| 需求15 批量批改 | Components - 批改工作台设计、状态机扩展 |
| 需求16 消息通知 | Components - 通知服务、Data Models - NOTIFICATION |
| 需求17 评价模板 | Data Models - EVALUATION_TEMPLATE |
| 需求18 相似度检测 | Components - 相似度检测引擎、Data Models - SIMILARITY_RECORD |
| 需求19 仪表盘 | Components - 仪表盘聚合服务 |
| 需求20 审计日志 | Components - 审计日志服务、Data Models - AUDIT_LOG、ADR-0007 |
| 需求21 批量导入导出 | Components - 批量导入导出设计、Data Models - IMPORT_JOB |
| 需求22 AI问答助手 | Components - AI问答助手设计、Function Calling Tools Registry、LLM Skills (chat 类) |
| 需求23 评价历史趋势 | Data Models - EVALUATION 索引扩展、前端组件 |
| 工程规范（横切） | Engineering Principles、Configuration Management、Logging & Observability、Testing Strategy、ADR |

## Correctness Properties

系统必须保持以下不变量。这些性质是验证测试与代码评审的关键检查点。

### Property 1: 权重总和守恒

对任意 `task_id`，`SUM(dimensions.weight WHERE task_id = X) = 100`。任务在 `published` 状态前必须满足此条件。

**Validates: Requirements 3.5, 7.1**

### Property 2: 评分范围约束

所有 `objective_score` 与 `subjective_score` 的取值必须在 `[0, 100]` 闭区间内。

**Validates: Requirements 7.3, 7.4**

### Property 3: 综合得分一致性

`evaluation.final_score` 在每次维度分变更或权重调整后必须重新计算，并等于：
`Σ(weight_i × (obj_i × α + subj_i × (1-α))) / 100`

**Validates: Requirements 7.5, 7.6**

### Property 4: 状态机单调性

`training_task.status` 仅允许 `draft → published → closed` 的单向流转，禁止逆向状态变更。

**Validates: Requirements 3.1, 3.4**

### Property 5: 上传归属唯一

每个 `upload` 必须严格归属于一个 `(task_id, student_id)` 对。学生只能为自己提交，不能代他人上传。

**Validates: Requirements 4.1, 2.4**

### Property 6: 密码不可逆

`user.password_hash` 必须经过 bcrypt（cost factor ≥ 10）或 Argon2 哈希存储。系统中不存在任何路径返回或日志记录明文密码。

**Validates: Requirements 11.1**

### Property 7: 越权防护

学生只能访问自己提交的 upload 与 evaluation。教师只能访问自己创建的 task 下的数据。任何跨用户访问必须返回 403。

**Validates: Requirements 2.3, 2.4, 11.2**

### Property 8: 会话有效性

JWT 过期、用户被禁用或被锁定后，下一次受保护请求必须返回 401，不可继续访问任何业务资源。

**Validates: Requirements 2.6, 11.2**

### Property 9: 文件类型可信

`upload.file_type` 必须同时通过扩展名白名单与文件头 magic number 校验。任一校验失败则拒绝存储。

**Validates: Requirements 4.1, 4.3, 11.6**

### Property 10: LLM降级可用

当 LLM_Service 不可用时，教师手动评分入口、报表生成与导出功能必须保持可用，不产生级联失败。

**Validates: Requirements 8.6, 11.4**

### Property 11: 任务幂等

Celery 解析、核查、评分任务设计为幂等。重复执行同一 `upload_id` 的解析任务不会产生重复 `parse_result`，由数据库唯一约束保障。

**Validates: Requirements 5.1, 5.5, 11.4**

### Property 12: 进度可观测且必终结

每个长耗时任务在执行过程中至少推送一次进度事件，最终必须以 `parsed` 或 `failed` 状态终结。不存在永久停留在 `parsing` 中间态的记录，由超时看门狗（>120秒）保障。

**Validates: Requirements 5.6, 10.4**

### Property 13: 班级归属一致性

学生只能向其所属班级被发布的实训任务提交成果。`upload.task_id` 关联的任务必须包含至少一个该学生归属的 `class_id`，否则提交被拒绝。

**Validates: Requirements 12.4, 12.5, 4.1**

### Property 14: 审计日志不可篡改

`audit_log` 表的任何记录一旦写入，其后通过应用层连接执行的 UPDATE 或 DELETE 必须被数据库触发器拒绝。审计日志只能通过 DBA 权限手动归档。

**Validates: Requirements 20.5**

### Property 15: 通知必达

每条 `NOTIFICATION` 记录写入数据库后，无论实时推送是否成功，离线用户登录后通过 `GET /api/notifications` 查询时必须能够看到该通知。未读计数与数据库实际未读记录数保持最终一致。

**Validates: Requirements 16.1, 16.2, 16.3**

### Property 16: 相似度比对范围限定

`SIMILARITY_RECORD` 中的两个 `upload_id` 必须归属于同一个 `task_id`。跨任务的相似度比对结果不会被生成或存储。

**Validates: Requirements 18.7**

### Property 17: AI 问答配额受控

单个学生在同一日期内调用 AI 问答助手不得超过 50 次。超出限额时 API 必须返回 HTTP 429，且不消耗 LLM 服务调用资源。

**Validates: Requirements 22.8**

### Property 18: 评分历史可追溯

任何对 `EVALUATION` 或其关联 `DIMENSION_SCORE` 的修改必须在 `EVALUATION_HISTORY` 中留下一条记录，包含操作人、操作时间、变更前后值。该记录与变更操作在同一事务中提交。

**Validates: Requirements 7.8**

### Property 19: 模板独立性

从 `EVALUATION_TEMPLATE` 加载创建的实训任务维度，其后续修改不会反向影响模板内容。模板与任务维度在数据上完全独立。

**Validates: Requirements 17.6**
