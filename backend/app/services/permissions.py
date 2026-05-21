"""权限/归属校验工具 - Property 13 班级归属一致性核心.

约束：学生只能提交到自己所在班级的实训任务；教师只能批改自己班级的提交。

主入口：assert_student_in_class / assert_teacher_owns_class
"""

from __future__ import annotations

from sqlalchemy.ext.asyncio import AsyncSession

from app.core.exceptions import AuthorizationError, ResourceNotFoundError
from app.models.course import Class
from app.models.user import User
from app.repositories.org_repo import ClassRepository, MembershipRepository

_membership_repo = MembershipRepository()
_class_repo = ClassRepository()


async def assert_student_in_class(
    db: AsyncSession, *, student_id: int, class_id: int
) -> None:
    """学生必须是班级成员；否则 AuthorizationError(error_code='NOT_IN_CLASS')."""
    if not await _membership_repo.is_student_in_class(
        db, student_id=student_id, class_id=class_id
    ):
        raise AuthorizationError(
            f"学生 {student_id} 不属于班级 {class_id}",
            field="class_id",
        )


async def assert_teacher_owns_class(
    db: AsyncSession, *, teacher: User, class_id: int
) -> Class:
    """教师必须是该班级的 owner（admin 总是允许）；返回 Class 实例."""
    cls = await _class_repo.get(db, class_id)
    if cls is None:
        raise ResourceNotFoundError(f"class {class_id} not found")
    if teacher.role == "admin":
        return cls
    if teacher.role != "teacher" or cls.teacher_id != teacher.id:
        raise AuthorizationError(
            f"教师 {teacher.id} 无权操作班级 {class_id}",
            field="class_id",
        )
    return cls


async def is_student_in_class(
    db: AsyncSession, *, student_id: int, class_id: int
) -> bool:
    """非抛异常版本，便于条件分支."""
    return await _membership_repo.is_student_in_class(
        db, student_id=student_id, class_id=class_id
    )
