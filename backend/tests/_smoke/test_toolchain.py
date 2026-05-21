"""Task 0.3 验收：ruff / mypy / pytest 工具链已正确配置.

- pytest 自身能跑通（本测试存在即证明）
- pyproject.toml 含三大工具配置段
"""
from __future__ import annotations

from pathlib import Path
import sys


_ROOT = Path(__file__).resolve().parent.parent.parent  # backend/
_PYPROJECT = _ROOT / "pyproject.toml"


def _read_pyproject() -> str:
    return _PYPROJECT.read_text(encoding="utf-8")


def test_pytest_is_running() -> None:
    """Given pytest 已安装；When 执行任意测试；Then 测试本身被收集到。"""
    assert sys.version_info >= (3, 10)


def test_pyproject_has_ruff_config() -> None:
    """Given backend/pyproject.toml；When 读文件；Then 含 ruff line-length=100 与 target=py310。"""
    content = _read_pyproject()
    assert "[tool.ruff]" in content
    assert "line-length = 100" in content
    assert 'target-version = "py310"' in content


def test_pyproject_has_ruff_format_config() -> None:
    """Given backend/pyproject.toml；When 读文件；Then 含 ruff.format 双引号 + 空格缩进。"""
    content = _read_pyproject()
    assert "[tool.ruff.format]" in content
    assert 'quote-style = "double"' in content
    assert 'indent-style = "space"' in content


def test_pyproject_has_mypy_strict_config() -> None:
    """Given backend/pyproject.toml；When 读文件；Then mypy strict + pydantic 插件已启用。"""
    content = _read_pyproject()
    assert "[tool.mypy]" in content
    assert "strict = true" in content
    assert '"pydantic.mypy"' in content


def test_pyproject_has_pytest_config_with_markers() -> None:
    """Given backend/pyproject.toml；When 读文件；Then pytest 配置含 strict-markers 与全部约定 markers。"""
    content = _read_pyproject()
    assert "[tool.pytest.ini_options]" in content
    assert "strict-markers" in content
    for marker in ("unit:", "integration:", "contract:", "e2e:", "real_llm:", "slow:"):
        assert marker in content, f"marker {marker} missing"


def test_pyproject_has_coverage_threshold_70() -> None:
    """Given backend/pyproject.toml；When 读 [tool.coverage.report]；Then fail_under=70。"""
    content = _read_pyproject()
    assert "[tool.coverage.report]" in content
    assert "fail_under = 70" in content


def test_lint_script_exists() -> None:
    """Given backend/scripts/；When 列文件；Then lint_all.sh / lint_all.ps1 同时存在。"""
    assert (_ROOT / "scripts" / "lint_all.sh").exists()
    assert (_ROOT / "scripts" / "lint_all.ps1").exists()
