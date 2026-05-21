"""Task 2.2 验收：Alembic 配置 + 初始迁移."""

from __future__ import annotations

import subprocess
import sys
from pathlib import Path


_BACKEND = Path(__file__).resolve().parent.parent.parent.parent


def test_alembic_ini_exists() -> None:
    """Given backend/；When 列文件；Then alembic.ini 存在。"""
    assert (_BACKEND / "alembic.ini").exists()


def test_alembic_env_imports_app_models() -> None:
    """Given alembic/env.py；When 读源码；Then 导入 app.models 与 Base。"""
    env = (_BACKEND / "alembic" / "env.py").read_text(encoding="utf-8")
    assert "import app.models" in env
    assert "Base.metadata" in env


def test_alembic_env_uses_async_engine() -> None:
    env = (_BACKEND / "alembic" / "env.py").read_text(encoding="utf-8")
    assert "async_engine_from_config" in env


def test_alembic_env_uses_settings_db_url() -> None:
    env = (_BACKEND / "alembic" / "env.py").read_text(encoding="utf-8")
    assert "get_settings" in env
    assert "settings.db_url" in env


def test_alembic_env_uses_naming_convention() -> None:
    """Given env.py 引用 Base.metadata；When 检查 Base 定义；Then 自动带 NAMING_CONVENTION."""
    from app.core.database import Base, NAMING_CONVENTION

    assert Base.metadata.naming_convention == NAMING_CONVENTION


def test_alembic_versions_dir_exists() -> None:
    assert (_BACKEND / "alembic" / "versions").is_dir()


def test_alembic_history_command_runs() -> None:
    """Given alembic 已配置；When 运行 alembic history；Then 命令成功（即使为空）。"""
    result = subprocess.run(
        [sys.executable, "-m", "alembic", "history"],
        cwd=_BACKEND,
        capture_output=True,
        text=True,
        check=False,
    )
    # alembic history 即使无迁移也返回 0
    assert result.returncode == 0


def test_initial_migration_present() -> None:
    """Given versions 目录；When 列；Then 至少 1 个 .py 迁移（除 __init__）。"""
    versions = _BACKEND / "alembic" / "versions"
    py_files = [f for f in versions.glob("*.py") if not f.name.startswith("__")]
    assert len(py_files) >= 1
