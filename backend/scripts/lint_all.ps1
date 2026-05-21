#!/usr/bin/env pwsh
# Windows 版 CI 入口
$ErrorActionPreference = "Stop"

Set-Location (Split-Path -Parent $PSScriptRoot)

Write-Host "==> ruff check" -ForegroundColor Cyan
python -m ruff check app
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "==> ruff format check" -ForegroundColor Cyan
python -m ruff format --check app
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "==> mypy strict" -ForegroundColor Cyan
python -m mypy app
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "==> pytest with coverage" -ForegroundColor Cyan
python -m pytest --cov=app --cov-report=term-missing --cov-fail-under=70
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "All checks passed" -ForegroundColor Green
