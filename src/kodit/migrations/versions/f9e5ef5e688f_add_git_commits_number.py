# ruff: noqa
"""add git commits number

Revision ID: f9e5ef5e688f
Revises: 04b80f802e0c
Create Date: 2025-09-23 10:55:15.553741

"""

from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
revision: str = "f9e5ef5e688f"
down_revision: Union[str, None] = "04b80f802e0c"
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    """Upgrade schema."""
    op.add_column(
        "git_repos",
        sa.Column("num_commits", sa.Integer(), nullable=False, server_default="0"),
    )
    op.add_column(
        "git_repos",
        sa.Column("num_branches", sa.Integer(), nullable=False, server_default="0"),
    )
    op.add_column(
        "git_repos",
        sa.Column("num_tags", sa.Integer(), nullable=False, server_default="0"),
    )


def downgrade() -> None:
    """Downgrade schema."""
    op.drop_column("git_repos", "num_commits")
    op.drop_column("git_repos", "num_branches")
    op.drop_column("git_repos", "num_tags")
