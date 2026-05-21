"""create matviews (Epic 19.1) - PG only, no-op on SQLite.

Revision ID: m0001
Revises: c4306b4343a2
Create Date: 2026-05-19
"""

from __future__ import annotations

from alembic import op


revision = "m0001"
down_revision = "c4306b4343a2"
branch_labels = None
depends_on = None


def _is_postgres() -> bool:
    bind = op.get_bind()
    return bind.dialect.name == "postgresql"


def upgrade() -> None:
    if not _is_postgres():
        return
    op.execute(
        """
        CREATE MATERIALIZED VIEW IF NOT EXISTS mv_class_progress AS
        SELECT
          c.id AS class_id,
          c.name AS class_name,
          tc.task_id,
          COUNT(DISTINCT u.id) AS submission_count,
          COUNT(DISTINCT cm.student_id) AS roster_count,
          COUNT(DISTINCT e.id) AS evaluation_count,
          COUNT(DISTINCT CASE WHEN e.status = 'finalized' THEN e.id END) AS finalized_count
        FROM classes c
        JOIN task_classes tc ON tc.class_id = c.id
        LEFT JOIN class_memberships cm ON cm.class_id = c.id
        LEFT JOIN uploads u ON u.task_id = tc.task_id AND u.student_id = cm.student_id
        LEFT JOIN evaluations e ON e.upload_id = u.id
        GROUP BY c.id, c.name, tc.task_id
        WITH NO DATA
        """
    )
    op.execute(
        "CREATE UNIQUE INDEX IF NOT EXISTS uq_mv_class_progress "
        "ON mv_class_progress(class_id, task_id)"
    )

    op.execute(
        """
        CREATE MATERIALIZED VIEW IF NOT EXISTS mv_course_metrics AS
        SELECT
          tt.course_id,
          COUNT(DISTINCT e.id) AS evaluation_count,
          COUNT(DISTINCT e.student_id) AS student_count,
          AVG(e.total_score) AS avg_score
        FROM training_tasks tt
        LEFT JOIN evaluations e ON e.task_id = tt.id
        GROUP BY tt.course_id
        WITH NO DATA
        """
    )
    op.execute(
        "CREATE UNIQUE INDEX IF NOT EXISTS uq_mv_course_metrics "
        "ON mv_course_metrics(course_id)"
    )

    op.execute(
        """
        CREATE MATERIALIZED VIEW IF NOT EXISTS mv_school_overview AS
        SELECT
          (SELECT COUNT(*) FROM users WHERE role = 'student') AS total_students,
          (SELECT COUNT(*) FROM evaluations) AS total_evaluations,
          (SELECT AVG(total_score) FROM evaluations
            WHERE total_score IS NOT NULL) AS avg_score
        WITH NO DATA
        """
    )


def downgrade() -> None:
    if not _is_postgres():
        return
    op.execute("DROP MATERIALIZED VIEW IF EXISTS mv_school_overview")
    op.execute("DROP MATERIALIZED VIEW IF EXISTS mv_course_metrics")
    op.execute("DROP MATERIALIZED VIEW IF EXISTS mv_class_progress")
