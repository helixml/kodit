# ruff: noqa
"""add task retry fields

Revision ID: 9d493502b3ac
Revises: af4c96f50d5a
Create Date: 2025-12-22

"""

from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
revision: str = '9d493502b3ac'
down_revision: Union[str, None] = 'af4c96f50d5a'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    """Upgrade schema."""
    with op.batch_alter_table('tasks', schema=None) as batch_op:
        batch_op.add_column(sa.Column('retry_count', sa.Integer(), nullable=False, server_default='0'))
        batch_op.add_column(sa.Column('next_retry_at', sa.DateTime(), nullable=True))


def downgrade() -> None:
    """Downgrade schema."""
    with op.batch_alter_table('tasks', schema=None) as batch_op:
        batch_op.drop_column('next_retry_at')
        batch_op.drop_column('retry_count')
