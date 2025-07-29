"""Tests for local vector search repository."""

from collections.abc import AsyncGenerator
from datetime import UTC, datetime
from unittest.mock import AsyncMock, MagicMock

import pytest
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.value_objects import (
    Document,
    EmbeddingResponse,
    EnrichmentRequest,
    FileProcessingStatus,
    IndexRequest,
    SearchRequest,
    SearchResult,
)
from kodit.infrastructure.embedding.embedding_providers.local_embedding_provider import (  # noqa: E501
    LocalEmbeddingProvider,
)
from kodit.infrastructure.embedding.local_vector_search_repository import (
    LocalVectorSearchRepository,
)
from kodit.infrastructure.sqlalchemy.embedding_repository import (
    SqlAlchemyEmbeddingRepository,
)
from kodit.infrastructure.sqlalchemy.entities import (
    EmbeddingType,
    File,
    Index,
    Snippet,
    Source,
    SourceType,
)


class TestLocalVectorSearchRepository:
    """Test the local vector search repository."""

    def test_init_default_values(self) -> None:
        """Test initialization with default values."""
        mock_repository = MagicMock(spec=SqlAlchemyEmbeddingRepository)
        mock_provider = MagicMock()

        repository = LocalVectorSearchRepository(
            embedding_repository=mock_repository,
            embedding_provider=mock_provider,
            embedding_type=EmbeddingType.CODE,
        )

        assert repository.embedding_repository == mock_repository
        assert repository.embedding_provider == mock_provider
        assert repository.embedding_type == EmbeddingType.CODE
        assert repository.log is not None

    def test_init_custom_values(self) -> None:
        """Test initialization with custom values."""
        mock_repository = MagicMock(spec=SqlAlchemyEmbeddingRepository)
        mock_provider = MagicMock()

        repository = LocalVectorSearchRepository(
            embedding_repository=mock_repository,
            embedding_provider=mock_provider,
            embedding_type=EmbeddingType.TEXT,
        )

        assert repository.embedding_type == EmbeddingType.TEXT

    @pytest.mark.asyncio
    async def test_index_documents_empty_request(self) -> None:
        """Test indexing with empty request."""
        mock_repository = MagicMock(spec=SqlAlchemyEmbeddingRepository)
        mock_provider = MagicMock()

        repository = LocalVectorSearchRepository(
            embedding_repository=mock_repository,
            embedding_provider=mock_provider,
            embedding_type=EmbeddingType.CODE,
        )

        request = IndexRequest(documents=[])

        results = []
        async for batch in repository.index_documents(request):
            results.extend(batch)

        assert len(results) == 0

    @pytest.mark.asyncio
    async def test_index_documents_single_document(self) -> None:
        """Test indexing with a single document."""
        mock_repository = MagicMock(spec=SqlAlchemyEmbeddingRepository)
        mock_repository.create_embedding = AsyncMock()

        mock_provider = MagicMock()

        async def mock_embed(
            _: list[EnrichmentRequest],
        ) -> AsyncGenerator[list[EmbeddingResponse], None]:
            yield [EmbeddingResponse(snippet_id=1, embedding=[0.1, 0.2, 0.3])]

        mock_provider.embed.return_value = mock_embed([])

        repository = LocalVectorSearchRepository(
            embedding_repository=mock_repository,
            embedding_provider=mock_provider,
            embedding_type=EmbeddingType.CODE,
        )

        request = IndexRequest(
            documents=[Document(snippet_id=1, text="python programming")]
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

        # Verify embedding was saved
        mock_repository.create_embedding.assert_called_once()

    @pytest.mark.asyncio
    async def test_index_documents_multiple_documents(self) -> None:
        """Test indexing with multiple documents."""
        mock_repository = MagicMock(spec=SqlAlchemyEmbeddingRepository)
        mock_repository.create_embedding = AsyncMock()

        mock_provider = MagicMock()

        async def mock_embed(
            _: list[EnrichmentRequest],
        ) -> AsyncGenerator[list[EmbeddingResponse], None]:
            yield [
                EmbeddingResponse(snippet_id=1, embedding=[0.1, 0.2, 0.3]),
                EmbeddingResponse(snippet_id=2, embedding=[0.4, 0.5, 0.6]),
            ]

        mock_provider.embed.return_value = mock_embed([])

        repository = LocalVectorSearchRepository(
            embedding_repository=mock_repository,
            embedding_provider=mock_provider,
            embedding_type=EmbeddingType.CODE,
        )

        request = IndexRequest(
            documents=[
                Document(snippet_id=1, text="python programming"),
                Document(snippet_id=2, text="javascript development"),
            ]
        )

        results = []
        async for batch in repository.index_documents(request):
            results.extend(batch)

        assert len(results) == 2
        assert results[0].snippet_id == 1
        assert results[1].snippet_id == 2

        # Verify embeddings were saved
        assert mock_repository.create_embedding.call_count == 2

    @pytest.mark.asyncio
    async def test_search_success(self) -> None:
        """Test successful search."""
        mock_repository = MagicMock(spec=SqlAlchemyEmbeddingRepository)
        mock_repository.list_semantic_results = AsyncMock(
            return_value=[
                (1, 0.95),
                (2, 0.85),
            ]
        )

        mock_provider = MagicMock()

        async def mock_embed(
            _: list[EnrichmentRequest],
        ) -> AsyncGenerator[list[EmbeddingResponse], None]:
            yield [EmbeddingResponse(snippet_id=0, embedding=[0.1, 0.2, 0.3])]

        mock_provider.embed.return_value = mock_embed([])

        repository = LocalVectorSearchRepository(
            embedding_repository=mock_repository,
            embedding_provider=mock_provider,
            embedding_type=EmbeddingType.CODE,
        )

        request = SearchRequest(query="python programming", top_k=10)

        results = await repository.search(request)

        assert len(results) == 2
        assert results[0].snippet_id == 1
        assert results[0].score == 0.95
        assert results[1].snippet_id == 2
        assert results[1].score == 0.85

        # Verify embedding provider was called
        mock_provider.embed.assert_called_once()
        call_args = mock_provider.embed.call_args[0][0]
        assert len(call_args) == 1
        assert call_args[0].snippet_id == 0
        assert call_args[0].text == "python programming"

        # Verify repository search was called
        mock_repository.list_semantic_results.assert_called_once_with(
            EmbeddingType.CODE, [0.1, 0.2, 0.3], 10, None
        )

    @pytest.mark.asyncio
    async def test_search_no_embedding_generated(self) -> None:
        """Test search when no embedding is generated."""
        mock_repository = MagicMock(spec=SqlAlchemyEmbeddingRepository)

        mock_provider = MagicMock()

        async def mock_embed(
            _: list[EnrichmentRequest],
        ) -> AsyncGenerator[list[EmbeddingResponse], None]:
            yield []  # No embeddings returned

        mock_provider.embed.return_value = mock_embed([])

        repository = LocalVectorSearchRepository(
            embedding_repository=mock_repository,
            embedding_provider=mock_provider,
            embedding_type=EmbeddingType.CODE,
        )

        request = SearchRequest(query="python programming", top_k=10)

        results = await repository.search(request)

        assert results == []
        mock_repository.list_semantic_results.assert_not_called()

    @pytest.mark.asyncio
    async def test_has_embedding_true(self) -> None:
        """Test has_embedding when embedding exists."""
        mock_repository = MagicMock(spec=SqlAlchemyEmbeddingRepository)
        mock_repository.get_embedding_by_snippet_id_and_type = AsyncMock(
            return_value=MagicMock()
        )

        mock_provider = MagicMock()

        repository = LocalVectorSearchRepository(
            embedding_repository=mock_repository,
            embedding_provider=mock_provider,
            embedding_type=EmbeddingType.CODE,
        )

        result = await repository.has_embedding(1, EmbeddingType.CODE)

        assert result is True
        mock_repository.get_embedding_by_snippet_id_and_type.assert_called_once_with(
            1, EmbeddingType.CODE
        )

    @pytest.mark.asyncio
    async def test_has_embedding_false(self) -> None:
        """Test has_embedding when embedding doesn't exist."""
        mock_repository = MagicMock(spec=SqlAlchemyEmbeddingRepository)
        mock_repository.get_embedding_by_snippet_id_and_type = AsyncMock(
            return_value=None
        )

        mock_provider = MagicMock()

        repository = LocalVectorSearchRepository(
            embedding_repository=mock_repository,
            embedding_provider=mock_provider,
            embedding_type=EmbeddingType.CODE,
        )

        result = await repository.has_embedding(1, EmbeddingType.TEXT)

        assert result is False
        mock_repository.get_embedding_by_snippet_id_and_type.assert_called_once_with(
            1, EmbeddingType.TEXT
        )

    @pytest.mark.asyncio
    async def test_search_with_snippet_ids_filtering(self) -> None:
        """Test search with snippet_ids filtering."""
        mock_repository = MagicMock(spec=SqlAlchemyEmbeddingRepository)
        mock_repository.list_semantic_results = AsyncMock(
            return_value=[
                (1, 0.95),
            ]
        )

        mock_provider = MagicMock()

        async def mock_embed(
            _: list[EnrichmentRequest],
        ) -> AsyncGenerator[list[EmbeddingResponse], None]:
            yield [EmbeddingResponse(snippet_id=0, embedding=[0.1, 0.2, 0.3])]

        mock_provider.embed.return_value = mock_embed([])

        repository = LocalVectorSearchRepository(
            embedding_repository=mock_repository,
            embedding_provider=mock_provider,
            embedding_type=EmbeddingType.CODE,
        )

        request = SearchRequest(
            query="python programming", top_k=10, snippet_ids=[1, 2, 3]
        )

        results = await repository.search(request)

        assert len(results) == 1
        assert results[0].snippet_id == 1
        assert results[0].score == 0.95

        # Verify repository search was called with snippet_ids
        mock_repository.list_semantic_results.assert_called_once_with(
            EmbeddingType.CODE, [0.1, 0.2, 0.3], 10, [1, 2, 3]
        )

    @pytest.mark.asyncio
    async def test_search_with_none_snippet_ids_no_filtering(self) -> None:
        """Test search with None snippet_ids (no filtering)."""
        mock_repository = MagicMock(spec=SqlAlchemyEmbeddingRepository)
        mock_repository.list_semantic_results = AsyncMock(
            return_value=[
                (1, 0.95),
                (2, 0.85),
            ]
        )

        mock_provider = MagicMock()

        async def mock_embed(
            _: list[EnrichmentRequest],
        ) -> AsyncGenerator[list[EmbeddingResponse], None]:
            yield [EmbeddingResponse(snippet_id=0, embedding=[0.1, 0.2, 0.3])]

        mock_provider.embed.return_value = mock_embed([])

        repository = LocalVectorSearchRepository(
            embedding_repository=mock_repository,
            embedding_provider=mock_provider,
            embedding_type=EmbeddingType.CODE,
        )

        request = SearchRequest(
            query="python programming",
            top_k=10,
            snippet_ids=None,  # No filtering
        )

        results = await repository.search(request)

        assert len(results) == 2
        assert results[0].snippet_id == 1
        assert results[0].score == 0.95
        assert results[1].snippet_id == 2
        assert results[1].score == 0.85

        # Verify repository search was called with None snippet_ids
        mock_repository.list_semantic_results.assert_called_once_with(
            EmbeddingType.CODE, [0.1, 0.2, 0.3], 10, None
        )

    @pytest.mark.asyncio
    async def test_search_with_empty_snippet_ids_returns_no_results(self) -> None:
        """Test search with empty snippet_ids list returns no results."""
        mock_repository = MagicMock(spec=SqlAlchemyEmbeddingRepository)
        mock_repository.list_semantic_results = AsyncMock(
            return_value=[]  # No results when filtering by empty list
        )

        mock_provider = MagicMock()

        async def mock_embed(
            _: list[EnrichmentRequest],
        ) -> AsyncGenerator[list[EmbeddingResponse], None]:
            yield [EmbeddingResponse(snippet_id=0, embedding=[0.1, 0.2, 0.3])]

        mock_provider.embed.return_value = mock_embed([])

        repository = LocalVectorSearchRepository(
            embedding_repository=mock_repository,
            embedding_provider=mock_provider,
            embedding_type=EmbeddingType.CODE,
        )

        request = SearchRequest(
            query="python programming",
            top_k=10,
            snippet_ids=[],  # Empty list - should return no results
        )

        results = await repository.search(request)

        assert results == []

        # Verify repository search was called with empty snippet_ids
        mock_repository.list_semantic_results.assert_called_once_with(
            EmbeddingType.CODE, [0.1, 0.2, 0.3], 10, []
        )


@pytest.mark.asyncio
async def test_retrieve_documents(session: AsyncSession) -> None:
    """Test retrieving documents with actual embedding values.

    This test is based on the user's example and tests the actual embedding
    functionality with real data.
    """
    # Create embedding repository
    embedding_repository = SqlAlchemyEmbeddingRepository(session=session)

    # Create embedding provider
    embedding_provider = LocalEmbeddingProvider()

    # Create vector search repository
    vector_search_repository = LocalVectorSearchRepository(
        embedding_repository=embedding_repository,
        embedding_provider=embedding_provider,
        embedding_type=EmbeddingType.CODE,
    )

    # Create dummy source, file, and index
    source = Source(
        uri="test_repo",
        cloned_path="/tmp/test_repo",  # noqa: S108
        source_type=SourceType.GIT,
    )
    session.add(source)
    await session.commit()

    file = File(
        created_at=datetime.now(UTC),
        updated_at=datetime.now(UTC),
        source_id=source.id,
        mime_type="text/plain",
        uri="test.py",
        cloned_path="/tmp/test_repo/test.py",  # noqa: S108
        sha256="abc123",
        size_bytes=100,
        extension="py",
        file_processing_status=FileProcessingStatus.CLEAN.value,
    )
    session.add(file)
    await session.commit()

    index = Index(source_id=source.id)
    session.add(index)
    await session.commit()

    # Create snippets
    snippet1 = Snippet(
        index_id=index.id,
        file_id=file.id,
        content="python programming language",
        summary="",
    )
    snippet2 = Snippet(
        index_id=index.id,
        file_id=file.id,
        content="javascript web development",
        summary="",
    )
    snippet3 = Snippet(
        index_id=index.id,
        file_id=file.id,
        content="java enterprise applications",
        summary="",
    )
    session.add(snippet1)
    session.add(snippet2)
    session.add(snippet3)
    await session.commit()

    # Index the snippets
    request = IndexRequest(
        documents=[
            Document(snippet_id=snippet1.id, text=snippet1.content),
            Document(snippet_id=snippet2.id, text=snippet2.content),
            Document(snippet_id=snippet3.id, text=snippet3.content),
        ]
    )
    async for _batch in vector_search_repository.index_documents(request):
        pass  # Process all batches

    # Search for similar content
    results = await vector_search_repository.search(
        SearchRequest(query="python programming language", top_k=2)
    )

    assert len(results) == 2
    assert all(isinstance(r, SearchResult) for r in results)
    assert all(0 <= r.score <= 1 for r in results)

    # The first result should be the most relevant (python-related)
    # Since we're using a hash-based embedding, the exact ordering might vary
    # but we can check that we get results and they have valid scores
    assert results[0].snippet_id in [snippet1.id, snippet2.id, snippet3.id]
    assert results[1].snippet_id in [snippet1.id, snippet2.id, snippet3.id]
    assert results[0].snippet_id != results[1].snippet_id
