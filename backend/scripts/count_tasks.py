"""统计 tasks.md 各 Epic 的勾选情况."""

import re
from collections import Counter

p = r"d:\测试区域文件夹\test\.kiro\specs\training-evaluation-system\tasks.md"
s = open(p, encoding="utf-8").read()

ck: Counter[str] = Counter()
un: Counter[str] = Counter()
for m in re.finditer(r"^- \[x\] (\d+)\.", s, re.M):
    ck.update([m.group(1)])
for m in re.finditer(r"^- \[ \] (\d+)\.", s, re.M):
    un.update([m.group(1)])

epics = sorted(set(list(ck.keys()) + list(un.keys())), key=int)
print(f"{'Epic':<6}{'Done':<6}{'Todo':<6}")
for e in epics:
    print(f"{e:<6}{ck.get(e, 0):<6}{un.get(e, 0):<6}")

print(f"\nTotal done: {sum(ck.values())}, todo: {sum(un.values())}")
