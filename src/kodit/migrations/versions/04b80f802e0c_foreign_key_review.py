# ruff: noqa
"""foreign key review

Revision ID: 04b80f802e0c
Revises: 7f15f878c3a1
Create Date: 2025-09-22 11:21:43.432880

"""

from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
revision: str = "04b80f802e0c"
down_revision: Union[str, None] = "7f15f878c3a1"
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    """Upgrade schema."""
    # SQLite doesn't support complex constraint alterations, so we'll drop and recreate tables

    # Drop and recreate commit_indexes table with commit_sha as primary key
    op.drop_table("commit_indexes")
    op.create_table(
        "commit_indexes",
        sa.Column("commit_sha", sa.String(64), nullable=False),
        sa.Column("status", sa.String(255), nullable=False),
        sa.Column("indexed_at", sa.DateTime(), nullable=True),
        sa.Column("error_message", sa.UnicodeText(), nullable=True),
        sa.Column("files_processed", sa.Integer(), nullable=False, default=0),
        sa.Column("processing_time_seconds", sa.Float(), nullable=False, default=0.0),
        sa.Column("created_at", sa.DateTime(), nullable=False),
        sa.Column("updated_at", sa.DateTime(), nullable=False),
        sa.PrimaryKeyConstraint("commit_sha", name="pk_commit_indexes"),
    )
    op.create_index("ix_commit_indexes_status", "commit_indexes", ["status"])

    # Drop and recreate git_tracking_branches table with proper constraints
    op.drop_table("git_tracking_branches")
    op.create_table(
        "git_tracking_branches",
        sa.Column("repo_id", sa.Integer(), nullable=False),
        sa.Column("name", sa.String(255), nullable=False),
        sa.Column("created_at", sa.DateTime(), nullable=False),
        sa.Column("updated_at", sa.DateTime(), nullable=False),
        sa.ForeignKeyConstraint(
            ["repo_id"], ["git_repos.id"], name="fk_tracking_branch_repo"
        ),
        sa.PrimaryKeyConstraint("repo_id", "name", name="pk_git_tracking_branches"),
    )
    op.create_index("ix_git_tracking_branches_name", "git_tracking_branches", ["name"])
    op.create_index(
        "ix_git_tracking_branches_repo_id", "git_tracking_branches", ["repo_id"]
    )


def downgrade() -> None:
    """Downgrade schema."""
    # Recreate the original tables

    # Recreate commit_indexes table with id-based primary key (original structure)
    op.drop_table("commit_indexes")
    op.create_table(
        "commit_indexes",
        sa.Column("id", sa.Integer(), nullable=False),
        sa.Column("commit_sha", sa.String(64), nullable=False),
        sa.Column("status", sa.String(255), nullable=False),
        sa.Column("indexed_at", sa.DateTime(), nullable=True),
        sa.Column("error_message", sa.UnicodeText(), nullable=True),
        sa.Column("files_processed", sa.Integer(), nullable=False, default=0),
        sa.Column("processing_time_seconds", sa.Float(), nullable=False, default=0.0),
        sa.Column("created_at", sa.DateTime(), nullable=False),
        sa.Column("updated_at", sa.DateTime(), nullable=False),
        sa.PrimaryKeyConstraint("id"),
    )
    op.create_index("ix_commit_indexes_status", "commit_indexes", ["status"])

    # Recreate git_tracking_branches table with original structure
    op.drop_table("git_tracking_branches")
    op.create_table(
        "git_tracking_branches",
        sa.Column("repo_id", sa.Integer(), nullable=False),
        sa.Column("name", sa.String(255), nullable=False),
        sa.Column("created_at", sa.DateTime(), nullable=False),
        sa.Column("updated_at", sa.DateTime(), nullable=False),
        sa.ForeignKeyConstraint(
            ["repo_id", "name"], ["git_branches.repo_id", "git_branches.name"]
        ),
        sa.PrimaryKeyConstraint("repo_id"),
        sa.UniqueConstraint("repo_id", "name", name="uix_repo_tracking_branch"),
    )
    op.create_index("ix_git_tracking_branches_name", "git_tracking_branches", ["name"])
    op.create_index(
        "ix_git_tracking_branches_repo_id", "git_tracking_branches", ["repo_id"]
    )
