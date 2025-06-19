"""SQLAlchemy implementation of snippet repository."""

from collections.abc import Sequence

from sqlalchemy import delete, select
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities import Snippet
from kodit.domain.repositories import SnippetRepository


class SqlAlchemySnippetRepository(SnippetRepository):
    """SQLAlchemy implementation of snippet repository."""

    def __init__(self, session: AsyncSession) -> None:
        """Initialize the SQLAlchemy snippet repository.

        Args:
            session: The SQLAlchemy async session to use for database operations

        """
        self.session = session

    async def save(self, snippet: Snippet) -> Snippet:
        """Save snippet using SQLAlchemy.

        Args:
            snippet: The snippet to save

        Returns:
            The saved snippet

        """
        self.session.add(snippet)
        await self.session.commit()
        return snippet

    async def get_by_id(self, snippet_id: int) -> Snippet | None:
        """Get a snippet by ID.

        Args:
            snippet_id: The ID of the snippet to retrieve

        Returns:
            The Snippet instance if found, None otherwise

        """
        query = select(Snippet).where(Snippet.id == snippet_id)
        result = await self.session.execute(query)
        return result.scalar_one_or_none()

    async def get_by_index(self, index_id: int) -> Sequence[Snippet]:
        """Get all snippets for an index.

        Args:
            index_id: The ID of the index to get snippets for

        Returns:
            A list of Snippet instances

        """
        query = select(Snippet).where(Snippet.index_id == index_id)
        result = await self.session.execute(query)
        return list(result.scalars())

    async def delete_by_index(self, index_id: int) -> None:
        """Delete all snippets for an index.

        Args:
            index_id: The ID of the index to delete snippets for

        """
        query = delete(Snippet).where(Snippet.index_id == index_id)
        await self.session.execute(query)
        await self.session.commit()
