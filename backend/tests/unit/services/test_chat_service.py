"""Epic 22.1/22.4 验收：ChatService."""

from __future__ import annotations

from collections.abc import AsyncIterator

import pytest
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from app.core.database import Base
from app.core.exceptions import (
    AuthorizationError,
    BusinessRuleError,
    RateLimitedError,
)
from app.models.chat import ChatMessage
from app.models.evaluation import Evaluation
from app.models.upload import Upload
from app.services.chat_service import (
    ChatService,
    SessionLimitExceededError,
)
from tests.factories.task_factory import TrainingTaskFactory
from tests.factories.user_factory import UserFactory


pytestmark = pytest.mark.unit


@pytest.fixture()
async def session() -> AsyncIterator[AsyncSession]:
    engine = create_async_engine("sqlite+aiosqlite:///:memory:")
    async with engine.begin() as conn:
        await conn.execute(text("PRAGMA foreign_keys = ON"))
        await conn.run_sync(Base.metadata.create_all)
    SessionLocal = async_sessionmaker(engine, expire_on_commit=False, class_=AsyncSession)
    async with SessionLocal() as s:
        yield s
    await engine.dispose()


async def _seed_eval(session: AsyncSession):
    student = await UserFactory.create_async(session)
    task = await TrainingTaskFactory.create_async(session)
    upload = Upload(
        task_id=task.id,
        student_id=student.id,
        filename="r",
        file_type="docx",
        file_size=10,
        storage_path="x",
        parse_status="parsed",
    )
    session.add(upload)
    await session.flush()
    ev = Evaluation(
        task_id=task.id,
        student_id=student.id,
        upload_id=upload.id,
        status="auto_scored",
        total_score=80.0,
    )
    session.add(ev)
    await session.commit()
    return student, ev


class TestCreateSession:
    async def test_self_can_create(self, session: AsyncSession) -> None:
        student, ev = await _seed_eval(session)
        svc = ChatService()
        sess = await svc.create_session(
            session, user=student, evaluation_id=ev.id, title="t"
        )
        await session.commit()
        assert sess.id is not None
        assert sess.evaluation_id == ev.id

    async def test_other_student_forbidden(
        self, session: AsyncSession
    ) -> None:
        student, ev = await _seed_eval(session)
        other = await UserFactory.create_async(session)
        await session.commit()
        svc = ChatService()
        with pytest.raises(AuthorizationError):
            await svc.create_session(
                session, user=other, evaluation_id=ev.id
            )


class TestPostUserMessage:
    async def test_too_long_rejected(self, session: AsyncSession) -> None:
        student, ev = await _seed_eval(session)
        svc = ChatService()
        sess = await svc.create_session(
            session, user=student, evaluation_id=ev.id
        )
        await session.commit()
        with pytest.raises(BusinessRuleError):
            await svc.post_user_message(
                session,
                session_id=sess.id,
                content="x" * 600,
                user=student,
            )

    async def test_quota_exhausted_rate_limited(
        self, session: AsyncSession
    ) -> None:
        student, ev = await _seed_eval(session)
        svc = ChatService(daily_quota=2)
        sess = await svc.create_session(
            session, user=student, evaluation_id=ev.id
        )
        await session.commit()
        await svc.post_user_message(
            session, session_id=sess.id, content="q1", user=student
        )
        await svc.post_user_message(
            session, session_id=sess.id, content="q2", user=student
        )
        with pytest.raises(RateLimitedError):
            await svc.post_user_message(
                session, session_id=sess.id, content="q3", user=student
            )

    async def test_session_user_count_limit(
        self, session: AsyncSession
    ) -> None:
        student, ev = await _seed_eval(session)
        svc = ChatService(daily_quota=999)
        sess = await svc.create_session(
            session, user=student, evaluation_id=ev.id
        )
        await session.commit()
        for i in range(20):
            await svc.post_user_message(
                session, session_id=sess.id, content=f"q{i}", user=student
            )
        with pytest.raises(SessionLimitExceededError):
            await svc.post_user_message(
                session, session_id=sess.id, content="q-overflow", user=student
            )


class TestSoftDelete:
    async def test_soft_delete_hides_from_list(
        self, session: AsyncSession
    ) -> None:
        student, ev = await _seed_eval(session)
        svc = ChatService()
        sess = await svc.create_session(
            session, user=student, evaluation_id=ev.id
        )
        await session.commit()
        await svc.soft_delete(session, session_id=sess.id, user=student)
        await session.commit()
        items = await svc.list_sessions(session, user=student)
        assert all(s.id != sess.id for s in items)


class TestAnswerStream:
    async def test_dev_mode_writes_assistant_message(
        self, session: AsyncSession
    ) -> None:
        student, ev = await _seed_eval(session)
        svc = ChatService()
        sess = await svc.create_session(
            session, user=student, evaluation_id=ev.id
        )
        await svc.post_user_message(
            session, session_id=sess.id, content="为什么我代码质量低？", user=student
        )
        await session.commit()
        out: list[str] = []
        async for chunk in svc.answer_stream(
            session, session_id=sess.id, user=student
        ):
            out.append(chunk)
        await session.commit()

        assert out
        from sqlalchemy import select

        msgs = list(
            (
                await session.execute(
                    select(ChatMessage).where(
                        ChatMessage.session_id == sess.id,
                        ChatMessage.role == "assistant",
                    )
                )
            ).scalars()
        )
        assert msgs
