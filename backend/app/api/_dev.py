"""Dev 调试端点 - Epic 26.

⚠️ prod 启动时 import 即崩溃，且 main.py 仅在 env != prod 时挂载。
"""

from __future__ import annotations

from typing import Any

from fastapi import APIRouter, Depends, Header, HTTPException, status

from app.api.deps import DbSession
from app.core.clock import FrozenClock, SystemClock, get_clock, set_clock
from app.core.config import get_settings

_settings = get_settings()
assert _settings.env != "prod", "DEV 端点禁止在 prod 启用"


async def _dev_token_guard(
    x_dev_token: str | None = Header(default=None, alias="X-Dev-Token"),
) -> None:
    if x_dev_token != _settings.dev_token:
        raise HTTPException(
            status_code=status.HTTP_403_FORBIDDEN,
            detail="invalid X-Dev-Token",
        )


router = APIRouter(
    prefix="/api/_dev",
    tags=["_dev"],
    dependencies=[Depends(_dev_token_guard)],
)


# ============== 26.2 seed / clock / cache ==============


@router.post("/seed")
async def dev_seed(
    db: DbSession, scale: str = "small", reset: bool = False
) -> dict[str, Any]:
    """填充 Factory 数据.

    生成的资源：
    - teachers / students（按 scale）
    - courses / classes / class_memberships
    - tasks（含 dimensions，状态分布 60%/30%/10% published/draft/closed）
    - uploads（每个 published task 给 80% 学生）
    - evaluations（uploads 50% 概率，状态分布）
    - notifications（每个学生 5-10 条，半数 unread）
    - audit logs（最近 7 天，模拟登录/上传/评价等动作）
    """
    sizes: dict[str, dict[str, int]] = {
        "small": {
            "teachers": 2,
            "courses": 2,
            "classes_per_course": 2,
            "students": 20,
            "tasks_per_teacher": 2,
            "audit_days": 3,
        },
        "medium": {
            "teachers": 3,
            "courses": 4,
            "classes_per_course": 2,
            "students": 80,
            "tasks_per_teacher": 4,
            "audit_days": 7,
        },
        "large": {
            "teachers": 5,
            "courses": 6,
            "classes_per_course": 3,
            "students": 200,
            "tasks_per_teacher": 6,
            "audit_days": 14,
        },
    }
    if scale not in sizes:
        raise HTTPException(status_code=400, detail=f"unknown scale {scale}")
    cfg = sizes[scale]

    from app.services import dashboard_service as _dash
    _dash._CACHE.clear()

    summary = await _seed_full(db, scale=scale, cfg=cfg, reset=reset)
    await db.commit()
    return summary


async def _seed_full(
    db: Any, *, scale: str, cfg: dict[str, int], reset: bool
) -> dict[str, Any]:
    """实际填充逻辑（独立函数便于测试）."""
    import random
    from datetime import UTC, datetime, timedelta

    from sqlalchemy import delete
    from tests.factories.audit_factory import AuditLogFactory
    from tests.factories.evaluation_factory import EvaluationFactory
    from tests.factories.notification_factory import NotificationFactory
    from tests.factories.org_factory import (
        ClassFactory,
        CourseFactory,
        MembershipFactory,
    )
    from tests.factories.task_factory import TrainingTaskFactory
    from tests.factories.upload_factory import UploadFactory
    from tests.factories.user_factory import (
        TeacherFactory,
        UserFactory,
    )

    from app.core.security import hash_password
    from app.models.audit import AuditLog
    from app.models.course import Class, ClassMembership, Course
    from app.models.evaluation import DimensionScore, Evaluation
    from app.models.notification import Notification
    from app.models.task import Dimension, TrainingTask
    from app.models.upload import ParseResult, Upload
    from app.models.user import User

    rng = random.Random(20260520 + len(scale))

    # 复用一次 hash 提速：对所有 seed 用户共用同一密码哈希
    seed_password_hash = hash_password("Pa$$w0rd2024")

    if reset:
        # 仅清非常驻表（不动 system templates、不动初始 admin/teacher01/student01/student02）
        keep_usernames = (
            "admin",
            "teacher01",
            "student01",
            "student02",
        )
        await db.execute(delete(AuditLog))
        await db.execute(delete(Notification))
        await db.execute(delete(DimensionScore))
        await db.execute(delete(Evaluation))
        await db.execute(delete(ParseResult))
        await db.execute(delete(Upload))
        await db.execute(delete(Dimension))
        # 删除任务-班级关联
        from app.models.task import task_classes
        await db.execute(delete(task_classes))
        await db.execute(delete(TrainingTask))
        await db.execute(delete(ClassMembership))
        await db.execute(delete(Class))
        await db.execute(delete(Course))
        from sqlalchemy import select
        keep_user_ids = (
            await db.execute(
                select(User.id).where(User.username.in_(keep_usernames))
            )
        ).scalars().all()
        await db.execute(
            delete(User).where(~User.id.in_(list(keep_user_ids)))
        )
        await db.flush()

    # 1. ensure 默认账号（admin/teacher01/student01/student02）
    from sqlalchemy import select as sa_select

    async def _ensure_user(
        username: str, display_name: str, role: str
    ) -> User:
        existing = (
            await db.execute(
                sa_select(User).where(User.username == username)
            )
        ).scalar_one_or_none()
        if existing:
            return existing
        return await UserFactory.create_async(
            db,
            username=username,
            display_name=display_name,
            role=role,
            password_hash=hash_password(
                {
                    "admin": "Admin@123",
                    "teacher01": "Teacher@123",
                    "student01": "Student@123",
                    "student02": "Student@123",
                }.get(username, "Pa$$w0rd2024")
            ),
        )

    admin = await _ensure_user("admin", "管理员", "admin")
    teacher01 = await _ensure_user("teacher01", "王伟", "teacher")
    student01 = await _ensure_user("student01", "李同学", "student")
    student02 = await _ensure_user("student02", "张文卓", "student")

    # 2. 额外教师（密码哈希复用）
    teachers: list[User] = [teacher01]
    for i in range(cfg["teachers"]):
        t = await TeacherFactory.create_async(
            db,
            username=f"seed_t_{scale}_{i:02d}",
            display_name=f"教师{i + 1:02d}",
            password_hash=seed_password_hash,
        )
        teachers.append(t)

    # 3. courses
    courses: list[Course] = []
    code_prefix = {"small": "S", "medium": "M", "large": "L"}[scale]
    for i in range(cfg["courses"]):
        c = await CourseFactory.create_async(
            db,
            name=f"实训课程-{code_prefix}-{i + 1:02d}",
            code=f"{code_prefix}D{i + 1:03d}{rng.randint(100, 999)}",
        )
        courses.append(c)

    # 4. classes（每课程多班）
    classes: list[Class] = []
    for c_idx, c in enumerate(courses):
        for j in range(cfg["classes_per_course"]):
            # 每个课程的第一个班指定给 teacher01，确保它至少有班级
            t = (
                teacher01
                if j == 0 and c_idx == 0
                else rng.choice(teachers)
            )
            cls = await ClassFactory.create_async(
                db,
                course=c,
                teacher=t,
                name=f"{c.name}-{j + 1}班",
            )
            classes.append(cls)

    # 5. 学生 + 班级成员
    students: list[User] = [student01, student02]
    for i in range(cfg["students"]):
        s = await UserFactory.create_async(
            db,
            username=f"seed_s_{scale}_{i:04d}",
            display_name=f"学生{i + 1:04d}",
            password_hash=seed_password_hash,
        )
        students.append(s)

    # 把每个学生分配到 1-2 个班级（保证 student01/02 至少在 teacher01 的班里）
    student_class_map: dict[int, list[int]] = {}
    teacher01_classes = [cl for cl in classes if cl.teacher_id == teacher01.id]
    for s in students:
        if s.id in (student01.id, student02.id) and teacher01_classes:
            # student01/02 强制加入 teacher01 的第一个班
            chosen = [teacher01_classes[0]]
            # 50% 再加一个其它班
            if len(classes) > 1 and rng.random() < 0.5:
                others = [c for c in classes if c.id != chosen[0].id]
                chosen.append(rng.choice(others))
        else:
            chosen = rng.sample(
                classes, k=min(rng.randint(1, 2), len(classes))
            )
        student_class_map[s.id] = [cl.id for cl in chosen]
        for cl in chosen:
            await MembershipFactory.create_async(
                db, class_obj=cl, student=s
            )
            cl.student_count = (cl.student_count or 0) + 1
    await db.flush()

    # 6. tasks（按 teacher 分配，状态分布）
    status_pool = (
        ["published"] * 6 + ["draft"] * 3 + ["closed"] * 1
    )
    tasks: list[TrainingTask] = []
    now = datetime.now(UTC)
    for t in teachers:
        for k in range(cfg["tasks_per_teacher"]):
            status = rng.choice(status_pool)
            # deadline：60% 未来、30% 已过、10% 无
            if rng.random() < 0.6:
                deadline = now + timedelta(days=rng.randint(1, 30))
            elif rng.random() < 0.7:
                deadline = now - timedelta(days=rng.randint(1, 14))
            else:
                deadline = None
            # 任务关联：教师所教班级（teacher_id 匹配）
            cand_classes = [
                cl for cl in classes if cl.teacher_id == t.id
            ]
            if not cand_classes:
                cand_classes = rng.sample(
                    classes, k=min(2, len(classes))
                )
            sel_classes = rng.sample(
                cand_classes, k=min(rng.randint(1, 2), len(cand_classes))
            )
            course = rng.choice(
                [c for c in courses if c.id == sel_classes[0].course_id]
                or courses
            )
            task = await TrainingTaskFactory.create_async(
                db,
                teacher=t,
                course_id=course.id,
                name=f"{course.name}-实训{k + 1:02d}",
                description=f"{t.display_name} 老师布置的第 {k + 1} 次实训",
                requirements="1. 提交源代码\n2. 实验报告 PDF\n3. 测试用例",
                status=status,
                deadline=deadline,
                with_dimensions=rng.randint(3, 5),
                classes=sel_classes,
            )
            tasks.append(task)

    # 7. uploads + evaluations
    uploads_total = 0
    eval_total = 0
    eval_status_pool = (
        ["finalized"] * 5
        + ["reviewed"] * 2
        + ["auto_scored"] * 2
        + ["rejected"] * 1
    )
    for task in tasks:
        if task.status not in ("published", "closed"):
            continue
        # 该任务关联班级中的学生
        cls_ids = [cl.id for cl in task.classes]
        eligible_students = [
            s for s in students
            if any(cid in cls_ids for cid in student_class_map.get(s.id, []))
        ]
        if not eligible_students:
            continue
        # 80% 学生提交（带 student01/02 优先）
        target_count = int(len(eligible_students) * 0.8)
        if (
            student01 in eligible_students
            and student01 not in eligible_students[:target_count]
        ):
            eligible_students.remove(student01)
            eligible_students.insert(0, student01)
        sub_students = eligible_students[:target_count]
        for s in sub_students:
            file_type = rng.choice(["pdf", "docx", "zip"])
            up = await UploadFactory.create_async(
                db,
                student=s,
                task=task,
                filename=f"{task.name}_{s.username}.{file_type}",
                file_type=file_type,
                file_size=rng.randint(50_000, 5_000_000),
                parse_status="parsed",
            )
            uploads_total += 1
            # 50% 概率创建 evaluation
            if rng.random() < 0.6 and task.dimensions:
                ev_status = rng.choice(eval_status_pool)
                await EvaluationFactory.create_async(
                    db,
                    upload=up,
                    task=task,
                    student=s,
                    status=ev_status,
                    rng=rng,
                )
                eval_total += 1

    # 8. notifications：每学生 5-10 条；teachers 也来 3-5 条
    notif_count = 0
    for s in students:
        n_count = rng.randint(5, 10)
        for _ in range(n_count):
            from app.services import notification_events as ne

            ev = rng.choice(
                [
                    ne.TASK_PUBLISHED,
                    ne.PARSE_COMPLETED,
                    ne.EVALUATION_COMPLETED,
                    ne.UPLOAD_REJECTED,
                    ne.DEADLINE_APPROACHING,
                ]
            )
            await NotificationFactory.create_async(
                db,
                user_id=s.id,
                event_type=ev,
                is_read=rng.random() < 0.5,
                rng=rng,
                task_name=rng.choice([t.name for t in tasks])
                if tasks
                else "实训任务",
            )
            notif_count += 1
    for t in teachers:
        for _ in range(rng.randint(3, 5)):
            from app.services import notification_events as ne

            await NotificationFactory.create_async(
                db,
                user_id=t.id,
                event_type=rng.choice(
                    [ne.SIMILARITY_DETECTED, ne.EVALUATION_COMPLETED]
                ),
                is_read=rng.random() < 0.3,
                rng=rng,
            )
            notif_count += 1

    # 9. audit logs：最近 N 天，模拟所有用户的活动
    audit_users = (
        [(admin.id, admin.username, "admin")]
        + [(t.id, t.username, "teacher") for t in teachers[:3]]
        + [(s.id, s.username, "student") for s in students[:8]]
    )
    audit_count = await AuditLogFactory.burst(
        db,
        users=audit_users,
        days=cfg["audit_days"],
        per_day_per_user=4,
        rng=rng,
    )

    return {
        "scale": scale,
        "reset": reset,
        # 兼容旧 contract test：保留扁平字段
        "courses": len(courses),
        "classes": len(classes),
        "students": cfg["students"],
        "users": {
            "teachers": len(teachers),
            "students": len(students),
        },
        "tasks": {
            "total": len(tasks),
            "published": sum(1 for t in tasks if t.status == "published"),
            "draft": sum(1 for t in tasks if t.status == "draft"),
            "closed": sum(1 for t in tasks if t.status == "closed"),
        },
        "uploads": uploads_total,
        "evaluations": eval_total,
        "notifications": notif_count,
        "audit_logs": audit_count,
    }


@router.post("/clock/freeze")
async def dev_clock_freeze(payload: dict[str, str]) -> dict[str, str]:
    from datetime import datetime

    iso = payload.get("time")
    if not iso:
        raise HTTPException(status_code=400, detail="time 必须 ISO8601")
    fc = FrozenClock(datetime.fromisoformat(iso.replace("Z", "+00:00")))
    set_clock(fc)
    return {"frozen_at": fc.now().isoformat()}


@router.post("/clock/advance")
async def dev_clock_advance(seconds: int = 0) -> dict[str, str]:
    c = get_clock()
    if isinstance(c, FrozenClock):
        c.advance(seconds=seconds)
        return {"now": c.now().isoformat()}
    raise HTTPException(
        status_code=400, detail="先 freeze 再 advance"
    )


@router.post("/clock/restore")
async def dev_clock_restore() -> dict[str, str]:
    set_clock(SystemClock())
    return {"status": "system"}


@router.post("/cache/flush")
async def dev_cache_flush() -> dict[str, object]:
    """清理内存缓存（system_config / dashboard）.

    生产用 Redis flushdb，这里 dev 仅清自家内存缓存。
    """
    try:
        from app.services import dashboard_service

        dashboard_service._CACHE.clear()
    except Exception:  # noqa: BLE001
        pass
    return {"status": "ok"}


@router.get("/health/full")
async def dev_health_full(db: DbSession) -> dict[str, Any]:
    from sqlalchemy import text as sa_text

    out: dict[str, Any] = {}
    try:
        await db.execute(sa_text("SELECT 1"))
        out["db"] = "ok"
    except Exception as e:  # noqa: BLE001
        out["db"] = f"failed: {e}"

    out["env"] = _settings.env
    out["clock"] = get_clock().now().isoformat()
    return out


# ============== 26.3 LLM mock / 上传强制处理 ==============


_DEV_LLM_OVERRIDE: Any = None


@router.post("/llm/mock")
async def dev_llm_mock(payload: dict[str, Any]) -> dict[str, str]:
    from tests.fakes.fake_llm import FakeLLM

    from app.llm.base import LLMResponse

    fake = FakeLLM()
    for r in payload.get("responses", []):
        fake._matchers.append(  # noqa: SLF001
            (lambda _msgs, _r=r: True, LLMResponse(**_r))
        )
    global _DEV_LLM_OVERRIDE
    _DEV_LLM_OVERRIDE = fake
    return {"status": "mocked"}


@router.post("/llm/restore")
async def dev_llm_restore() -> dict[str, str]:
    global _DEV_LLM_OVERRIDE
    _DEV_LLM_OVERRIDE = None
    return {"status": "restored"}


@router.post("/uploads/{upload_id}/force-fail")
async def dev_force_fail(upload_id: int, db: DbSession) -> dict[str, Any]:
    from app.models.upload import Upload

    u = await db.get(Upload, upload_id)
    if u is None:
        raise HTTPException(status_code=404, detail="upload not found")
    u.parse_status = "failed"
    await db.commit()
    return {"upload_id": upload_id, "parse_status": u.parse_status}


# ============== 26.4 通知注入 / 任务执行 / 状态透视 ==============


@router.post("/notifications/{user_id}/inject")
async def dev_inject_notification(
    user_id: int, payload: dict[str, str], db: DbSession
) -> dict[str, Any]:
    from app.services.notification_events import (
        EVALUATION_COMPLETED,
    )
    from app.services.notification_service import NotificationService

    svc = NotificationService()
    await svc.send(
        db,
        recipient_ids=[user_id],
        event_type=payload.get("event_type") or EVALUATION_COMPLETED,
        title=payload.get("title", "测试通知"),
        content=payload.get("content", ""),
    )
    await db.commit()
    return {"status": "sent"}


@router.get("/state/{entity}/{entity_id}")
async def dev_state(
    entity: str, entity_id: int, db: DbSession
) -> dict[str, Any]:
    """透视任意实体（含外键展开 1 层）."""
    from app.models.evaluation import Evaluation
    from app.models.task import TrainingTask
    from app.models.upload import Upload
    from app.models.user import User

    table = {
        "user": User,
        "task": TrainingTask,
        "upload": Upload,
        "evaluation": Evaluation,
    }
    Model = table.get(entity)
    if Model is None:
        raise HTTPException(
            status_code=404, detail=f"unknown entity {entity}"
        )
    obj = await db.get(Model, entity_id)
    if obj is None:
        raise HTTPException(
            status_code=404, detail=f"{entity}:{entity_id} not found"
        )
    return {
        col.name: getattr(obj, col.name, None)
        for col in obj.__table__.columns
    }


@router.post("/audit/dump")
async def dev_audit_dump(db: DbSession, limit: int = 100) -> dict[str, Any]:
    from app.services.audit_service import AuditService

    svc = AuditService()
    items = await svc.list_logs(db, limit=limit)
    return {
        "count": len(items),
        "items": [
            {
                "id": x.id,
                "occurred_at": x.occurred_at.isoformat(),
                "action": x.action,
                "user_id": x.user_id,
                "result": x.result,
            }
            for x in items
        ],
    }
