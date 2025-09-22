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

    # Drop and recreate git_commit_files table with repo_id column
    # Delete all data that depends on git_commit_files
    op.execute("DELETE FROM snippet_v2_files")
    op.execute("DELETE FROM snippets_v2")

    # For PostgreSQL, we need to drop the constraint first
    # For SQLite, constraints are dropped automatically with table drop
    try:
        op.drop_constraint("snippet_v2_files_commit_sha_file_path_fkey", "snippet_v2_files", type_="foreignkey")
    except NotImplementedError:
        # SQLite doesn't support dropping constraints - skip this step
        pass

    op.drop_table("git_commit_files")
    op.create_table(
        "git_commit_files",
        sa.Column("commit_sha", sa.String(64), nullable=False),
        sa.Column("repo_id", sa.Integer(), nullable=False),
        sa.Column("path", sa.String(1024), nullable=False),
        sa.Column("blob_sha", sa.String(64), nullable=False),
        sa.Column("mime_type", sa.String(255), nullable=False),
        sa.Column("extension", sa.String(255), nullable=False),
        sa.Column("size", sa.Integer(), nullable=False),
        sa.Column("created_at", sa.DateTime(timezone=True), nullable=False),
        sa.ForeignKeyConstraint(
            ["commit_sha"], ["git_commits.commit_sha"], name="fk_commit_file_commit"
        ),
        sa.ForeignKeyConstraint(
            ["repo_id"], ["git_repos.id"], name="fk_commit_file_repo"
        ),
        sa.PrimaryKeyConstraint("commit_sha", "path", name="pk_git_commit_files"),
        sa.UniqueConstraint("commit_sha", "path", name="uix_commit_file"),
    )
    op.create_index("ix_git_commit_files_repo_id", "git_commit_files", ["repo_id"])
    op.create_index("ix_git_commit_files_blob_sha", "git_commit_files", ["blob_sha"])
    op.create_index("ix_git_commit_files_mime_type", "git_commit_files", ["mime_type"])
    op.create_index("ix_git_commit_files_extension", "git_commit_files", ["extension"])

    # Recreate the foreign key constraint from snippet_v2_files to git_commit_files
    # This will be recreated automatically for SQLite when the table is referenced
    try:
        op.create_foreign_key(
            "snippet_v2_files_commit_sha_file_path_fkey",
            "snippet_v2_files",
            "git_commit_files",
            ["commit_sha", "file_path"],
            ["commit_sha", "path"]
        )
    except Exception:
        # If constraint creation fails (e.g., table already has it), continue
        pass


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

    # Recreate git_commit_files table with original structure (without repo_id)
    op.drop_table("git_commit_files")
    op.create_table(
        "git_commit_files",
        sa.Column("commit_sha", sa.String(64), nullable=False),
        sa.Column("path", sa.String(1024), nullable=False),
        sa.Column("blob_sha", sa.String(64), nullable=False),
        sa.Column("mime_type", sa.String(255), nullable=False),
        sa.Column("extension", sa.String(255), nullable=False),
        sa.Column("size", sa.Integer(), nullable=False),
        sa.Column("created_at", sa.DateTime(timezone=True), nullable=False),
        sa.ForeignKeyConstraint(["commit_sha"], ["git_commits.commit_sha"]),
        sa.PrimaryKeyConstraint("commit_sha", "path"),
        sa.UniqueConstraint("commit_sha", "path", name="uix_commit_file"),
    )
    op.create_index("ix_git_commit_files_blob_sha", "git_commit_files", ["blob_sha"])
    op.create_index("ix_git_commit_files_mime_type", "git_commit_files", ["mime_type"])
    op.create_index("ix_git_commit_files_extension", "git_commit_files", ["extension"])
