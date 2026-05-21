"""Task 8.8: 文件清理任务（孤儿文件 + 老旧 failed upload）."""

from __future__ import annotations

from datetime import UTC, datetime, timedelta
from typing import TYPE_CHECKING

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from app.core.logging import get_logger
from app.models.upload import Upload

if TYPE_CHECKING:
    from app.storage.base import FileStorage


log = get_logger(__name__)


_OLD_FAILED_DAYS = 90


async def find_orphan_paths(
    db: AsyncSession,
    *,
    known_paths: set[str],
) -> set[str]:
    """从 storage 列出来的所有 paths 中，找出 DB 中没有引用的（孤儿）.

    实现策略：调用方提供 storage 中的所有 paths，本函数比对 DB。
    """
    db_paths = {
        row[0]
        for row in (
            await db.execute(select(Upload.storage_path).where(Upload.is_deleted == 0))
        ).all()
    }
    return known_paths - db_paths


async def cleanup_old_failed_uploads(
    db: AsyncSession,
    *,
    storage: FileStorage,
    now: datetime | None = None,
) -> int:
    """删除 status=failed 且超过 90 天未更新的 upload + 对应 storage 文件；返回删除数量."""
    cutoff = (now or datetime.now(UTC)) - timedelta(days=_OLD_FAILED_DAYS)
    stmt = select(Upload).where(
        Upload.parse_status == "failed",
        Upload.updated_at < cutoff,
        Upload.is_deleted == 0,
    )
    rows = list((await db.execute(stmt)).scalars().all())
    deleted = 0
    for upload in rows:
        # 删 storage（同 path 多记录的话由调用方保证已 dedup）
        try:
            await storage.delete(upload.storage_path)
        except Exception as e:
            log.warning(
                "cleanup.storage_delete_failed",
                upload_id=upload.id,
                error=str(e),
            )
        upload.is_deleted = 1
        deleted += 1
    await db.flush()
    log.info("cleanup.old_failed.batch", count=deleted)
    return deleted
