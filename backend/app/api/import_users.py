"""批量导入用户路由."""

from __future__ import annotations

import csv
import io

from fastapi import APIRouter, UploadFile

from app.api.deps import CurrentUser, DbSession
from app.core.exceptions import AuthorizationError, BusinessRuleError
from app.core.security import hash_password
from app.models.user import User

router = APIRouter(prefix="/api/import", tags=["import"])


@router.post("/users")
async def import_users_csv(file: UploadFile, db: DbSession, current: CurrentUser) -> dict[str, object]:
    """CSV 批量导入用户。格式：username,display_name,role,password."""
    if current.role != "admin":
        raise AuthorizationError("仅管理员可导入")

    if not file.filename or not file.filename.endswith(".csv"):
        raise BusinessRuleError("请上传 CSV 文件", field="file")

    content = (await file.read()).decode("utf-8-sig")
    reader = csv.DictReader(io.StringIO(content))

    created = 0
    errors: list[dict[str, str]] = []

    for i, row in enumerate(reader, start=2):
        username = row.get("username", "").strip()
        display_name = row.get("display_name", "").strip()
        role = row.get("role", "").strip()
        password = row.get("password", "").strip()

        if not username or not display_name or not role or not password:
            errors.append({"row": str(i), "error": "字段不完整"})
            continue
        if role not in ("admin", "teacher", "student"):
            errors.append({"row": str(i), "error": f"角色无效: {role}"})
            continue
        if len(password) < 8:
            errors.append({"row": str(i), "error": "密码长度不足 8 位"})
            continue

        user = User(
            username=username,
            display_name=display_name,
            role=role,
            password_hash=hash_password(password),
            is_active=True,
        )
        db.add(user)
        created += 1

    if created > 0:
        try:
            await db.commit()
        except Exception as e:
            await db.rollback()
            raise BusinessRuleError(f"写入失败（可能用户名重复）: {e}") from e

    return {"created": created, "errors": errors, "total_rows": created + len(errors)}


@router.get("/users/template")
async def download_template() -> dict[str, str]:
    """返回 CSV 模板说明."""
    return {
        "format": "username,display_name,role,password",
        "example": "student03,张三,student,Pass@1234",
        "notes": "role 可选值: admin / teacher / student；password ≥ 8 位",
    }
