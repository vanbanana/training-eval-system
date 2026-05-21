"""Epic 22.2 验收：ChatTools 7 个工具."""

from __future__ import annotations

import pytest

from app.llm.tools.base import ToolExecutionContext, ToolPermissionError
from app.llm.tools.chat_tools import (
    CHAT_TOOLS,
    GetClassStatisticsTool,
    GetDimensionDetailTool,
    GetParseSegmentTool,
)


pytestmark = pytest.mark.unit


def _ctx(role: str = "student", evaluation_id: int | None = 1) -> ToolExecutionContext:
    return ToolExecutionContext(
        actor_id=10, actor_role=role, evaluation_id=evaluation_id
    )


class TestChatTools:
    def test_seven_tools_registered(self) -> None:
        assert len(CHAT_TOOLS) == 7

    async def test_class_statistics_no_pii(self) -> None:
        tool = GetClassStatisticsTool()
        result = await tool({"task_id": 1}, _ctx())
        assert result.success
        data = result.data or {}
        # 不能包含 user_id / username
        assert "user_id" not in data  # type: ignore[operator]
        assert "username" not in data  # type: ignore[operator]

    async def test_dimension_detail_other_eval_denied(self) -> None:
        tool = GetDimensionDetailTool()
        with pytest.raises(ToolPermissionError):
            await tool(
                {"evaluation_id": 999, "dimension_id": 1},
                _ctx(evaluation_id=1),
            )

    async def test_parse_segment_requires_evaluation(self) -> None:
        tool = GetParseSegmentTool()
        with pytest.raises(ToolPermissionError):
            await tool({"upload_id": 1, "keyword": ""}, _ctx(evaluation_id=None))

    async def test_teacher_role_denied(self) -> None:
        tool = GetClassStatisticsTool()
        with pytest.raises(ToolPermissionError):
            await tool({"task_id": 1}, _ctx(role="teacher"))


class TestStudentQaSkill:
    def test_metadata(self) -> None:
        from app.llm.skills.chat import StudentQaSkill

        s = StudentQaSkill()
        assert s.name == "chat.student_qa"

    def test_render_includes_context(self) -> None:
        from app.llm.skills.chat import StudentQaInput, StudentQaSkill

        s = StudentQaSkill()
        msgs = s.render_prompt(
            StudentQaInput(
                student_name="张三",
                task_name="任务A",
                user_question="为什么我得了 80 分？",
            )
        )
        joined = "\n".join(m.content for m in msgs)
        assert "张三" in joined
        assert "任务A" in joined
        assert "80 分" in joined
