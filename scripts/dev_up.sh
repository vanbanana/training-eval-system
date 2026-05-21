#!/usr/bin/env bash
# 启动本地开发依赖（PostgreSQL + Redis + Adminer）
set -euo pipefail
cd "$(dirname "$0")/.."

docker compose -f docker-compose.dev.yml up -d
echo "✓ 已启动 dev 容器"
echo "  - PostgreSQL: localhost:5432 (user=tes, db=tes_dev)"
echo "  - Redis:      localhost:6379"
echo "  - Adminer UI: http://localhost:8080"
