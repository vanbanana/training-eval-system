"""组织数据访问 - 课程 / 班级 / 班级成员."""

from __future__ import annotations

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from app.models.course import Class, ClassMembership, Course
from app.repositories.base import BaseRepository


class CourseRepository(BaseRepository[Course]):
    model = Course

    async def get_by_code(self, db: AsyncSession, code: str) -> Course | None:
        stmt = select(Course).where(Course.code == code)
        return (await db.execute(stmt)).scalar_one_or_none()

    async def list_active(self, db: AsyncSession) -> list[Course]:
        stmt = select(Course).where(Course.is_archived.is_(False)).order_by(Course.id)
        return list((await db.execute(stmt)).scalars().all())


class ClassRepository(BaseRepository[Class]):
    model = Class

    async def list_by_teacher(self, db: AsyncSession, teacher_id: int) -> list[Class]:
        stmt = select(Class).where(Class.teacher_id == teacher_id).order_by(Class.id)
        return list((await db.execute(stmt)).scalars().all())

    async def list_by_course(self, db: AsyncSession, course_id: int) -> list[Class]:
        stmt = select(Class).where(Class.course_id == course_id).order_by(Class.id)
        return list((await db.execute(stmt)).scalars().all())


class MembershipRepository(BaseRepository[ClassMembership]):
    model = ClassMembership

    async def is_student_in_class(
        self, db: AsyncSession, *, student_id: int, class_id: int
    ) -> bool:
        """检查学生是否在班级内（已归档班级也算 True，归档不影响关系）."""
        stmt = select(ClassMembership).where(
            ClassMembership.student_id == student_id,
            ClassMembership.class_id == class_id,
        )
        return (await db.execute(stmt)).scalar_one_or_none() is not None

    async def list_students_of_class(
        self, db: AsyncSession, class_id: int
    ) -> list[ClassMembership]:
        stmt = (
            select(ClassMembership)
            .where(ClassMembership.class_id == class_id)
            .order_by(ClassMembership.id)
        )
        return list((await db.execute(stmt)).scalars().all())

    async def list_classes_of_student(
        self, db: AsyncSession, student_id: int
    ) -> list[ClassMembership]:
        stmt = (
            select(ClassMembership)
            .where(ClassMembership.student_id == student_id)
            .order_by(ClassMembership.id)
        )
        return list((await db.execute(stmt)).scalars().all())

    async def remove_membership(
        self, db: AsyncSession, *, student_id: int, class_id: int
    ) -> int:
        """移除学生班级关系；返回 affected rows."""
        from sqlalchemy import delete

        stmt = delete(ClassMembership).where(
            ClassMembership.student_id == student_id,
            ClassMembership.class_id == class_id,
        )
        result = await db.execute(stmt)
        return int(result.rowcount or 0)
