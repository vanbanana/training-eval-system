"""SkillRegistry - 注册中心，按 name + version 查询."""

from __future__ import annotations

from app.llm.skills.base import Skill


class SkillRegistry:
    def __init__(self) -> None:
        # key: f"{name}@{version}" or name (latest)
        self._skills: dict[str, type[Skill]] = {}
        # name → 最新 version
        self._latest: dict[str, str] = {}

    def register(self, skill_cls: type[Skill]) -> None:
        if not skill_cls.name or not skill_cls.version:
            raise ValueError("skill 缺少 name/version")
        key = f"{skill_cls.name}@{skill_cls.version}"
        self._skills[key] = skill_cls
        # 简单按字符串比较保留最新版本
        cur = self._latest.get(skill_cls.name)
        if cur is None or skill_cls.version > cur:
            self._latest[skill_cls.name] = skill_cls.version

    def get(self, name: str, *, version: str | None = None) -> Skill:
        if version is None:
            version = self._latest.get(name)
            if version is None:
                raise KeyError(f"skill {name} 未注册")
        key = f"{name}@{version}"
        if key not in self._skills:
            raise KeyError(f"skill {key} 未注册")
        return self._skills[key]()

    def list_names(self) -> list[str]:
        return sorted(self._latest.keys())

    def clear(self) -> None:
        self._skills.clear()
        self._latest.clear()


registry = SkillRegistry()
