"""Repository for searching for relevant snippets."""

from typing import TypeVar

import numpy as np
from sqlalchemy import (
    select,
)
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.embedding.embedding_models import Embedding, EmbeddingType
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

    async def list_semantic_results(
        self, embedding_type: EmbeddingType, embedding: list[float], top_k: int = 10
    ) -> list[tuple[int, float]]:
        """List semantic results using cosine similarity.

        This implementation fetches all embeddings of the given type and computes
        cosine similarity in Python using NumPy for better performance.

        Args:
            embedding_type: The type of embeddings to search
            embedding: The query embedding vector
            top_k: Number of results to return

        Returns:
            List of (snippet_id, similarity_score) tuples, sorted by similarity

        """
        # Step 1: Fetch embeddings from database
        embeddings = await self._list_embedding_values(embedding_type)
        if not embeddings:
            return []

        # Step 2: Convert to numpy arrays
        stored_vecs, query_vec = self._prepare_vectors(embeddings, embedding)

        # Step 3: Compute similarities
        similarities = self._compute_similarities(stored_vecs, query_vec)

        # Step 4: Get top-k results
        return self._get_top_k_results(similarities, embeddings, top_k)

    async def _list_embedding_values(
        self, embedding_type: EmbeddingType
    ) -> list[tuple[int, list[float]]]:
        """List all embeddings of a given type from the database.

        Args:
            embedding_type: The type of embeddings to fetch

        Returns:
            List of (snippet_id, embedding) tuples

        """
        # Only select the fields we need and use a more efficient query
        query = select(Embedding.snippet_id, Embedding.embedding).where(
            Embedding.type == embedding_type
        )
        rows = await self.session.execute(query)
        return [tuple(row) for row in rows.all()]  # Convert Row objects to tuples

    def _prepare_vectors(
        self, embeddings: list[tuple[int, list[float]]], query_embedding: list[float]
    ) -> tuple[np.ndarray, np.ndarray]:
        """Convert embeddings to numpy arrays.

        Args:
            embeddings: List of (snippet_id, embedding) tuples
            query_embedding: Query embedding vector

        Returns:
            Tuple of (stored_vectors, query_vector) as numpy arrays

        """
        try:
            stored_vecs = np.array(
                [emb[1] for emb in embeddings]
            )  # Use index 1 to get embedding
        except ValueError as e:
            if "inhomogeneous" in str(e):
                msg = (
                    "The database has returned embeddings of different sizes. If you"
                    "have recently updated the embedding model, you will need to"
                    "delete your database and re-index your snippets."
                )
                raise ValueError(msg) from e
            raise

        query_vec = np.array(query_embedding)
        return stored_vecs, query_vec

    def _compute_similarities(
        self, stored_vecs: np.ndarray, query_vec: np.ndarray
    ) -> np.ndarray:
        """Compute cosine similarities between stored vectors and query vector.

        Args:
            stored_vecs: Array of stored embedding vectors
            query_vec: Query embedding vector

        Returns:
            Array of similarity scores

        """
        stored_norms = np.linalg.norm(stored_vecs, axis=1)
        query_norm = np.linalg.norm(query_vec)
        return np.dot(stored_vecs, query_vec) / (stored_norms * query_norm)

    def _get_top_k_results(
        self,
        similarities: np.ndarray,
        embeddings: list[tuple[int, list[float]]],
        top_k: int,
    ) -> list[tuple[int, float]]:
        """Get top-k results by similarity score.

        Args:
            similarities: Array of similarity scores
            embeddings: List of (snippet_id, embedding) tuples
            top_k: Number of results to return

        Returns:
            List of (snippet_id, similarity_score) tuples

        """
        top_indices = np.argsort(similarities)[::-1][:top_k]
        return [
            (embeddings[i][0], float(similarities[i])) for i in top_indices
        ]  # Use index 0 to get snippet_id
