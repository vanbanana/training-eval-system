"""Task 0.6 验收：README / CONTRIBUTING 文档完备."""

from __future__ import annotations

from pathlib import Path
import re


_REPO = Path(__file__).resolve().parent.parent.parent.parent


def _read(name: str) -> str:
    return (_REPO / name).read_text(encoding="utf-8")


def test_readme_exists_with_required_sections() -> None:
    """Given README.md；When 读；Then 含目录 / 技术栈 / 快速启动 / 常用命令 章节。"""
    content = _read("README.md")
    for section in ("## 技术栈", "## 快速启动", "## 项目结构", "## 常用命令"):
        assert section in content, f"README 缺少章节 {section}"


def test_readme_links_to_design_docs() -> None:
    """Given README；When 检查；Then 链接到 requirements.md / design.md / tasks.md / handbook。"""
    content = _read("README.md")
    assert "requirements.md" in content
    assert "design.md" in content
    assert "tasks.md" in content
    assert "handbook" in content


def test_readme_quickstart_mentions_uvicorn_and_pnpm() -> None:
    """Given README 快速启动；When 检查；Then 含 uvicorn 与 pnpm dev 命令。"""
    content = _read("README.md")
    assert "uvicorn" in content
    assert "pnpm dev" in content or "pnpm install" in content


def test_readme_includes_troubleshooting() -> None:
    """Given README；When 检查；Then 含故障排查段。"""
    content = _read("README.md")
    assert "故障排查" in content


def test_contributing_exists_with_pr_template() -> None:
    """Given CONTRIBUTING.md；When 读；Then 含分支策略 / 提交规范 / PR 模板。"""
    content = _read("CONTRIBUTING.md")
    assert "分支策略" in content
    assert "Conventional Commits" in content
    assert "PR" in content
    # PR 模板含必备章节
    assert "## 变更概述" in content or "变更概述" in content
    assert "测试" in content


def test_contributing_lists_all_commit_types() -> None:
    """Given CONTRIBUTING；When 检查；Then 列出全部 10 种 conventional types。"""
    content = _read("CONTRIBUTING.md")
    for kind in ("feat", "fix", "docs", "test", "chore", "refactor", "perf", "build", "ci", "style"):
        assert re.search(rf"\b{kind}\b", content), f"提交类型 {kind} 缺失"


def test_no_dead_links_in_readme() -> None:
    """Given README；When 提取相对链接；Then 所有指向仓库内的文件路径都存在。"""
    content = _read("README.md")
    # 匹配 markdown 相对链接：](path)
    relative_links = re.findall(r"\]\(([^)]+)\)", content)
    for link in relative_links:
        # 跳过外链与锚点
        if link.startswith(("http://", "https://", "#", "mailto:")):
            continue
        # 去掉锚点
        link_path = link.split("#", 1)[0]
        if not link_path:
            continue
        target = _REPO / link_path
        assert target.exists(), f"README 中相对链接死链：{link}"
