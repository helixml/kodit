# ruff: noqa
"""refactorings

Revision ID: 4b1a3b2c8fa5
Revises: 19f8c7faf8b9
Create Date: 2025-10-29 13:38:10.737704

"""

from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa

from kodit.domain.tracking.trackable import TrackableReferenceType


# revision identifiers, used by Alembic.
revision: str = "4b1a3b2c8fa5"
down_revision: Union[str, None] = "19f8c7faf8b9"
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    """Upgrade schema."""
    op.drop_index("ix_commit_snippets_v2_commit_sha", table_name="commit_snippets_v2")
    op.drop_index("ix_commit_snippets_v2_snippet_sha", table_name="commit_snippets_v2")
    op.drop_table("commit_snippets_v2")
    op.drop_index("ix_snippet_v2_files_blob_sha", table_name="snippet_v2_files")
    op.drop_index("ix_snippet_v2_files_commit_sha", table_name="snippet_v2_files")
    op.drop_index("ix_snippet_v2_files_file_path", table_name="snippet_v2_files")
    op.drop_index("ix_snippet_v2_files_snippet_sha", table_name="snippet_v2_files")
    op.drop_table("snippet_v2_files")
    op.drop_index("ix_snippets_v2_extension", table_name="snippets_v2")
    op.drop_table("snippets_v2")
    op.drop_index("ix_git_tracking_branches_name", table_name="git_tracking_branches")
    op.drop_index(
        "ix_git_tracking_branches_repo_id", table_name="git_tracking_branches"
    )
    op.drop_table("git_tracking_branches")

    # Use batch_alter_table for SQLite compatibility
    with op.batch_alter_table("git_repos", schema=None) as batch_op:
        # Add columns as nullable first
        batch_op.add_column(
            sa.Column("tracking_type", sa.String(length=255), nullable=True)
        )
        batch_op.add_column(
            sa.Column("tracking_name", sa.String(length=255), nullable=True)
        )

    # Set default values for existing rows
    op.execute(
        f"UPDATE git_repos SET tracking_type = '{TrackableReferenceType.BRANCH}' WHERE tracking_type IS NULL"
    )
    op.execute(
        f"UPDATE git_repos SET tracking_name = 'main' WHERE tracking_name IS NULL"
    )

    # Make columns non-nullable using batch_alter_table for SQLite compatibility
    with op.batch_alter_table("git_repos", schema=None) as batch_op:
        batch_op.alter_column("tracking_type", nullable=False)
        batch_op.alter_column("tracking_name", nullable=False)
        batch_op.create_index(
            op.f("ix_git_repos_tracking_name"),
            ["tracking_name"],
            unique=False,
        )
        batch_op.create_index(
            op.f("ix_git_repos_tracking_type"),
            ["tracking_type"],
            unique=False,
        )


def downgrade() -> None:
    """Downgrade schema."""
    op.create_table(
        "snippets_v2",
        sa.Column("sha", sa.VARCHAR(length=64), nullable=False),
        sa.Column("created_at", sa.DATETIME(), nullable=False),
        sa.Column("updated_at", sa.DATETIME(), nullable=False),
        sa.Column("content", sa.TEXT(), nullable=False),
        sa.Column("extension", sa.VARCHAR(length=255), nullable=False),
        sa.PrimaryKeyConstraint("sha"),
    )
    op.create_index(
        "ix_snippets_v2_extension", "snippets_v2", ["extension"], unique=False
    )
    op.create_table(
        "snippet_v2_files",
        sa.Column("id", sa.INTEGER(), nullable=False),
        sa.Column("snippet_sha", sa.VARCHAR(length=64), nullable=False),
        sa.Column("blob_sha", sa.VARCHAR(length=64), nullable=False),
        sa.Column("commit_sha", sa.VARCHAR(length=64), nullable=False),
        sa.Column("file_path", sa.VARCHAR(length=1024), nullable=False),
        sa.ForeignKeyConstraint(
            ["commit_sha", "file_path"],
            ["git_commit_files.commit_sha", "git_commit_files.path"],
        ),
        sa.ForeignKeyConstraint(
            ["snippet_sha"],
            ["snippets_v2.sha"],
        ),
        sa.PrimaryKeyConstraint("id"),
        sa.UniqueConstraint(
            "snippet_sha",
            "blob_sha",
            "commit_sha",
            "file_path",
            name="uix_snippet_file",
        ),
    )
    op.create_index(
        "ix_snippet_v2_files_snippet_sha",
        "snippet_v2_files",
        ["snippet_sha"],
        unique=False,
    )
    op.create_index(
        "ix_snippet_v2_files_file_path", "snippet_v2_files", ["file_path"], unique=False
    )
    op.create_index(
        "ix_snippet_v2_files_commit_sha",
        "snippet_v2_files",
        ["commit_sha"],
        unique=False,
    )
    op.create_index(
        "ix_snippet_v2_files_blob_sha", "snippet_v2_files", ["blob_sha"], unique=False
    )
    op.create_table(
        "commit_snippets_v2",
        sa.Column("id", sa.INTEGER(), nullable=False),
        sa.Column("commit_sha", sa.VARCHAR(length=64), nullable=False),
        sa.Column("snippet_sha", sa.VARCHAR(length=64), nullable=False),
        sa.ForeignKeyConstraint(
            ["commit_sha"],
            ["git_commits.commit_sha"],
        ),
        sa.ForeignKeyConstraint(
            ["snippet_sha"],
            ["snippets_v2.sha"],
        ),
        sa.PrimaryKeyConstraint("id"),
        sa.UniqueConstraint("commit_sha", "snippet_sha", name="uix_commit_snippet"),
    )
    op.create_index(
        "ix_commit_snippets_v2_snippet_sha",
        "commit_snippets_v2",
        ["snippet_sha"],
        unique=False,
    )
    op.create_index(
        "ix_commit_snippets_v2_commit_sha",
        "commit_snippets_v2",
        ["commit_sha"],
        unique=False,
    )
    # Use batch_alter_table for SQLite compatibility
    with op.batch_alter_table("git_repos", schema=None) as batch_op:
        batch_op.drop_index(op.f("ix_git_repos_tracking_type"))
        batch_op.drop_index(op.f("ix_git_repos_tracking_name"))
        batch_op.drop_column("tracking_name")
        batch_op.drop_column("tracking_type")

    op.create_table(
        "git_tracking_branches",
        sa.Column("repo_id", sa.INTEGER(), nullable=False),
        sa.Column("name", sa.VARCHAR(length=255), nullable=False),
        sa.Column("created_at", sa.DATETIME(), nullable=False),
        sa.Column("updated_at", sa.DATETIME(), nullable=False),
        sa.ForeignKeyConstraint(
            ["repo_id"], ["git_repos.id"], name="fk_tracking_branch_repo"
        ),
        sa.PrimaryKeyConstraint("repo_id", "name", name="pk_git_tracking_branches"),
    )
    op.create_index(
        "ix_git_tracking_branches_repo_id",
        "git_tracking_branches",
        ["repo_id"],
        unique=False,
    )
    op.create_index(
        "ix_git_tracking_branches_name",
        "git_tracking_branches",
        ["name"],
        unique=False,
    )
