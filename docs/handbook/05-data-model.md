# 05 数据模型

## 实体关系图（ERD）

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
```

## 实体清单

### USER
- `id`, `username` (UK), `password_hash`, `role` (admin/teacher/student)
- `display_name`, `is_active`, `failed_login_count`, `locked_until`, `created_at`

### COURSE
- `id`, `code` (UK), `name`, `description`, `is_archived`, `created_at`

### CLASS
- `id`, `course_id` (FK), `teacher_id` (FK), `name`, `semester`, `is_archived`, `created_at`

### TRAINING_TASK
- `id`, `teacher_id`, `course_id`
- `name`, `description`, `requirements`, `evaluation_criteria`
- `deadline`, `status` (draft/published/closed), `created_at`

### DIMENSION
- `id`, `task_id` (FK), `name`, `description`
- `weight` (1-100，总和=100), `order_index`

### EVALUATION_TEMPLATE
- `id`, `owner_id` (FK), `name`, `description`
- `visibility` (private/team/system), `created_at`

### TEMPLATE_DIMENSION
- `id`, `template_id` (FK), `name`, `description`, `weight`, `order_index`

### UPLOAD
- `id`, `task_id`, `student_id`
- `filename`, `file_type`, `file_size`, `file_path`, `sha256`
- `status` (pending/parsing/parsed/failed), `created_at`

### PARSE_RESULT
- `id`, `upload_id` (FK UK)
- `structured_content` (JSONB), `raw_text`
- `simhash` (bigint, 64位指纹)
- `embedding` (vector, 512维)
- `error_message`, `parsed_at`

### VERIFY_RESULT
- `id`, `upload_id` (FK UK)
- `match_rate`, `checkpoints` (JSONB), `missing_items` (JSONB), `logic_issues` (JSONB)
- `overall_confidence`, `verified_at`

### EVALUATION
- `id`, `upload_id` (FK UK), `teacher_id`
- `final_score`, `objective_ratio`
- `status` (auto_scored/reviewed/finalized/rejected)
- `overall_comment`, `updated_at`

### DIMENSION_SCORE
- `id`, `evaluation_id` (FK), `dimension_id` (FK)
- `objective_score`, `objective_rationale`
- `subjective_score`, `subjective_comment`

### EVALUATION_HISTORY
- `id`, `evaluation_id` (FK), `operator_id` (FK)
- `action`, `before_value` (JSONB), `after_value` (JSONB), `changed_at`

### SIMILARITY_RECORD
- `id`, `task_id` (FK), `upload_a_id` (FK), `upload_b_id` (FK)
- `hamming_distance`, `cosine_similarity`
- `state` (suspect/confirmed/ignored), `reviewed_by`, `created_at`

### STUDENT_PROFILE
- `id`, `student_id` (FK UK)
- `radar_data` (JSONB), `weakness_list` (JSONB), `suggestions` (JSONB)
- `computed_at`

### NOTIFICATION
- `id`, `recipient_id` (FK), `event_type`, `title`, `content`
- `payload` (JSONB), `is_read`, `created_at`

### NOTIFICATION_PREF
- `id`, `user_id` (FK), `event_type`, `enabled`

### CHAT_SESSION
- `id`, `user_id` (FK), `evaluation_id` (FK), `title`, `created_at`

### CHAT_MESSAGE
- `id`, `session_id` (FK), `role` (user/assistant/tool)
- `content`, `prompt_tokens`, `completion_tokens`, `created_at`

### AUDIT_LOG（仅追加）
- `id`, `occurred_at`, `user_id`, `role`, `action`
- `target_type`, `target_id`, `result`
- `client_ip`, `user_agent`, `suspicious_flag`

### IMPORT_JOB
- `id`, `operator_id` (FK), `job_type` (user/student)
- `status` (pending/processing/done/failed)
- `total_count`, `success_count`, `failed_count`
- `failed_file_path`, `created_at`

### IMPORT_RECORD
- `id`, `job_id` (FK), `row_number`, `status`, `error_message`

## 关键字段规则

- **USER.password_hash**：bcrypt（cost factor=12）哈希存储
- **USER.locked_until**：账号锁定到期时间，登录时若该字段大于当前时间则拒绝
- **TRAINING_TASK.status** 流转：`draft` → `published` → `closed`，单向不可逆
- **DIMENSION.weight**：在事务中校验同一 task 下所有维度权重之和等于 100
- **UPLOAD.file_path**：相对存储根的路径，如 `task_{id}/student_{id}/{uuid}.docx`
- **UPLOAD.sha256**：用于断点续传校验完整性，并识别重复上传
- **PARSE_RESULT.simhash**：64 位整数，用于快速文本相似度粗筛
- **PARSE_RESULT.embedding**：pgvector 类型 512 维向量，用于语义相似度精排
- **EVALUATION.objective_ratio**：客观评分占比（默认0.6），主观占比为 `1 - objective_ratio`
- **EVALUATION.status** 扩展：`rejected` 用于审批流的"打回重做"
- **EVALUATION_HISTORY**：每次评分变更（字段、操作人、前后值）记录一行，不删除
- **SIMILARITY_RECORD.state**：`suspect` 系统自动标记，`confirmed` / `ignored` 由教师人工裁定
- **AUDIT_LOG**：仅追加表，触发器拒绝 UPDATE/DELETE

## 索引策略

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

## 物化视图

| 视图 | 用途 | 刷新频率 |
|------|------|---------|
| `mv_class_progress` | 班级批改进度聚合 | 10 分钟 |
| `mv_course_metrics` | 课程级评分分布、维度对比 | 1 小时 |
| `mv_school_overview` | 全校汇总 | 6 小时 |

## 不变量（详见手册 11）

- 权重总和守恒：每个 task 下 SUM(weight) = 100
- 评分范围：所有 score ∈ [0, 100]
- 状态机单调性：task.status 不可逆向
- 上传归属唯一：upload 必须严格归属一个 (task_id, student_id) 对
