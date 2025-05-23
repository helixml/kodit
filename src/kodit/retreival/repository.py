"""Repository for retrieving code snippets and search results.

This module provides the RetrievalRepository class which handles all database operations
related to searching and retrieving code snippets, including string-based searches
and their associated file information.
"""

import math
from typing import Any, TypeVar

import pydantic
from sqlalchemy import (
    ColumnElement,
    Float,
    cast,
    desc,
    func,
    literal,
    select,
)
from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy.orm import Mapped

from kodit.embedding.models import Embedding, EmbeddingType
from kodit.indexing.models import Snippet
from kodit.sources.models import File

T = TypeVar("T")


class RetrievalResult(pydantic.BaseModel):
    """Data transfer object for search results.

    This model represents a single search result, containing both the file path
    and the matching snippet content.
    """

    id: int
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
                id=snippet.id,
                uri=file.uri,
                content=snippet.content,
                score=1.0,
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
            A list of snippets in the same order as the input IDs.

        """
        query = (
            select(Snippet, File)
            .where(Snippet.id.in_(ids))
            .join(File, Snippet.file_id == File.id)
        )
        rows = await self.session.execute(query)

        # Create a dictionary for O(1) lookup of results by ID
        id_to_result = {
            snippet.id: RetrievalResult(
                id=snippet.id,
                uri=file.uri,
                content=snippet.content,
                score=1.0,
            )
            for snippet, file in rows.all()
        }

        # Return results in the same order as input IDs
        return [id_to_result[i] for i in ids]

    async def list_semantic_results(
        self, embedding_type: EmbeddingType, embedding: list[float], top_k: int = 10
    ) -> list[tuple[int, float]]:
        """List semantic results."""
        cosine_similarity = cosine_similarity_json(Embedding.embedding, embedding)

        query = (
            select(Embedding, cosine_similarity)
            .where(Embedding.type == embedding_type)
            .order_by(desc(cosine_similarity))
            .limit(top_k)
        )
        rows = await self.session.execute(query)
        return [(embedding.snippet_id, distance) for embedding, distance in rows.all()]


def cosine_similarity_json(
    col: Mapped[Any], query_vec: list[float]
) -> ColumnElement[Any]:
    """Calculate the cosine similarity using pure sqlalchemy.

    Works for a *fixed-length* vector stored as a JSON array in SQLite.
    The calculation is done entirely in SQL using SQLite's JSON functions.

    Args:
        col: The column containing the JSON array of floats
        query_vec: The query vector to compare against

    Returns:
        A SQLAlchemy expression that computes the cosine similarity

    """
    # Pre-compute query norm
    q_norm = math.sqrt(sum(x * x for x in query_vec))

    # Calculate dot product using JSON array functions
    dot = sum(
        cast(func.json_extract(col, f"$[{i}]"), Float) * literal(float(q))
        for i, q in enumerate(query_vec)
    )

    # Calculate row norm on the fly
    row_norm = func.sqrt(
        sum(
            cast(func.json_extract(col, f"$[{i}]"), Float)
            * cast(func.json_extract(col, f"$[{i}]"), Float)
            for i in range(len(query_vec))
        )
    )

    # Calculate cosine similarity
    return (dot / (row_norm * literal(q_norm))).label("cosine_similarity")
