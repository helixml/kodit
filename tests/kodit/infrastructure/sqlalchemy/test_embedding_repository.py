"""Tests for the SQLAlchemy embedding repository."""

import numpy as np
import pytest
from unittest.mock import AsyncMock, MagicMock

from kodit.domain.entities import Embedding, EmbeddingType
from kodit.infrastructure.sqlalchemy.embedding_repository import (
    SqlAlchemyEmbeddingRepository,
)


class TestSqlAlchemyEmbeddingRepository:
    """Test the SQLAlchemy embedding repository."""

    def test_init(self):
        """Test initialization."""
        mock_session = MagicMock()
        repository = SqlAlchemyEmbeddingRepository(session=mock_session)
        assert repository.session == mock_session

    @pytest.mark.asyncio
    async def test_create_embedding(self):
        """Test creating an embedding."""
        mock_session = MagicMock()
        mock_session.commit = AsyncMock()

        repository = SqlAlchemyEmbeddingRepository(session=mock_session)

        embedding = Embedding()
        embedding.snippet_id = 1
        embedding.type = EmbeddingType.CODE
        embedding.embedding = [0.1, 0.2, 0.3]

        result = await repository.create_embedding(embedding)

        assert result == embedding
        mock_session.add.assert_called_once_with(embedding)
        mock_session.commit.assert_not_called()

    @pytest.mark.asyncio
    async def test_get_embedding_by_snippet_id_and_type_found(self):
        """Test getting an embedding that exists."""
        mock_session = MagicMock()
        mock_result = MagicMock()
        mock_result.scalar_one_or_none.return_value = MagicMock()
        mock_session.execute = AsyncMock(return_value=mock_result)

        repository = SqlAlchemyEmbeddingRepository(session=mock_session)

        result = await repository.get_embedding_by_snippet_id_and_type(
            1, EmbeddingType.CODE
        )

        assert result is not None
        mock_session.execute.assert_called_once()

    @pytest.mark.asyncio
    async def test_get_embedding_by_snippet_id_and_type_not_found(self):
        """Test getting an embedding that doesn't exist."""
        mock_session = MagicMock()
        mock_result = MagicMock()
        mock_result.scalar_one_or_none.return_value = None
        mock_session.execute = AsyncMock(return_value=mock_result)

        repository = SqlAlchemyEmbeddingRepository(session=mock_session)

        result = await repository.get_embedding_by_snippet_id_and_type(
            1, EmbeddingType.CODE
        )

        assert result is None
        mock_session.execute.assert_called_once()

    @pytest.mark.asyncio
    async def test_list_semantic_results_empty_database(self):
        """Test semantic search with empty database."""
        mock_session = MagicMock()
        mock_result = MagicMock()
        mock_result.all.return_value = []
        mock_session.execute = AsyncMock(return_value=mock_result)

        repository = SqlAlchemyEmbeddingRepository(session=mock_session)

        query_embedding = [0.1, 0.2, 0.3]
        results = await repository.list_semantic_results(
            EmbeddingType.CODE, query_embedding, top_k=10
        )

        assert results == []

    @pytest.mark.asyncio
    async def test_list_semantic_results_single_embedding(self):
        """Test semantic search with a single embedding."""
        mock_session = MagicMock()
        mock_result = MagicMock()
        mock_result.all.return_value = [(1, [0.1, 0.2, 0.3])]
        mock_session.execute = AsyncMock(return_value=mock_result)

        repository = SqlAlchemyEmbeddingRepository(session=mock_session)

        query_embedding = [0.1, 0.2, 0.3]
        results = await repository.list_semantic_results(
            EmbeddingType.CODE, query_embedding, top_k=10
        )

        assert len(results) == 1
        assert results[0][0] == 1  # snippet_id
        assert isinstance(results[0][1], float)  # score
        assert 0 <= results[0][1] <= 1  # cosine similarity should be in [0, 1]

    @pytest.mark.asyncio
    async def test_list_semantic_results_multiple_embeddings(self):
        """Test semantic search with multiple embeddings."""
        mock_session = MagicMock()
        mock_result = MagicMock()
        # Create embeddings with different similarities
        mock_result.all.return_value = [
            (1, [1.0, 0.0, 0.0]),  # Very similar to query
            (2, [0.0, 1.0, 0.0]),  # Less similar
            (3, [0.0, 0.0, 1.0]),  # Least similar
        ]
        mock_session.execute = AsyncMock(return_value=mock_result)

        repository = SqlAlchemyEmbeddingRepository(session=mock_session)

        query_embedding = [1.0, 0.0, 0.0]  # Should match embedding 1 best
        results = await repository.list_semantic_results(
            EmbeddingType.CODE, query_embedding, top_k=3
        )

        assert len(results) == 3
        # Results should be sorted by similarity (highest first)
        assert results[0][0] == 1  # Most similar
        assert results[0][1] == pytest.approx(1.0, abs=1e-6)  # Perfect similarity

    @pytest.mark.asyncio
    async def test_list_semantic_results_top_k_limit(self):
        """Test that top_k limits the number of results."""
        mock_session = MagicMock()
        mock_result = MagicMock()
        mock_result.all.return_value = [(i, [0.1, 0.2, 0.3]) for i in range(10)]
        mock_session.execute = AsyncMock(return_value=mock_result)

        repository = SqlAlchemyEmbeddingRepository(session=mock_session)

        query_embedding = [0.1, 0.2, 0.3]
        results = await repository.list_semantic_results(
            EmbeddingType.CODE, query_embedding, top_k=5
        )

        assert len(results) == 5

    @pytest.mark.asyncio
    async def test_list_semantic_results_inhomogeneous_embeddings(self):
        """Test handling of embeddings with different dimensions."""
        mock_session = MagicMock()
        mock_result = MagicMock()
        # Create embeddings with different dimensions
        mock_result.all.return_value = [
            (1, [0.1, 0.2, 0.3]),  # 3 dimensions
            (2, [0.1, 0.2]),  # 2 dimensions - different!
        ]
        mock_session.execute = AsyncMock(return_value=mock_result)

        repository = SqlAlchemyEmbeddingRepository(session=mock_session)

        query_embedding = [0.1, 0.2, 0.3]

        with pytest.raises(ValueError, match="different sizes"):
            await repository.list_semantic_results(
                EmbeddingType.CODE, query_embedding, top_k=10
            )

    @pytest.mark.asyncio
    async def test_list_semantic_results_zero_vectors(self):
        """Test handling of zero vectors."""
        mock_session = MagicMock()
        mock_result = MagicMock()
        mock_result.all.return_value = [
            (1, [0.0, 0.0, 0.0]),  # Zero vector
            (2, [0.1, 0.2, 0.3]),  # Non-zero vector
        ]
        mock_session.execute = AsyncMock(return_value=mock_result)

        repository = SqlAlchemyEmbeddingRepository(session=mock_session)

        query_embedding = [0.1, 0.2, 0.3]

        # Should handle zero vectors gracefully
        results = await repository.list_semantic_results(
            EmbeddingType.CODE, query_embedding, top_k=10
        )

        assert len(results) == 2
        # Results should be sorted by similarity (highest first)
        # Non-zero vector should be first (higher similarity)
        assert results[0][0] == 2  # Non-zero vector snippet_id
        assert results[0][1] > 0  # Should have positive similarity
        # Zero vector should be second (lower similarity)
        assert results[1][0] == 1  # Zero vector snippet_id
        assert results[1][1] == 0.0  # Should have zero similarity

    def test_prepare_vectors(self):
        """Test vector preparation."""
        repository = SqlAlchemyEmbeddingRepository(session=MagicMock())

        embeddings = [
            (1, [0.1, 0.2, 0.3]),
            (2, [0.4, 0.5, 0.6]),
        ]
        query_embedding = [0.7, 0.8, 0.9]

        stored_vecs, query_vec = repository._prepare_vectors(
            embeddings, query_embedding
        )

        assert isinstance(stored_vecs, np.ndarray)
        assert isinstance(query_vec, np.ndarray)
        assert stored_vecs.shape == (2, 3)
        assert query_vec.shape == (3,)

    def test_compute_similarities(self):
        """Test similarity computation."""
        repository = SqlAlchemyEmbeddingRepository(session=MagicMock())

        stored_vecs = np.array(
            [
                [1.0, 0.0, 0.0],  # Unit vector in x direction
                [0.0, 1.0, 0.0],  # Unit vector in y direction
            ]
        )
        query_vec = np.array([1.0, 0.0, 0.0])  # Unit vector in x direction

        similarities = repository._compute_similarities(stored_vecs, query_vec)

        assert isinstance(similarities, np.ndarray)
        assert len(similarities) == 2
        assert similarities[0] == pytest.approx(1.0, abs=1e-6)  # Perfect similarity
        assert similarities[1] == pytest.approx(0.0, abs=1e-6)  # Orthogonal

    def test_get_top_k_results(self):
        """Test top-k result selection."""
        repository = SqlAlchemyEmbeddingRepository(session=MagicMock())

        similarities = np.array([0.5, 0.9, 0.3, 0.7])
        embeddings = [
            (1, [0.1, 0.2, 0.3]),
            (2, [0.4, 0.5, 0.6]),
            (3, [0.7, 0.8, 0.9]),
            (4, [1.0, 1.1, 1.2]),
        ]

        results = repository._get_top_k_results(similarities, embeddings, top_k=3)

        assert len(results) == 3
        # Should be sorted by similarity (highest first)
        assert results[0][0] == 2  # snippet_id with highest similarity (0.9)
        assert results[0][1] == pytest.approx(0.9, abs=1e-6)
        assert results[1][0] == 4  # snippet_id with second highest similarity (0.7)
        assert results[1][1] == pytest.approx(0.7, abs=1e-6)
        assert results[2][0] == 1  # snippet_id with third highest similarity (0.5)
        assert results[2][1] == pytest.approx(0.5, abs=1e-6)

    @pytest.mark.asyncio
    async def test_list_embedding_values(self):
        """Test listing embedding values from database."""
        mock_session = MagicMock()
        mock_result = MagicMock()
        mock_result.all.return_value = [
            (1, [0.1, 0.2, 0.3]),
            (2, [0.4, 0.5, 0.6]),
        ]
        mock_session.execute = AsyncMock(return_value=mock_result)

        repository = SqlAlchemyEmbeddingRepository(session=mock_session)

        results = await repository._list_embedding_values(EmbeddingType.CODE)

        assert len(results) == 2
        assert results[0] == (1, [0.1, 0.2, 0.3])
        assert results[1] == (2, [0.4, 0.5, 0.6])
        mock_session.execute.assert_called_once()
