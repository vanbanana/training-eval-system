"""Task 0.4 验收：Git 仓库与提交规范配置."""

from __future__ import annotations

from pathlib import Path


_REPO = Path(__file__).resolve().parent.parent.parent.parent  # repo root


def _read(p: Path) -> str:
    return p.read_text(encoding="utf-8") if p.exists() else ""


def test_gitignore_exists_and_covers_critical_paths() -> None:
    """Given 仓库根目录；When 读 .gitignore；Then 含 .env / __pycache__ / node_modules / dist。"""
    gi = _REPO / ".gitignore"
    assert gi.exists(), "仓库根 .gitignore 必须存在"
    content = gi.read_text(encoding="utf-8")
    for entry in ("__pycache__", ".env", "node_modules", "dist", ".venv", ".pytest_cache"):
        assert entry in content, f".gitignore 缺少条目：{entry}"


def test_gitignore_does_not_exclude_specs() -> None:
    """Given .gitignore；When 检查；Then 不会忽略设计文档（tasks.md / requirements.md / design.md）。"""
    gi = _REPO / ".gitignore"
    content = gi.read_text(encoding="utf-8")
    # 这些不能在 .gitignore 中独立出现
    for forbidden in ("tasks.md", "requirements.md", "design.md", "/docs/"):
        # 允许以注释形式提到，禁止真正忽略
        for line in content.splitlines():
            stripped = line.strip()
            if stripped.startswith("#") or not stripped:
                continue
            assert stripped != forbidden, f".gitignore 不应忽略 {forbidden}"


def test_gitattributes_enforces_lf() -> None:
    """Given .gitattributes；When 读；Then 含 'text=auto eol=lf' 强制 LF 行尾。"""
    ga = _REPO / ".gitattributes"
    assert ga.exists(), ".gitattributes 必须存在"
    content = ga.read_text(encoding="utf-8")
    assert "text=auto eol=lf" in content


def test_pre_commit_config_exists_with_required_hooks() -> None:
    """Given .pre-commit-config.yaml；When 读；Then 含 ruff / mypy / detect-secrets / conventional-pre-commit。"""
    pc = _REPO / ".pre-commit-config.yaml"
    assert pc.exists(), ".pre-commit-config.yaml 必须存在"
    content = pc.read_text(encoding="utf-8")
    for hook in ("ruff", "mypy", "detect-secrets", "conventional-pre-commit"):
        assert hook in content, f"pre-commit 缺少 {hook} 钩子"


def test_env_example_exists_and_env_ignored() -> None:
    """Given .env.example 与 .gitignore；When 读；Then example 入库、.env 被忽略。"""
    assert (_REPO / ".env.example").exists()
    gi_content = (_REPO / ".gitignore").read_text(encoding="utf-8")
    assert ".env" in gi_content
    # .env.example 不应被忽略
    assert ".env.example" not in [
        line.strip() for line in gi_content.splitlines() if not line.strip().startswith("#")
    ]
