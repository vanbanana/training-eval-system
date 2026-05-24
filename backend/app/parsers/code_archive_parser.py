"""CodeArchiveParser - 解析 ZIP 压缩包中的源代码文件.

支持 .zip 格式的源代码压缩包。
解析策略：
1. 解压并遍历文件树
2. 按扩展名识别源代码文件
3. 提取代码内容（限制总大小防止内存溢出）
4. 生成文件结构树 + 代码内容摘要
"""

from __future__ import annotations

import io
import zipfile
from pathlib import PurePosixPath

from app.parsers.base import ParsedDocument, ParserError, TitleNode


# 支持的源代码扩展名
_CODE_EXTENSIONS: set[str] = {
    ".py", ".java", ".c", ".cpp", ".h", ".hpp", ".cs",
    ".js", ".ts", ".jsx", ".tsx", ".vue", ".svelte",
    ".html", ".css", ".scss", ".less",
    ".go", ".rs", ".rb", ".php", ".swift", ".kt",
    ".sql", ".sh", ".bat", ".ps1",
    ".json", ".yaml", ".yml", ".toml", ".xml", ".ini", ".cfg",
    ".md", ".txt", ".rst",
    ".dockerfile", ".makefile",
}

# 忽略的目录
_IGNORE_DIRS: set[str] = {
    "__pycache__", "node_modules", ".git", ".svn", ".hg",
    ".venv", "venv", "env", ".env",
    "dist", "build", "target", "bin", "obj",
    ".idea", ".vscode", ".vs",
    "__MACOSX",
}

# 单文件最大读取字节数
_MAX_FILE_BYTES = 100 * 1024  # 100KB per file
# 总提取文本上限
_MAX_TOTAL_CHARS = 500_000  # 500K chars total


class CodeArchiveParser:
    file_type = "zip"

    async def parse(self, content: bytes) -> ParsedDocument:
        try:
            zf = zipfile.ZipFile(io.BytesIO(content))
        except (zipfile.BadZipFile, Exception) as e:
            raise ParserError(f"ZIP 解压失败: {e}", field="file") from e

        # 安全检查：防止 zip bomb
        total_uncompressed = sum(info.file_size for info in zf.infolist())
        if total_uncompressed > 500 * 1024 * 1024:  # 500MB 解压上限
            raise ParserError(
                "压缩包解压后超过 500MB，疑似异常文件", field="file"
            )

        file_tree: list[TitleNode] = []
        code_files: list[tuple[str, str]] = []  # (path, content)
        raw_text_parts: list[str] = []
        total_chars = 0

        # 收集有效文件列表
        valid_entries: list[zipfile.ZipInfo] = []
        for info in zf.infolist():
            if info.is_dir():
                continue
            # 过滤隐藏文件和忽略目录
            parts = PurePosixPath(info.filename).parts
            if any(p.startswith(".") and p not in {".env.example", ".gitignore"} for p in parts):
                if not any(p in _IGNORE_DIRS for p in parts):
                    pass  # 允许 .gitignore 等
            if any(p in _IGNORE_DIRS for p in parts):
                continue
            valid_entries.append(info)

        # 按路径排序
        valid_entries.sort(key=lambda x: x.filename)

        # 构建文件树（一级目录作为标题）
        dir_nodes: dict[str, TitleNode] = {}
        for info in valid_entries:
            parts = PurePosixPath(info.filename).parts
            if len(parts) > 1:
                top_dir = parts[0] if len(parts) > 1 else ""
                if top_dir and top_dir not in dir_nodes:
                    dir_nodes[top_dir] = TitleNode(
                        level=1, text=f"📁 {top_dir}", children=[]
                    )

        # 提取代码内容
        for info in valid_entries:
            if total_chars >= _MAX_TOTAL_CHARS:
                raw_text_parts.append("\n[... 内容截断：已达提取上限 ...]")
                break

            suffix = PurePosixPath(info.filename).suffix.lower()
            name_lower = PurePosixPath(info.filename).name.lower()

            # 特殊文件名（无扩展名但是代码文件）
            is_code = (
                suffix in _CODE_EXTENSIONS
                or name_lower in {"makefile", "dockerfile", "rakefile", "gemfile"}
            )

            if not is_code:
                continue

            # 读取文件内容
            try:
                file_bytes = zf.read(info.filename)
                if len(file_bytes) > _MAX_FILE_BYTES:
                    file_text = file_bytes[:_MAX_FILE_BYTES].decode(
                        "utf-8", errors="replace"
                    )
                    file_text += "\n[... 文件截断 ...]"
                else:
                    file_text = file_bytes.decode("utf-8", errors="replace")
            except Exception:  # noqa: BLE001
                file_text = "[二进制文件，无法读取]"

            code_files.append((info.filename, file_text))
            raw_text_parts.append(f"\n{'='*60}")
            raw_text_parts.append(f"文件: {info.filename}")
            raw_text_parts.append(f"{'='*60}")
            raw_text_parts.append(file_text)
            total_chars += len(file_text)

        zf.close()

        # 生成文件结构概览
        structure_lines = ["项目文件结构:"]
        for info in valid_entries[:200]:  # 最多列出 200 个文件
            structure_lines.append(f"  {info.filename}")
        if len(valid_entries) > 200:
            structure_lines.append(f"  ... 共 {len(valid_entries)} 个文件")

        structure_text = "\n".join(structure_lines)
        raw_text_parts.insert(0, structure_text)

        # 文件树节点
        file_tree = list(dir_nodes.values())
        if not file_tree and valid_entries:
            file_tree = [
                TitleNode(level=1, text="📁 项目根目录", children=[])
            ]

        return ParsedDocument(
            raw_text="\n".join(raw_text_parts),
            paragraphs=[structure_text] + [f"文件: {p}" for p, _ in code_files[:50]],
            title_tree=file_tree,
            metadata={
                "total_files": str(len(valid_entries)),
                "code_files": str(len(code_files)),
                "archive_type": "zip",
            },
            page_count=len(code_files),
        )
