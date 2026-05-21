"""Task 0.8 验收：测试 Factory + conftest 骨架."""

from __future__ import annotations

import subprocess
import sys
from pathlib import Path


_BACKEND = Path(__file__).resolve().parent.parent.parent


def test_factory_module_imports() -> None:
    """Given tests/factories；When import；Then 不抛异常并提供 faker。"""
    from tests import factories

    assert factories.faker is not None
    # 中文姓名生成可用
    name = factories.faker.name()
    assert isinstance(name, str)
    assert len(name) >= 2


def test_conftest_provides_event_loop_fixture() -> None:
    """Given tests/conftest.py；When 读源码；Then 注册 event_loop fixture。"""
    src = (_BACKEND / "tests" / "conftest.py").read_text(encoding="utf-8")
    assert "event_loop" in src
    assert "AsyncClient" in src
    assert "http_client" in src


def test_pytest_markers_registered() -> None:
    """Given pyproject.toml；When 运行 pytest --markers；Then 输出含全部 6 个 markers。"""
    result = subprocess.run(
        [sys.executable, "-m", "pytest", "--markers"],
        cwd=_BACKEND,
        capture_output=True,
        text=True,
        check=True,
    )
    output = result.stdout
    for marker in ("@pytest.mark.unit", "@pytest.mark.integration", "@pytest.mark.contract",
                   "@pytest.mark.e2e", "@pytest.mark.real_llm", "@pytest.mark.slow"):
        assert marker in output, f"marker {marker} 未注册"


def test_strict_markers_rejects_unknown() -> None:
    """Given pytest --strict-markers；When 测试用未注册 marker；Then 拒绝运行。"""
    bogus = _BACKEND / "tests" / "_smoke" / "_bogus_test.py"
    bogus.write_text(
        '''
import pytest

@pytest.mark.totally_invented_marker
def test_x() -> None:
    pass
''',
        encoding="utf-8",
    )
    try:
        result = subprocess.run(
            [sys.executable, "-m", "pytest", "--strict-markers", str(bogus)],
            cwd=_BACKEND,
            capture_output=True,
            text=True,
            check=False,
        )
        assert result.returncode != 0
        # 错误消息可能落在 stdout 或 stderr
        combined = result.stdout + result.stderr
        assert "totally_invented_marker" in combined or "not registered" in combined.lower()
    finally:
        bogus.unlink(missing_ok=True)


def test_dev_dependencies_installed() -> None:
    """Given pyproject 标记 factory-boy / Faker / freezegun 为 dev；When import；Then 可导入。"""
    import factory  # noqa: F401
    import faker  # noqa: F401
    import freezegun  # noqa: F401
