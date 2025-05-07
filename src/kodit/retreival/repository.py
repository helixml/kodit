"""Repository for retrieving code snippets and search results.

This module provides the RetrievalRepository class which handles all database operations
related to searching and retrieving code snippets, including string-based searches
and their associated file information.
"""

from typing import Any, TypeVar

import pydantic
from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy.sql import Select

from kodit.indexing.models import File, Snippet

T = TypeVar("T")


class RetrievalResult(pydantic.BaseModel):
    """Data transfer object for search results.

    This model represents a single search result, containing both the file path
    and the matching snippet content.
    """

    file_path: str
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

    async def _execute_query(
        self, query: Select[Any], *, return_single: bool = False
    ) -> Any:
        """Execute a SQLAlchemy query and return the results.

        Args:
            query: The SQLAlchemy select query to execute.
            return_single: Whether to return a single result or a list of results.

        Returns:
            The query results, either as a single item or a list.

        """
        result = await self.session.execute(query)
        return result.scalar_one_or_none() if return_single else result.all()

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
            .where(Snippet.content.ilike(f"%{query}%"))
            .outerjoin(File, Snippet.file_id == File.id)
            .order_by(Snippet.created_at)
            .limit(10)
        )
        rows = await self._execute_query(search_query)

        return [
            RetrievalResult(
                file_path=file.path,
                content=snippet.content,
            )
            for (snippet, file) in rows
        ]
