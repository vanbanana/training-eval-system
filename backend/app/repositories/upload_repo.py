"""UploadRepository."""

from __future__ import annotations

from sqlalchemy import func, select
from sqlalchemy.ext.asyncio import AsyncSession

from app.core.exceptions import ConflictError
from app.models.upload import Upload
from app.repositories.base import BaseRepository

# 合法状态流转
_VALID_TRANSITIONS: dict[str, set[str]] = {
    "pending": {"parsing", "failed"},
    "parsing": {"parsed", "failed"},
    "parsed": {"failed"},  # parsed 不应回退；仅可在重解析时被上游服务重置 status=pending
    "failed": {"pending"},  # 允许重试
}


class UploadRepository(BaseRepository[Upload]):
    model = Upload

    async def list_by_task_student(
        self, db: AsyncSession, *, task_id: int, student_id: int
    ) -> list[Upload]:
        stmt = (
            select(Upload)
            .where(
                Upload.task_id == task_id,
                Upload.student_id == student_id,
                Upload.is_deleted == 0,
            )
            .order_by(Upload.version.desc())
        )
        return list((await db.execute(stmt)).scalars().all())

    async def count_by_task_student(
        self, db: AsyncSession, *, task_id: int, student_id: int
    ) -> int:
        stmt = (
            select(func.count(Upload.id))
            .where(
                Upload.task_id == task_id,
                Upload.student_id == student_id,
                Upload.is_deleted == 0,
            )
        )
        return int((await db.execute(stmt)).scalar_one())

    async def find_by_sha256(
        self,
        db: AsyncSession,
        *,
        student_id: int,
        sha256: str,
        task_id: int | None = None,
    ) -> Upload | None:
        """按 sha256 查找该学生已有的同内容上传（用于查重 / 文件去重）."""
        if not sha256:
            return None
        stmt = select(Upload).where(
            Upload.student_id == student_id,
            Upload.sha256 == sha256,
            Upload.is_deleted == 0,
        )
        if task_id is not None:
            stmt = stmt.where(Upload.task_id == task_id)
        return (await db.execute(stmt)).scalar_one_or_none()

    async def update_status(
        self, db: AsyncSession, upload_id: int, new_status: str
    ) -> Upload:
        upload = await self.get(db, upload_id)
        if upload is None:
            from app.core.exceptions import ResourceNotFoundError

            raise ResourceNotFoundError(f"upload {upload_id} not found")
        if new_status not in _VALID_TRANSITIONS.get(upload.parse_status, set()):
            raise ConflictError(
                f"非法状态流转 {upload.parse_status} -> {new_status}",
                field="parse_status",
            )
        upload.parse_status = new_status
        await db.flush()
        await db.refresh(upload)
        return upload

    async def soft_delete(self, db: AsyncSession, upload_id: int) -> int:
        return await self.update(db, upload_id, is_deleted=1)
