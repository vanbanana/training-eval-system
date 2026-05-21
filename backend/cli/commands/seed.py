"""tes-cli seed - 27.2."""

from __future__ import annotations

import asyncio

import typer


def seed(scale: str = typer.Option("small", help="small|medium|large")) -> None:
    """注入种子数据."""
    from app.core.database import SessionLocal
    from tests.factories.org_factory import ClassFactory, CourseFactory
    from tests.factories.user_factory import TeacherFactory, UserFactory

    sizes = {
        "small": (2, 4, 20),
        "medium": (5, 15, 100),
        "large": (10, 30, 500),
    }
    if scale not in sizes:
        typer.secho(f"unknown scale {scale}", fg=typer.colors.RED)
        raise typer.Exit(code=2)
    courses, classes, students = sizes[scale]

    async def _run() -> None:
        async with SessionLocal() as db:
            t = await TeacherFactory.create_async(
                db, username=f"seed_t_{scale}"
            )
            for _ in range(courses):
                await CourseFactory.create_async(db)
            for _ in range(classes):
                await ClassFactory.create_async(db, teacher=t)
            for _ in range(students):
                await UserFactory.create_async(db)
            await db.commit()

    asyncio.run(_run())
    typer.echo(
        f"seeded scale={scale} courses={courses} classes={classes} students={students}"
    )
