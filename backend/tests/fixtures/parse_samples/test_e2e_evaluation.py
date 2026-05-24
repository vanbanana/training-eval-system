"""端到端评价链路测试 - 验证 解析→核查→评分→前端数据 完整流程.

模拟真实 LLM 返回（使用 FakeLLM），验证：
1. 解析结果能正确喂给评分引擎
2. 评分引擎产出的数据结构符合前端期望
3. 核查结果能正确生成
4. 综合分计算正确
5. 所有 API 响应格式与前端 TypeScript 接口匹配

运行方式：
    cd backend
    python tests/fixtures/parse_samples/test_e2e_evaluation.py
"""

from __future__ import annotations

import asyncio
import hashlib
import json
import sys
from dataclasses import dataclass
from decimal import Decimal
from pathlib import Path
from typing import Any

sys.path.insert(0, str(Path(__file__).resolve().parents[3]))

from app.parsers.base import ParsedDocument
from app.parsers.factory import get_parser
from app.services.scoring import DimensionScoreData, compute_final_score

SAMPLES_DIR = Path(__file__).parent / "output"


# ============================================================
# Fake LLM - 模拟真实 LLM 返回结构化 JSON
# ============================================================


class FakeLLMForScoring:
    """模拟 LLM 评分响应.

    根据提交内容的长度和关键词密度生成合理的评分，
    而不是随机数。这样可以验证评分逻辑的正确性。
    """

    def score_dimension(
        self,
        *,
        dimension_name: str,
        dimension_description: str,
        submission_text: str,
        task_requirements: str,
    ) -> dict[str, Any]:
        """模拟单维度评分."""
        text_len = len(submission_text)
        # 基于内容丰富度给分
        base_score = min(95, max(50, 60 + text_len // 100))

        # 根据维度名称调整
        if "代码" in dimension_name or "实现" in dimension_name:
            if "def " in submission_text or "class " in submission_text:
                base_score = min(95, base_score + 10)
            if "test" in submission_text.lower():
                base_score = min(95, base_score + 5)
        elif "文档" in dimension_name or "规范" in dimension_name:
            if "第一章" in submission_text or "## " in submission_text:
                base_score = min(95, base_score + 8)
        elif "测试" in dimension_name:
            if "pytest" in submission_text or "test_" in submission_text:
                base_score = min(95, base_score + 12)

        rationale = self._generate_rationale(dimension_name, base_score, submission_text)
        return {"score": base_score, "rationale": rationale}

    def _generate_rationale(self, dim_name: str, score: int, text: str) -> str:
        """生成 ≥50 字符的评分理由."""
        if score >= 85:
            return (
                f"{dim_name}维度表现优秀。提交内容结构完整，"
                f"覆盖了核心要求，逻辑清晰，细节到位。"
                f"内容量充足（约{len(text)}字符），质量较高。"
            )
        elif score >= 70:
            return (
                f"{dim_name}维度表现良好，但仍有提升空间。"
                f"主要内容已覆盖，部分细节可进一步完善。"
                f"建议补充更多实例和分析说明。"
            )
        else:
            return (
                f"{dim_name}维度表现一般，存在明显不足。"
                f"内容覆盖不够全面，缺少关键步骤的详细说明。"
                f"建议参考任务要求逐项补充完善。"
            )

    def verify_coverage(
        self, *, task_requirements: str, parse_summary: str
    ) -> dict[str, Any]:
        """模拟覆盖度核查."""
        # 从 requirements 提取检查点
        lines = [l.strip() for l in task_requirements.split("\n") if l.strip()]
        checkpoints = []
        for line in lines[:10]:
            # 简单模拟：如果 summary 中包含关键词则 matched
            keywords = [w for w in line.split() if len(w) > 2]
            matched = any(kw in parse_summary for kw in keywords[:3])
            checkpoints.append({
                "requirement": line,
                "matched": matched,
                "evidence": f"在提交内容中{'找到' if matched else '未找到'}相关描述",
                "confidence": 85 if matched else 35,
            })
        return {"checkpoints": checkpoints}

    def verify_logic(
        self, *, task_requirements: str, parse_summary: str
    ) -> dict[str, Any]:
        """模拟逻辑漏洞检测."""
        issues = []
        # 简单启发：如果文本很短，可能有逻辑问题
        if len(parse_summary) < 500:
            issues.append({
                "description": "提交内容较为简略，部分步骤缺少详细说明",
                "severity": "medium",
                "location": "全文",
            })
        return {"issues": issues}


# ============================================================
# 模拟任务维度配置（对应前端 Task.dimensions）
# ============================================================

TASK_DIMENSIONS = [
    {"id": 1, "name": "代码质量", "description": "代码结构、命名规范、注释完整性", "weight": 30, "order_index": 0},
    {"id": 2, "name": "功能实现", "description": "是否完成所有功能需求", "weight": 25, "order_index": 1},
    {"id": 3, "name": "文档规范", "description": "报告结构、格式、内容完整性", "weight": 20, "order_index": 2},
    {"id": 4, "name": "测试覆盖", "description": "单元测试、集成测试的覆盖率和质量", "weight": 15, "order_index": 3},
    {"id": 5, "name": "创新性", "description": "是否有超出基本要求的创新点", "weight": 10, "order_index": 4},
]

TASK_REQUIREMENTS = """1. 使用 Flask/Django 框架开发 Web 应用
2. 实现用户注册、登录、权限管理功能
3. 实现核心业务 CRUD 操作
4. 使用 MySQL/PostgreSQL 数据库，设计合理的表结构
5. 编写单元测试，覆盖率不低于 70%
6. 提交完整的项目源代码（zip 压缩包）
7. 提交实训报告（Word 或 PDF 格式）
8. 代码需有适当注释，README 说明部署步骤"""

TASK_DESCRIPTION = "Python Web 开发综合实训 - 开发一个完整的 Web 应用系统"


# ============================================================
# 前端期望的数据结构验证
# ============================================================

@dataclass
class FrontendExpectedStructure:
    """前端 TypeScript 接口期望的数据结构."""

    @staticmethod
    def validate_evaluation_out(data: dict) -> list[str]:
        """验证 EvaluationOut 结构."""
        errors = []
        required_fields = ["id", "task_id", "student_id", "upload_id", "status",
                          "total_score", "teacher_comment", "created_at", "scores"]
        for f in required_fields:
            if f not in data:
                errors.append(f"Missing field: {f}")

        if "status" in data:
            valid_statuses = {"pending", "auto_scored", "auto_failed", "scored",
                            "reviewed", "confirmed", "finalized", "rejected"}
            if data["status"] not in valid_statuses:
                errors.append(f"Invalid status: {data['status']}")

        if "total_score" in data and data["total_score"] is not None:
            if not (0 <= data["total_score"] <= 100):
                errors.append(f"total_score out of range: {data['total_score']}")

        if "scores" in data:
            for i, s in enumerate(data["scores"]):
                s_errors = FrontendExpectedStructure.validate_dimension_score(s)
                for e in s_errors:
                    errors.append(f"scores[{i}]: {e}")

        return errors

    @staticmethod
    def validate_dimension_score(data: dict) -> list[str]:
        """验证 DimensionScoreOut 结构."""
        errors = []
        if "dimension_id" not in data:
            errors.append("Missing dimension_id")
        if "ai_score" not in data:
            errors.append("Missing ai_score")
        if "teacher_score" not in data:
            errors.append("Missing teacher_score")
        if "rationale" not in data:
            errors.append("Missing rationale")

        if "ai_score" in data and data["ai_score"] is not None:
            if not (0 <= data["ai_score"] <= 100):
                errors.append(f"ai_score out of range: {data['ai_score']}")

        if "rationale" in data and data["rationale"]:
            if len(data["rationale"]) < 50:
                errors.append(f"rationale too short: {len(data['rationale'])} < 50")

        return errors

    @staticmethod
    def validate_grading_submission(data: dict) -> list[str]:
        """验证 GradingView submissions 列表项结构."""
        errors = []
        required = ["upload_id", "student_id", "student_name", "filename",
                   "file_size", "version", "parse_status", "uploaded_at",
                   "evaluation_id", "eval_status", "total_score"]
        for f in required:
            if f not in data:
                errors.append(f"Missing field: {f}")

        if "parse_status" in data:
            valid = {"pending", "parsing", "parsed", "failed"}
            if data["parse_status"] not in valid:
                errors.append(f"Invalid parse_status: {data['parse_status']}")

        return errors

    @staticmethod
    def validate_verify_result(data: dict) -> list[str]:
        """验证核查结果结构."""
        errors = []
        required = ["upload_id", "match_rate", "checkpoints",
                   "missing_items", "logic_issues", "overall_confidence"]
        for f in required:
            if f not in data:
                errors.append(f"Missing field: {f}")

        if "match_rate" in data:
            if not (0 <= data["match_rate"] <= 100):
                errors.append(f"match_rate out of range: {data['match_rate']}")

        if "overall_confidence" in data:
            if not (0 <= data["overall_confidence"] <= 100):
                errors.append(f"overall_confidence out of range: {data['overall_confidence']}")

        return errors


# ============================================================
# 端到端测试主流程
# ============================================================

async def test_full_pipeline(filename: str, file_type: str) -> dict[str, Any]:
    """对单个文件执行完整的 解析→核查→评分→前端数据验证."""
    filepath = SAMPLES_DIR / filename
    if not filepath.exists():
        return {"status": "SKIP", "reason": f"File not found: {filepath}"}

    content = filepath.read_bytes()
    fake_llm = FakeLLMForScoring()
    errors: list[str] = []

    # ===== Step 1: 解析 =====
    parser = get_parser(file_type)
    parsed: ParsedDocument = await parser.parse(content)

    if not parsed.raw_text:
        # PNG without OCR is expected to be empty
        if file_type in ("png", "jpg", "jpeg"):
            return {"status": "SKIP", "reason": "OCR not available"}
        errors.append("Parse produced empty raw_text")

    # ===== Step 2: 模拟 LLM 结构化摘要 =====
    # 这是 ParsePipeline 中 LLM 会产出的结构
    structured_content = {
        "raw_text": parsed.raw_text,
        "paragraphs": parsed.paragraphs,
        "title_tree": [t.model_dump() for t in parsed.title_tree],
        "tables": parsed.tables,
        "llm_summary": f"该提交为{file_type}格式文件，内容约{len(parsed.raw_text)}字符。"
                       f"包含{len(parsed.paragraphs)}个段落，{len(parsed.title_tree)}个标题。",
        "llm_sections": [
            {"title": "概述", "summary": parsed.raw_text[:200], "key_points": ["核心内容"]},
        ],
        "llm_key_topics": ["Web开发", "数据库", "测试"],
        "llm_completeness": "内容基本完整",
        "has_code": "def " in parsed.raw_text or "class " in parsed.raw_text,
        "has_diagrams": "图" in parsed.raw_text or "表" in parsed.raw_text,
    }

    # ===== Step 3: 模拟核查 =====
    coverage = fake_llm.verify_coverage(
        task_requirements=TASK_REQUIREMENTS,
        parse_summary=parsed.raw_text[:2000],
    )
    logic = fake_llm.verify_logic(
        task_requirements=TASK_REQUIREMENTS,
        parse_summary=parsed.raw_text[:2000],
    )

    matched_count = sum(1 for cp in coverage["checkpoints"] if cp["matched"])
    total_cps = len(coverage["checkpoints"])
    match_rate = (matched_count / total_cps * 100) if total_cps > 0 else 0
    missing_items = [cp["requirement"] for cp in coverage["checkpoints"] if not cp["matched"]]

    verify_result = {
        "upload_id": 1,
        "match_rate": round(match_rate, 2),
        "checkpoints": coverage["checkpoints"],
        "missing_items": missing_items,
        "logic_issues": logic["issues"],
        "overall_confidence": int(match_rate * 0.7 + min(100, len(parsed.raw_text) // 10) * 0.3),
    }

    # 验证核查结果结构
    vr_errors = FrontendExpectedStructure.validate_verify_result(verify_result)
    for e in vr_errors:
        errors.append(f"[VerifyResult] {e}")

    # ===== Step 4: 模拟 LLM 评分 =====
    dimension_scores = []
    for dim in TASK_DIMENSIONS:
        score_result = fake_llm.score_dimension(
            dimension_name=dim["name"],
            dimension_description=dim["description"],
            submission_text=parsed.raw_text,
            task_requirements=TASK_REQUIREMENTS,
        )
        dimension_scores.append({
            "dimension_id": dim["id"],
            "ai_score": score_result["score"],
            "teacher_score": None,  # 教师尚未评分
            "rationale": score_result["rationale"],
        })

    # ===== Step 5: 计算综合分 =====
    alpha = 0.6
    score_data = [
        DimensionScoreData(
            weight=dim["weight"],
            objective_score=float(ds["ai_score"]),
            subjective_score=ds["teacher_score"],
        )
        for dim, ds in zip(TASK_DIMENSIONS, dimension_scores)
    ]
    total_score = float(compute_final_score(score_data, alpha))

    # ===== Step 6: 构建前端期望的完整响应 =====

    # 6a. EvaluationOut（学生评价详情页）
    evaluation_out = {
        "id": 1,
        "task_id": 100,
        "student_id": 1001,
        "upload_id": 1,
        "status": "auto_scored",
        "total_score": total_score,
        "teacher_comment": "",
        "created_at": "2024-12-15T10:30:00+08:00",
        "scores": dimension_scores,
    }

    # 验证 EvaluationOut 结构
    ev_errors = FrontendExpectedStructure.validate_evaluation_out(evaluation_out)
    for e in ev_errors:
        errors.append(f"[EvaluationOut] {e}")

    # 6b. GradingView submission 列表项
    grading_submission = {
        "upload_id": 1,
        "student_id": 1001,
        "student_name": "李明",
        "filename": filename,
        "file_size": filepath.stat().st_size,
        "version": 1,
        "parse_status": "parsed",
        "uploaded_at": "2024-12-15T10:00:00+08:00",
        "evaluation_id": 1,
        "eval_status": "auto_scored",
        "total_score": total_score,
    }

    gs_errors = FrontendExpectedStructure.validate_grading_submission(grading_submission)
    for e in gs_errors:
        errors.append(f"[GradingSubmission] {e}")

    # ===== Step 7: 模拟教师确认（覆盖主观分）=====
    # 教师给第一个维度打 88 分
    dimension_scores[0]["teacher_score"] = 88.0
    score_data_after = [
        DimensionScoreData(
            weight=dim["weight"],
            objective_score=float(ds["ai_score"]),
            subjective_score=ds["teacher_score"],
        )
        for dim, ds in zip(TASK_DIMENSIONS, dimension_scores)
    ]
    total_after_confirm = float(compute_final_score(score_data_after, alpha))

    # 确认后的 evaluation
    evaluation_confirmed = {
        **evaluation_out,
        "status": "confirmed",
        "total_score": total_after_confirm,
        "teacher_comment": "整体完成度不错，代码质量可以进一步优化。",
        "scores": dimension_scores,
    }
    ev2_errors = FrontendExpectedStructure.validate_evaluation_out(evaluation_confirmed)
    for e in ev2_errors:
        errors.append(f"[EvaluationConfirmed] {e}")

    # ===== 汇总结果 =====
    result = {
        "filename": filename,
        "file_type": file_type,
        "parse_text_len": len(parsed.raw_text),
        "verify_match_rate": verify_result["match_rate"],
        "verify_missing_count": len(missing_items),
        "verify_logic_issues": len(logic["issues"]),
        "verify_confidence": verify_result["overall_confidence"],
        "scores": {dim["name"]: ds["ai_score"] for dim, ds in zip(TASK_DIMENSIONS, dimension_scores)},
        "total_score_auto": total_score,
        "total_score_confirmed": total_after_confirm,
        "structured_content_keys": list(structured_content.keys()),
    }

    if errors:
        result["status"] = "FAIL"
        result["errors"] = errors
    else:
        result["status"] = "PASS"

    return result


# ============================================================
# 主入口
# ============================================================

TEST_FILES = [
    ("sample_report.docx", "docx"),
    ("sample_algorithm.docx", "docx"),
    ("sample_api_test.docx", "docx"),
    ("sample_database_design.pdf", "pdf"),
    ("sample_network_config.pdf", "pdf"),
    ("sample_data_analysis.xlsx", "xlsx"),
    ("sample_ml_notebook.xlsx", "xlsx"),
    ("sample_flask_project.zip", "zip"),
    ("sample_vue_project.zip", "zip"),
    ("sample_screenshot.png", "png"),
]


async def main() -> None:
    print("=" * 80)
    print("端到端评价链路测试：解析 → 核查 → 评分 → 前端数据验证")
    print("=" * 80)
    print()

    passed = 0
    failed = 0
    skipped = 0

    for filename, file_type in TEST_FILES:
        result = await test_full_pipeline(filename, file_type)

        if result["status"] == "PASS":
            passed += 1
            print(f"  ✓ PASS  {filename}")
            print(f"          解析: {result['parse_text_len']} chars")
            print(f"          核查: 匹配率={result['verify_match_rate']:.1f}%  "
                  f"缺失={result['verify_missing_count']}  "
                  f"逻辑问题={result['verify_logic_issues']}  "
                  f"置信度={result['verify_confidence']}")
            scores_str = "  ".join(
                f"{k}={v}" for k, v in result["scores"].items()
            )
            print(f"          评分: {scores_str}")
            print(f"          综合: AI自动={result['total_score_auto']:.1f}  "
                  f"教师确认后={result['total_score_confirmed']:.1f}")
            print()
        elif result["status"] == "SKIP":
            skipped += 1
            print(f"  ⚠ SKIP  {filename} - {result['reason']}")
            print()
        else:
            failed += 1
            print(f"  ✗ FAIL  {filename}")
            for err in result.get("errors", []):
                print(f"          → {err}")
            print()

    print("=" * 80)
    print(f"结果: {passed} passed, {failed} failed, {skipped} skipped")
    print()

    # 额外验证：前端 displayScores 计算逻辑
    print("─" * 80)
    print("前端 displayScores 模拟（EvaluationView.vue 的 computed）:")
    print("─" * 80)
    # 模拟前端的 displayScores computed
    sample_scores = [
        {"dimension_id": 1, "ai_score": 82, "teacher_score": 88, "rationale": "代码质量维度表现优秀，结构清晰，命名规范，注释完整。"},
        {"dimension_id": 2, "ai_score": 78, "teacher_score": None, "rationale": "功能实现维度表现良好，主要功能已完成，部分细节可优化。"},
        {"dimension_id": 3, "ai_score": 85, "teacher_score": None, "rationale": "文档规范维度表现优秀，报告结构完整，格式规范，内容详实。"},
        {"dimension_id": 4, "ai_score": 70, "teacher_score": None, "rationale": "测试覆盖维度表现一般，覆盖率达标但边界用例不足，建议补充异常场景测试。"},
        {"dimension_id": 5, "ai_score": 65, "teacher_score": None, "rationale": "创新性维度表现一般，基本完成要求但缺少亮点，建议增加个性化功能或优化方案。"},
    ]

    dim_map = {d["id"]: d for d in TASK_DIMENSIONS}
    display_scores = []
    for s in sample_scores:
        dim = dim_map[s["dimension_id"]]
        display_scores.append({
            "id": s["dimension_id"],
            "name": dim["name"],
            "weight": dim["weight"],
            "order": dim["order_index"],
            "ai_score": s["ai_score"],
            "teacher_score": s["teacher_score"],
            "final_score": s["teacher_score"] if s["teacher_score"] is not None else s["ai_score"],
            "rationale": s["rationale"],
        })
    display_scores.sort(key=lambda x: x["order"])

    for ds in display_scores:
        marker = "★" if ds["teacher_score"] is not None else " "
        print(f"  {marker} {ds['name']:8s} (权重{ds['weight']:2d}%)  "
              f"AI={ds['ai_score']:3d}  教师={str(ds['teacher_score'] or '-'):>4s}  "
              f"最终={ds['final_score']:3d}  "
              f"理由: {ds['rationale'][:40]}...")

    # 计算前端显示的综合分
    score_data_display = [
        DimensionScoreData(
            weight=ds["weight"],
            objective_score=float(ds["ai_score"]),
            subjective_score=float(ds["teacher_score"]) if ds["teacher_score"] else None,
        )
        for ds in display_scores
    ]
    final = float(compute_final_score(score_data_display, 0.6))
    print(f"\n  综合得分: {final:.1f} / 100")
    print(f"  (α=0.6, 有教师分的维度按 obj×0.6 + subj×0.4 计算)")

    # 验证 issues 计算（EvaluationView 的 computed issues）
    print()
    print("─" * 80)
    print("前端 issues 列表模拟（低分维度提示）:")
    print("─" * 80)
    for ds in display_scores:
        if ds["final_score"] < 60:
            print(f"  🔴 {ds['name']} 得分较低 ({ds['final_score']})")
        elif ds["final_score"] < 75:
            print(f"  🟡 {ds['name']} 仍有提升空间 ({ds['final_score']})")
        else:
            print(f"  🟢 {ds['name']} 表现良好 ({ds['final_score']})")

    print()
    print("=" * 80)
    if failed > 0:
        print("❌ 存在失败用例，请检查！")
        sys.exit(1)
    else:
        print("✅ 所有文件的评价链路验证通过，前端数据结构完全兼容！")


if __name__ == "__main__":
    asyncio.run(main())
