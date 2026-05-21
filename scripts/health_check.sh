#!/usr/bin/env bash
# 健康探测：验证 PostgreSQL 与 Redis 连通性 + pgvector 扩展
set -euo pipefail

echo "==> PostgreSQL"
docker exec tes-postgres-dev pg_isready -U tes -d tes_dev
docker exec tes-postgres-dev psql -U tes -d tes_dev \
  -c "SELECT extversion FROM pg_extension WHERE extname='vector'" \
  | tee /tmp/pgvector.out

if ! grep -E "0\.[5-9]|[1-9]\." /tmp/pgvector.out > /dev/null; then
  echo "✗ pgvector 版本不足 0.5.0"
  exit 1
fi

echo "==> Redis"
docker exec tes-redis-dev redis-cli ping

echo "✓ 所有依赖健康"
