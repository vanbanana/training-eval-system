#!/usr/bin/env bash
# 停止本地开发依赖；--volumes 选项可清理数据卷
set -euo pipefail
cd "$(dirname "$0")/.."

if [[ "${1:-}" == "--volumes" ]]; then
  docker compose -f docker-compose.dev.yml down --volumes
  rm -rf ./data/pg ./data/redis
  echo "✓ 已停止并清理数据卷"
else
  docker compose -f docker-compose.dev.yml down
  echo "✓ 已停止（数据卷保留，下次启动数据可恢复）"
fi
