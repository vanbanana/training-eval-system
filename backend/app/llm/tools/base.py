"""Tool 抽象基类 - LLM 可调用的工具."""

from __future__ import annotations

from abc import ABC, abstractmethod
from typing import ClassVar, Generic, TypeVar

from pydantic import BaseModel, Field, ValidationError

from app.core.exceptions import BusinessRuleError
from app.core.logging import get_logger
from app.models.user import User


log = get_logger(__name__)


class ToolExecutionError(BusinessRuleError):
    error_code = "TOOL_EXECUTION_FAILED"


class ToolPermissionError(BusinessRuleError):
    error_code = "TOOL_PERMISSION_DENIED"
    http_status = 403


class ToolExecutionContext(BaseModel):
    """工具执行上下文：附带调用者、关联实体 ID、时间戳."""

    model_config = {"arbitrary_types_allowed": True}

    actor_id: int
    actor_role: str
    evaluation_id: int | None = None
    task_id: int | None = None
    student_id: int | None = None
    extra: dict[str, object] = Field(default_factory=dict)
    db: object | None = Field(default=None, exclude=True)  # AsyncSession，不序列化


class ToolResult(BaseModel):
    success: bool
    data: object | None = None
    error: str | None = None


InputT = TypeVar("InputT", bound=BaseModel)


class Tool(ABC, Generic[InputT]):
    """工具抽象基类.

    每个 Tool 必须设置：name / description / input_schema / allowed_roles.
    """

    name: ClassVar[str] = ""
    description: ClassVar[str] = ""
    input_schema: ClassVar[type[BaseModel]]
    allowed_roles: ClassVar[set[str]] = set()  # 空集合 = 所有角色都可调用

    def __init_subclass__(cls, **kwargs: object) -> None:
        super().__init_subclass__(**kwargs)
        if getattr(cls, "__abstractmethods__", None):
            return
        if not cls.name:
            raise ValueError(f"{cls.__name__}.name 必须设置")
        if not cls.description:
            raise ValueError(f"{cls.__name__}.description 必须设置")

    @abstractmethod
    async def execute(
        self, args: InputT, ctx: ToolExecutionContext
    ) -> ToolResult:
        """子类实现：在权限校验后执行工具逻辑."""

    def to_openai_schema(self) -> dict[str, object]:
        """生成 OpenAI Function Calling JSON schema."""
        return {
            "type": "function",
            "function": {
                "name": self.name,
                "description": self.description,
                "parameters": self.input_schema.model_json_schema(),
            },
        }

    async def __call__(
        self, raw_args: dict[str, object], ctx: ToolExecutionContext
    ) -> ToolResult:
        """对 LLM 提供的入口：自动校验权限 + schema + 异常封装."""
        # 权限校验
        if self.allowed_roles and ctx.actor_role not in self.allowed_roles:
            raise ToolPermissionError(
                f"角色 {ctx.actor_role} 无权调用工具 {self.name}",
                field="actor_role",
            )
        # schema 校验
        try:
            args = self.input_schema(**raw_args)  # type: ignore[arg-type]
        except ValidationError as e:
            raise ToolExecutionError(
                f"工具 {self.name} 入参校验失败: {e}", field="args"
            ) from e

        try:
            result = await self.execute(args, ctx)  # type: ignore[arg-type]
        except (ToolPermissionError, ToolExecutionError):
            raise
        except Exception as e:  # noqa: BLE001
            log.exception(
                "tool.execute.failed", tool=self.name, error=str(e)
            )
            return ToolResult(success=False, error=str(e))

        return result


def is_actor_admin(user: User) -> bool:
    """便捷工具：调用者是否为管理员."""
    return user.role == "admin"
