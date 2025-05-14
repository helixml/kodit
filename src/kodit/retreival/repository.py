"""Repository for retrieving code snippets and search results.

This module provides the RetrievalRepository class which handles all database operations
related to searching and retrieving code snippets, including string-based searches
and their associated file information.
"""

from typing import TypeVar

import pydantic
from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.indexing.models import Snippet
from kodit.sources.models import File

T = TypeVar("T")


class RetrievalResult(pydantic.BaseModel):
    """Data transfer object for search results.

    This model represents a single search result, containing both the file path
    and the matching snippet content.
    """

    uri: str
    content: str


class RetrievalRepository:
    """Repository for retrieving code snippets and search results.

    This class provides methods for searching and retrieving code snippets from
    the database, including string-based searches and their associated file information.
    """

    def __init__(self, session: AsyncSession) -> None:
        """Initialize the retrieval repository.

        Args:
            session: The SQLAlchemy async session to use for database operations.

        """
        self.session = session

    async def string_search(self, query: str) -> list[RetrievalResult]:
        """Search for snippets containing the given query string.

        This method performs a case-insensitive search for the query string within
        snippet contents, returning up to 10 most recent matches.

        Args:
            query: The string to search for within snippet contents.

        Returns:
            A list of RetrievalResult objects containing the matching snippets
            and their associated file paths.

        """
        search_query = (
            select(Snippet, File)
            .join(File, Snippet.file_id == File.id)
            .where(Snippet.content.ilike(f"%{query}%"))
            .limit(10)
        )
        rows = await self.session.execute(search_query)
        results = list(rows.all())

        return [
            RetrievalResult(
                uri=file.uri,
                content=snippet.content,
            )
            for snippet, file in results
        ]

    async def list_snippet_ids(self) -> list[int]:
        """List all snippet IDs.

        Returns:
            A list of all snippets.

        """
        query = select(Snippet.id)
        rows = await self.session.execute(query)
        return list(rows.scalars().all())

    async def list_snippets_by_ids(self, ids: list[int]) -> list[RetrievalResult]:
        """List snippets by IDs.

        Returns:
            A list of snippets.

        """
        query = (
            select(Snippet, File)
            .where(Snippet.id.in_(ids))
            .join(File, Snippet.file_id == File.id)
        )
        rows = await self.session.execute(query)
        return [
            RetrievalResult(
                uri=file.uri,
                content=snippet.content,
            )
            for snippet, file in rows.all()
        ]
