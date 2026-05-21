# 01 技术选型一览

## 选型摘要表

| 层次 | 选型 | 选型理由 |
|------|------|--------|
| 后端运行时 | Python 3.10+ | 银河麒麟V10/V11官方源已提供LoongArch版本，生态成熟 |
| Web框架 | FastAPI + Uvicorn | 纯Python实现，async原生，自动 OpenAPI 文档 |
| 前端框架 | Vue 3 + Vite + TypeScript | SFC + Composition API，编译产物为静态资源，架构无关 |
| 前端 UI 库 | shadcn-vue（基于 Reka UI + Tailwind CSS）| 无头组件，源码可定制，视觉现代，演示加分 |
| 原子化 CSS | Tailwind CSS | shadcn-vue 标配，主题系统完善，dark 模式开箱即用 |
| 表格库 | TanStack Table v8（无头表格逻辑） | shadcn-vue 官方推荐，配合 Tailwind 自定义视觉 |
| 文件上传 | filepond-vue | 支持断点续传、SHA256 校验、进度回调 |
| 图标 | Lucide-vue-next | shadcn-vue 默认配套，图标统一一致 |
| 图表库 | ECharts | 中文标签渲染优秀，可视化类型齐全 |
| 状态管理 | Pinia + VueUse | Vue 3 标准生态 |
| 路由 | Vue Router 4 | Vue 3 标准生态 |
| HTTP 客户端 | OpenAPI 自动生成 + Axios | 后端契约即前端契约 |
| 数据库 | PostgreSQL 14+ | 官方支持LoongArch；JSONB + 全文检索 + pgvector 一站式 |
| 缓存/任务队列 | Redis + Celery | 业界标准，重试/调度/限流功能齐全 |
| 大模型服务 | 云端 API（OpenAI 兼容协议） | 稳定、并发能力强、不占用部署服务器资源 |
| 嵌入向量服务 | 云端 Embedding API（如 text-embedding-v3） | 与主 LLM 同源，配置统一 |
| OCR引擎 | Tesseract（含中文语言包） | LoongArch 编译方案成熟 |
| 文档解析 | python-docx / PyMuPDF | 纯Python或可在LoongArch编译 |
| 报表导出 | openpyxl / WeasyPrint / matplotlib | 纯Python实现 |
| 反向代理 | Nginx | 国产化生态最成熟的反代 |
| 相似度检测 | simhash-py + pgvector | 文本指纹粗筛 + 向量精排双引擎 |
| 嵌入向量存储 | pgvector扩展 | PostgreSQL扩展，源码编译可在LoongArch运行 |
| 实时推送 | FastAPI WebSocket + Redis Pub/Sub | 解析进度、通知、AI流式响应共用 |

## 关于大模型部署模式的说明

考虑到龙芯云平台测试环境为 4 核 8GB 内存的资源约束，**本地部署 7B 级别大模型在性能与稳定性上不可行**（推理延迟高、并发能力弱、占用大部分内存影响其他服务），因此本系统**仅采用云端 API 调用模式**。

系统通过抽象的 LLM Provider 接口设计，未来可在更高规格服务器上扩展支持本地部署，无需改动业务代码。

## 支持的云端 LLM 供应商

| 供应商 | Chat Endpoint 示例 | Embedding 模型示例 |
|-------|-------------------|------------------|
| 阿里通义千问 | `https://dashscope.aliyuncs.com/compatible-mode/v1` | `text-embedding-v3` |
| DeepSeek | `https://api.deepseek.com/v1` | （走通义或智谱） |
| 智谱 GLM | `https://open.bigmodel.cn/api/paas/v4` | `embedding-3` |
| Moonshot Kimi | `https://api.moonshot.cn/v1` | （走通义或智谱） |

所有供应商均通过统一的 OpenAI 兼容协议调用，运行时由管理员通过界面切换。

## 资源预算（8GB 内存）

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

## 设计目标对应的选型

- **国产化兼容**：所有部署组件可在 LoongArch + 银河麒麟原生运行
- **模块化解耦**：每个核心引擎独立成模块，通过 Protocol 接口通信
- **大模型抽象**：统一 LLM Provider 接口，热切换供应商
- **稳定优先**：AI 能力外置到云端，服务器仅承载业务与数据
- **数据安全合规**：API Key AES-256 加密、操作可审计追溯
