"""Course / Class / ClassMembership factories."""

from __future__ import annotations

from typing import Any

from sqlalchemy.ext.asyncio import AsyncSession

from app.models.course import Class, ClassMembership, Course
from app.models.user import User
from tests.factories import faker
from tests.factories.user_factory import TeacherFactory


_COURSE_CODE_COUNTER = 0


class CourseFactory:
    @classmethod
    async def create_async(
        cls,
        session: AsyncSession,
        *,
        name: str | None = None,
        code: str | None = None,
        is_archived: bool = False,
        **extra: Any,
    ) -> Course:
        global _COURSE_CODE_COUNTER
        _COURSE_CODE_COUNTER += 1
        course = Course(
            name=name or faker.sentence(nb_words=3),
            code=code or f"C{_COURSE_CODE_COUNTER:04d}",
            is_archived=is_archived,
            **extra,
        )
        session.add(course)
        await session.flush()
        await session.refresh(course)
        return course


class ClassFactory:
    @classmethod
    async def create_async(
        cls,
        session: AsyncSession,
        *,
        name: str | None = None,
        course: Course | None = None,
        course_id: int | None = None,
        teacher: User | None = None,
        teacher_id: int | None = None,
        is_archived: bool = False,
        **extra: Any,
    ) -> Class:
        if course is None and course_id is None:
            course = await CourseFactory.create_async(session)
        if teacher is None and teacher_id is None:
            teacher = await TeacherFactory.create_async(
                session, username=f"t_{faker.uuid4()[:6]}"
            )

        cls_obj = Class(
            name=name or f"班级 {faker.random_int(min=1, max=99)}",
            course_id=course_id or (course.id if course else 0),
            teacher_id=teacher_id or (teacher.id if teacher else 0),
            is_archived=is_archived,
            **extra,
        )
        session.add(cls_obj)
        await session.flush()
        await session.refresh(cls_obj)
        return cls_obj


class MembershipFactory:
    @classmethod
    async def create_async(
        cls,
        session: AsyncSession,
        *,
        class_obj: Class | None = None,
        student: User | None = None,
        class_id: int | None = None,
        student_id: int | None = None,
    ) -> ClassMembership:
        m = ClassMembership(
            class_id=class_id or (class_obj.id if class_obj else 0),
            student_id=student_id or (student.id if student else 0),
        )
        session.add(m)
        await session.flush()
        await session.refresh(m)
        return m
