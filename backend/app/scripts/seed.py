"""开发用种子数据."""

from __future__ import annotations

import asyncio
from datetime import UTC, datetime, timedelta

from sqlalchemy import select

from app.core.database import Base, SessionLocal, engine
from app.core.security import hash_password
from app.models.course import Class, Course
from app.models.task import Dimension, TrainingTask
from app.models.user import User


async def seed() -> None:
    async with engine.begin() as conn:
        await conn.run_sync(Base.metadata.create_all)

    async with SessionLocal() as db:
        # Users
        users_data = [
            ("admin", "管理员", "admin", "Admin@123"),
            ("teacher01", "王伟", "teacher", "Teacher@123"),
            ("student01", "李同学", "student", "Student@123"),
            ("student02", "张文卓", "student", "Student@123"),
        ]
        for username, display_name, role, password in users_data:
            existing = (await db.execute(select(User).where(User.username == username))).scalar_one_or_none()
            if not existing:
                db.add(User(username=username, display_name=display_name, role=role, password_hash=hash_password(password), is_active=True))
        await db.flush()

        teacher = (await db.execute(select(User).where(User.username == "teacher01"))).scalar_one()

        # Course
        course = (await db.execute(select(Course).where(Course.code == "SE2026"))).scalar_one_or_none()
        if not course:
            course = Course(name="软件工程实践", code="SE2026")
            db.add(course)
            await db.flush()

        # Class
        cls = (await db.execute(select(Class).where(Class.name == "软工21-3班"))).scalar_one_or_none()
        if not cls:
            cls = Class(name="软工21-3班", course_id=course.id, teacher_id=teacher.id, student_count=56)
            db.add(cls)
            await db.flush()

        # Task
        task = (await db.execute(select(TrainingTask).where(TrainingTask.name == "第三次实训：并发编程"))).scalar_one_or_none()
        if not task:
            task = TrainingTask(
                name="第三次实训：并发编程",
                description="基于课程第三章并发与同步内容，使用 Java 实现生产者-消费者模型。",
                requirements="1. 源代码 zip\n2. 实验报告 PDF\n3. 测试用例",
                teacher_id=teacher.id,
                course_id=course.id,
                status="published",
                deadline=datetime.now(UTC) + timedelta(days=7),
            )
            db.add(task)
            await db.flush()

            dims = [
                Dimension(task_id=task.id, name="代码规范", weight=30, order_index=0),
                Dimension(task_id=task.id, name="功能实现", weight=35, order_index=1),
                Dimension(task_id=task.id, name="测试验证", weight=20, order_index=2),
                Dimension(task_id=task.id, name="文档规范", weight=15, order_index=3),
            ]
            db.add_all(dims)

        await db.commit()
    print("seed done: users + course + class + task")


if __name__ == "__main__":
    asyncio.run(seed())
