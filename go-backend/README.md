# 智能实训评价管理系统 — Go 后端

Go 语言重写版本，输出单一静态链接 ELF 二进制文件，零 CGO 依赖，可交叉编译至 LoongArch (loong64)。

## 技术栈

- **语言**: Go 1.25+, CGO_ENABLED=0
- **HTTP**: go-chi/chi v5
- **数据库**: modernc.org/sqlite (纯 Go, database/sql)
- **认证**: HMAC-SHA256 JWT + bcrypt
- **加密**: AES-256-GCM
- **异步**: goroutine worker pool + buffered channel
- **实时**: SSE (net/http 原生)
- **LLM**: net/http 直调 OpenAI 兼容 API
- **日志**: log/slog (JSON to stdout)

## 目录结构

```
go-backend/
├── cmd/server/main.go       # 入口
├── internal/
│   ├── config/              # TES_ 环境变量配置
│   ├── store/               # SQLite 连接池 + 迁移
│   ├── model/               # 领域模型
│   ├── repository/          # 数据访问接口 + SQL 实现
│   ├── service/             # 业务编排
│   ├── handler/             # HTTP 路由 (chi)
│   ├── dto/                 # 请求/响应 DTO
│   ├── middleware/          # auth, cors, trace, ratelimit, logger
│   ├── worker/              # goroutine pool
│   ├── sse/                 # SSE broker
│   ├── llm/                 # LLM HTTP client + 熔断
│   ├── similarity/          # SimHash + cosine
│   ├── crypto/              # AES + bcrypt + JWT
│   ├── apperr/              # 统一错误类型
│   ├── cache/               # LRU + TTL
│   ├── parser/              # docx + pdf + ocr
│   ├── report/              # PDF/Excel 生成
│   └── backup/              # SQLite 备份
├── migrations/*.sql         # 嵌入式 DDL
├── go.mod
└── Makefile
```

## 构建

```bash
# 本地构建
make build

# 交叉编译 (LoongArch)
make cross-compile

# 运行
make run

# 测试
make test

# 代码检查
make lint
```

## 配置

通过 `TES_` 前缀环境变量配置，支持 `.env` 文件。必填项：

- `TES_JWT_SECRET` — JWT 签名密钥 (≥32 字符)
- `TES_LLM_KEY_MASTER` — LLM API Key 加密主密钥 (base64, 32 字节)

## 部署

```bash
# 交叉编译
CGO_ENABLED=0 GOOS=linux GOARCH=loong64 go build -ldflags="-s -w" -o training-eval-system ./cmd/server

# 部署到目标机器
scp training-eval-system user@server:/opt/app/
scp -r dist/ user@server:/opt/app/dist/

# 运行
./training-eval-system
```
