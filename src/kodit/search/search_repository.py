"""Repository for searching for relevant snippets."""

from typing import TypeVar

from sqlalchemy import (
    select,
)
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.indexing.indexing_models import Snippet
from kodit.source.source_models import File

T = TypeVar("T")


class SearchRepository:
    """Repository for searching for relevant snippets."""

    def __init__(self, session: AsyncSession) -> None:
        """Initialize the search repository.

        Args:
            session: The SQLAlchemy async session to use for database operations.

        """
        self.session = session

    async def list_snippet_ids(self) -> list[int]:
        """List all snippet IDs.

        Returns:
            A list of all snippets.

        """
        query = select(Snippet.id)
        rows = await self.session.execute(query)
        return list(rows.scalars().all())

    async def list_snippets_by_ids(self, ids: list[int]) -> list[tuple[File, Snippet]]:
        """List snippets by IDs.

        Returns:
            A list of snippets in the same order as the input IDs.

        """
        query = (
            select(Snippet, File)
            .where(Snippet.id.in_(ids))
            .join(File, Snippet.file_id == File.id)
        )
        rows = await self.session.execute(query)

        # Create a dictionary for O(1) lookup of results by ID
        id_to_result = {snippet.id: (file, snippet) for snippet, file in rows.all()}

        # Return results in the same order as input IDs
        return [id_to_result[i] for i in ids]
