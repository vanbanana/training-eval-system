"""阶段 C 验收脚本：登录 admin/teacher01/student01/student02，

并发拉所有 list 接口，断言关键页面有真实数据；输出 PASS/FAIL 矩阵。

用法:
    .venv\\Scripts\\python.exe -m scripts.verify_real_data
    .venv\\Scripts\\python.exe -m scripts.verify_real_data --base-url http://127.0.0.1:8000
    .venv\\Scripts\\python.exe -m scripts.verify_real_data --reseed   # 先 POST /api/_dev/seed?scale=medium

注意：脚本不依赖项目内部代码（只用 stdlib + httpx），方便手工排查。
"""

from __future__ import annotations

import argparse
import asyncio
import sys
from dataclasses import dataclass, field
from typing import Any

import httpx

DEFAULT_BASE_URL = "http://127.0.0.1:8000"
DEFAULT_DEV_TOKEN = "dev-token"

ACCOUNTS: dict[str, str] = {
    "admin": "Admin@123",
    "teacher01": "Teacher@123",
    "student01": "Student@123",
    "student02": "Student@123",
}


@dataclass
class CheckResult:
    name: str
    ok: bool
    detail: str = ""

    @property
    def icon(self) -> str:
        return "✓" if self.ok else "✗"


@dataclass
class UserReport:
    username: str
    role: str
    checks: list[CheckResult] = field(default_factory=list)

    @property
    def passed(self) -> bool:
        return all(c.ok for c in self.checks)


async def login(
    client: httpx.AsyncClient, username: str, password: str
) -> str:
    r = await client.post(
        "/api/auth/login",
        json={"username": username, "password": password},
    )
    r.raise_for_status()
    body = r.json()
    return str(body["access_token"])


async def call(
    client: httpx.AsyncClient,
    token: str,
    method: str,
    path: str,
    *,
    params: dict[str, Any] | None = None,
) -> tuple[int, Any]:
    r = await client.request(
        method,
        path,
        params=params,
        headers={"Authorization": f"Bearer {token}"},
    )
    try:
        body = r.json()
    except ValueError:
        body = r.text
    return r.status_code, body


def assert_list_has_items(
    body: Any, min_count: int, label: str
) -> CheckResult:
    if isinstance(body, list):
        n = len(body)
    elif isinstance(body, dict) and "items" in body:
        n = len(body.get("items") or [])
    else:
        return CheckResult(label, False, f"unexpected shape: {type(body).__name__}")
    return CheckResult(
        label,
        n >= min_count,
        f"{n} items (>= {min_count} required)",
    )


def assert_field_at_least(
    body: dict[str, Any], key: str, min_value: int, label: str
) -> CheckResult:
    val = body.get(key)
    if not isinstance(val, (int, float)):
        return CheckResult(label, False, f"{key}={val!r} not numeric")
    return CheckResult(label, val >= min_value, f"{key}={val} (>= {min_value})")


# ============== 各角色检查 ==============


async def check_admin(
    client: httpx.AsyncClient, token: str
) -> UserReport:
    rep = UserReport(username="admin", role="admin")
    # dashboard
    code, body = await call(client, token, "GET", "/api/dashboard")
    if code != 200 or not isinstance(body, dict):
        rep.checks.append(CheckResult("/api/dashboard", False, f"HTTP {code}"))
    else:
        rep.checks.append(
            assert_field_at_least(body, "user_count", 4, "dashboard.user_count")
        )
        rep.checks.append(
            assert_field_at_least(body, "task_count", 1, "dashboard.task_count")
        )
        rep.checks.append(
            assert_field_at_least(
                body, "eval_count", 1, "dashboard.eval_count"
            )
        )
    # courses
    code, body = await call(client, token, "GET", "/api/courses")
    rep.checks.append(
        assert_list_has_items(body, 1, "/api/courses")
        if code == 200
        else CheckResult("/api/courses", False, f"HTTP {code}")
    )
    # tasks
    code, body = await call(client, token, "GET", "/api/tasks")
    rep.checks.append(
        assert_list_has_items(body, 1, "/api/tasks")
        if code == 200
        else CheckResult("/api/tasks", False, f"HTTP {code}")
    )
    # audit
    code, body = await call(
        client, token, "GET", "/api/audit", params={"page_size": 5}
    )
    if code == 200 and isinstance(body, dict):
        rep.checks.append(
            assert_field_at_least(body, "total", 1, "audit.total")
        )
    else:
        rep.checks.append(CheckResult("/api/audit", False, f"HTTP {code}"))
    # notifications
    code, body = await call(client, token, "GET", "/api/notifications")
    rep.checks.append(
        CheckResult(
            "/api/notifications",
            code == 200 and isinstance(body, dict),
            f"HTTP {code}",
        )
    )
    return rep


async def check_teacher(
    client: httpx.AsyncClient, token: str
) -> UserReport:
    rep = UserReport(username="teacher01", role="teacher")
    # dashboard
    code, body = await call(client, token, "GET", "/api/dashboard")
    if code != 200 or not isinstance(body, dict):
        rep.checks.append(CheckResult("/api/dashboard", False, f"HTTP {code}"))
    else:
        # teacher dashboard 至少应有 my_tasks 字段
        rep.checks.append(
            CheckResult(
                "dashboard.has_my_tasks",
                "my_tasks" in body,
                f"keys: {sorted(body)[:6]}",
            )
        )
    # tasks
    code, body = await call(client, token, "GET", "/api/tasks")
    rep.checks.append(
        assert_list_has_items(body, 1, "/api/tasks")
        if code == 200
        else CheckResult("/api/tasks", False, f"HTTP {code}")
    )
    # classes
    code, body = await call(client, token, "GET", "/api/classes")
    rep.checks.append(
        assert_list_has_items(body, 1, "/api/classes")
        if code == 200
        else CheckResult("/api/classes", False, f"HTTP {code}")
    )
    # 选第一个 task 看 grading submissions
    if isinstance(body, list) and body:
        pass  # already used for classes
    code, body = await call(client, token, "GET", "/api/tasks")
    if code == 200 and isinstance(body, list) and body:
        first_task = body[0]
        tid = first_task.get("id")
        if tid:
            code2, b2 = await call(
                client,
                token,
                "GET",
                f"/api/grading/tasks/{tid}/submissions",
            )
            rep.checks.append(
                CheckResult(
                    f"grading task#{tid} submissions",
                    code2 == 200 and isinstance(b2, list),
                    f"HTTP {code2} count={len(b2) if isinstance(b2, list) else 'n/a'}",
                )
            )
    return rep


async def check_student(
    client: httpx.AsyncClient, token: str, username: str
) -> UserReport:
    rep = UserReport(username=username, role="student")
    code, body = await call(client, token, "GET", "/api/dashboard")
    rep.checks.append(
        CheckResult(
            "/api/dashboard",
            code == 200 and isinstance(body, dict),
            f"HTTP {code}",
        )
    )
    # tasks（学生视图）
    code, body = await call(client, token, "GET", "/api/tasks")
    rep.checks.append(
        assert_list_has_items(body, 1, "/api/tasks (published)")
        if code == 200
        else CheckResult("/api/tasks", False, f"HTTP {code}")
    )
    # 我的评价
    code, body = await call(client, token, "GET", "/api/evaluations/my")
    rep.checks.append(
        # 注意：学生可能尚未有评价，但接口 200 是必须的
        CheckResult(
            "/api/evaluations/my",
            code == 200 and isinstance(body, list),
            f"HTTP {code} count={len(body) if isinstance(body, list) else 'n/a'}",
        )
    )
    # 通知
    code, body = await call(client, token, "GET", "/api/notifications")
    rep.checks.append(
        CheckResult(
            "/api/notifications",
            code == 200 and isinstance(body, dict),
            f"HTTP {code}",
        )
    )
    # 聊天会话（200 即可，学生可能没有会话）
    code, body = await call(client, token, "GET", "/api/chat/sessions")
    rep.checks.append(
        CheckResult(
            "/api/chat/sessions",
            code == 200,
            f"HTTP {code}",
        )
    )
    return rep


# ============== 主入口 ==============


async def reseed(
    client: httpx.AsyncClient,
    *,
    scale: str,
    dev_token: str,
) -> dict[str, Any]:
    r = await client.post(
        "/api/_dev/seed",
        params={"scale": scale, "reset": "true"},
        headers={"X-Dev-Token": dev_token},
        timeout=180,
    )
    r.raise_for_status()
    body = r.json()
    return dict(body) if isinstance(body, dict) else {"raw": body}


def render_report(reports: list[UserReport]) -> str:
    lines: list[str] = []
    lines.append("=" * 64)
    lines.append("阶段 C · 真实数据验收报告")
    lines.append("=" * 64)
    for rep in reports:
        title = f"[{rep.username}] role={rep.role}"
        status = "PASS" if rep.passed else "FAIL"
        lines.append(f"{status} {title}")
        for c in rep.checks:
            lines.append(f"  {c.icon} {c.name:<32} {c.detail}")
        lines.append("")
    total = sum(len(r.checks) for r in reports)
    passed = sum(1 for r in reports for c in r.checks if c.ok)
    lines.append("-" * 64)
    lines.append(f"Total: {passed}/{total} checks passed")
    return "\n".join(lines)


async def main_async(args: argparse.Namespace) -> int:
    async with httpx.AsyncClient(base_url=args.base_url, timeout=30) as client:
        if args.reseed:
            print(f"→ POST /api/_dev/seed?scale={args.scale}&reset=true ...")
            seed_body = await reseed(
                client, scale=args.scale, dev_token=args.dev_token
            )
            print("  seed result:", seed_body)
            print()

        # 登录
        tokens: dict[str, str] = {}
        for username, password in ACCOUNTS.items():
            try:
                tokens[username] = await login(client, username, password)
            except Exception as e:
                print(f"⚠ login {username} failed: {e}")
                tokens[username] = ""

        # 跑 checks
        reports: list[UserReport] = []
        if tokens.get("admin"):
            reports.append(await check_admin(client, tokens["admin"]))
        if tokens.get("teacher01"):
            reports.append(await check_teacher(client, tokens["teacher01"]))
        for s in ("student01", "student02"):
            if tokens.get(s):
                reports.append(
                    await check_student(client, tokens[s], username=s)
                )

        print(render_report(reports))
        # 失败时返回非零退出码
        if any(not r.passed for r in reports):
            return 1
        return 0


def main() -> int:
    parser = argparse.ArgumentParser(description="阶段 C 真实数据验收脚本")
    parser.add_argument("--base-url", default=DEFAULT_BASE_URL)
    parser.add_argument("--dev-token", default=DEFAULT_DEV_TOKEN)
    parser.add_argument(
        "--reseed",
        action="store_true",
        help="先 POST /api/_dev/seed?scale=...&reset=true 再做检查",
    )
    parser.add_argument("--scale", default="medium", choices=("small", "medium", "large"))
    args = parser.parse_args()
    return asyncio.run(main_async(args))


if __name__ == "__main__":
    sys.exit(main())
