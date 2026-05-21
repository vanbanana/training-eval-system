# 11 系统正确性属性（不变量清单）

系统必须保持以下 19 条不变量。这些性质是验证测试与代码评审的关键检查点，**PR 审查时按此清单逐条验证相关变更是否破坏不变量**。

## 数据一致性类

### Property 1: 权重总和守恒

对任意 `task_id`，`SUM(dimensions.weight WHERE task_id = X) = 100`。任务在 `published` 状态前必须满足此条件。

**Validates**: Requirements 3.5, 7.1

---

### Property 2: 评分范围约束

所有 `objective_score` 与 `subjective_score` 的取值必须在 `[0, 100]` 闭区间内。

**Validates**: Requirements 7.3, 7.4

---

### Property 3: 综合得分一致性

`evaluation.final_score` 在每次维度分变更或权重调整后必须重新计算，并等于：
`Σ(weight_i × (obj_i × α + subj_i × (1-α))) / 100`

**Validates**: Requirements 7.5, 7.6

---

### Property 4: 状态机单调性

`training_task.status` 仅允许 `draft → published → closed` 的单向流转，禁止逆向状态变更。

**Validates**: Requirements 3.1, 3.4

---

### Property 5: 上传归属唯一

每个 `upload` 必须严格归属于一个 `(task_id, student_id)` 对。学生只能为自己提交，不能代他人上传。

**Validates**: Requirements 4.1, 2.4

---

## 安全性类

### Property 6: 密码不可逆

`user.password_hash` 必须经过 bcrypt（cost factor ≥ 10）或 Argon2 哈希存储。系统中不存在任何路径返回或日志记录明文密码。

**Validates**: Requirements 11.1

---

### Property 7: 越权防护

学生只能访问自己提交的 upload 与 evaluation。教师只能访问自己创建的 task 下的数据。任何跨用户访问必须返回 403。

**Validates**: Requirements 2.3, 2.4, 11.2

---

### Property 8: 会话有效性

JWT 过期、用户被禁用或被锁定后，下一次受保护请求必须返回 401，不可继续访问任何业务资源。

**Validates**: Requirements 2.6, 11.2

---

### Property 9: 文件类型可信

`upload.file_type` 必须同时通过扩展名白名单与文件头 magic number 校验。任一校验失败则拒绝存储。

**Validates**: Requirements 4.1, 4.3, 11.6

---

## 可用性类

### Property 10: LLM降级可用

当 LLM_Service 不可用时，教师手动评分入口、报表生成与导出功能必须保持可用，不产生级联失败。

**Validates**: Requirements 8.6, 11.4

---

### Property 11: 任务幂等

Celery 解析、核查、评分任务设计为幂等。重复执行同一 `upload_id` 的解析任务不会产生重复 `parse_result`，由数据库唯一约束保障。

**Validates**: Requirements 5.1, 5.5, 11.4

---

### Property 12: 进度可观测且必终结

每个长耗时任务在执行过程中至少推送一次进度事件，最终必须以 `parsed` 或 `failed` 状态终结。不存在永久停留在 `parsing` 中间态的记录，由超时看门狗（>120秒）保障。

**Validates**: Requirements 5.6, 10.4

---

## 业务规则类

### Property 13: 班级归属一致性

学生只能向其所属班级被发布的实训任务提交成果。`upload.task_id` 关联的任务必须包含至少一个该学生归属的 `class_id`，否则提交被拒绝。

**Validates**: Requirements 12.4, 12.5, 4.1

---

### Property 14: 审计日志不可篡改

`audit_log` 表的任何记录一旦写入，其后通过应用层连接执行的 UPDATE 或 DELETE 必须被数据库触发器拒绝。审计日志只能通过 DBA 权限手动归档。

**Validates**: Requirements 20.5

---

### Property 15: 通知必达

每条 `NOTIFICATION` 记录写入数据库后，无论实时推送是否成功，离线用户登录后通过 `GET /api/notifications` 查询时必须能够看到该通知。未读计数与数据库实际未读记录数保持最终一致。

**Validates**: Requirements 16.1, 16.2, 16.3

---

### Property 16: 相似度比对范围限定

`SIMILARITY_RECORD` 中的两个 `upload_id` 必须归属于同一个 `task_id`。跨任务的相似度比对结果不会被生成或存储。

**Validates**: Requirements 18.7

---

### Property 17: AI 问答配额受控

单个学生在同一日期内调用 AI 问答助手不得超过 50 次。超出限额时 API 必须返回 HTTP 429，且不消耗 LLM 服务调用资源。

**Validates**: Requirements 22.8

---

### Property 18: 评分历史可追溯

任何对 `EVALUATION` 或其关联 `DIMENSION_SCORE` 的修改必须在 `EVALUATION_HISTORY` 中留下一条记录，包含操作人、操作时间、变更前后值。该记录与变更操作在同一事务中提交。

**Validates**: Requirements 7.8

---

### Property 19: 模板独立性

从 `EVALUATION_TEMPLATE` 加载创建的实训任务维度，其后续修改不会反向影响模板内容。模板与任务维度在数据上完全独立。

**Validates**: Requirements 17.6

---

## 用法

### 设计阶段

设计任何新功能时，先思考：

- 这个功能是否会触及上述任何不变量？
- 触及的部分是否仍然保持不变量？
- 如果不变量需要扩展，是否需要新增 Property？

### 开发阶段

实现核心算法时，把不变量直接写成断言或 DB 约束：

```python
# Property 1 实现示例
async def publish_task(task_id: int) -> None:
    weights_sum = await dimension_repo.sum_weights(task_id)
    assert weights_sum == 100, f"Property 1 violated: sum={weights_sum}"
    ...
```

### 测试阶段

每条 Property 至少有一个对应的集成或单元测试用例（参见 `tests/properties/`）。

### 评审阶段

PR 审查时检查：变更是否可能破坏任何 Property？变更是否带来新的 Property？

## 不变量演进规则

- **新增**：在 design.md 与本手册同步加 Property N
- **修改**：原 Property 标记为 superseded，新建 Property N' 替代
- **废弃**：必须先证明所有引用此 Property 的代码已不存在
