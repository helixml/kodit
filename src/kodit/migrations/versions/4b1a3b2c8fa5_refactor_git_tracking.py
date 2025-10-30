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
