# 09 日志与可观测性

## 日志格式

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

## 日志级别使用规范

| 级别 | 用途 | 频率上限 |
|------|------|---------|
| DEBUG | 详细排查信息，仅 dev 环境启用 | 不限 |
| INFO | 关键业务事件入口/出口 | 每请求 5-20 条 |
| WARNING | 业务降级、重试、性能异常 | 偶发 |
| ERROR | 业务异常、外部依赖失败 | 罕见 |
| CRITICAL | 服务不可用、数据丢失风险 | 仅紧急 |

## 强制日志事件命名约定

`<domain>.<action>.<outcome>`，全部小写下划线。

例：`upload.create.success / parse.execute.failed / chat.tool_call.dispatched / audit.log.write_attempt_blocked`

### 各域常用事件名

| 域 | 典型事件 |
|----|---------|
| auth | login.success, login.failed, lockout.triggered, token.refresh.success |
| upload | create.start, create.success, parse_enqueued, validation.failed |
| parse | execute.start, execute.success, execute.failed, timeout.exceeded |
| verify | execute.start, execute.success, llm.retry, output.parse_failed |
| evaluation | auto_score.success, manual_update, bulk_confirm.success, weight.recalculated |
| similarity | check.start, check.match_found, check.no_match |
| chat | session.created, message.sent, tool_call.dispatched, quota.exceeded |
| llm | call.start, call.success, call.failed, circuit.opened, circuit.half_open |
| audit | log.written, log.write_attempt_blocked, suspicious.detected |
| notification | sent, delivered, dismissed |

## Trace ID 透传

| 入口 | 来源 | 透传机制 |
|------|------|---------|
| HTTP API | `X-Trace-Id` 请求头，缺失则生成 UUID4 | FastAPI 中间件写入 `contextvars` |
| WebSocket | 握手 query param `trace_id` | 同上 |
| Celery 任务 | 由调用方写入 `headers["trace_id"]` | Celery signal 接收时写入 contextvars |
| LLM 调用 | trace_id 作为 metadata 传入 LLM 元数据（部分 API 支持） | 透明 |

`structlog` processor 自动从 contextvars 读 trace_id 注入每条日志。

## 入口/出口/异常日志范例

```python
@router.post("/uploads")
async def create_upload(...):
    log.info("upload.create.start", task_id=task_id, file_size=file.size)
    try:
        result = await upload_service.create(...)
        log.info("upload.create.success", upload_id=result.id, duration_ms=elapsed)
        return result
    except UploadTooLargeError as e:
        log.warning("upload.create.rejected", reason="too_large", file_size=file.size)
        raise
    except Exception as e:
        log.exception("upload.create.failed", error=str(e), error_type=type(e).__name__)
        raise
```

## 度量指标（Metrics）

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

## 健康检查端点

| 端点 | 用途 |
|------|------|
| `GET /healthz` | 存活检查（仅返回 200） |
| `GET /readyz` | 就绪检查（含 DB/Redis 探测） |
| `GET /api/_dev/health/full` | 完整健康（dev 启用，含 LLM/OCR/磁盘） |

## 错误追踪

接入轻量的 Sentry 兼容服务（如 GlitchTip 自部署）记录未捕获异常。生产环境告警渠道为邮件 + 站内通知给管理员。

## 敏感字段过滤

日志中以下字段会自动用 `***` 替换：

- `password` / `password_hash`
- `api_key` / `secret_key` / `token`
- `authorization` 头
- `jwt_secret` / `llm_key_master`

实现：自定义 `structlog` processor，在 emit 前递归扫描 dict，匹配上述 key 名（不区分大小写）。

## 日志保留与归档

- 应用日志：本地 `/var/log/tes/*.json`，按天滚动，保留 30 天
- 审计日志：DB 表 `audit_log` 至少保留 12 个月（需求 20.3）
- 错误追踪：Sentry/GlitchTip 至少保留 90 天

## 排查问题的标准流程

1. 拿到用户反馈的 trace_id（前端在错误页面显示）
2. 在日志查询界面（如 Loki/CLI grep）按 `trace_id="xxx"` 过滤所有相关日志
3. 沿时间线还原请求 → service → DB/LLM 调用链
4. 关键决策点查看 `event` 字段定位失败原因
5. 如涉及 LLM 调用，可在 `tes_llm_call_duration_seconds` 指标查看历史趋势
