"""Repository for managing embeddings."""

import numpy as np
from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.embedding.embedding_models import Embedding, EmbeddingType


class EmbeddingRepository:
    """Repository for managing embeddings.

    This class provides methods for creating and retrieving embeddings from the
    database. It handles the low-level database operations and transaction management.

    Args:
        session: The SQLAlchemy async session to use for database operations.

    """

    def __init__(self, session: AsyncSession) -> None:
        """Initialize the embedding repository."""
        self.session = session

    async def create_embedding(self, embedding: Embedding) -> Embedding:
        """Create a new embedding record in the database.

        Args:
            embedding: The Embedding instance to create.

        Returns:
            The created Embedding instance.

        """
        self.session.add(embedding)
        await self.session.commit()
        return embedding

    async def get_embedding_by_snippet_id_and_type(
        self, snippet_id: int, embedding_type: EmbeddingType
    ) -> Embedding | None:
        """Get an embedding by its snippet ID and type.

        Args:
            snippet_id: The ID of the snippet to get the embedding for.
            embedding_type: The type of embedding to get.

        Returns:
            The Embedding instance if found, None otherwise.

        """
        query = select(Embedding).where(
            Embedding.snippet_id == snippet_id,
            Embedding.type == embedding_type,
        )
        result = await self.session.execute(query)
        return result.scalar_one_or_none()

    async def list_embeddings_by_type(
        self, embedding_type: EmbeddingType
    ) -> list[Embedding]:
        """List all embeddings of a given type.

        Args:
            embedding_type: The type of embeddings to list.

        Returns:
            A list of Embedding instances.

        """
        query = select(Embedding).where(Embedding.type == embedding_type)
        result = await self.session.execute(query)
        return list(result.scalars())

    async def delete_embeddings_by_snippet_id(self, snippet_id: int) -> None:
        """Delete all embeddings for a snippet.

        Args:
            snippet_id: The ID of the snippet to delete embeddings for.

        """
        query = select(Embedding).where(Embedding.snippet_id == snippet_id)
        result = await self.session.execute(query)
        embeddings = result.scalars().all()
        for embedding in embeddings:
            await self.session.delete(embedding)
        await self.session.commit()

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
