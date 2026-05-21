"""Task 0.5 验收：Docker 开发环境 compose 文件正确配置.

注：执行 `docker compose up` 的端到端测试归 CI/集成层，本测试仅验证：
1. compose 文件存在且 YAML 合法
2. 含 postgres / redis / adminer 三服务
3. postgres 镜像为 pgvector
4. 卷映射、端口映射符合 spec
5. 启停脚本存在
"""

from __future__ import annotations

from pathlib import Path

import pytest


_REPO = Path(__file__).resolve().parent.parent.parent.parent


def _load_yaml() -> dict[str, object]:
    yaml = pytest.importorskip("yaml")  # PyYAML 可能不在 dev deps
    return yaml.safe_load((_REPO / "docker-compose.dev.yml").read_text(encoding="utf-8"))


def test_compose_file_exists() -> None:
    """Given 仓库根；When 列文件；Then docker-compose.dev.yml 存在。"""
    assert (_REPO / "docker-compose.dev.yml").exists()


def test_compose_has_three_services() -> None:
    """Given compose 文件；When 解析 YAML；Then services 含 postgres、redis、adminer。"""
    data = _load_yaml()
    services = data["services"]
    assert "postgres" in services
    assert "redis" in services
    assert "adminer" in services


def test_postgres_uses_pgvector_image() -> None:
    """Given postgres 服务定义；When 检查 image；Then 使用 pgvector/pgvector:pg14。"""
    data = _load_yaml()
    img = data["services"]["postgres"]["image"]
    assert "pgvector" in img
    assert "pg14" in img


def test_postgres_has_pgvector_init_script() -> None:
    """Given scripts/postgres-init.sql；When 读；Then 含 CREATE EXTENSION vector。"""
    init = _REPO / "scripts" / "postgres-init.sql"
    assert init.exists()
    content = init.read_text(encoding="utf-8")
    assert "CREATE EXTENSION" in content
    assert "vector" in content


def test_compose_exposes_correct_ports() -> None:
    """Given compose；When 检查 ports；Then 5432/6379/8080 全部映射。"""
    data = _load_yaml()
    pg_ports = data["services"]["postgres"]["ports"]
    redis_ports = data["services"]["redis"]["ports"]
    adminer_ports = data["services"]["adminer"]["ports"]
    assert "5432:5432" in pg_ports
    assert "6379:6379" in redis_ports
    assert "8080:8080" in adminer_ports


def test_compose_persists_data_volumes() -> None:
    """Given compose；When 检查 volumes；Then ./data/pg 与 ./data/redis 映射存在。"""
    data = _load_yaml()
    pg_volumes = data["services"]["postgres"].get("volumes", [])
    redis_volumes = data["services"]["redis"].get("volumes", [])
    assert any("./data/pg" in v for v in pg_volumes)
    assert any("./data/redis" in v for v in redis_volumes)


def test_health_checks_defined() -> None:
    """Given compose；When 检查 healthcheck；Then postgres 与 redis 都有探针。"""
    data = _load_yaml()
    assert "healthcheck" in data["services"]["postgres"]
    assert "healthcheck" in data["services"]["redis"]


def test_dev_lifecycle_scripts_exist() -> None:
    """Given 仓库 scripts；When 列文件；Then dev_up.sh / dev_down.sh / health_check.sh 全部存在。"""
    assert (_REPO / "scripts" / "dev_up.sh").exists()
    assert (_REPO / "scripts" / "dev_down.sh").exists()
    assert (_REPO / "scripts" / "health_check.sh").exists()


def test_dev_down_supports_volumes_flag() -> None:
    """Given dev_down.sh；When 读；Then 支持 --volumes 选项清理数据。"""
    content = (_REPO / "scripts" / "dev_down.sh").read_text(encoding="utf-8")
    assert "--volumes" in content
