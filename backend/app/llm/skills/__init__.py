"""LLM Skills 框架."""

from app.llm.skills.base import Skill, SkillOutputError
from app.llm.skills.parse.document_structure import DocumentStructureSkill
from app.llm.skills.registry import SkillRegistry, registry
from app.llm.skills.verify.coverage_check import CoverageCheckSkill
from app.llm.skills.verify.logic_audit import LogicAuditSkill

__all__ = ["Skill", "SkillOutputError", "SkillRegistry", "registry"]


def _register_all() -> None:
    """注册所有内置 Skills."""
    registry.register(DocumentStructureSkill)
    registry.register(CoverageCheckSkill)
    registry.register(LogicAuditSkill)


_register_all()

