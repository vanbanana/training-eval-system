"""Epic 18.5 + 19.5: Profile API（基于 ProfileService）."""

from __future__ import annotations

from fastapi import APIRouter
from sqlalchemy import select

from app.api.deps import CurrentUser, DbSession
from app.core.exceptions import AuthorizationError
from app.models.course import Class, ClassMembership
from app.models.evaluation import Evaluation
from app.models.profile import StudentProfile
from app.schemas.profile import ProfileOut
from app.services.profile_service import (
    InsufficientDataError,
    ProfileService,
)

router = APIRouter(prefix="/api/profiles", tags=["profiles"])


async def _student_in_teacher_classes(
    db: DbSession, *, student_id: int, teacher_id: int
) -> bool:
    """Return True if student belongs to any class taught by teacher."""
    rows = list(
        (
            await db.execute(
                select(ClassMembership)
                .join(Class, Class.id == ClassMembership.class_id)
                .where(
                    ClassMembership.student_id == student_id,
                    Class.teacher_id == teacher_id,
                )
                .limit(1)
            )
        )
        .scalars()
        .all()
    )
    return bool(rows)


@router.get("/student/{user_id}", response_model=ProfileOut)
async def get_student_profile(
    user_id: int, db: DbSession, current: CurrentUser
) -> ProfileOut:
    """学生本人 / 该班级教师 / admin 可访问."""
    if current.role == "student" and current.id != user_id:
        raise AuthorizationError("无权查看他人画像")
    if current.role == "teacher":
        ok = await _student_in_teacher_classes(
            db, student_id=user_id, teacher_id=current.id
        )
        if not ok:
            raise AuthorizationError("学生不在你的班级")

    profile = (
        await db.execute(
            select(StudentProfile).where(StudentProfile.student_id == user_id)
        )
    ).scalar_one_or_none()

    # 检查评价数
    eval_count = len(
        list(
            (
                await db.execute(
                    select(Evaluation).where(Evaluation.student_id == user_id)
                )
            ).scalars()
        )
    )

    if profile is None:
        # 尝试触发计算
        svc = ProfileService()
        try:
            profile = await svc.compute_student_profile(db, student_id=user_id)
            await db.commit()
        except InsufficientDataError:
            return ProfileOut(
                student_id=user_id,
                radar_data={},
                weakness_list=[],
                suggestions=[],
                score_trend=[],
                source_evaluation_count=eval_count,
                insufficient_data=True,
            )

    return ProfileOut(
        student_id=user_id,
        radar_data=profile.radar_data or {},
        weakness_list=profile.weakness_list or [],
        suggestions=profile.suggestions or [],
        score_trend=profile.score_trend or [],
        source_evaluation_count=profile.source_evaluation_count,
        computed_at=profile.computed_at,
        insufficient_data=profile.source_evaluation_count < 3,
    )



@router.get("/course/{course_id}")
async def get_course_profile(
    course_id: int,
    db: DbSession,
    current: CurrentUser,
    range: str = "1m",
) -> dict[str, object]:
    """Epic 19.5: 课程级教学画像."""
    if current.role == "student":
        raise AuthorizationError("仅教师/管理员可查看")

    # 教师权限：必须是该课程下任一任务的教师，或 admin
    if current.role == "teacher":
        from sqlalchemy import select

        from app.models.task import TrainingTask

        owns = (
            await db.execute(
                select(TrainingTask)
                .where(
                    TrainingTask.course_id == course_id,
                    TrainingTask.teacher_id == current.id,
                )
                .limit(1)
            )
        ).scalar_one_or_none()
        if owns is None:
            raise AuthorizationError("无权查看该课程")

    from app.services.profile_service import aggregate_course_metrics

    metrics = await aggregate_course_metrics(db, course_id=course_id)
    return {
        "course_id": course_id,
        "range": range,
        "metrics": metrics,
        "chart_data": {
            "radar": [
                {"name": d["name"], "value": d["avg"]}
                for d in metrics["dimension_distributions"]
            ]
        },
    }


@router.get("/school")
async def get_school_profile(
    db: DbSession,
    current: CurrentUser,
    range: str = "1m",
) -> dict[str, object]:
    """Epic 19.5: 学校级画像（仅 admin）."""
    if current.role != "admin":
        raise AuthorizationError("仅 admin 可查看")
    from sqlalchemy import select

    from app.models.evaluation import Evaluation as Ev
    from app.models.user import User as Us

    students = list(
        (await db.execute(select(Us).where(Us.role == "student"))).scalars()
    )
    evals = list(
        (await db.execute(select(Ev))).scalars()
    )
    scores = [e.total_score for e in evals if e.total_score is not None]
    return {
        "range": range,
        "total_students": len(students),
        "total_evaluations": len(evals),
        "avg_score": round(sum(scores) / len(scores), 1) if scores else 0.0,
    }
