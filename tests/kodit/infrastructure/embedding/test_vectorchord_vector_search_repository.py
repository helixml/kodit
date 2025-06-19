"""Tests for the VectorChord vector search repository."""

import pytest
from unittest.mock import AsyncMock, MagicMock
from sqlalchemy import text

from kodit.domain.entities import Snippet, EmbeddingType
from kodit.domain.value_objects import (
    VectorSearchRequest,
    VectorSearchResult,
    EmbeddingRequest,
    EmbeddingResponse,
    IndexResult,
    VectorIndexRequest,
    VectorSearchQueryRequest,
)
from kodit.infrastructure.embedding.vectorchord_vector_search_repository import (
    VectorChordVectorSearchRepository,
)


class TestVectorChordVectorSearchRepository:
    """Test the VectorChord vector search repository."""

    def test_init(self):
        """Test initialization."""
        mock_session = MagicMock()
        mock_provider = MagicMock()

        repository = VectorChordVectorSearchRepository(
            task_name="code",
            session=mock_session,
            embedding_provider=mock_provider,
        )

        assert repository.embedding_provider == mock_provider
        assert repository._session == mock_session
        assert repository._initialized is False
        assert repository.table_name == "vectorchord_code_embeddings"
        assert repository.index_name == "vectorchord_code_embeddings_idx"
        assert repository.log is not None

    def test_init_text_task(self):
        """Test initialization with text task."""
        mock_session = MagicMock()
        mock_provider = MagicMock()

        repository = VectorChordVectorSearchRepository(
            task_name="text",
            session=mock_session,
            embedding_provider=mock_provider,
        )

        assert repository.table_name == "vectorchord_text_embeddings"
        assert repository.index_name == "vectorchord_text_embeddings_idx"

    @pytest.mark.asyncio
    async def test_initialize_success(self):
        """Test successful initialization."""
        mock_session = MagicMock()
        mock_session.execute = AsyncMock()
        mock_session.commit = AsyncMock()

        # Mock the result for dimension check
        mock_result = MagicMock()
        mock_result.scalar_one.return_value = 3  # Match the embedding size
        mock_session.execute.return_value = mock_result

        mock_provider = MagicMock()

        async def mock_embed(requests):
            yield [EmbeddingResponse(snippet_id=0, embedding=[0.1, 0.2, 0.3])]

        mock_provider.embed.return_value = mock_embed([])

        repository = VectorChordVectorSearchRepository(
            task_name="code",
            session=mock_session,
            embedding_provider=mock_provider,
        )

        await repository._initialize()

        assert repository._initialized is True
        # Verify extensions and tables were created
        assert mock_session.execute.call_count >= 3  # At least 3 SQL calls
        assert mock_session.commit.call_count >= 1

    @pytest.mark.asyncio
    async def test_initialize_embedding_provider_failure(self):
        """Test initialization when embedding provider fails."""
        mock_session = MagicMock()
        mock_session.execute = AsyncMock()
        mock_session.commit = AsyncMock()

        mock_provider = MagicMock()

        async def mock_embed(requests):
            yield []  # No embeddings returned

        mock_provider.embed.return_value = mock_embed([])

        repository = VectorChordVectorSearchRepository(
            task_name="code",
            session=mock_session,
            embedding_provider=mock_provider,
        )

        with pytest.raises(RuntimeError, match="Failed to obtain embedding dimension"):
            await repository._initialize()

    @pytest.mark.asyncio
    async def test_index_documents_empty_request(self):
        """Test indexing with empty request."""
        mock_session = MagicMock()
        mock_provider = MagicMock()

        repository = VectorChordVectorSearchRepository(
            task_name="code",
            session=mock_session,
            embedding_provider=mock_provider,
        )

        request = VectorIndexRequest(documents=[])

        results = []
        async for batch in repository.index_documents(request):
            results.extend(batch)

        assert len(results) == 0
        mock_provider.embed.assert_not_called()

    @pytest.mark.asyncio
    async def test_index_documents_single_document(self):
        """Test indexing with a single document."""
        mock_session = MagicMock()
        mock_session.execute = AsyncMock()
        mock_session.commit = AsyncMock()

        mock_provider = MagicMock()

        async def mock_embed(requests):
            yield [EmbeddingResponse(snippet_id=1, embedding=[0.1, 0.2, 0.3])]

        mock_provider.embed.return_value = mock_embed([])

        repository = VectorChordVectorSearchRepository(
            task_name="code",
            session=mock_session,
            embedding_provider=mock_provider,
        )

        # Mock initialization
        repository._initialized = True

        request = VectorIndexRequest(
            documents=[VectorSearchRequest(snippet_id=1, text="python programming")]
        )

        results = []
        async for batch in repository.index_documents(request):
            results.extend(batch)

        assert len(results) == 1
        assert results[0].snippet_id == 1

        # Verify embedding provider was called
        mock_provider.embed.assert_called_once()
        call_args = mock_provider.embed.call_args[0][0]
        assert len(call_args) == 1
        assert call_args[0].snippet_id == 1
        assert call_args[0].text == "python programming"

        # Verify database operations
        mock_session.execute.assert_called_once()
        mock_session.commit.assert_called_once()

    @pytest.mark.asyncio
    async def test_index_documents_multiple_documents(self):
        """Test indexing with multiple documents."""
        mock_session = MagicMock()
        mock_session.execute = AsyncMock()
        mock_session.commit = AsyncMock()

        mock_provider = MagicMock()

        async def mock_embed(requests):
            yield [
                EmbeddingResponse(snippet_id=1, embedding=[0.1, 0.2, 0.3]),
                EmbeddingResponse(snippet_id=2, embedding=[0.4, 0.5, 0.6]),
            ]

        mock_provider.embed.return_value = mock_embed([])

        repository = VectorChordVectorSearchRepository(
            task_name="code",
            session=mock_session,
            embedding_provider=mock_provider,
        )

        # Mock initialization
        repository._initialized = True

        request = VectorIndexRequest(
            documents=[
                VectorSearchRequest(snippet_id=1, text="python programming"),
                VectorSearchRequest(snippet_id=2, text="javascript development"),
            ]
        )

        results = []
        async for batch in repository.index_documents(request):
            results.extend(batch)

        assert len(results) == 2
        assert results[0].snippet_id == 1
        assert results[1].snippet_id == 2

        # Verify database operations
        assert mock_session.execute.call_count == 1
        assert mock_session.commit.call_count == 1

    @pytest.mark.asyncio
    async def test_search_success(self):
        """Test successful search."""
        mock_session = MagicMock()
        mock_session.execute = AsyncMock()

        # Mock search results
        mock_result = MagicMock()
        mock_result.mappings.return_value.all.return_value = [
            {"snippet_id": 1, "score": 0.1},
            {"snippet_id": 2, "score": 0.2},
        ]
        mock_session.execute.return_value = mock_result

        mock_provider = MagicMock()

        async def mock_embed(requests):
            yield [EmbeddingResponse(snippet_id=0, embedding=[0.1, 0.2, 0.3])]

        mock_provider.embed.return_value = mock_embed([])

        repository = VectorChordVectorSearchRepository(
            task_name="code",
            session=mock_session,
            embedding_provider=mock_provider,
        )

        # Mock initialization
        repository._initialized = True

        request = VectorSearchQueryRequest(query="python programming", top_k=10)

        results = await repository.search(request)

        assert len(results) == 2
        assert results[0].snippet_id == 1
        assert results[0].score == 0.1
        assert results[1].snippet_id == 2
        assert results[1].score == 0.2

        # Verify embedding provider was called
        mock_provider.embed.assert_called_once()
        call_args = mock_provider.embed.call_args[0][0]
        assert len(call_args) == 1
        assert call_args[0].snippet_id == 0
        assert call_args[0].text == "python programming"

        # Verify database search was called
        mock_session.execute.assert_called_once()

    @pytest.mark.asyncio
    async def test_search_no_embedding_generated(self):
        """Test search when no embedding is generated."""
        mock_session = MagicMock()

        mock_provider = MagicMock()

        async def mock_embed(requests):
            yield []  # No embeddings returned

        mock_provider.embed.return_value = mock_embed([])

        repository = VectorChordVectorSearchRepository(
            task_name="code",
            session=mock_session,
            embedding_provider=mock_provider,
        )

        # Mock initialization
        repository._initialized = True

        request = VectorSearchQueryRequest(query="python programming", top_k=10)

        results = await repository.search(request)

        assert results == []
        mock_session.execute.assert_not_called()

    @pytest.mark.asyncio
    async def test_has_embedding_true(self):
        """Test has_embedding when embedding exists."""
        mock_session = MagicMock()
        mock_session.execute = AsyncMock()

        # Mock result indicating embedding exists
        mock_result = MagicMock()
        mock_result.scalar.return_value = True
        mock_session.execute.return_value = mock_result

        mock_provider = MagicMock()

        repository = VectorChordVectorSearchRepository(
            task_name="code",
            session=mock_session,
            embedding_provider=mock_provider,
        )

        # Mock initialization
        repository._initialized = True

        result = await repository.has_embedding(1, EmbeddingType.CODE)

        assert result is True
        mock_session.execute.assert_called_once()

    @pytest.mark.asyncio
    async def test_has_embedding_false(self):
        """Test has_embedding when embedding doesn't exist."""
        mock_session = MagicMock()
        mock_session.execute = AsyncMock()

        # Mock result indicating embedding doesn't exist
        mock_result = MagicMock()
        mock_result.scalar.return_value = False
        mock_session.execute.return_value = mock_result

        mock_provider = MagicMock()

        repository = VectorChordVectorSearchRepository(
            task_name="code",
            session=mock_session,
            embedding_provider=mock_provider,
        )

        # Mock initialization
        repository._initialized = True

        result = await repository.has_embedding(1, EmbeddingType.TEXT)

        assert result is False
        mock_session.execute.assert_called_once()

    @pytest.mark.asyncio
    async def test_has_embedding_ignores_embedding_type(self):
        """Test that has_embedding ignores embedding_type parameter."""
        mock_session = MagicMock()
        mock_session.execute = AsyncMock()

        mock_result = MagicMock()
        mock_result.scalar.return_value = True
        mock_session.execute.return_value = mock_result

        mock_provider = MagicMock()

        repository = VectorChordVectorSearchRepository(
            task_name="code",
            session=mock_session,
            embedding_provider=mock_provider,
        )

        # Mock initialization
        repository._initialized = True

        # Should work regardless of embedding_type since VectorChord uses separate tables
        result1 = await repository.has_embedding(1, EmbeddingType.CODE)
        result2 = await repository.has_embedding(1, EmbeddingType.TEXT)

        assert result1 is True
        assert result2 is True
        # Both calls should use the same table (code table)
        assert mock_session.execute.call_count == 2

    @pytest.mark.asyncio
    async def test_execute_auto_initializes(self):
        """Test that _execute auto-initializes if not initialized."""
        mock_session = MagicMock()
        mock_session.execute = AsyncMock()
        mock_session.commit = AsyncMock()

        # Mock the result for dimension check
        mock_result = MagicMock()
        mock_result.scalar_one.return_value = 3  # Match the embedding size
        mock_session.execute.return_value = mock_result

        mock_provider = MagicMock()

        async def mock_embed(requests):
            yield [EmbeddingResponse(snippet_id=0, embedding=[0.1, 0.2, 0.3])]

        mock_provider.embed.return_value = mock_embed([])

        repository = VectorChordVectorSearchRepository(
            task_name="code",
            session=mock_session,
            embedding_provider=mock_provider,
        )

        # Should auto-initialize
        await repository._execute(text("SELECT 1"))

        assert repository._initialized is True
        mock_session.execute.assert_called()

    @pytest.mark.asyncio
    async def test_create_tables_dimension_mismatch(self):
        """Test handling of dimension mismatch during table creation."""
        mock_session = MagicMock()
        mock_session.execute = AsyncMock()

        mock_provider = MagicMock()

        async def mock_embed(requests):
            yield [EmbeddingResponse(snippet_id=0, embedding=[0.1, 0.2, 0.3])]

        mock_provider.embed.return_value = mock_embed([])

        # Mock dimension check to return different dimension
        mock_result = MagicMock()
        mock_result.scalar_one.return_value = 5  # Different from embedding size (3)
        mock_session.execute.return_value = mock_result

        repository = VectorChordVectorSearchRepository(
            task_name="code",
            session=mock_session,
            embedding_provider=mock_provider,
        )

        with pytest.raises(
            ValueError, match="Embedding vector dimension does not match"
        ):
            await repository._create_tables()
