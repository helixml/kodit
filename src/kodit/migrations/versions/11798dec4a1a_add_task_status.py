# ruff: noqa
"""add task status

Revision ID: 11798dec4a1a
Revises: 9cf0e87de578
Create Date: 2025-09-04 08:36:30.951906

"""

from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
revision: str = "11798dec4a1a"
down_revision: Union[str, None] = "9cf0e87de578"
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    """Upgrade schema."""
    op.create_table(
        "task_status",
        sa.Column("trackable_id", sa.Integer(), nullable=True),
        sa.Column("trackable_type", sa.String(length=255), nullable=True),
        sa.Column("parent", sa.Integer(), nullable=True),
        sa.Column("name", sa.String(length=255), nullable=False),
        sa.Column("state", sa.String(length=255), nullable=False),
        sa.Column("message", sa.UnicodeText(), nullable=False),
        sa.Column("error", sa.UnicodeText(), nullable=False),
        sa.Column("total", sa.Integer(), nullable=False),
        sa.Column("current", sa.Integer(), nullable=False),
        sa.Column("id", sa.Integer(), autoincrement=True, nullable=False),
        sa.Column("created_at", sa.DateTime(timezone=True), nullable=False),
        sa.Column("updated_at", sa.DateTime(timezone=True), nullable=False),
        sa.ForeignKeyConstraint(
            ["parent"],
            ["task_status.id"],
        ),
        sa.PrimaryKeyConstraint("id"),
    )
    op.create_index(op.f("ix_task_status_name"), "task_status", ["name"], unique=False)
    op.create_index(
        op.f("ix_task_status_parent"), "task_status", ["parent"], unique=False
    )
    op.create_index(
        op.f("ix_task_status_trackable_id"),
        "task_status",
        ["trackable_id"],
        unique=False,
    )
    op.create_index(
        op.f("ix_task_status_trackable_type"),
        "task_status",
        ["trackable_type"],
        unique=False,
    )


def downgrade() -> None:
    """Downgrade schema."""
    op.drop_index(op.f("ix_task_status_trackable_type"), table_name="task_status")
    op.drop_index(op.f("ix_task_status_trackable_id"), table_name="task_status")
    op.drop_index(op.f("ix_task_status_parent"), table_name="task_status")
    op.drop_index(op.f("ix_task_status_name"), table_name="task_status")
    op.drop_table("task_status")
