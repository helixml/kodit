"""SQLAlchemy implementation of embedding repository."""

from collections.abc import Callable

import numpy as np
from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.infrastructure.sqlalchemy.entities import Embedding, EmbeddingType
from kodit.infrastructure.sqlalchemy.unit_of_work import SqlAlchemyUnitOfWork


def create_embedding_repository(
    session_factory: Callable[[], AsyncSession],
) -> "SqlAlchemyEmbeddingRepository":
    """Create an embedding repository."""
    uow = SqlAlchemyUnitOfWork(session_factory=session_factory)
    return SqlAlchemyEmbeddingRepository(uow)


class SqlAlchemyEmbeddingRepository:
    """SQLAlchemy implementation of embedding repository."""

    def __init__(self, uow: SqlAlchemyUnitOfWork) -> None:
        """Initialize the SQLAlchemy embedding repository."""
        self.uow = uow

    async def create_embedding(self, embedding: Embedding) -> None:
        """Create a new embedding record in the database."""
        async with self.uow:
            self.uow.session.add(embedding)

    async def get_embedding_by_snippet_id_and_type(
        self, snippet_id: int, embedding_type: EmbeddingType
    ) -> Embedding | None:
        """Get an embedding by its snippet ID and type."""
        async with self.uow:
            query = select(Embedding).where(
                Embedding.snippet_id == snippet_id,
                Embedding.type == embedding_type,
            )
            result = await self.uow.session.execute(query)
            return result.scalar_one_or_none()

    async def list_embeddings_by_type(
        self, embedding_type: EmbeddingType
    ) -> list[Embedding]:
        """List all embeddings of a given type."""
        async with self.uow:
            query = select(Embedding).where(Embedding.type == embedding_type)
            result = await self.uow.session.execute(query)
            return list(result.scalars())

    async def delete_embeddings_by_snippet_id(self, snippet_id: int) -> None:
        """Delete all embeddings for a snippet."""
        async with self.uow:
            query = select(Embedding).where(Embedding.snippet_id == snippet_id)
            result = await self.uow.session.execute(query)
            embeddings = result.scalars().all()
            for embedding in embeddings:
                await self.uow.session.delete(embedding)

    async def list_semantic_results(
        self,
        embedding_type: EmbeddingType,
        embedding: list[float],
        top_k: int = 10,
        snippet_ids: list[int] | None = None,
    ) -> list[tuple[int, float]]:
        """List semantic results using cosine similarity.

        This implementation fetches all embeddings of the given type and computes
        cosine similarity in Python using NumPy for better performance.

        Args:
            embedding_type: The type of embeddings to search
            embedding: The query embedding vector
            top_k: Number of results to return
            snippet_ids: Optional list of snippet IDs to filter by

        Returns:
            List of (snippet_id, similarity_score) tuples, sorted by similarity

        """
        # Step 1: Fetch embeddings from database
        embeddings = await self._list_embedding_values(embedding_type, snippet_ids)
        if not embeddings:
            return []

        # Step 2: Convert to numpy arrays
        stored_vecs, query_vec = self._prepare_vectors(embeddings, embedding)

        # Step 3: Compute similarities
        similarities = self._compute_similarities(stored_vecs, query_vec)

        # Step 4: Get top-k results
        return self._get_top_k_results(similarities, embeddings, top_k)

    async def _list_embedding_values(
        self, embedding_type: EmbeddingType, snippet_ids: list[int] | None = None
    ) -> list[tuple[int, list[float]]]:
        """List all embeddings of a given type from the database.

        Args:
            embedding_type: The type of embeddings to fetch
            snippet_ids: Optional list of snippet IDs to filter by

        Returns:
            List of (snippet_id, embedding) tuples

        """
        async with self.uow:
            query = select(Embedding.snippet_id, Embedding.embedding).where(
                Embedding.type == embedding_type
            )

            # Add snippet_ids filter if provided
            if snippet_ids is not None:
                query = query.where(Embedding.snippet_id.in_(snippet_ids))

            rows = await self.uow.session.execute(query)
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

        # Handle zero vectors to avoid division by zero
        if query_norm == 0:
            # If query vector is zero, return zeros for all similarities
            return np.zeros(len(stored_vecs))

        # Handle stored vectors with zero norms
        zero_stored_mask = stored_norms == 0
        similarities = np.zeros(len(stored_vecs))

        # Only compute similarities for non-zero stored vectors
        non_zero_mask = ~zero_stored_mask
        if np.any(non_zero_mask):
            non_zero_stored_vecs = stored_vecs[non_zero_mask]
            non_zero_stored_norms = stored_norms[non_zero_mask]
            non_zero_similarities = np.dot(non_zero_stored_vecs, query_vec) / (
                non_zero_stored_norms * query_norm
            )
            similarities[non_zero_mask] = non_zero_similarities

        return similarities

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
        # Get indices of top-k similarities
        top_indices = np.argsort(similarities)[::-1][:top_k]
        return [(embeddings[i][0], float(similarities[i])) for i in top_indices]
