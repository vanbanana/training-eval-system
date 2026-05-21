"""audit_logs append-only triggers (Epic 23.1) - PG only.

Revision ID: m0002
Revises: m0001
"""

from __future__ import annotations

from alembic import op


revision = "m0002"
down_revision = "m0001"
branch_labels = None
depends_on = None


def _is_postgres() -> bool:
    return op.get_bind().dialect.name == "postgresql"


def upgrade() -> None:
    if not _is_postgres():
        return
    op.execute(
        """
        CREATE OR REPLACE FUNCTION audit_logs_no_modify()
        RETURNS TRIGGER AS $$
        BEGIN
            RAISE EXCEPTION 'audit_log is append-only';
        END;
        $$ LANGUAGE plpgsql;
        """
    )
    op.execute(
        """
        DROP TRIGGER IF EXISTS audit_logs_no_update ON audit_logs;
        CREATE TRIGGER audit_logs_no_update
        BEFORE UPDATE ON audit_logs
        FOR EACH ROW EXECUTE FUNCTION audit_logs_no_modify();
        """
    )
    op.execute(
        """
        DROP TRIGGER IF EXISTS audit_logs_no_delete ON audit_logs;
        CREATE TRIGGER audit_logs_no_delete
        BEFORE DELETE ON audit_logs
        FOR EACH ROW EXECUTE FUNCTION audit_logs_no_modify();
        """
    )


def downgrade() -> None:
    if not _is_postgres():
        return
    op.execute("DROP TRIGGER IF EXISTS audit_logs_no_update ON audit_logs")
    op.execute("DROP TRIGGER IF EXISTS audit_logs_no_delete ON audit_logs")
    op.execute("DROP FUNCTION IF EXISTS audit_logs_no_modify()")
