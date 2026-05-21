"""Epic 15.4 验收：核查结果 API."""

from __future__ import annotations

import pytest
from httpx import AsyncClient
from sqlalchemy.ext.asyncio import AsyncSession

from app.core.security import create_access_token
from app.models.upload import Upload, VerifyResult
from tests.factories.task_factory import TrainingTaskFactory
from tests.factories.user_factory import TeacherFactory, UserFactory


pytestmark = [pytest.mark.contract]


async def _bearer(user_id: int, role: str) -> dict[str, str]:
    token = create_access_token(user_id=user_id, role=role)
    return {"Authorization": f"Bearer {token}"}


class TestGetVerifyResult:
    async def test_owner_student_can_read(
        self, http_client: AsyncClient, sqlite_session: AsyncSession
    ) -> None:
        """Given upload + verify_result + 学生本人 When GET Then 200."""
        from app.api.deps import get_db
        from app.main import app

        async def _override():
            yield sqlite_session

        app.dependency_overrides[get_db] = _override
        try:
            student = await UserFactory.create_async(sqlite_session)
            task = await TrainingTaskFactory.create_async(sqlite_session)
            upload = Upload(
                task_id=task.id,
                student_id=student.id,
                filename="r.docx",
                file_type="docx",
                file_size=10,
                storage_path="x",
                parse_status="parsed",
            )
            sqlite_session.add(upload)
            await sqlite_session.flush()
            sqlite_session.add(
                VerifyResult(
                    upload_id=upload.id,
                    match_rate=80.0,
                    checkpoints=[{"text": "r1", "matched": True}],
                    missing_items=[],
                    logic_issues=[],
                    overall_confidence=85,
                )
            )
            await sqlite_session.commit()

            r = await http_client.get(
                f"/api/uploads/{upload.id}/verify-result",
                headers=await _bearer(student.id, "student"),
            )
            assert r.status_code == 200
            body = r.json()
            assert body["match_rate"] == 80.0
            assert body["overall_confidence"] == 85
        finally:
            app.dependency_overrides.pop(get_db, None)

    async def test_other_student_forbidden(
        self, http_client: AsyncClient, sqlite_session: AsyncSession
    ) -> None:
        """Given 其他学生 When GET 别人的 verify-result Then 403."""
        from app.api.deps import get_db
        from app.main import app

        async def _override():
            yield sqlite_session

        app.dependency_overrides[get_db] = _override
        try:
            owner = await UserFactory.create_async(sqlite_session)
            other = await UserFactory.create_async(sqlite_session)
            task = await TrainingTaskFactory.create_async(sqlite_session)
            upload = Upload(
                task_id=task.id,
                student_id=owner.id,
                filename="r",
                file_type="docx",
                file_size=10,
                storage_path="x",
                parse_status="parsed",
            )
            sqlite_session.add(upload)
            await sqlite_session.flush()
            sqlite_session.add(
                VerifyResult(
                    upload_id=upload.id,
                    match_rate=10,
                    checkpoints=[],
                    missing_items=[],
                    logic_issues=[],
                    overall_confidence=10,
                )
            )
            await sqlite_session.commit()

            r = await http_client.get(
                f"/api/uploads/{upload.id}/verify-result",
                headers=await _bearer(other.id, "student"),
            )
            assert r.status_code == 403
        finally:
            app.dependency_overrides.pop(get_db, None)

    async def test_not_ready_returns_404(
        self, http_client: AsyncClient, sqlite_session: AsyncSession
    ) -> None:
        """Given upload 未核查 When GET verify-result Then 404."""
        from app.api.deps import get_db
        from app.main import app

        async def _override():
            yield sqlite_session

        app.dependency_overrides[get_db] = _override
        try:
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
            await sqlite_session.commit()

            r = await http_client.get(
                f"/api/uploads/{upload.id}/verify-result",
                headers=await _bearer(student.id, "student"),
            )
            assert r.status_code == 404
        finally:
            app.dependency_overrides.pop(get_db, None)

    async def test_teacher_can_read_others(
        self, http_client: AsyncClient, sqlite_session: AsyncSession
    ) -> None:
        """Given 教师 When 读他人 upload Then 200."""
        from app.api.deps import get_db
        from app.main import app

        async def _override():
            yield sqlite_session

        app.dependency_overrides[get_db] = _override
        try:
            teacher = await TeacherFactory.create_async(sqlite_session)
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
            sqlite_session.add(
                VerifyResult(
                    upload_id=upload.id,
                    match_rate=70,
                    checkpoints=[],
                    missing_items=[],
                    logic_issues=[],
                    overall_confidence=70,
                )
            )
            await sqlite_session.commit()

            r = await http_client.get(
                f"/api/uploads/{upload.id}/verify-result",
                headers=await _bearer(teacher.id, "teacher"),
            )
            assert r.status_code == 200
        finally:
            app.dependency_overrides.pop(get_db, None)
