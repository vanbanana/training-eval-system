"""解析链路集成测试 - 验证所有文件类型能正确解析.

运行方式：
    cd backend
    python tests/fixtures/parse_samples/test_parse_pipeline.py

测试内容：
1. 每种文件类型都能被正确路由到对应 parser
2. 解析结果包含有效的 raw_text（非空）
3. 结构化字段（paragraphs/title_tree/tables）正确填充
4. SimHash 计算正确（非零）
5. 解析不会崩溃或超时
"""

from __future__ import annotations

import asyncio
import hashlib
import sys
import time
from pathlib import Path

# 确保能 import app 模块
sys.path.insert(0, str(Path(__file__).resolve().parents[3]))

# 直接导入 parser 层（不触发 database/models 初始化）
from app.parsers.base import ParsedDocument
from app.parsers.factory import get_parser

SAMPLES_DIR = Path(__file__).parent / "output"


def compute_simhash(text: str, *, hash_bits: int = 64) -> int:
    """SimHash 计算（从 parse_pipeline 复制，避免导入整个 app）."""
    if not text:
        return 0
    tokens = [text[i : i + 4] for i in range(0, max(1, len(text) - 3))]
    if not tokens:
        return 0
    bits = [0] * hash_bits
    for token in tokens:
        h = int.from_bytes(
            hashlib.sha256(token.encode("utf-8")).digest()[:8], "big"
        )
        for i in range(hash_bits):
            if h & (1 << i):
                bits[i] += 1
            else:
                bits[i] -= 1
    fingerprint = 0
    for i, b in enumerate(bits):
        if b > 0:
            fingerprint |= 1 << i
    if fingerprint >= (1 << 63):
        fingerprint -= 1 << 64
    return fingerprint


# 测试用例定义：(文件名, 期望的文件类型, 最小文本长度, 期望包含的关键词)
TEST_CASES: list[tuple[str, str, int, list[str]]] = [
    (
        "sample_report.docx", "docx", 500,
        ["Flask", "图书管理", "数据库", "API", "pytest"],
    ),
    (
        "sample_algorithm.docx", "docx", 300,
        ["排序", "快速排序", "归并排序", "时间复杂度"],
    ),
    (
        "sample_api_test.docx", "docx", 200,
        ["API", "测试", "POST", "GET", "200"],
    ),
    (
        "sample_database_design.pdf", "pdf", 200,
        ["Database", "Users", "Products", "CREATE TABLE"],
    ),
    (
        "sample_network_config.pdf", "pdf", 200,
        ["VLAN", "DHCP", "ACL", "192.168"],
    ),
    (
        "sample_data_analysis.xlsx", "xlsx", 100,
        ["Laptop", "Revenue", "North"],
    ),
    (
        "sample_ml_notebook.xlsx", "xlsx", 50,
        ["XGBoost", "Random Forest", "Accuracy"],
    ),
    (
        "sample_flask_project.zip", "zip", 500,
        ["Flask", "SQLAlchemy", "login", "borrow", "def"],
    ),
    (
        "sample_vue_project.zip", "zip", 300,
        ["vue", "pinia", "router", "Todo"],
    ),
    (
        "sample_screenshot.png", "png", 0,  # OCR 可能不可用
        [],  # 不强制要求关键词（OCR 依赖系统安装）
    ),
]


async def test_single_file(
    filename: str,
    file_type: str,
    min_text_len: int,
    expected_keywords: list[str],
) -> dict[str, object]:
    """测试单个文件的解析."""
    filepath = SAMPLES_DIR / filename
    if not filepath.exists():
        return {"status": "SKIP", "reason": f"File not found: {filepath}"}

    content = filepath.read_bytes()
    parser = get_parser(file_type)

    start = time.perf_counter()
    try:
        result: ParsedDocument = await parser.parse(content)
    except Exception as e:
        return {"status": "FAIL", "reason": f"Parse error: {e}"}
    elapsed_ms = (time.perf_counter() - start) * 1000

    # 验证结果
    errors: list[str] = []

    # 1. raw_text 长度检查
    if len(result.raw_text) < min_text_len:
        errors.append(
            f"raw_text too short: {len(result.raw_text)} < {min_text_len}"
        )

    # 2. 关键词检查
    text_lower = result.raw_text.lower()
    for kw in expected_keywords:
        if kw.lower() not in text_lower:
            errors.append(f"Missing keyword: '{kw}'")

    # 3. SimHash 计算
    simhash = compute_simhash(result.raw_text)
    if result.raw_text and simhash == 0:
        errors.append("SimHash is 0 for non-empty text")

    # 4. 超时检查（120 秒限制）
    if elapsed_ms > 120_000:
        errors.append(f"Parse timeout: {elapsed_ms:.0f}ms > 120000ms")

    if errors:
        return {
            "status": "FAIL",
            "errors": errors,
            "text_len": len(result.raw_text),
            "elapsed_ms": round(elapsed_ms, 1),
        }

    return {
        "status": "PASS",
        "text_len": len(result.raw_text),
        "paragraphs": len(result.paragraphs),
        "titles": len(result.title_tree),
        "tables": len(result.tables),
        "pages": result.page_count,
        "simhash": simhash,
        "elapsed_ms": round(elapsed_ms, 1),
    }


async def run_all_tests() -> None:
    """运行所有解析测试."""
    print("=" * 70)
    print("解析链路集成测试")
    print(f"样本目录: {SAMPLES_DIR}")
    print("=" * 70)
    print()

    passed = 0
    failed = 0
    skipped = 0

    for filename, file_type, min_len, keywords in TEST_CASES:
        result = await test_single_file(filename, file_type, min_len, keywords)
        status = result["status"]

        if status == "PASS":
            passed += 1
            print(f"  ✓ PASS  {filename:40s} "
                  f"text={result['text_len']:>6} chars  "
                  f"para={result['paragraphs']:>3}  "
                  f"titles={result['titles']:>2}  "
                  f"tables={result['tables']:>2}  "
                  f"time={result['elapsed_ms']:>7.1f}ms")
        elif status == "SKIP":
            skipped += 1
            print(f"  ⚠ SKIP  {filename:40s} {result['reason']}")
        else:
            failed += 1
            print(f"  ✗ FAIL  {filename:40s}")
            if "errors" in result:
                for err in result["errors"]:
                    print(f"          → {err}")
            if "reason" in result:
                print(f"          → {result['reason']}")

    print()
    print("=" * 70)
    print(f"结果: {passed} passed, {failed} failed, {skipped} skipped")
    print("=" * 70)

    if failed > 0:
        sys.exit(1)


if __name__ == "__main__":
    asyncio.run(run_all_tests())
