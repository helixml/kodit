"""Index repository."""

from sqlalchemy.ext.asyncio import AsyncSession


class IndexRepository:
    """Index repository."""

    def __init__(self, session: AsyncSession) -> None:
        """Initialize the index repository."""
        self.session = session
