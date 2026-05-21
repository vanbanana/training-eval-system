"""ToolRegistry - 注册中心."""

from __future__ import annotations

from app.llm.tools.base import Tool


class ToolRegistry:
    def __init__(self) -> None:
        self._tools: dict[str, type[Tool]] = {}

    def register(self, tool_cls: type[Tool]) -> None:
        if not tool_cls.name:
            raise ValueError("tool 缺少 name")
        self._tools[tool_cls.name] = tool_cls

    def get(self, name: str) -> Tool:
        if name not in self._tools:
            raise KeyError(f"tool {name} 未注册")
        return self._tools[name]()  # type: ignore[abstract]

    def list_names(self) -> list[str]:
        return sorted(self._tools.keys())

    def to_openai_schemas(
        self, *, names: list[str] | None = None
    ) -> list[dict[str, object]]:
        """生成给 LLM 的 tools 数组."""
        target = names or list(self._tools.keys())
        return [self.get(n).to_openai_schema() for n in target if n in self._tools]

    def clear(self) -> None:
        self._tools.clear()


tool_registry = ToolRegistry()
