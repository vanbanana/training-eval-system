"""Epic 12 验收：Function Calling 工具框架."""

from __future__ import annotations

import pytest
from pydantic import BaseModel, Field

from app.llm.tools import (
    Tool,
    ToolExecutionContext,
    ToolResult,
    tool_registry,
)
from app.llm.tools.base import ToolExecutionError, ToolPermissionError


pytestmark = pytest.mark.unit


class _SearchInput(BaseModel):
    query: str = Field(..., min_length=1)
    limit: int = Field(default=10, ge=1, le=100)


class _SearchTool(Tool[_SearchInput]):
    name = "search.kb"
    description = "搜索知识库"
    input_schema = _SearchInput
    allowed_roles = {"teacher", "admin"}

    async def execute(
        self, args: _SearchInput, ctx: ToolExecutionContext
    ) -> ToolResult:
        return ToolResult(
            success=True,
            data={"query": args.query, "results": [f"hit-{i}" for i in range(args.limit)]},
        )


class _FailTool(Tool[_SearchInput]):
    name = "fail.tool"
    description = "Always fails"
    input_schema = _SearchInput

    async def execute(
        self, args: _SearchInput, ctx: ToolExecutionContext
    ) -> ToolResult:
        raise RuntimeError("intentional failure")


class TestToolValidation:
    def test_subclass_must_set_name(self) -> None:
        with pytest.raises(ValueError, match="name"):

            class _NoName(Tool[_SearchInput]):
                name = ""
                description = "x"
                input_schema = _SearchInput

                async def execute(
                    self, args: _SearchInput, ctx: ToolExecutionContext
                ) -> ToolResult:
                    return ToolResult(success=True)

    def test_subclass_must_set_description(self) -> None:
        with pytest.raises(ValueError, match="description"):

            class _NoDesc(Tool[_SearchInput]):
                name = "x"
                description = ""
                input_schema = _SearchInput

                async def execute(
                    self, args: _SearchInput, ctx: ToolExecutionContext
                ) -> ToolResult:
                    return ToolResult(success=True)


class TestPermission:
    async def test_role_allowed_passes(self) -> None:
        tool = _SearchTool()
        ctx = ToolExecutionContext(actor_id=1, actor_role="teacher")
        result = await tool({"query": "hi"}, ctx)
        assert result.success is True

    async def test_role_denied_raises(self) -> None:
        tool = _SearchTool()
        ctx = ToolExecutionContext(actor_id=1, actor_role="student")
        with pytest.raises(ToolPermissionError):
            await tool({"query": "hi"}, ctx)


class TestSchemaValidation:
    async def test_invalid_args_raises(self) -> None:
        tool = _SearchTool()
        ctx = ToolExecutionContext(actor_id=1, actor_role="admin")
        with pytest.raises(ToolExecutionError):
            await tool({"limit": 10}, ctx)  # query missing

    async def test_extra_keys_ignored(self) -> None:
        """Pydantic 默认忽略额外字段."""
        tool = _SearchTool()
        ctx = ToolExecutionContext(actor_id=1, actor_role="admin")
        result = await tool({"query": "x", "ignored": "value"}, ctx)
        assert result.success


class TestExecutionFailure:
    async def test_runtime_error_returns_failure_result(self) -> None:
        tool = _FailTool()
        ctx = ToolExecutionContext(actor_id=1, actor_role="admin")
        result = await tool({"query": "x"}, ctx)
        assert result.success is False
        assert "intentional failure" in (result.error or "")


class TestOpenAISchema:
    def test_schema_has_required_fields(self) -> None:
        tool = _SearchTool()
        schema = tool.to_openai_schema()
        assert schema["type"] == "function"
        assert schema["function"]["name"] == "search.kb"
        assert "parameters" in schema["function"]
        # 包含 query 字段
        params = schema["function"]["parameters"]
        assert "query" in params["properties"]


class TestRegistry:
    def test_register_and_get(self) -> None:
        tool_registry.clear()
        tool_registry.register(_SearchTool)
        instance = tool_registry.get("search.kb")
        assert isinstance(instance, _SearchTool)

    def test_list_names_sorted(self) -> None:
        tool_registry.clear()
        tool_registry.register(_FailTool)
        tool_registry.register(_SearchTool)
        names = tool_registry.list_names()
        assert names == ["fail.tool", "search.kb"]

    def test_to_openai_schemas_for_subset(self) -> None:
        tool_registry.clear()
        tool_registry.register(_SearchTool)
        tool_registry.register(_FailTool)
        only_search = tool_registry.to_openai_schemas(names=["search.kb"])
        assert len(only_search) == 1
        assert only_search[0]["function"]["name"] == "search.kb"


class TestContext:
    async def test_context_passed_to_execute(self) -> None:
        captured: dict[str, object] = {}

        class _CtxTool(Tool[_SearchInput]):
            name = "ctx.test"
            description = "Test ctx"
            input_schema = _SearchInput

            async def execute(
                self, args: _SearchInput, ctx: ToolExecutionContext
            ) -> ToolResult:
                captured["actor_id"] = ctx.actor_id
                captured["task_id"] = ctx.task_id
                return ToolResult(success=True)

        tool = _CtxTool()
        ctx = ToolExecutionContext(
            actor_id=42, actor_role="admin", task_id=99
        )
        await tool({"query": "x"}, ctx)
        assert captured["actor_id"] == 42
        assert captured["task_id"] == 99
