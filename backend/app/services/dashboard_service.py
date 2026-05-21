"""DashboardService - Epic 24."""

from __future__ import annotations

from datetime import UTC, datetime, timedelta
from typing import Any

from sqlalchemy import func as sqlfunc
from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from app.core.logging import get_logger
from app.models.evaluation import Evaluation
from app.models.notification import Notification
from app.models.task import TrainingTask
from app.models.upload import Upload
from app.models.user import User


log = get_logger(__name__)


# 简单内存缓存（生产替换为 Redis）
_CACHE: dict[str, tuple[datetime, dict[str, Any]]] = {}
_TTL = timedelta(minutes=5)


def _cache_key(role: str, user_id: int) -> str:
    return f"dashboard:{role}:{user_id}"


async def get_system_resources() -> dict[str, float | None]:
    """采集系统资源 - psutil 失败时返回 None."""
    try:
        import psutil  # type: ignore[import-not-found]
    except ImportError:
        return {"cpu_percent": None, "mem_percent": None, "disk_percent": None}
    try:
        return {
            "cpu_percent": psutil.cpu_percent(interval=0.0),
            "mem_percent": psutil.virtual_memory().percent,
            "disk_percent": psutil.disk_usage("/").percent,
        }
    except Exception as e:  # noqa: BLE001
        log.warning("dashboard.psutil_failed", error=str(e))
        return {"cpu_percent": None, "mem_percent": None, "disk_percent": None}


class DashboardService:
    async def get_dashboard(
        self, db: AsyncSession, *, user: User
    ) -> dict[str, Any]:
        cached = _CACHE.get(_cache_key(user.role, user.id))
        if cached:
            ts, data = cached
            if datetime.now(UTC) - ts < _TTL:
                return data

        if user.role == "admin":
            data = await self._admin(db)
        elif user.role == "teacher":
            data = await self._teacher(db, user)
        else:
            data = await self._student(db, user)
        data["role"] = user.role

        _CACHE[_cache_key(user.role, user.id)] = (datetime.now(UTC), data)
        return data

    async def invalidate(self, user_ids: list[int]) -> None:
        """删除指定用户的缓存."""
        for uid in user_ids:
            for role in ("admin", "teacher", "student"):
                _CACHE.pop(_cache_key(role, uid), None)

    # ------ 角色特定聚合 ------

    async def _admin(self, db: AsyncSession) -> dict[str, Any]:
        user_count = (
            await db.execute(select(sqlfunc.count(User.id)))
        ).scalar() or 0
        task_count = (
            await db.execute(select(sqlfunc.count(TrainingTask.id)))
        ).scalar() or 0
        eval_count = (
            await db.execute(select(sqlfunc.count(Evaluation.id)))
        ).scalar() or 0
        active_30d = (
            await db.execute(
                select(sqlfunc.count(sqlfunc.distinct(Upload.student_id))).where(
                    Upload.created_at
                    >= datetime.now(UTC) - timedelta(days=30)
                )
            )
        ).scalar() or 0
        resources = await get_system_resources()
        return {
            "user_count": int(user_count),
            "task_count": int(task_count),
            "eval_count": int(eval_count),
            "monthly_active_students": int(active_30d),
            "system_resources": resources,
        }

    async def _teacher(self, db: AsyncSession, user: User) -> dict[str, Any]:
        from app.models.course import Class, ClassMembership

        my_tasks = (
            await db.execute(
                select(sqlfunc.count(TrainingTask.id)).where(
                    TrainingTask.teacher_id == user.id
                )
            )
        ).scalar() or 0
        pending = (
            await db.execute(
                select(sqlfunc.count(Evaluation.id)).where(
                    Evaluation.status == "auto_scored",
                    Evaluation.task_id.in_(
                        select(TrainingTask.id).where(
                            TrainingTask.teacher_id == user.id
                        )
                    ),
                )
            )
        ).scalar() or 0
        week_ago = datetime.now(UTC) - timedelta(days=7)
        graded_week = (
            await db.execute(
                select(sqlfunc.count(Evaluation.id)).where(
                    Evaluation.status == "finalized",
                    Evaluation.updated_at >= week_ago,
                    Evaluation.task_id.in_(
                        select(TrainingTask.id).where(
                            TrainingTask.teacher_id == user.id
                        )
                    ),
                )
            )
        ).scalar() or 0

        # 班级平均分：该教师所有班级学生的 finalized 评价平均分
        my_class_ids = list(
            (
                await db.execute(
                    select(Class.id).where(Class.teacher_id == user.id)
                )
            ).scalars()
        )
        class_avg_score: float | None = None
        if my_class_ids:
            student_ids_in_classes = list(
                (
                    await db.execute(
                        select(ClassMembership.student_id).where(
                            ClassMembership.class_id.in_(my_class_ids)
                        )
                    )
                ).scalars()
            )
            if student_ids_in_classes:
                avg = (
                    await db.execute(
                        select(sqlfunc.avg(Evaluation.total_score)).where(
                            Evaluation.student_id.in_(student_ids_in_classes),
                            Evaluation.total_score.isnot(None),
                        )
                    )
                ).scalar()
                class_avg_score = round(float(avg), 1) if avg else None

        # 班级活跃度：近 7 天每天的上传数（该教师班级学生）
        activity_7d: list[dict[str, Any]] = []
        now = datetime.now(UTC)
        for day_offset in range(6, -1, -1):
            day_start = (now - timedelta(days=day_offset)).replace(
                hour=0, minute=0, second=0, microsecond=0
            )
            day_end = day_start + timedelta(days=1)
            count = 0
            if my_class_ids:
                student_ids_in_classes_set = set(
                    (
                        await db.execute(
                            select(ClassMembership.student_id).where(
                                ClassMembership.class_id.in_(my_class_ids)
                            )
                        )
                    ).scalars()
                )
                if student_ids_in_classes_set:
                    count = (
                        await db.execute(
                            select(sqlfunc.count(Upload.id)).where(
                                Upload.student_id.in_(list(student_ids_in_classes_set)),
                                Upload.created_at >= day_start,
                                Upload.created_at < day_end,
                            )
                        )
                    ).scalar() or 0
            activity_7d.append({
                "date": day_start.strftime("%m/%d"),
                "count": int(count),
            })

        recent_tasks = list(
            (
                await db.execute(
                    select(TrainingTask)
                    .where(TrainingTask.teacher_id == user.id)
                    .order_by(TrainingTask.created_at.desc())
                    .limit(5)
                )
            )
            .scalars()
            .all()
        )

        # 每个任务的提交进度（已提交/总人数、已批改/已提交）
        recent_tasks_data: list[dict[str, Any]] = []
        for t in recent_tasks:
            # 该任务关联班级的学生总数
            from app.models.task import task_classes
            cls_ids = list(
                (
                    await db.execute(
                        select(task_classes.c.class_id).where(
                            task_classes.c.task_id == t.id
                        )
                    )
                ).scalars()
            )
            total_students = 0
            if cls_ids:
                total_students = (
                    await db.execute(
                        select(sqlfunc.count(ClassMembership.id)).where(
                            ClassMembership.class_id.in_(cls_ids)
                        )
                    )
                ).scalar() or 0
            submitted = (
                await db.execute(
                    select(sqlfunc.count(Upload.id)).where(
                        Upload.task_id == t.id
                    )
                )
            ).scalar() or 0
            graded = (
                await db.execute(
                    select(sqlfunc.count(Evaluation.id)).where(
                        Evaluation.task_id == t.id,
                        Evaluation.status.in_(["finalized", "confirmed", "reviewed"]),
                    )
                )
            ).scalar() or 0
            recent_tasks_data.append({
                "id": t.id,
                "name": t.name,
                "status": t.status,
                "deadline": t.deadline.isoformat() if t.deadline else None,
                "total_students": int(total_students),
                "submitted": int(submitted),
                "graded": int(graded),
            })

        recent_notifs = list(
            (
                await db.execute(
                    select(Notification)
                    .where(Notification.user_id == user.id)
                    .order_by(Notification.created_at.desc())
                    .limit(5)
                )
            )
            .scalars()
            .all()
        )
        return {
            "my_tasks": int(my_tasks),
            "pending_grading": int(pending),
            "graded_this_week": int(graded_week),
            "class_avg_score": class_avg_score,
            "activity_7d": activity_7d,
            "recent_tasks": recent_tasks_data,
            "recent_notifications": [
                {"id": n.id, "title": n.title, "type": n.type}
                for n in recent_notifs
            ],
        }

    async def _student(self, db: AsyncSession, user: User) -> dict[str, Any]:
        from app.models.course import Class, ClassMembership

        # 待提交任务：published 状态 + 当前学生未提交
        submitted_task_ids = list(
            (
                await db.execute(
                    select(Upload.task_id).where(Upload.student_id == user.id)
                )
            ).scalars()
        )
        pending_tasks = list(
            (
                await db.execute(
                    select(TrainingTask)
                    .where(
                        TrainingTask.status == "published",
                        ~TrainingTask.id.in_(submitted_task_ids)
                        if submitted_task_ids
                        else True,
                    )
                    .order_by(TrainingTask.deadline.asc())
                    .limit(5)
                )
            )
            .scalars()
            .all()
        )

        # 所有评价（用于趋势 + 排名 + 最近评分）
        all_evals = list(
            (
                await db.execute(
                    select(Evaluation)
                    .where(
                        Evaluation.student_id == user.id,
                        Evaluation.total_score.isnot(None),
                    )
                    .order_by(Evaluation.created_at.asc())
                )
            )
            .scalars()
            .all()
        )

        # 近期评分（最近一次）
        latest_score: float | None = None
        prev_score: float | None = None
        if all_evals:
            latest_score = all_evals[-1].total_score
            if len(all_evals) >= 2:
                prev_score = all_evals[-2].total_score

        # 评分趋势（最近 8 次）
        score_trend = [
            {
                "label": f"T{i + 1}",
                "score": round(e.total_score, 1) if e.total_score else 0,
                "task_id": e.task_id,
            }
            for i, e in enumerate(all_evals[-8:])
        ]

        # 班级排名：找到学生所在班级，计算同班同学的平均分排名
        rank: int | None = None
        class_size: int | None = None
        my_memberships = list(
            (
                await db.execute(
                    select(ClassMembership.class_id).where(
                        ClassMembership.student_id == user.id
                    )
                )
            ).scalars()
        )
        if my_memberships:
            # 取第一个班级
            first_class_id = my_memberships[0]
            classmates = list(
                (
                    await db.execute(
                        select(ClassMembership.student_id).where(
                            ClassMembership.class_id == first_class_id
                        )
                    )
                ).scalars()
            )
            class_size = len(classmates)
            # 每个同学的平均分
            scores_by_student: dict[int, float] = {}
            for sid in classmates:
                avg = (
                    await db.execute(
                        select(sqlfunc.avg(Evaluation.total_score)).where(
                            Evaluation.student_id == sid,
                            Evaluation.total_score.isnot(None),
                        )
                    )
                ).scalar()
                if avg is not None:
                    scores_by_student[sid] = float(avg)
            # 排名（降序）
            sorted_students = sorted(
                scores_by_student.items(), key=lambda x: x[1], reverse=True
            )
            rank = next(
                (i + 1 for i, (sid, _) in enumerate(sorted_students) if sid == user.id),
                None,
            )

        # 能力雷达 + 薄弱点（从 profile service 取）
        radar_data: dict[str, float] = {}
        weakness_list: list[dict[str, Any]] = []
        try:
            from app.models.evaluation import DimensionScore
            from app.models.task import Dimension

            # 聚合该学生所有维度的平均分
            dim_scores = list(
                (
                    await db.execute(
                        select(
                            Dimension.name,
                            sqlfunc.avg(DimensionScore.ai_score),
                        )
                        .join(
                            DimensionScore,
                            DimensionScore.dimension_id == Dimension.id,
                        )
                        .join(
                            Evaluation,
                            Evaluation.id == DimensionScore.evaluation_id,
                        )
                        .where(
                            Evaluation.student_id == user.id,
                            DimensionScore.ai_score.isnot(None),
                        )
                        .group_by(Dimension.name)
                    )
                ).all()
            )
            for name, avg_score in dim_scores:
                radar_data[name] = round(float(avg_score), 1) if avg_score else 0
            # 薄弱点：按分数升序取前 3
            sorted_dims = sorted(
                [(name, score) for name, score in radar_data.items()],
                key=lambda x: x[1],
            )
            weakness_list = [
                {"name": name, "score": score}
                for name, score in sorted_dims[:3]
            ]
        except Exception:  # noqa: BLE001
            pass

        # AI 助手配额
        from app.models.chat import ChatMessage

        today_start = datetime.now(UTC).replace(
            hour=0, minute=0, second=0, microsecond=0
        )
        ai_used_today = (
            await db.execute(
                select(sqlfunc.count(ChatMessage.id)).where(
                    ChatMessage.role == "user",
                    ChatMessage.created_at >= today_start,
                )
            )
        ).scalar() or 0
        ai_daily_limit = 50  # 从 settings 取更好，但这里先用默认值

        recent_notifs = list(
            (
                await db.execute(
                    select(Notification)
                    .where(Notification.user_id == user.id)
                    .order_by(Notification.created_at.desc())
                    .limit(5)
                )
            )
            .scalars()
            .all()
        )
        return {
            "pending_tasks": [
                {
                    "id": t.id,
                    "name": t.name,
                    "deadline": t.deadline.isoformat() if t.deadline else None,
                    "course_id": t.course_id,
                }
                for t in pending_tasks
            ],
            "pending_task_count": len(pending_tasks),
            "latest_score": latest_score,
            "score_diff": round(latest_score - prev_score, 1)
            if latest_score and prev_score
            else None,
            "score_trend": score_trend,
            "rank": rank,
            "class_size": class_size,
            "radar_data": radar_data,
            "weakness_list": weakness_list,
            "ai_used_today": int(ai_used_today),
            "ai_daily_limit": ai_daily_limit,
            "recent_evaluations": [
                {
                    "id": e.id,
                    "task_id": e.task_id,
                    "total_score": e.total_score,
                    "status": e.status,
                }
                for e in all_evals[-3:]
            ],
            "recent_notifications": [
                {"id": n.id, "title": n.title, "type": n.type}
                for n in recent_notifs
            ],
        }
