# 智能实训评价管理系统

基于 LLM Function Calling 的实训作业自动评价系统，部署目标为龙芯 LoongArch + 银河麒麟 V10/V11。

## 目录

- [技术栈](#技术栈)
- [快速启动](#快速启动)
- [项目结构](#项目结构)
- [常用命令](#常用命令)
- [架构文档](#架构文档)
- [贡献指南](#贡献指南)

## 技术栈

| 层 | 选型 |
|----|------|
| 后端 | FastAPI + SQLAlchemy 2.0（async）+ structlog + pydantic-settings |
| 前端 | Vue 3 + Vite + Tailwind CSS v4 + shadcn-vue + Pinia + vue-router |
| 数据库 | PostgreSQL 14 + pgvector（dev 默认 SQLite） |
| 缓存/锁/限流 | Redis 7 |
| 异步任务 | Celery + Redis broker |
| LLM | OpenAI 兼容协议（DeepSeek / 通义 / 智谱 / Moonshot） |
| AI 编排 | Function Calling（不引入 LangGraph） |

详见 [`docs/handbook/01-tech-stack.md`](docs/handbook/01-tech-stack.md)。

## 快速启动

### 前置条件

- Python ≥ 3.10
- Node.js ≥ 20 LTS + pnpm
- Docker Engine ≥ 20.x（用于 PostgreSQL + Redis）

### 第 1 步：克隆并安装依赖

```bash
# 后端
cd backend
python -m venv .venv
.venv/Scripts/activate     # Windows
# source .venv/bin/activate  # Linux/macOS
pip install -e ".[dev]"

# 前端
cd ../frontend
pnpm install
```

### 第 2 步：启动开发依赖

```bash
# 在仓库根
./scripts/dev_up.sh        # 启动 PostgreSQL + Redis + Adminer
```

### 第 3 步：配置环境变量

```bash
cp .env.example .env
# 编辑 .env：填写 JWT_SECRET、LLM_KEY_MASTER 等
```

### 第 4 步：启动后端

```bash
cd backend
.venv/Scripts/uvicorn app.main:app --reload --port 8000
# 访问 http://localhost:8000/docs 查看 Swagger UI
```

### 第 5 步：启动前端

```bash
cd frontend
pnpm dev
# 访问 http://localhost:5174
```

## 项目结构

```
.
├── backend/              FastAPI 后端
│   ├── app/
│   │   ├── api/          路由层
│   │   ├── services/     业务编排
│   │   ├── repositories/ 数据访问
│   │   ├── models/       SQLAlchemy ORM
│   │   ├── schemas/      Pydantic v2
│   │   ├── llm/          LLM 适配器 + Function Calling
│   │   ├── parsers/      文档解析
│   │   ├── tasks/        Celery 异步任务
│   │   └── core/         配置/日志/异常/中间件
│   ├── tests/            pytest 测试
│   └── pyproject.toml
├── frontend/             Vue 3 前端
│   └── src/
│       ├── views/        页面（admin/teacher/student/shared）
│       ├── components/   通用组件
│       ├── api/          axios 客户端
│       └── stores/       Pinia
├── frontend-preview/     HTML 视觉契约（设计稿翻译）
├── docs/                 设计文档与开发手册
├── designs/              .pen 设计源文件
└── .kiro/specs/          需求/设计/任务文档
```

详见 [`docs/handbook/03-project-structure.md`](docs/handbook/03-project-structure.md)。

## 常用命令

### 后端

```bash
cd backend

# 安装依赖
pip install -e ".[dev]"

# 启动 dev server
uvicorn app.main:app --reload

# 运行所有质量检查（ruff + mypy + pytest + 覆盖率）
./scripts/lint_all.sh        # Linux/macOS
./scripts/lint_all.ps1       # Windows PowerShell

# 单独跑测试
pytest                       # 全部
pytest -m unit               # 仅单元测试
pytest --cov=app             # 含覆盖率

# 数据库迁移
alembic upgrade head
alembic revision --autogenerate -m "add new column"
```

### 前端

```bash
cd frontend

pnpm install
pnpm dev          # 开发服务器
pnpm build        # 生产构建
pnpm typecheck    # TypeScript 检查
pnpm lint         # ESLint
```

### Docker 依赖

```bash
./scripts/dev_up.sh                    # 启动
./scripts/health_check.sh              # 健康探测
./scripts/dev_down.sh                  # 停止（保留数据）
./scripts/dev_down.sh --volumes        # 停止并清理数据
```

## 架构文档

| 文档 | 说明 |
|------|------|
| [`.kiro/specs/training-evaluation-system/requirements.md`](.kiro/specs/training-evaluation-system/requirements.md) | 需求规格 |
| [`.kiro/specs/training-evaluation-system/design.md`](.kiro/specs/training-evaluation-system/design.md) | 设计文档 |
| [`.kiro/specs/training-evaluation-system/tasks.md`](.kiro/specs/training-evaluation-system/tasks.md) | 实施任务 |
| [`docs/handbook/00-INDEX.md`](docs/handbook/00-INDEX.md) | 开发手册总目录 |
| [`docs/design/00-INDEX.md`](docs/design/00-INDEX.md) | 设计 token 与 HTML 契约 |

## 故障排查

| 现象 | 解决 |
|------|------|
| `pip install` 报 `requires-python` | 升级 Python ≥ 3.10 |
| `pnpm build` 报 TypeScript 错误 | 运行 `pnpm typecheck` 定位 |
| Docker 启动失败 `port already allocated` | 修改 `docker-compose.dev.yml` 端口或停止占用进程 |
| pgvector 扩展未安装 | 确认使用 `pgvector/pgvector:pg14` 镜像而非纯 postgres |
| `pre-commit` 钩子被绕过 | 使用 `git commit --no-verify` 仅在紧急情况下使用 |

## 贡献指南

详见 [`CONTRIBUTING.md`](CONTRIBUTING.md)。

## 许可证

本项目为内部教学研发项目，未公开授权许可。
