"""Repository for retrieving code snippets and search results.

This module provides the RetrievalRepository class which handles all database operations
related to searching and retrieving code snippets, including string-based searches
and their associated file information.
"""

import math
from typing import TypeVar

import pydantic
from sqlalchemy import (
    Float,
    cast,
    func,
    literal,
    select,
)
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.indexing.models import Embedding, Snippet
from kodit.sources.models import File

T = TypeVar("T")


class RetrievalResult(pydantic.BaseModel):
    """Data transfer object for search results.

    This model represents a single search result, containing both the file path
    and the matching snippet content.
    """

    uri: str
    content: str
    score: float


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
                score=1.0,
            )
            for snippet, file in rows.all()
        ]

    async def list_semantic_results(
        self, embedding: list[float], top_k: int = 10
    ) -> list[tuple[int, float]]:
        """List semantic results."""
        cos_dist = cosine_distance_json(Embedding.embedding, embedding).label(
            "cos_dist"
        )

        query = select(Embedding, cos_dist).order_by(cos_dist).limit(top_k)
        rows = await self.session.execute(query)
        return [(embedding.snippet_id, distance) for embedding, distance in rows.all()]


def cosine_distance_json(col, query_vec):
    """Calculate the cosine distance using pure sqlalchemy.

    Build a SQLAlchemy scalar expression that returns
    1 – cosine_similarity(json_array, query_vec).
    Works for a *fixed-length* vector.
    """
    q_norm = math.sqrt(sum(x * x for x in query_vec))

    dot = sum(
        cast(func.json_extract(col, f"$[{i}]"), Float) * literal(float(q))
        for i, q in enumerate(query_vec)
    )
    row_norm = func.sqrt(
        sum(
            cast(func.json_extract(col, f"$[{i}]"), Float)
            * cast(func.json_extract(col, f"$[{i}]"), Float)
            for i in range(len(query_vec))
        )
    )

    return 1 - dot / (row_norm * literal(q_norm))
