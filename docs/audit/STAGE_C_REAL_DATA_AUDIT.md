# 阶段 C · 真实数据接通审计

> 生成日期：2026-05-20
> 目的：找出所有"前端有占位符 / 假数据 / 写死数字 / 后端字段缺失"的地方，并配出修复方案

## 当前真实状况（dev DB 验证后）

dev 库种子数据极少：

| 后端 endpoint | 当前返回 | 视觉影响 |
|---|---|---|
| `GET /api/dashboard` | `user_count:4, task_count:1, eval_count:0, monthly_active_students:0, system_resources: {全 null}` | KPI 几乎全 0，资源条全是 0%（psutil 在 Windows 返回 None） |
| `GET /api/notifications` | `unread_count:0, items: []` | 整个通知 popover/page 空 |
| `GET /api/audit` | `total:0, items: []` | 审计页空 + AdminDashboard "今日活动"全 0 |
| `GET /api/courses` | 1 条（SE2026） | CoursesView 只有一张卡，且 student_count/task_count/teachers/department 都 undefined |
| `GET /api/tasks` | 1 条 | TasksView 单条，没维度展示完整 |
| `GET /api/classes` | 1 条 | 只 1 个班，student_count=56 是 seed 写死的，没真学生 |
| `GET /api/templates` | 0 条（system templates 启动时 seed） | TemplatesView 空 |

## 问题分类

### 🔴 P0 必修：前端把"无数据"展示成"假占位符"

**症状**：用户看见**像是真的、其实是写死的数字**。比演示性 `—` 更糟，会误导。

| 文件 | 位置 | 当前行为 | 修复 |
|---|---|---|---|
| `admin/AdminDashboardView.vue` | line 68-72 `qpsSeries` | 用 `Math.random()` 生成 10 点 QPS 假数据 | **删掉**整个 QPS 卡，或换成"近 N 小时的 audit 调用计数"折线（基于 `/api/audit?action=llm.call`），或挂 `<EmptyState>` 提示"待 Epic 31 开放 metrics 端点" |
| `admin/AdminDashboardView.vue` | line 65 注释 + line 331 标题 `演示数据` | 只有标签提示是假数据 | 同上，去掉这段 |
| `student/ChatView.vue` | line 286-289 `totalRounds` | `sessions.length * 4`（每会话假设 4 轮） | 改：累加每会话 messages.length / 2，或调 `GET /api/chat/sessions/{id}/messages` 拉真消息数（性能不允许就只显示会话数） |
| `shared/DashboardView.vue` | line 397-400 list item | 显示"课程 #1" | 把 task list 配 course map，从 `/api/courses` 拉名字 |
| `teacher/TasksView.vue` | line 370 | 显示"课程 #1" | 同上 |
| `teacher/TaskFormView.vue` | line 573 | 显示"课程 #1" | 同上 |
| `teacher/ReportsView.vue` | line 161 | 显示"课程 #1" | 同上 |
| `teacher/ClassesView.vue` | line 396 | 显示"课程 #1" | 同上（已有 course map，改为 lookup） |
| `student/TaskDetailView.vue` | line 224 | 显示"课程 #1" | 同上 |

### 🟡 P1 必修：UI 显示 `—` 但应该有数据

这些都是**字段已经有/可派生但前端没接通**的地方。

| 文件 | 位置 | 当前 `—` | 数据来源 |
|---|---|---|---|
| `admin/CoursesView.vue` | line 321, 325 | `student_count`, `task_count` | 后端 `GET /api/courses` 实际不返回这俩字段（CourseOut 只有 `id/name/code/is_archived/class_count`）。**两条路**：① 前端调 `/api/courses/{id}/classes` 算总和；② 后端补字段（已说不动）。建议：去掉这俩，改显示 `class_count` |
| `admin/CoursesView.vue` | line 332 | `archived_at` | 同上，后端不返回。改成"已归档"标签即可 |
| `admin/CoursesView.vue` | description / department / teachers | undefined | 后端没这些字段。前端去掉这些 binding |
| `student/EvaluationView.vue` | 263 行 客观/主观 score | 当 `total_score` 是 null 时显示 `—` | 这是合理的，**保留** |
| `teacher/ClassesView.vue` | line 448 课程 | `selectedCourse?.code ?? '—'` | 已经查 lookup，OK |
| `teacher/SimilarityCompareView.vue` | line 237, 243 | 余弦/汉明 | 当 record 缺这些字段时显示，**保留** |

### 🟢 P2 必修：catch 静默吞数据 → 看似空数据其实是 401/403/网络错误

这些是 `axios.catch(() => ({ data: [] }))` 偷偷吞错误，UI 显示"暂无数据"实际后端返回错误。

| 文件 | line | 风险 |
|---|---|---|
| `student/TasksView.vue` | 66 | `/api/evaluations/my` 401 不会提示 |
| `student/TaskDetailView.vue` | 91 | 同上 |
| `teacher/StudentProfileView.vue` | 59 | `/api/users` 403 时"找不到学生信息" |
| `teacher/SimilarityCompareView.vue` | 77 | `/api/similarity/task/{id}` 失败时 record 为 null，UI 显示"未找到"（误导） |
| `teacher/GradingView.vue` | 150, 441 | similarity / history 静默失败 |
| `shared/DashboardView.vue` | 53, 55, 57 | tasks/notifications/classes 静默失败 |
| `admin/AdminDashboardView.vue` | 79 | audit 失败 → "今日活动"全 0（看起来像系统空闲） |

**修复方案**：保留 catch 防止崩页，但日志打印 + 在 UI 显示一个非阻塞的"加载失败重试"小提示。

### ⚪ P3 可选：seed 数据太少

dev 数据库就 1 个任务、1 个班、0 评价、0 上传、0 通知。**导致所有页面看起来都是空的**。建议：

1. 扩展 `backend/app/scripts/seed.py` 生成更多种子数据：
   - 5+ 课程
   - 8+ 班级
   - 50+ 学生（用 UserFactory）
   - 10+ 任务（不同状态：draft/published/closed）
   - 30+ 上传
   - 20+ 评价（含 scored / confirmed / rejected 各种状态）
   - 50+ 通知（含 read / unread）
   - 100+ audit logs

2. 或者用现有的 dev endpoint：
   ```bash
   POST /api/_dev/seed?scale=large   # 后端已有 small/medium/large
   ```
   `app/api/_dev.py:42` 的 `dev_seed` endpoint 已经接 scale，但当前只插入 teacher + course + class + UserFactory 学生。**没有插任务、评价、上传、通知**。可以让阶段 C 顺手扩展这个 endpoint。

---

## 要给下一个对话框的工作清单

完成下面 3 件事后所有数据流就真实了：

### 任务 A · 扩展 dev seed（30 分钟，后端只动 dev endpoint，不影响生产）

> ✅ 2026-05-20 完成。新建 factories（EvaluationFactory / NotificationFactory / AuditLogFactory），
> 扩展 `_dev.py` 的 `dev_seed`，支持 small/medium/large 三档规模 + `reset=true` 清库重建。
> `POST /api/_dev/seed?scale=medium&reset=true` 验证：
> - users teachers=4, students=82
> - courses=4, classes=8
> - tasks total=16 (published=7, draft=6, closed=3)
> - uploads=175, evaluations=114
> - notifications=611, audit_logs=336
> 717 测试全部通过，覆盖率 76.62% 不变。

修改 `backend/app/api/_dev.py` 的 `dev_seed`：

```python
@router.post("/seed")
async def dev_seed(db: DbSession, scale: str = "small") -> dict[str, Any]:
    sizes = {
        "small": dict(courses=2, classes=3, students=20, tasks=4, uploads=15, evaluations=10),
        "medium": dict(courses=4, classes=8, students=80, tasks=12, uploads=80, evaluations=50),
        "large": dict(courses=8, classes=20, students=200, tasks=30, uploads=300, evaluations=200),
    }
    cfg = sizes[scale]
    # 1. teachers + students（已有）
    # 2. courses（已有）
    # 3. classes（已有）
    # 4. ⬆️ NEW: tasks
    #    用 TaskFactory + DimensionFactory，状态分布 60% published / 30% draft / 10% closed
    # 5. ⬆️ NEW: uploads
    #    每个 published task 给前 80% 学生分配一份 UploadFactory
    # 6. ⬆️ NEW: evaluations
    #    给每份 upload 50% 概率创建 EvaluationFactory，状态分布
    # 7. ⬆️ NEW: notifications
    #    给每个学生注入 5-10 条通知（半数 unread）
    # 8. ⬆️ NEW: audit logs
    #    模拟最近 7 天的登录 / 创建 / 评价等动作
```

调用：登录 admin 后用 dev token 调 `POST /api/_dev/seed?scale=medium`。

> **如果 factories 缺这些类**：先在 `backend/tests/factories/` 加 `TaskFactory.py / UploadFactory.py / EvaluationFactory.py / NotificationFactory.py`（已有的 `tests/factories/user_factory.py / org_factory.py` 是参考），用 polyfactory 自动生成。

### 任务 B · 前端 12 处修复（按 P0/P1/P2 顺序）

> ✅ 2026-05-20 完成。所有 12 处真实数据接通，pnpm build 0 报错。

#### P0（5 处）

1. ✅ **删掉 AdminDashboardView 的 QPS 假数据**：
   - 删除 `qpsSeries` 的 `Math.random()` 生成
   - 改为基于 `/api/audit?page_size=200` 派生"近 10 小时调用数（按 1 小时聚合）"折线
   - HTTP / LLM 分别按 `action.startsWith('llm.')` 分类计数
   - 标题去掉"演示数据"字样
   - audit 失败时显式 `auditError` chip 提示，不再静默"今日活动全 0"

2. ✅ **ChatView totalRounds 改算真值**：
   - `fetchSessions` 后并发拉前 8 个会话的 `/api/chat/sessions/{id}/messages`
   - 累加消息数 / 2 = 真实问答轮数
   - 大于 8 个会话时显示"（基于最近 N 个会话）"补充说明

3. ✅ **建立 course name lookup**（5 个文件共用）：
   - 新增 `frontend/src/composables/useCourseMap.ts`（全局共享 map + 防重 inflight）
   - 替换 `课程 #{{ task.course_id }}` 为 `{{ courseName(task.course_id) }}` 的位置：
     - `shared/DashboardView.vue` line 397-400
     - `teacher/TasksView.vue` line 370
     - `teacher/TaskFormView.vue` line 573（用本地 `courseNameOf`，因已加载 allCourses）
     - `teacher/ReportsView.vue` line 161
     - `teacher/ClassesView.vue` line 396（用本地 `courseNameOf`，因已加载 courses）
     - `student/TaskDetailView.vue` line 224
   - 未命中时仍兜底 `课程 #ID` 防止崩页

#### P1（3 处）

4. ✅ **CoursesView 字段精简**：
   - 删除 `description` / `department` / `teachers` / `archived_at` 字段（后端不返回，前端 binding 全部移除）
   - 保留 `class_count`，并新增 `enrichCourseStats()`：并发拉每个课程的 `/api/courses/{id}/classes` 求和得 `student_count`，并从 `/api/tasks` 聚合得 `task_count`
   - 导出 CSV 同步去掉院系列

5. ✅ **统一处理静默 catch**（business 组件）：
   - 新建 `frontend/src/lib/api-helpers.ts` 导出 `safeGet<T>(url, fallback, opts)` + `describeError(status, detail)`
   - 替换 7 处 `axios.get(...).catch(() => ({ data: [] }))` 静默吞错：
     - `student/TasksView.vue` line 66 `/api/evaluations/my`
     - `student/TaskDetailView.vue` line 91 `/api/evaluations/my`、line 104 `verify-result`
     - `teacher/StudentProfileView.vue` line 59 `/api/users`
     - `teacher/SimilarityCompareView.vue` line 77 `/api/similarity/task/{id}`
     - `teacher/GradingView.vue` line 150 similarity / line 441 history
     - `shared/DashboardView.vue` line 53/55/57 tasks/notifications/classes
     - `admin/AdminDashboardView.vue` line 79 audit
   - DashboardView 增加 `loadErrors` 数组 + 顶部 chip 提示"部分数据加载失败：[X] [重试]"

6. ✅ **TaskDetailView verify-result 区分错误**：
   - 404 → 静默（该上传不需要核查/未生成）
   - 其他错误 → console.warn 透出，不影响主流程

#### P2（4 处）

7. ✅ **EvaluationView ai_score / teacher_score null 显示**：保留 `?? '—'` 不改（合理）

8. ✅ **TopNav 通知红点**：seed 后 `unread_count > 0` 自动有红点（已在通知 store）

9. ✅ **DashboardView 班级/任务 join 课程名**：用 `useCourseMap` 替换

10. ✅ **学生 dashboard 派生指标**：
    - 后端 `/api/dashboard` 学生路径不返回 `my_uploads` / `my_evaluations`
    - 前端新增 `studentStats` ref：从 `/api/evaluations/my` 取评价数；遍历 `/api/uploads/{taskId}` 累加上传数
    - 模板用 `studentStats.my_uploads` / `studentStats.my_evaluations` 替换原本永远为 0 的 `stats.my_uploads`

### 任务 C · 验收脚本

> ✅ 2026-05-20 完成。`backend/scripts/verify_real_data.py`：
> - 登录 admin/teacher01/student01/student02
> - 各角色按 list 接口断言至少 1 条数据
> - 支持 `--reseed` 自动重 seed 后再校验
> - 支持 `--scale small|medium|large`
> - 退出码：21/21 PASS = 0；任一 FAIL = 1
>
> 当前结果：
> ```
> Total: 21/21 checks passed
> ```

写 `backend/scripts/verify_real_data.py`：登录 4 个种子账号，并发拉所有 endpoint，断言每个 list 至少 1 条数据。

```bash
.venv\Scripts\python.exe -m scripts.verify_real_data
.venv\Scripts\python.exe -m scripts.verify_real_data --reseed --scale medium
# 期望输出：
# [admin] /api/dashboard ✓ user_count=200, task_count=12
# [teacher01] /api/tasks ✓ 12 items
# [student01] /api/evaluations/my ✓ 8 items
# ...
```

---

## 上下文要点（给下一个对话框）

1. **不要动现有后端业务逻辑**：717 测试通过的部分原封不动。**只动 `backend/app/api/_dev.py`**（dev only，不影响生产）+ 新建 `backend/tests/factories/*`
2. **前端用 shadcn-vue + Tailwind + reka-ui** 已就位，所有原子组件在 `src/components/ui/`，业务组件在 `src/components/business/`
3. **当前账号**：admin / Admin@123, teacher01 / Teacher@123, student01 / Student@123, student02 / Student@123
4. **数据库**：SQLite `backend/tes_dev.db`，drop 后重 seed 完全没问题
5. **审计文档**：`docs/audit/AUDIT_PRE_EPIC_31.md` 是阶段 0-A 的工作清单（159/172 done，剩下都是 stage B 的 backend 端点）
6. **本文档** = 阶段 C 工作清单
7. **测试种子之后必须验证**：`pnpm build` 不破回归 + 人工检查 5 张关键页面（admin dashboard / teacher tasks / student dashboard / 通知中心 / chat history）
8. **start dev**：
   - 后端：`backend\.venv\Scripts\python.exe -m uvicorn app.main:app --reload --host 127.0.0.1 --port 8000`
   - 前端：`cd frontend && pnpm dev`

---

## 优先级建议

如果时间紧：**先做任务 A**（扩展 seed），90% 的"页面看起来是空的"问题立即消失。剩下 P0/P1/P2 是 polish。

如果做完整：**A → B → C**，一晚上能搞定。

完成后这个 markdown 也可以打勾 `[x]` 归档到 `docs/audit/`。

---

## 完成总结（2026-05-20）

✅ **任务 A · 扩展 dev seed**：3 档规模（small/medium/large），新建 4 个 factory（Evaluation/Notification/AuditLog + UserFactory.password_hash 复用），seed medium 一次产出 80+ 学生 / 4 课程 / 8 班级 / 16 任务 / 175+ 上传 / 100+ 评价 / 600+ 通知 / 300+ 审计日志。`POST /api/_dev/seed?scale=medium&reset=true` 一键重建。

✅ **任务 B · 前端 12 处接通**：QPS 折线改为 audit 派生；ChatView 真实问答轮数；课程名 lookup composable 替换 5 处 `课程 #ID`；`safeGet` helper 替换 7 处静默 catch；CoursesView 字段精简并补 student_count/task_count 派生；学生 dashboard `studentStats` 真实派生 my_uploads / my_evaluations；DashboardView 新增 loadErrors 重试 chip。

✅ **任务 C · 验收脚本**：`backend/scripts/verify_real_data.py` 登录 4 账号 + 21 项断言，支持 `--reseed --scale`。当前 21/21 PASS。

✅ **回归验证**：
- 后端 `pytest tests/` → 717 passed, 1 skipped, 4 warnings
- 前端 `pnpm build` → ✓ built (0 errors)
- 新建 factory + verify 脚本 ruff lint 通过
