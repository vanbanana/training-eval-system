"""tes-cli audit archive - 27.5."""

from __future__ import annotations

import asyncio
from datetime import datetime
from pathlib import Path

import typer


def archive(
    before: str = typer.Option(..., "--before", help="YYYY-MM-DD"),
    output_dir: str = typer.Option(
        "data/backups/audit", "--output-dir"
    ),
) -> None:
    from app.cli.commands.audit_archive import archive_before

    target = datetime.strptime(before, "%Y-%m-%d")
    out = asyncio.run(archive_before(target, Path(output_dir)))
    typer.echo(out)
