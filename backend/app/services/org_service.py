"""OrgService - 组织管理业务编排.

权限矩阵：
- 管理员：全部操作
- 教师：仅可管理自己创建的班级（增/编/批量加学生）
- 学生：仅可读
"""

from __future__ import annotations

from dataclasses import dataclass

from sqlalchemy.ext.asyncio import AsyncSession

from app.core.exceptions import (
    AuthorizationError,
    ConflictError,
    ResourceNotFoundError,
)
from app.core.logging import get_logger
from app.models.course import Class, ClassMembership, Course
from app.models.user import User
from app.repositories.org_repo import (
    ClassRepository,
    CourseRepository,
    MembershipRepository,
)
from app.repositories.user_repo import UserRepository

log = get_logger(__name__)


@dataclass(slots=True)
class BulkAddResult:
    added: list[int]
    failed: list[tuple[int, str]]  # (student_id, reason)


class OrgService:
    def __init__(
        self,
        course_repo: CourseRepository | None = None,
        class_repo: ClassRepository | None = None,
        membership_repo: MembershipRepository | None = None,
        user_repo: UserRepository | None = None,
    ) -> None:
        self.course_repo = course_repo or CourseRepository()
        self.class_repo = class_repo or ClassRepository()
        self.membership_repo = membership_repo or MembershipRepository()
        self.user_repo = user_repo or UserRepository()

    @staticmethod
    def _ensure_admin(actor: User) -> None:
        if actor.role != "admin":
            raise AuthorizationError("仅管理员可执行此操作")

    @staticmethod
    def _ensure_teacher_or_admin(actor: User) -> None:
        if actor.role not in {"teacher", "admin"}:
            raise AuthorizationError("仅教师/管理员可执行此操作")

    # ============ Course ============

    async def create_course(
        self, db: AsyncSession, *, actor: User, name: str, code: str
    ) -> Course:
        self._ensure_admin(actor)
        existing = await self.course_repo.get_by_code(db, code)
        if existing is not None:
            raise ConflictError(f"课程编号 {code} 已存在", field="code")
        course = await self.course_repo.create(db, name=name, code=code)
        log.info("course.created", course_id=course.id, code=code, actor_id=actor.id)
        return course

    async def archive_course(
        self, db: AsyncSession, *, actor: User, course_id: int
    ) -> Course:
        self._ensure_admin(actor)
        course = await self.course_repo.get(db, course_id)
        if course is None:
            raise ResourceNotFoundError(f"course {course_id} not found")
        await self.course_repo.update(db, course_id, is_archived=True)
        await db.refresh(course)
        log.info("course.archived", course_id=course_id, actor_id=actor.id)
        return course

    # ============ Class ============

    async def create_class(
        self,
        db: AsyncSession,
        *,
        actor: User,
        name: str,
        course_id: int,
        teacher_id: int | None = None,
    ) -> Class:
        self._ensure_teacher_or_admin(actor)
        course = await self.course_repo.get(db, course_id)
        if course is None:
            raise ResourceNotFoundError(f"course {course_id} not found")
        if course.is_archived:
            raise ConflictError(
                f"课程 {course_id} 已归档，无法新建班级", field="course_id"
            )

        # 教师只能给自己建班；管理员可指定 teacher_id
        owner = actor.id if actor.role == "teacher" else (teacher_id or actor.id)

        cls = await self.class_repo.create(
            db, name=name, course_id=course_id, teacher_id=owner
        )
        log.info(
            "class.created",
            class_id=cls.id,
            course_id=course_id,
            teacher_id=owner,
            actor_id=actor.id,
        )
        return cls

    async def archive_class(
        self, db: AsyncSession, *, actor: User, class_id: int
    ) -> Class:
        self._ensure_teacher_or_admin(actor)
        cls = await self.class_repo.get(db, class_id)
        if cls is None:
            raise ResourceNotFoundError(f"class {class_id} not found")
        if actor.role == "teacher" and cls.teacher_id != actor.id:
            raise AuthorizationError("无权操作他人班级")
        if cls.is_archived:
            raise ConflictError(f"class {class_id} 已归档", field="class_id")
        await self.class_repo.update(db, class_id, is_archived=True)
        await db.refresh(cls)
        log.info("class.archived", class_id=class_id, actor_id=actor.id)
        return cls

    # ============ Membership ============

    async def bulk_add_students(
        self,
        db: AsyncSession,
        *,
        actor: User,
        class_id: int,
        student_ids: list[int],
    ) -> BulkAddResult:
        """批量添加学生；不存在或非 student 角色的项目记入 failed，不抛异常."""
        self._ensure_teacher_or_admin(actor)
        cls = await self.class_repo.get(db, class_id)
        if cls is None:
            raise ResourceNotFoundError(f"class {class_id} not found")
        if actor.role == "teacher" and cls.teacher_id != actor.id:
            raise AuthorizationError("无权操作他人班级")

        added: list[int] = []
        failed: list[tuple[int, str]] = []

        for sid in student_ids:
            user = await self.user_repo.get(db, sid)
            if user is None:
                failed.append((sid, "用户不存在"))
                continue
            if user.role != "student":
                failed.append((sid, f"用户角色为 {user.role}，非学生"))
                continue
            if await self.membership_repo.is_student_in_class(
                db, student_id=sid, class_id=class_id
            ):
                failed.append((sid, "已在班级中"))
                continue
            db.add(ClassMembership(class_id=class_id, student_id=sid))
            added.append(sid)

        if added:
            cls.student_count = (cls.student_count or 0) + len(added)
        await db.flush()

        log.info(
            "class.bulk_add_students",
            class_id=class_id,
            added_count=len(added),
            failed_count=len(failed),
            actor_id=actor.id,
        )
        return BulkAddResult(added=added, failed=failed)


org_service = OrgService()
