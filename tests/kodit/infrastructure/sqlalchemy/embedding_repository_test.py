"""Tests for the SQLAlchemy embedding repository."""

from unittest.mock import AsyncMock, MagicMock

import numpy as np
import pytest

from kodit.infrastructure.sqlalchemy.embedding_repository import (
    SqlAlchemyEmbeddingRepository,
)
from kodit.infrastructure.sqlalchemy.entities import Embedding, EmbeddingType


class TestSqlAlchemyEmbeddingRepository:
    """Test the SQLAlchemy embedding repository."""

    @pytest.mark.asyncio
    async def test_create_embedding(self) -> None:
        """Test creating an embedding."""
        mock_session_factory = MagicMock()
        mock_session = AsyncMock()
        mock_session.add = MagicMock()  # add() is sync
        mock_session.get = AsyncMock(return_value=None)  # No existing entity
        mock_session.flush = AsyncMock()  # flush() is async
        mock_session.commit = AsyncMock()  # commit() is async
        mock_session.close = AsyncMock()   # close() is async
        mock_session_factory.return_value = mock_session

        repository = SqlAlchemyEmbeddingRepository(session_factory=mock_session_factory)

        embedding = Embedding()
        embedding.snippet_id = "1"
        embedding.type = EmbeddingType.CODE
        embedding.embedding = [0.1, 0.2, 0.3]

        await repository.create_embedding(embedding)

        # Verify the embedding was added
        # (not the exact same object due to to_db mapping)
        assert mock_session.add.call_count == 1
        # Verify get was NOT called since the entity has no ID (None)
        mock_session.get.assert_not_called()
        # Verify flush was called
        mock_session.flush.assert_called_once()

    @pytest.mark.asyncio
    async def test_list_semantic_results_empty_database(self) -> None:
        """Test semantic search with empty database."""
        mock_session_factory = MagicMock()
        mock_session = AsyncMock()
        mock_session_factory.return_value = mock_session

        mock_result = MagicMock()
        mock_result.all.return_value = []
        mock_session.execute = AsyncMock(return_value=mock_result)

        repository = SqlAlchemyEmbeddingRepository(session_factory=mock_session_factory)

        query_embedding = [0.1, 0.2, 0.3]
        results = await repository.list_semantic_results(
            EmbeddingType.CODE, query_embedding, top_k=10
        )

        assert results == []

    @pytest.mark.asyncio
    async def test_list_semantic_results_single_embedding(self) -> None:
        """Test semantic search with a single embedding."""
        mock_session_factory = MagicMock()
        mock_session = AsyncMock()
        mock_session_factory.return_value = mock_session

        mock_result = MagicMock()
        mock_result.all.return_value = [("1", [0.1, 0.2, 0.3])]
        mock_session.execute = AsyncMock(return_value=mock_result)

        repository = SqlAlchemyEmbeddingRepository(session_factory=mock_session_factory)

        query_embedding = [0.1, 0.2, 0.3]
        results = await repository.list_semantic_results(
            EmbeddingType.CODE, query_embedding, top_k=10
        )

        assert len(results) == 1
        assert results[0][0] == "1"  # snippet_id
        assert isinstance(results[0][1], float)  # score
        assert 0 <= results[0][1] <= 1  # cosine similarity should be in [0, 1]

    @pytest.mark.asyncio
    async def test_list_semantic_results_multiple_embeddings(self) -> None:
        """Test semantic search with multiple embeddings."""
        mock_session_factory = MagicMock()
        mock_session = AsyncMock()
        mock_session_factory.return_value = mock_session

        mock_result = MagicMock()
        # Create embeddings with different similarities
        mock_result.all.return_value = [
            ("1", [1.0, 0.0, 0.0]),  # Very similar to query
            ("2", [0.0, 1.0, 0.0]),  # Less similar
            ("3", [0.0, 0.0, 1.0]),  # Least similar
        ]
        mock_session.execute = AsyncMock(return_value=mock_result)

        repository = SqlAlchemyEmbeddingRepository(session_factory=mock_session_factory)

        query_embedding = [1.0, 0.0, 0.0]  # Should match embedding 1 best
        results = await repository.list_semantic_results(
            EmbeddingType.CODE, query_embedding, top_k=3
        )

        assert len(results) == 3
        # Results should be sorted by similarity (highest first)
        assert results[0][0] == "1"  # Most similar
        assert results[0][1] == pytest.approx(1.0, abs=1e-6)  # Perfect similarity

    @pytest.mark.asyncio
    async def test_list_semantic_results_top_k_limit(self) -> None:
        """Test that top_k limits the number of results."""
        mock_session_factory = MagicMock()
        mock_session = AsyncMock()
        mock_session_factory.return_value = mock_session

        mock_result = MagicMock()
        mock_result.all.return_value = [(str(i), [0.1, 0.2, 0.3]) for i in range(10)]
        mock_session.execute = AsyncMock(return_value=mock_result)

        repository = SqlAlchemyEmbeddingRepository(session_factory=mock_session_factory)

        query_embedding = [0.1, 0.2, 0.3]
        results = await repository.list_semantic_results(
            EmbeddingType.CODE, query_embedding, top_k=5
        )

        assert len(results) == 5

    @pytest.mark.asyncio
    async def test_list_semantic_results_inhomogeneous_embeddings(self) -> None:
        """Test handling of embeddings with different dimensions."""
        mock_session_factory = MagicMock()
        mock_session = AsyncMock()
        mock_session_factory.return_value = mock_session

        mock_result = MagicMock()
        # Create embeddings with different dimensions
        mock_result.all.return_value = [
            ("1", [0.1, 0.2, 0.3]),  # 3 dimensions
            ("2", [0.1, 0.2]),  # 2 dimensions - different!
        ]
        mock_session.execute = AsyncMock(return_value=mock_result)

        repository = SqlAlchemyEmbeddingRepository(session_factory=mock_session_factory)

        query_embedding = [0.1, 0.2, 0.3]

        with pytest.raises(ValueError, match="different sizes"):
            await repository.list_semantic_results(
                EmbeddingType.CODE, query_embedding, top_k=10
            )

    @pytest.mark.asyncio
    async def test_list_semantic_results_zero_vectors(self) -> None:
        """Test handling of zero vectors."""
        mock_session_factory = MagicMock()
        mock_session = AsyncMock()
        mock_session_factory.return_value = mock_session

        mock_result = MagicMock()
        mock_result.all.return_value = [
            ("1", [0.0, 0.0, 0.0]),  # Zero vector
            ("2", [0.1, 0.2, 0.3]),  # Non-zero vector
        ]
        mock_session.execute = AsyncMock(return_value=mock_result)

        repository = SqlAlchemyEmbeddingRepository(session_factory=mock_session_factory)

        query_embedding = [0.1, 0.2, 0.3]

        # Should handle zero vectors gracefully
        results = await repository.list_semantic_results(
            EmbeddingType.CODE, query_embedding, top_k=10
        )

        assert len(results) == 2
        # Results should be sorted by similarity (highest first)
        # Non-zero vector should be first (higher similarity)
        assert results[0][0] == "2"  # Non-zero vector snippet_id
        assert results[0][1] > 0  # Should have positive similarity
        # Zero vector should be second (lower similarity)
        assert results[1][0] == "1"  # Zero vector snippet_id
        assert results[1][1] == 0.0  # Should have zero similarity

    def test_prepare_vectors(self) -> None:
        """Test vector preparation."""
        repository = SqlAlchemyEmbeddingRepository(session_factory=MagicMock())

        embeddings = [
            ("1", [0.1, 0.2, 0.3]),
            ("2", [0.4, 0.5, 0.6]),
        ]
        query_embedding = [0.7, 0.8, 0.9]

        stored_vecs, query_vec = repository._prepare_vectors(  # noqa: SLF001
            embeddings, query_embedding
        )

        assert isinstance(stored_vecs, np.ndarray)
        assert isinstance(query_vec, np.ndarray)
        assert stored_vecs.shape == (2, 3)
        assert query_vec.shape == (3,)

    def test_compute_similarities(self) -> None:
        """Test similarity computation."""
        repository = SqlAlchemyEmbeddingRepository(session_factory=MagicMock())

        stored_vecs = np.array(
            [
                [1.0, 0.0, 0.0],  # Unit vector in x direction
                [0.0, 1.0, 0.0],  # Unit vector in y direction
            ]
        )
        query_vec = np.array([1.0, 0.0, 0.0])  # Unit vector in x direction

        similarities = repository._compute_similarities(stored_vecs, query_vec)  # noqa: SLF001

        assert isinstance(similarities, np.ndarray)
        assert len(similarities) == 2
        assert similarities[0] == pytest.approx(1.0, abs=1e-6)  # Perfect similarity
        assert similarities[1] == pytest.approx(0.0, abs=1e-6)  # Orthogonal

    def test_get_top_k_results(self) -> None:
        """Test top-k result selection."""
        repository = SqlAlchemyEmbeddingRepository(session_factory=MagicMock())

        similarities = np.array([0.5, 0.9, 0.3, 0.7])
        embeddings = [
            ("1", [0.1, 0.2, 0.3]),
            ("2", [0.4, 0.5, 0.6]),
            ("3", [0.7, 0.8, 0.9]),
            ("4", [1.0, 1.1, 1.2]),
        ]

        results = repository._get_top_k_results(similarities, embeddings, top_k=3)  # noqa: SLF001

        assert len(results) == 3
        # Should be sorted by similarity (highest first)
        assert results[0][0] == "2"  # snippet_id with highest similarity (0.9)
        assert results[0][1] == pytest.approx(0.9, abs=1e-6)
        assert results[1][0] == "4"  # snippet_id with second highest similarity (0.7)
        assert results[1][1] == pytest.approx(0.7, abs=1e-6)
        assert results[2][0] == "1"  # snippet_id with third highest similarity (0.5)
        assert results[2][1] == pytest.approx(0.5, abs=1e-6)

    @pytest.mark.asyncio
    async def test_list_embedding_values(self) -> None:
        """Test listing embedding values from database."""
        mock_session_factory = MagicMock()
        mock_session = AsyncMock()
        mock_session_factory.return_value = mock_session

        mock_result = MagicMock()
        mock_result.all.return_value = [
            ("1", [0.1, 0.2, 0.3]),
            ("2", [0.4, 0.5, 0.6]),
        ]
        mock_session.execute = AsyncMock(return_value=mock_result)

        repository = SqlAlchemyEmbeddingRepository(session_factory=mock_session_factory)

        results = await repository._list_embedding_values(EmbeddingType.CODE)  # noqa: SLF001

        assert len(results) == 2
        assert results[0] == ("1", [0.1, 0.2, 0.3])
        assert results[1] == ("2", [0.4, 0.5, 0.6])

    @pytest.mark.asyncio
    async def test_list_semantic_results_with_snippet_ids_filtering(self) -> None:
        """Test semantic search with snippet_ids filtering."""
        mock_session_factory = MagicMock()
        mock_session = AsyncMock()
        mock_session_factory.return_value = mock_session

        mock_result = MagicMock()
        # Only return embeddings for specific snippet IDs
        mock_result.all.return_value = [("1", [0.1, 0.2, 0.3])]
        mock_session.execute = AsyncMock(return_value=mock_result)

        repository = SqlAlchemyEmbeddingRepository(session_factory=mock_session_factory)

        query_embedding = [0.1, 0.2, 0.3]
        results = await repository.list_semantic_results(
            EmbeddingType.CODE, query_embedding, top_k=10, snippet_ids=["1", "2"]
        )

        assert len(results) == 1
        assert results[0][0] == "1"  # snippet_id
        assert isinstance(results[0][1], float)  # score

    @pytest.mark.asyncio
    async def test_list_semantic_results_with_none_snippet_ids_no_filtering(
        self,
    ) -> None:
        """Test semantic search with None snippet_ids (no filtering)."""
        mock_session_factory = MagicMock()
        mock_session = AsyncMock()
        mock_session_factory.return_value = mock_session

        mock_result = MagicMock()
        mock_result.all.return_value = [
            ("1", [0.1, 0.2, 0.3]),
            ("2", [0.4, 0.5, 0.6]),
        ]
        mock_session.execute = AsyncMock(return_value=mock_result)

        repository = SqlAlchemyEmbeddingRepository(session_factory=mock_session_factory)

        query_embedding = [0.1, 0.2, 0.3]
        results = await repository.list_semantic_results(
            EmbeddingType.CODE, query_embedding, top_k=10, snippet_ids=None
        )

        assert len(results) == 2
        assert results[0][0] == "1"  # snippet_id
        assert results[1][0] == "2"  # snippet_id

    @pytest.mark.asyncio
    async def test_list_semantic_results_with_empty_snippet_ids_returns_no_results(
        self,
    ) -> None:
        """Test semantic search with empty snippet_ids list returns no results."""
        mock_session_factory = MagicMock()
        mock_session = AsyncMock()
        mock_session_factory.return_value = mock_session

        mock_result = MagicMock()
        mock_result.all.return_value = []  # No results when filtering by empty list
        mock_session.execute = AsyncMock(return_value=mock_result)

        repository = SqlAlchemyEmbeddingRepository(session_factory=mock_session_factory)

        query_embedding = [0.1, 0.2, 0.3]
        results = await repository.list_semantic_results(
            EmbeddingType.CODE, query_embedding, top_k=10, snippet_ids=[]
        )

        assert results == []

    @pytest.mark.asyncio
    async def test_list_embedding_values_with_snippet_ids_filtering(self) -> None:
        """Test listing embedding values with snippet_ids filtering."""
        mock_session_factory = MagicMock()
        mock_session = AsyncMock()
        mock_session_factory.return_value = mock_session

        mock_result = MagicMock()
        mock_result.all.return_value = [("1", [0.1, 0.2, 0.3])]
        mock_session.execute = AsyncMock(return_value=mock_result)

        repository = SqlAlchemyEmbeddingRepository(session_factory=mock_session_factory)

        results = await repository._list_embedding_values(  # noqa: SLF001
            EmbeddingType.CODE, snippet_ids=["1", "2"]
        )

        assert len(results) == 1
        assert results[0] == ("1", [0.1, 0.2, 0.3])

    @pytest.mark.asyncio
    async def test_list_embedding_values_with_none_snippet_ids_no_filtering(
        self,
    ) -> None:
        """Test listing embedding values with None snippet_ids (no filtering)."""
        mock_session_factory = MagicMock()
        mock_session = AsyncMock()
        mock_session_factory.return_value = mock_session

        mock_result = MagicMock()
        mock_result.all.return_value = [
            ("1", [0.1, 0.2, 0.3]),
            ("2", [0.4, 0.5, 0.6]),
        ]
        mock_session.execute = AsyncMock(return_value=mock_result)

        repository = SqlAlchemyEmbeddingRepository(session_factory=mock_session_factory)

        results = await repository._list_embedding_values(  # noqa: SLF001
            EmbeddingType.CODE, snippet_ids=None
        )

        assert len(results) == 2
        assert results[0] == ("1", [0.1, 0.2, 0.3])
        assert results[1] == ("2", [0.4, 0.5, 0.6])


class TestSqlAlchemyEmbeddingRepositoryIntegration:
    """Integration tests using real database."""

    @pytest.mark.asyncio
    async def test_create_and_get_embedding(
        self, session_factory  # noqa: ANN001
    ) -> None:
        """Test creating and retrieving an embedding."""
        repository = SqlAlchemyEmbeddingRepository(session_factory=session_factory)

        embedding = Embedding()
        embedding.snippet_id = "1"
        embedding.type = EmbeddingType.CODE
        embedding.embedding = [0.1, 0.2, 0.3]

        await repository.create_embedding(embedding)

        result = await repository.get_embedding_by_snippet_id_and_type(
            1, EmbeddingType.CODE
        )

        assert result is not None
        assert result.snippet_id == "1"
        assert result.type == EmbeddingType.CODE
        assert result.embedding == [0.1, 0.2, 0.3]

    @pytest.mark.asyncio
    async def test_list_embeddings_by_type(
        self, session_factory  # noqa: ANN001
    ) -> None:
        """Test listing embeddings by type."""
        repository = SqlAlchemyEmbeddingRepository(session_factory=session_factory)

        embedding1 = Embedding()
        embedding1.snippet_id = "snippet_1"
        embedding1.type = EmbeddingType.CODE
        embedding1.embedding = [0.1, 0.2, 0.3]

        embedding2 = Embedding()
        embedding2.snippet_id = "snippet_2"
        embedding2.type = EmbeddingType.CODE
        embedding2.embedding = [0.4, 0.5, 0.6]

        embedding3 = Embedding()
        embedding3.snippet_id = "snippet_3"
        embedding3.type = EmbeddingType.TEXT
        embedding3.embedding = [0.7, 0.8, 0.9]

        await repository.create_embedding(embedding1)
        await repository.create_embedding(embedding2)
        await repository.create_embedding(embedding3)

        code_embeddings = await repository.list_embeddings_by_type(EmbeddingType.CODE)

        assert len(code_embeddings) == 2
        snippet_ids = {e.snippet_id for e in code_embeddings}
        assert snippet_ids == {"snippet_1", "snippet_2"}

    @pytest.mark.asyncio
    async def test_semantic_search_basic(
        self, session_factory  # noqa: ANN001
    ) -> None:
        """Test basic semantic search with cosine similarity."""
        repository = SqlAlchemyEmbeddingRepository(session_factory=session_factory)

        embedding1 = Embedding()
        embedding1.snippet_id = "snippet_1"
        embedding1.type = EmbeddingType.CODE
        embedding1.embedding = [1.0, 0.0, 0.0]

        embedding2 = Embedding()
        embedding2.snippet_id = "snippet_2"
        embedding2.type = EmbeddingType.CODE
        embedding2.embedding = [0.0, 1.0, 0.0]

        embedding3 = Embedding()
        embedding3.snippet_id = "snippet_3"
        embedding3.type = EmbeddingType.CODE
        embedding3.embedding = [0.0, 0.0, 1.0]

        await repository.create_embedding(embedding1)
        await repository.create_embedding(embedding2)
        await repository.create_embedding(embedding3)

        query_embedding = [1.0, 0.0, 0.0]
        results = await repository.list_semantic_results(
            EmbeddingType.CODE, query_embedding, top_k=3
        )

        assert len(results) == 3
        assert results[0][0] == "snippet_1"
        assert results[0][1] == pytest.approx(1.0, abs=1e-6)

    @pytest.mark.asyncio
    async def test_delete_embeddings_by_snippet_id(
        self, session_factory  # noqa: ANN001
    ) -> None:
        """Test deleting all embeddings for a snippet."""
        repository = SqlAlchemyEmbeddingRepository(session_factory=session_factory)

        embedding1 = Embedding()
        embedding1.snippet_id = "snippet_to_delete"
        embedding1.type = EmbeddingType.CODE
        embedding1.embedding = [0.1, 0.2, 0.3]

        embedding2 = Embedding()
        embedding2.snippet_id = "snippet_to_delete"
        embedding2.type = EmbeddingType.TEXT
        embedding2.embedding = [0.4, 0.5, 0.6]

        embedding3 = Embedding()
        embedding3.snippet_id = "snippet_to_keep"
        embedding3.type = EmbeddingType.CODE
        embedding3.embedding = [0.7, 0.8, 0.9]

        await repository.create_embedding(embedding1)
        await repository.create_embedding(embedding2)
        await repository.create_embedding(embedding3)

        await repository.delete_embeddings_by_snippet_id("snippet_to_delete")

        # Note: get_embedding_by_snippet_id_and_type uses int, not str
        # Using list method to verify deletion instead
        all_embeddings = await repository.list_embeddings_by_type(EmbeddingType.CODE)
        deleted_count = sum(
            1 for e in all_embeddings if e.snippet_id == "snippet_to_delete"
        )
        assert deleted_count == 0

        kept_count = sum(1 for e in all_embeddings if e.snippet_id == "snippet_to_keep")
        assert kept_count == 1

    @pytest.mark.asyncio
    async def test_list_embeddings_by_snippet_ids_and_type(
        self, session_factory  # noqa: ANN001
    ) -> None:
        """Test getting embeddings for multiple snippet IDs."""
        repository = SqlAlchemyEmbeddingRepository(session_factory=session_factory)

        embedding1 = Embedding()
        embedding1.snippet_id = "snippet_1"
        embedding1.type = EmbeddingType.CODE
        embedding1.embedding = [0.1, 0.2, 0.3]

        embedding2 = Embedding()
        embedding2.snippet_id = "snippet_2"
        embedding2.type = EmbeddingType.CODE
        embedding2.embedding = [0.4, 0.5, 0.6]

        embedding3 = Embedding()
        embedding3.snippet_id = "snippet_3"
        embedding3.type = EmbeddingType.TEXT
        embedding3.embedding = [0.7, 0.8, 0.9]

        await repository.create_embedding(embedding1)
        await repository.create_embedding(embedding2)
        await repository.create_embedding(embedding3)

        results = await repository.list_embeddings_by_snippet_ids_and_type(
            ["snippet_1", "snippet_2"], EmbeddingType.CODE
        )

        assert len(results) == 2
        snippet_ids = {e.snippet_id for e in results}
        assert snippet_ids == {"snippet_1", "snippet_2"}

    @pytest.mark.asyncio
    async def test_get_embeddings_by_snippet_ids(
        self, session_factory  # noqa: ANN001
    ) -> None:
        """Test getting all embeddings for snippet IDs regardless of type."""
        repository = SqlAlchemyEmbeddingRepository(session_factory=session_factory)

        embedding1 = Embedding()
        embedding1.snippet_id = "snippet_1"
        embedding1.type = EmbeddingType.CODE
        embedding1.embedding = [0.1, 0.2, 0.3]

        embedding2 = Embedding()
        embedding2.snippet_id = "snippet_1"
        embedding2.type = EmbeddingType.TEXT
        embedding2.embedding = [0.4, 0.5, 0.6]

        embedding3 = Embedding()
        embedding3.snippet_id = "snippet_2"
        embedding3.type = EmbeddingType.CODE
        embedding3.embedding = [0.7, 0.8, 0.9]

        await repository.create_embedding(embedding1)
        await repository.create_embedding(embedding2)
        await repository.create_embedding(embedding3)

        results = await repository.get_embeddings_by_snippet_ids(["snippet_1"])

        assert len(results) == 2
        types = {e.type for e in results}
        assert types == {EmbeddingType.CODE, EmbeddingType.TEXT}

    @pytest.mark.asyncio
    async def test_semantic_search_with_snippet_id_filter(
        self, session_factory  # noqa: ANN001
    ) -> None:
        """Test semantic search filtered by snippet IDs."""
        repository = SqlAlchemyEmbeddingRepository(session_factory=session_factory)

        embedding1 = Embedding()
        embedding1.snippet_id = "snippet_1"
        embedding1.type = EmbeddingType.CODE
        embedding1.embedding = [1.0, 0.0, 0.0]

        embedding2 = Embedding()
        embedding2.snippet_id = "snippet_2"
        embedding2.type = EmbeddingType.CODE
        embedding2.embedding = [1.0, 0.0, 0.0]

        embedding3 = Embedding()
        embedding3.snippet_id = "snippet_3"
        embedding3.type = EmbeddingType.CODE
        embedding3.embedding = [1.0, 0.0, 0.0]

        await repository.create_embedding(embedding1)
        await repository.create_embedding(embedding2)
        await repository.create_embedding(embedding3)

        query_embedding = [1.0, 0.0, 0.0]
        results = await repository.list_semantic_results(
            EmbeddingType.CODE,
            query_embedding,
            top_k=10,
            snippet_ids=["snippet_1", "snippet_2"],
        )

        assert len(results) == 2
        snippet_ids = {r[0] for r in results}
        assert snippet_ids == {"snippet_1", "snippet_2"}
