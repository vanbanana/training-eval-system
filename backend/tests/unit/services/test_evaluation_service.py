"""Epic 16.4 - 16.7 验收：EvaluationService 自动评分/手动评分/批量审批/重算."""

from __future__ import annotations

import json
from collections.abc import AsyncIterator

import pytest
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from app.core.database import Base
from app.core.exceptions import AuthorizationError, BusinessRuleError
from app.llm.base import LLMResponse
from app.models.evaluation import Evaluation, EvaluationHistory
from app.models.upload import Upload
from app.services.evaluation_service import (
    EvaluationService,
    SubjectiveIncompleteError,
)
from tests.factories.task_factory import TrainingTaskFactory
from tests.factories.user_factory import AdminFactory, TeacherFactory, UserFactory
from tests.fakes.fake_llm import FakeLLM


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


def _good_score_payload(score: int = 85) -> str:
    rationale = (
        "代码风格规范，命名清晰且语义明确；"
        "存在少量函数缺乏类型注解，建议补全提升可维护性；"
        "整体覆盖度良好满足核心要求；"
        "测试用例覆盖完整无遗漏。"
    )
    return json.dumps({"score": score, "rationale": rationale}, ensure_ascii=False)


async def _create_upload(session: AsyncSession, *, with_dims: int = 2) -> tuple[
    Upload, "TrainingTask", "User"
]:
    teacher = await TeacherFactory.create_async(session)
    student = await UserFactory.create_async(session)
    task = await TrainingTaskFactory.create_async(
        session, teacher=teacher, with_dimensions=with_dims
    )
    upload = Upload(
        task_id=task.id,
        student_id=student.id,
        filename="r.docx",
        file_type="docx",
        file_size=10,
        storage_path="x",
        parse_status="parsed",
    )
    session.add(upload)
    await session.commit()
    return upload, task, teacher  # type: ignore[return-value]


class TestAutoScore:
    async def test_creates_evaluation_with_total(self, session: AsyncSession) -> None:
        """Given 2 维度任务 When auto_score Then 创建 Evaluation+2 DimensionScore+History."""
        upload, _, _ = await _create_upload(session, with_dims=2)
        fake = FakeLLM(default=LLMResponse(content=_good_score_payload(80)))

        svc = EvaluationService(llm=fake, alpha=0.6)
        ev = await svc.auto_score(session, upload_id=upload.id, parse_summary="m")
        await session.commit()

        assert ev.status == "auto_scored"
        assert ev.total_score == 80.0
        assert len(ev.scores) == 2
        # 历史 1 行
        from sqlalchemy import select

        hist = (
            await session.execute(
                select(EvaluationHistory).where(
                    EvaluationHistory.evaluation_id == ev.id
                )
            )
        ).scalars().all()
        assert len(list(hist)) == 1

    async def test_all_llm_failed_status_auto_failed(
        self, session: AsyncSession
    ) -> None:
        """Given LLM 全部失败 When auto_score Then status=auto_failed."""
        upload, _, _ = await _create_upload(session, with_dims=2)

        class AlwaysFail(FakeLLM):
            async def chat(self, *args, **kwargs):  # type: ignore[no-untyped-def]
                raise RuntimeError("boom")

        svc = EvaluationService(llm=AlwaysFail(), alpha=0.6)
        ev = await svc.auto_score(session, upload_id=upload.id)
        await session.commit()
        assert ev.status == "auto_failed"
        assert all(s.ai_score is None for s in ev.scores)


class TestUpdateDimensionSubjective:
    async def test_teacher_can_update(self, session: AsyncSession) -> None:
        """Given 任务教师 When 改主观分 Then total 更新+history."""
        upload, task, teacher = await _create_upload(session, with_dims=2)
        fake = FakeLLM(default=LLMResponse(content=_good_score_payload(60)))
        svc = EvaluationService(llm=fake, alpha=0.6)
        ev = await svc.auto_score(session, upload_id=upload.id)
        await session.commit()

        first_dim_id = ev.scores[0].dimension_id
        ev2 = await svc.update_dimension_subjective(
            session,
            evaluation_id=ev.id,
            dimension_id=first_dim_id,
            subj_score=100,
            comment="赞",
            operator=teacher,
        )
        await session.commit()

        updated = next(s for s in ev2.scores if s.dimension_id == first_dim_id)
        assert updated.teacher_score == 100
        assert ev2.status == "reviewed"
        # 历史 ≥ 2 行（auto_score + manual_score）
        from sqlalchemy import select

        hist = list(
            (
                await session.execute(
                    select(EvaluationHistory).where(
                        EvaluationHistory.evaluation_id == ev.id
                    )
                )
            ).scalars()
        )
        assert any(h.action == "manual_score" for h in hist)

    async def test_other_teacher_forbidden(
        self, session: AsyncSession
    ) -> None:
        """Given 教师 B（非任务教师）When 改 Then AuthorizationError."""
        upload, _, teacher_a = await _create_upload(session, with_dims=2)
        teacher_b = await TeacherFactory.create_async(session)
        fake = FakeLLM(default=LLMResponse(content=_good_score_payload()))
        svc = EvaluationService(llm=fake, alpha=0.6)
        ev = await svc.auto_score(session, upload_id=upload.id)
        await session.commit()

        with pytest.raises(AuthorizationError):
            await svc.update_dimension_subjective(
                session,
                evaluation_id=ev.id,
                dimension_id=ev.scores[0].dimension_id,
                subj_score=80,
                operator=teacher_b,
            )

    async def test_score_out_of_range_raises(
        self, session: AsyncSession
    ) -> None:
        upload, _, teacher = await _create_upload(session, with_dims=2)
        fake = FakeLLM(default=LLMResponse(content=_good_score_payload()))
        svc = EvaluationService(llm=fake, alpha=0.6)
        ev = await svc.auto_score(session, upload_id=upload.id)
        await session.commit()
        with pytest.raises(BusinessRuleError):
            await svc.update_dimension_subjective(
                session,
                evaluation_id=ev.id,
                dimension_id=ev.scores[0].dimension_id,
                subj_score=101,
                operator=teacher,
            )

    async def test_admin_can_update(self, session: AsyncSession) -> None:
        upload, _, _ = await _create_upload(session, with_dims=2)
        admin = await AdminFactory.create_async(session)
        fake = FakeLLM(default=LLMResponse(content=_good_score_payload()))
        svc = EvaluationService(llm=fake, alpha=0.6)
        ev = await svc.auto_score(session, upload_id=upload.id)
        await session.commit()
        ev2 = await svc.update_dimension_subjective(
            session,
            evaluation_id=ev.id,
            dimension_id=ev.scores[0].dimension_id,
            subj_score=70,
            operator=admin,
        )
        assert any(s.teacher_score == 70 for s in ev2.scores)


class TestBulkAction:
    async def test_confirm_requires_subjective_complete(
        self, session: AsyncSession
    ) -> None:
        """Given 部分维度未填主观 When confirm Then SubjectiveIncompleteError."""
        upload, _, teacher = await _create_upload(session, with_dims=2)
        fake = FakeLLM(default=LLMResponse(content=_good_score_payload()))
        svc = EvaluationService(llm=fake, alpha=0.6)
        ev = await svc.auto_score(session, upload_id=upload.id)
        await session.commit()
        with pytest.raises(SubjectiveIncompleteError):
            await svc.bulk_action(
                session,
                evaluation_ids=[ev.id],
                action="confirm",
                operator=teacher,
            )

    async def test_reject_without_reason_raises(
        self, session: AsyncSession
    ) -> None:
        upload, _, teacher = await _create_upload(session, with_dims=2)
        fake = FakeLLM(default=LLMResponse(content=_good_score_payload()))
        svc = EvaluationService(llm=fake, alpha=0.6)
        ev = await svc.auto_score(session, upload_id=upload.id)
        await session.commit()
        with pytest.raises(BusinessRuleError):
            await svc.bulk_action(
                session,
                evaluation_ids=[ev.id],
                action="reject",
                operator=teacher,
                reason="",
            )

    async def test_reject_with_reason(self, session: AsyncSession) -> None:
        upload, _, teacher = await _create_upload(session, with_dims=2)
        fake = FakeLLM(default=LLMResponse(content=_good_score_payload()))
        svc = EvaluationService(llm=fake, alpha=0.6)
        ev = await svc.auto_score(session, upload_id=upload.id)
        await session.commit()
        result = await svc.bulk_action(
            session,
            evaluation_ids=[ev.id],
            action="reject",
            operator=teacher,
            reason="逻辑严重缺失，请重新提交。",
        )
        assert result["affected"] == 1
        await session.refresh(ev)
        assert ev.status == "rejected"
        assert "逻辑" in ev.teacher_comment


class TestRecalcForTask:
    async def test_alpha_change_updates_total(
        self, session: AsyncSession
    ) -> None:
        """Given alpha 由 0.6 改 0.0 When recalc Then total 重算."""
        upload, task, teacher = await _create_upload(session, with_dims=2)
        fake = FakeLLM(default=LLMResponse(content=_good_score_payload(80)))
        svc = EvaluationService(llm=fake, alpha=0.6)
        ev = await svc.auto_score(session, upload_id=upload.id)
        # 设置 teacher_score 让 alpha 切换有差别
        for s in ev.scores:
            s.teacher_score = 60
        await session.commit()

        affected = await svc.recalc_for_task(session, task_id=task.id, alpha=0.0)
        await session.commit()
        await session.refresh(ev)
        assert affected == 1
        # alpha=0 → 仅看 subj=60
        assert ev.total_score == 60.0
