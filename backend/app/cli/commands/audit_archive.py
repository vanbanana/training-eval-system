"""审计日志归档 CLI - Epic 23.6.

用法（开发环境）：
    python -m app.cli.commands.audit_archive --before 2024-12-31

实际生产中应使用 DBA 高权限连接，可绕过 audit_logs 的 append-only 触发器。
"""

from __future__ import annotations

import argparse
import asyncio
import csv
import gzip
from datetime import datetime
from pathlib import Path
from typing import Any

from sqlalchemy import delete, select

from app.core.database import SessionLocal
from app.core.logging import get_logger
from app.models.audit import AuditLog


log = get_logger(__name__)


async def archive_before(before: datetime, output_dir: Path) -> dict[str, Any]:
    output_dir.mkdir(parents=True, exist_ok=True)
    out_path = (
        output_dir
        / f"audit_logs_before_{before.strftime('%Y%m%d')}.csv.gz"
    )
    async with SessionLocal() as db:
        rows = list(
            (
                await db.execute(
                    select(AuditLog).where(AuditLog.occurred_at < before)
                )
            )
            .scalars()
            .all()
        )
        if not rows:
            return {"archived": 0, "path": str(out_path)}

        with gzip.open(out_path, "wt", encoding="utf-8", newline="") as gz:
            writer = csv.writer(gz)
            writer.writerow(
                [
                    "id",
                    "occurred_at",
                    "user_id",
                    "username",
                    "role",
                    "action",
                    "target_type",
                    "target_id",
                    "result",
                    "client_ip",
                ]
            )
            for r in rows:
                writer.writerow(
                    [
                        r.id,
                        r.occurred_at.isoformat(),
                        r.user_id or "",
                        r.username,
                        r.role,
                        r.action,
                        r.target_type,
                        r.target_id,
                        r.result,
                        r.client_ip,
                    ]
                )

        # 用 DBA 权限删除（dev 测试模式直接删；生产必须切换连接角色）
        try:
            await db.execute(
                delete(AuditLog).where(AuditLog.occurred_at < before)
            )
            await db.commit()
        except Exception as e:  # noqa: BLE001
            await db.rollback()
            return {
                "archived": len(rows),
                "path": str(out_path),
                "delete_failed": str(e),
            }
    return {"archived": len(rows), "path": str(out_path)}


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--before", required=True, help="YYYY-MM-DD")
    parser.add_argument(
        "--output-dir",
        default="data/backups/audit",
        help="archive 输出目录",
    )
    args = parser.parse_args()
    before = datetime.strptime(args.before, "%Y-%m-%d")
    out = asyncio.run(archive_before(before, Path(args.output_dir)))
    log.info("audit_archive.done", **out)
    print(out)


if __name__ == "__main__":
    main()
