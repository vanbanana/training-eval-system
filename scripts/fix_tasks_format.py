"""批量将 tasks.md 中老格式的 Task 章节转换为 Kiro 要求的 checkbox 格式。

规则：
- `### Task X.Y: Title`            → `- [ ] X.Y. Title`
- `- **所属 Epic**: ...`              删除（信息冗余，已被 Epic 章节覆盖）
- `#### 实施要点`                  → `  **实施要点**`（缩进 2 空格）
- `#### 测试验收标准`              → `  **测试验收标准**`
- `**主路径 (Happy Path)**`        → `  _主路径 (Happy Path)_`
- `**异常路径 (Error Path)**`      → `  _异常路径 (Error Path)_`
- `**边界路径 (Boundary Path)**`   → `  _边界路径 (Boundary Path)_`
- `#### 测试文件位置`              → `  **测试文件位置**`
- `#### 验收检查清单`              → `  **验收检查清单**`
- 普通的 `- xxx` 列表项缩进 2 空格，变成 `  - xxx`
- 列表项前的 `- [ ]` 子检查框（在 `验收检查清单` 章节内）保持，但缩进 2 空格

只处理位于 "## Tasks" 之后的内容，且仅转换 `### Task ` 标题及其下方块。
"""

from __future__ import annotations
import re
from pathlib import Path

PATH = Path(r"d:\测试区域文件夹\test\.kiro\specs\training-evaluation-system\tasks.md")
text = PATH.read_text(encoding="utf-8")

# 找到 "## Tasks" 位置之前的内容（保留原样）
header_end = text.index("\n## Tasks\n") + len("\n## Tasks\n")
prefix = text[:header_end]
body = text[header_end:]

# 切分成 Epic 段（## Epic N:）与 Task 段（### Task X.Y:）
# 我们用正则定位每个 "### Task " 的起始位置，以及紧跟其后的 "### Task " 或 "## Epic" 边界

# 方法：基于行处理
lines = body.split("\n")
out: list[str] = []
i = 0
in_task = False  # 是否在某个 task 块内

while i < len(lines):
    line = lines[i]
    m = re.match(r"^### Task (\d+\.\d+): (.+?)(?:（.*?）)?\s*$", line)
    if m:
        # 进入新 task
        in_task = True
        num, title = m.group(1), m.group(2).strip()
        # 去掉末尾"（旧格式占位待删）"等无意义后缀
        title = re.sub(r"（旧格式.*?）$", "", title).strip()
        out.append(f"- [ ] {num}. {title}")
        i += 1
        continue

    # 在 task 内部，需要做缩进转换
    if in_task:
        # 检查是否到了下一个 task / 下一个 epic / 文件结尾
        if line.startswith("### Task "):
            in_task = False
            continue  # 不消费，下一轮 while 处理新 task
        if line.startswith("## "):
            in_task = False
            out.append(line)
            i += 1
            continue

        # 跳过单独的元数据行：- **所属 Epic**: Epic X
        if re.match(r"^- \*\*所属 Epic\*\*:.*$", line):
            i += 1
            continue

        # 转换章节小标题
        sec_map = {
            "#### 实施要点": "  **实施要点**",
            "#### 测试验收标准": "  **测试验收标准**",
            "#### 测试文件位置": "  **测试文件位置**",
            "#### 验收检查清单": "  **验收检查清单**",
        }
        if line in sec_map:
            out.append("")
            out.append(sec_map[line])
            i += 1
            continue

        # 子标题：主路径/异常路径/边界路径
        bold_map = {
            "**主路径 (Happy Path)**": "  _主路径 (Happy Path)_",
            "**异常路径 (Error Path)**": "  _异常路径 (Error Path)_",
            "**边界路径 (Boundary Path)**": "  _边界路径 (Boundary Path)_",
        }
        if line.strip() in bold_map:
            out.append("")
            out.append(bold_map[line.strip()])
            i += 1
            continue

        # 普通元数据行：- **架构层**: ... → 保持但缩进
        if line.startswith("- **") and i < 20:  # task 头部元数据
            out.append("  " + line)
            i += 1
            continue

        # 列表项：- xxx → 缩进
        if line.startswith("- "):
            # 验收清单的 [ ] 列表项已经是 - [ ]，不再嵌套加 [ ]
            out.append("  " + line)
            i += 1
            continue

        # 空行或其他段落文字
        if line.strip() == "":
            out.append("")
        else:
            out.append("  " + line)
        i += 1
        continue

    # 不在 task 内，原样保留
    out.append(line)
    i += 1

new_body = "\n".join(out)
PATH.write_text(prefix + new_body, encoding="utf-8")
print("Done. Total lines:", len(out))
