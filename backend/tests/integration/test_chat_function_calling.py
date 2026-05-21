"""Epic 22.6 验收：Chat function-calling 集成流程（FakeLLM）."""

from __future__ import annotations

import pytest
from sqlalchemy.ext.asyncio import AsyncSession

from app.llm.base import LLMResponse
from app.models.chat import ChatMessage
from app.models.evaluation import Evaluation
from app.models.upload import Upload
from app.services.chat_service import ChatService
from tests.factories.task_factory import TrainingTaskFactory
from tests.factories.user_factory import UserFactory
from tests.fakes.fake_llm import FakeLLM


pytestmark = pytest.mark.integration


class TestChatIntegration:
    async def test_full_round_writes_messages(
        self, sqlite_session: AsyncSession
    ) -> None:
        student = await UserFactory.create_async(sqlite_session)
        task = await TrainingTaskFactory.create_async(sqlite_session)
        upload = Upload(
            task_id=task.id,
            student_id=student.id,
            filename="r",
            file_type="docx",
            file_size=10,
            storage_path="x",
            parse_status="parsed",
        )
        sqlite_session.add(upload)
        await sqlite_session.flush()
        ev = Evaluation(
            task_id=task.id,
            student_id=student.id,
            upload_id=upload.id,
            status="auto_scored",
            total_score=80.0,
        )
        sqlite_session.add(ev)
        await sqlite_session.commit()

        fake = FakeLLM(default=LLMResponse(content="代码风格不错，但测试覆盖率偏低，建议补全单元测试。"))
        svc = ChatService(llm=fake)
        sess = await svc.create_session(
            sqlite_session, user=student, evaluation_id=ev.id
        )
        await svc.post_user_message(
            sqlite_session,
            session_id=sess.id,
            content="为什么我代码质量分较低？",
            user=student,
        )
        await sqlite_session.commit()

        chunks: list[str] = []
        async for c in svc.answer_stream(
            sqlite_session, session_id=sess.id, user=student
        ):
            chunks.append(c)
        await sqlite_session.commit()

        assert chunks, "应有流式输出"
        from sqlalchemy import select

        msgs = list(
            (
                await sqlite_session.execute(
                    select(ChatMessage).where(
                        ChatMessage.session_id == sess.id
                    )
                )
            ).scalars()
        )
        roles = sorted(m.role for m in msgs)
        assert "user" in roles
        assert "assistant" in roles
