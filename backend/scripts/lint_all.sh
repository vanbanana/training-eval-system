#!/usr/bin/env bash
# CI 入口：依次运行 ruff、mypy、pytest，任一失败整体失败
set -euo pipefail

cd "$(dirname "$0")/.."

echo "==> ruff check"
python -m ruff check app

echo "==> ruff format check"
python -m ruff format --check app

echo "==> mypy strict"
python -m mypy app

echo "==> pytest with coverage"
python -m pytest --cov=app --cov-report=term-missing --cov-fail-under=70

echo "✓ all checks passed"
