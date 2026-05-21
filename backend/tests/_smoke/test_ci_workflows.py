"""Task 0.7 验收：GitHub Actions CI 工作流配置."""

from __future__ import annotations

from pathlib import Path

import pytest


_REPO = Path(__file__).resolve().parent.parent.parent.parent
_WORKFLOWS = _REPO / ".github" / "workflows"


def _load(name: str) -> dict[str, object]:
    yaml = pytest.importorskip("yaml")
    return yaml.safe_load((_WORKFLOWS / name).read_text(encoding="utf-8"))


def test_backend_workflow_exists() -> None:
    """Given .github/workflows/；When 列；Then ci.yml 存在。"""
    assert (_WORKFLOWS / "ci.yml").exists()


def test_frontend_workflow_exists() -> None:
    """Given .github/workflows/；When 列；Then frontend.yml 存在。"""
    assert (_WORKFLOWS / "frontend.yml").exists()


def test_backend_ci_runs_full_pipeline() -> None:
    """Given ci.yml；When 解析；Then 含 ruff / mypy / pytest 步骤。"""
    data = _load("ci.yml")
    jobs = data["jobs"]
    assert "lint-test" in jobs
    steps = jobs["lint-test"]["steps"]
    step_names = [s.get("name", "") for s in steps]
    joined = " ".join(step_names).lower()
    assert "ruff" in joined
    assert "mypy" in joined
    assert "pytest" in joined


def test_backend_ci_uses_python_310_matrix() -> None:
    """Given ci.yml；When 检查 matrix；Then 含 3.10。"""
    data = _load("ci.yml")
    matrix = data["jobs"]["lint-test"]["strategy"]["matrix"]
    versions = [str(v) for v in matrix["python-version"]]
    assert "3.10" in versions


def test_backend_ci_caches_pip() -> None:
    """Given ci.yml；When 检查 setup-python 步骤；Then 启用 cache: pip。"""
    data = _load("ci.yml")
    steps = data["jobs"]["lint-test"]["steps"]
    setup = next(s for s in steps if "setup-python" in s.get("uses", ""))
    assert setup["with"]["cache"] == "pip"


def test_frontend_ci_uses_pnpm_with_cache() -> None:
    """Given frontend.yml；When 检查；Then 用 pnpm + 缓存 store。"""
    data = _load("frontend.yml")
    steps = data["jobs"]["build"]["steps"]
    joined = " ".join(s.get("uses", "") for s in steps)
    assert "pnpm/action-setup" in joined
    assert "actions/cache" in joined


def test_frontend_ci_runs_build() -> None:
    """Given frontend.yml；When 检查；Then 含 pnpm build 步骤。"""
    data = _load("frontend.yml")
    steps = data["jobs"]["build"]["steps"]
    has_build = any("pnpm build" in s.get("run", "") for s in steps)
    assert has_build


def test_workflows_trigger_on_pr() -> None:
    """Given workflow；When 检查 on；Then 至少 pull_request 与 push 触发。"""
    for name in ("ci.yml", "frontend.yml"):
        data = _load(name)
        # YAML 'on' 可能被 PyYAML 解析为 True 关键字
        triggers = data.get("on") or data.get(True)
        assert triggers is not None
        assert "pull_request" in triggers
        assert "push" in triggers
