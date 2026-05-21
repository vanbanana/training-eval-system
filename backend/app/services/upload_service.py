"""UploadService - 学生上传成果的全流程编排.

流程：
1. 校验 task=published 且 deadline > now
2. 校验学生属于任务关联班级（Property 13）
3. 校验文件大小 1KB ~ N MB
4. 校验扩展名与 magic 一致
5. 写入 storage（原子）
6. 同 sha256 已存在 → 复用 file_path（去重）
7. 单 task 上传 ≤ 20
8. 创建 Upload 记录 status=pending
"""

from __future__ import annotations

import uuid
from dataclasses import dataclass
from datetime import UTC, datetime
from typing import TYPE_CHECKING, Protocol

from sqlalchemy.ext.asyncio import AsyncSession

from app.core.config import get_settings
from app.core.exceptions import (
    AuthorizationError,
    BusinessRuleError,
    ResourceNotFoundError,
    TaskClosedForSubmissionError,
    UploadLimitExceededError,
    UploadTooLargeError,
    UploadTooSmallError,
)
from app.core.logging import get_logger
from app.models.task import TrainingTask
from app.models.upload import Upload
from app.models.user import User
from app.repositories.task_repo import TaskRepository
from app.repositories.upload_repo import UploadRepository
from app.services.permissions import is_student_in_class
from app.utils.magic_check import assert_extension_matches

if TYPE_CHECKING:
    pass


log = get_logger(__name__)


_MIN_UPLOAD_SIZE_BYTES = 1024  # 1KB
_MAX_UPLOAD_COUNT_PER_TASK = 20


class _StorageProto(Protocol):
    async def save(self, path: str, data: bytes) -> str: ...
    async def exists(self, path: str) -> bool: ...
    async def delete(self, path: str) -> bool: ...


@dataclass(slots=True)
class UploadInput:
    filename: str
    content: bytes


class UploadService:
    def __init__(
        self,
        storage: _StorageProto,
        upload_repo: UploadRepository | None = None,
        task_repo: TaskRepository | None = None,
    ) -> None:
        self.storage = storage
        self.upload_repo = upload_repo or UploadRepository()
        self.task_repo = task_repo or TaskRepository()

    async def create_upload(
        self,
        db: AsyncSession,
        *,
        student: User,
        task_id: int,
        file: UploadInput,
    ) -> Upload:
        task = await self.task_repo.get(db, task_id)
        if task is None:
            raise ResourceNotFoundError(f"task {task_id} not found")

        # 1. 任务状态
        if task.status != "published":
            raise TaskClosedForSubmissionError(
                f"任务状态 {task.status}，不可提交", field="task_id"
            )
        # 2. 截止时间
        if task.deadline is not None:
            now = datetime.now(UTC)
            deadline = task.deadline
            if deadline.tzinfo is None:
                deadline = deadline.replace(tzinfo=UTC)
            if deadline <= now:
                raise TaskClosedForSubmissionError(
                    "已过截止时间，不可提交", field="deadline"
                )

        # 3. 学生班级归属（Property 13）
        if not await self._student_can_submit(db, student=student, task=task):
            raise AuthorizationError("学生不在任务关联班级", field="task_id")

        # 4. 文件大小
        size = len(file.content)
        settings = get_settings()
        max_bytes = settings.max_upload_size_mb * 1024 * 1024
        if size < _MIN_UPLOAD_SIZE_BYTES:
            raise UploadTooSmallError(
                f"文件 < {_MIN_UPLOAD_SIZE_BYTES} 字节", field="file"
            )
        if size > max_bytes:
            raise UploadTooLargeError(
                f"文件 > {settings.max_upload_size_mb} MB", field="file"
            )

        # 5. 扩展名 / magic 校验
        head = file.content[: 4 * 1024]
        ext = assert_extension_matches(file.filename, head)

        # 6. 上传次数限制
        existing_count = await self.upload_repo.count_by_task_student(
            db, task_id=task_id, student_id=student.id
        )
        if existing_count >= _MAX_UPLOAD_COUNT_PER_TASK:
            raise UploadLimitExceededError(
                f"单任务最多 {_MAX_UPLOAD_COUNT_PER_TASK} 次提交",
                field="task_id",
            )

        # 7. 同 sha256 复用：先计算
        import hashlib

        sha = hashlib.sha256(file.content).hexdigest()
        duplicate = await self.upload_repo.find_by_sha256(
            db, student_id=student.id, sha256=sha, task_id=task_id
        )

        if duplicate is not None:
            # 复用已有 file_path，仅新增一条记录指向同一 storage（version+1）
            storage_path = duplicate.storage_path
        else:
            # 8. 写入 storage
            storage_path = (
                f"task_{task_id}/student_{student.id}/{uuid.uuid4().hex}.{ext}"
            )
            digest = await self.storage.save(storage_path, file.content)
            assert digest == sha, "storage sha 与计算 sha 不一致"

        upload = Upload(
            task_id=task_id,
            student_id=student.id,
            filename=file.filename,
            file_type=ext,
            file_size=size,
            storage_path=storage_path,
            sha256=sha,
            parse_status="pending",
            version=existing_count + 1,
        )
        db.add(upload)
        await db.flush()
        await db.refresh(upload)
        log.info(
            "upload.created",
            upload_id=upload.id,
            task_id=task_id,
            student_id=student.id,
            size=size,
            duplicate=duplicate is not None,
        )
        return upload

    async def delete_upload(
        self, db: AsyncSession, *, actor: User, upload_id: int
    ) -> None:
        """学生本人在截止前可删；删 storage + soft delete DB."""
        upload = await self.upload_repo.get(db, upload_id)
        if upload is None or upload.is_deleted:
            raise ResourceNotFoundError(f"upload {upload_id} not found")
        if actor.role != "admin" and upload.student_id != actor.id:
            raise AuthorizationError("仅可删除自己的提交")

        task = await self.task_repo.get(db, upload.task_id)
        if task and task.deadline is not None:
            deadline = task.deadline
            if deadline.tzinfo is None:
                deadline = deadline.replace(tzinfo=UTC)
            if deadline <= datetime.now(UTC):
                raise BusinessRuleError("已过截止时间，不可删除", field="deadline")

        # 检查是否还有其他 upload 共用此 storage_path（去重场景）
        from sqlalchemy import select

        same_path_count = (
            await db.execute(
                select(Upload).where(
                    Upload.storage_path == upload.storage_path,
                    Upload.id != upload.id,
                    Upload.is_deleted == 0,
                )
            )
        ).scalars().all()
        if not same_path_count:
            await self.storage.delete(upload.storage_path)

        await self.upload_repo.soft_delete(db, upload_id)
        log.info("upload.deleted", upload_id=upload_id, actor_id=actor.id)

    async def reparse(
        self, db: AsyncSession, *, actor: User, upload_id: int
    ) -> Upload:
        if actor.role not in {"teacher", "admin"}:
            raise AuthorizationError("仅教师可重新解析")
        upload = await self.upload_repo.get(db, upload_id)
        if upload is None or upload.is_deleted:
            raise ResourceNotFoundError(f"upload {upload_id} not found")
        # 直接重置状态（不走严格状态机）
        upload.parse_status = "pending"
        await db.flush()
        await db.refresh(upload)
        log.info("upload.reparse", upload_id=upload_id, actor_id=actor.id)
        return upload

    @staticmethod
    async def _student_can_submit(
        db: AsyncSession, *, student: User, task: TrainingTask
    ) -> bool:
        """学生是否在 task 关联的任一班级中（Property 13）."""
        if not task.classes:
            return False
        for cls in task.classes:
            if await is_student_in_class(
                db, student_id=student.id, class_id=cls.id
            ):
                return True
        return False
