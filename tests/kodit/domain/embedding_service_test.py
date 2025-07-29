"""Tests for the embedding domain service."""

from collections.abc import AsyncGenerator
from unittest.mock import MagicMock

import pytest

from kodit.domain.services.embedding_service import (
    EmbeddingDomainService,
    EmbeddingProvider,
    VectorSearchRepository,
)
from kodit.domain.value_objects import (
    Document,
    EmbeddingRequest,
    EmbeddingResponse,
    IndexRequest,
    IndexResult,
    SearchRequest,
    SearchResult,
)
from kodit.infrastructure.sqlalchemy.entities import EmbeddingType


class MockEmbeddingProvider(EmbeddingProvider):
    """Mock embedding provider for testing."""

    def __init__(self, responses: list[EmbeddingResponse] | None = None) -> None:
        """Initialize the mock embedding provider."""
        super().__init__()
        self.responses = responses or []

    async def embed(
        self,
        data: list[EmbeddingRequest],  # noqa: ARG002
    ) -> AsyncGenerator[list[EmbeddingResponse], None]:
        """Embed the data."""
        yield self.responses


class MockVectorSearchRepository(VectorSearchRepository):
    """Mock vector search repository for testing."""

    def __init__(
        self,
        index_results: list[IndexResult] | None = None,
        search_results: list[SearchResult] | None = None,
        has_embedding_result: bool = True,  # noqa: FBT001, FBT002
    ) -> None:
        """Initialize the mock vector search repository."""
        super().__init__()
        self.index_results = index_results or []
        self.search_results = search_results or []
        self.has_embedding_result = has_embedding_result

    async def index_documents(
        self,
        request: IndexRequest,  # noqa: ARG002
    ) -> AsyncGenerator[list[IndexResult], None]:
        """Index the documents."""
        yield self.index_results

    async def search(self, request: SearchRequest) -> list[SearchResult]:  # noqa: ARG002
        """Search the documents."""
        return self.search_results

    async def has_embedding(
        self,
        snippet_id: int,  # noqa: ARG002
        embedding_type: EmbeddingType,  # noqa: ARG002
    ) -> bool:
        """Check if the snippet has an embedding."""
        return self.has_embedding_result


class TestEmbeddingDomainService:
    """Test the embedding domain service."""

    def test_init(self) -> None:
        """Test initialization."""
        mock_provider = MagicMock(spec=EmbeddingProvider)
        mock_repository = MagicMock(spec=VectorSearchRepository)

        service = EmbeddingDomainService(
            embedding_provider=mock_provider,
            vector_search_repository=mock_repository,
        )

        assert service.embedding_provider == mock_provider
        assert service.vector_search_repository == mock_repository

    @pytest.mark.asyncio
    async def test_index_documents_empty_request(self) -> None:
        """Test indexing with empty request."""
        mock_provider = MagicMock(spec=EmbeddingProvider)
        mock_repository = MagicMock(spec=VectorSearchRepository)

        service = EmbeddingDomainService(
            embedding_provider=mock_provider,
            vector_search_repository=mock_repository,
        )

        request = IndexRequest(documents=[])

        results = []
        async for result in service.index_documents(request):
            results.extend(result)

        assert len(results) == 0
        mock_repository.index_documents.assert_not_called()

    @pytest.mark.asyncio
    async def test_index_documents_valid_request(self) -> None:
        """Test indexing with valid request."""
        mock_provider = MagicMock(spec=EmbeddingProvider)
        mock_repository = MagicMock(spec=VectorSearchRepository)

        # Mock repository response
        async def mock_index() -> AsyncGenerator[list[IndexResult], None]:
            yield [IndexResult(snippet_id=1)]
            yield [IndexResult(snippet_id=2)]

        mock_repository.index_documents.return_value = mock_index()

        service = EmbeddingDomainService(
            embedding_provider=mock_provider,
            vector_search_repository=mock_repository,
        )

        request = IndexRequest(
            documents=[
                Document(snippet_id=1, text="python programming"),
                Document(snippet_id=2, text="javascript development"),
            ]
        )

        results = []
        async for result in service.index_documents(request):
            results.extend(result)

        assert len(results) == 2
        assert results[0].snippet_id == 1
        assert results[1].snippet_id == 2

        # Verify repository was called with validated request
        mock_repository.index_documents.assert_called_once()
        call_args = mock_repository.index_documents.call_args[0][0]
        assert len(call_args.documents) == 2

    @pytest.mark.asyncio
    async def test_index_documents_filters_invalid_documents(self) -> None:
        """Test that invalid documents are filtered out."""
        mock_provider = MagicMock(spec=EmbeddingProvider)
        mock_repository = MagicMock(spec=VectorSearchRepository)

        # Mock repository response
        async def mock_index() -> AsyncGenerator[list[IndexResult], None]:
            yield [IndexResult(snippet_id=1)]

        mock_repository.index_documents.return_value = mock_index()

        service = EmbeddingDomainService(
            embedding_provider=mock_provider,
            vector_search_repository=mock_repository,
        )

        request = IndexRequest(
            documents=[
                Document(snippet_id=1, text="valid text"),
                Document(snippet_id=2, text=""),  # Empty text
                Document(snippet_id=3, text="   "),  # Whitespace only
                # Note: VectorSearchRequest requires snippet_id to be int
                # This test case is handled by the domain service validation
            ]
        )

        results = []
        async for result in service.index_documents(request):
            results.extend(result)

        # Should only process the valid document
        assert len(results) == 1
        assert results[0].snippet_id == 1

        # Verify repository was called with only valid documents
        call_args = mock_repository.index_documents.call_args[0][0]
        assert len(call_args.documents) == 1
        assert call_args.documents[0].snippet_id == 1
        assert call_args.documents[0].text == "valid text"

    @pytest.mark.asyncio
    async def test_search_valid_request(self) -> None:
        """Test search with valid request."""
        mock_provider = MagicMock(spec=EmbeddingProvider)
        mock_repository = MagicMock(spec=VectorSearchRepository)

        # Mock repository response
        mock_repository.search.return_value = [
            SearchResult(snippet_id=1, score=0.95),
            SearchResult(snippet_id=2, score=0.85),
        ]

        service = EmbeddingDomainService(
            embedding_provider=mock_provider,
            vector_search_repository=mock_repository,
        )

        request = SearchRequest(query="python programming", top_k=10)

        results = await service.search(request)

        assert len(results) == 2
        assert results[0].snippet_id == 1
        assert results[0].score == 0.95
        assert results[1].snippet_id == 2
        assert results[1].score == 0.85

        # Verify repository was called with normalized request
        mock_repository.search.assert_called_once()
        call_args = mock_repository.search.call_args[0][0]
        assert call_args.query == "python programming"
        assert call_args.top_k == 10

    @pytest.mark.asyncio
    async def test_search_empty_query(self) -> None:
        """Test search with empty query."""
        mock_provider = MagicMock(spec=EmbeddingProvider)
        mock_repository = MagicMock(spec=VectorSearchRepository)

        service = EmbeddingDomainService(
            embedding_provider=mock_provider,
            vector_search_repository=mock_repository,
        )

        request = SearchRequest(query="", top_k=10)

        with pytest.raises(ValueError, match="Search query cannot be empty"):
            await service.search(request)

        mock_repository.search.assert_not_called()

    @pytest.mark.asyncio
    async def test_search_whitespace_only_query(self) -> None:
        """Test search with whitespace-only query."""
        mock_provider = MagicMock(spec=EmbeddingProvider)
        mock_repository = MagicMock(spec=VectorSearchRepository)

        service = EmbeddingDomainService(
            embedding_provider=mock_provider,
            vector_search_repository=mock_repository,
        )

        request = SearchRequest(query="   ", top_k=10)

        with pytest.raises(ValueError, match="Search query cannot be empty"):
            await service.search(request)

        mock_repository.search.assert_not_called()

    @pytest.mark.asyncio
    async def test_search_invalid_top_k(self) -> None:
        """Test search with invalid top_k."""
        mock_provider = MagicMock(spec=EmbeddingProvider)
        mock_repository = MagicMock(spec=VectorSearchRepository)

        service = EmbeddingDomainService(
            embedding_provider=mock_provider,
            vector_search_repository=mock_repository,
        )

        request = SearchRequest(query="python", top_k=0)

        with pytest.raises(ValueError, match="Top-k must be positive"):
            await service.search(request)

        mock_repository.search.assert_not_called()

    @pytest.mark.asyncio
    async def test_search_normalizes_query(self) -> None:
        """Test that query is normalized."""
        mock_provider = MagicMock(spec=EmbeddingProvider)
        mock_repository = MagicMock(spec=VectorSearchRepository)

        mock_repository.search.return_value = []

        service = EmbeddingDomainService(
            embedding_provider=mock_provider,
            vector_search_repository=mock_repository,
        )

        request = SearchRequest(query="  python programming  ", top_k=10)

        await service.search(request)

        # Verify repository was called with normalized query
        call_args = mock_repository.search.call_args[0][0]
        assert call_args.query == "python programming"

    @pytest.mark.asyncio
    async def test_has_embedding_valid_request(self) -> None:
        """Test has_embedding with valid request."""
        mock_provider = MagicMock(spec=EmbeddingProvider)
        mock_repository = MagicMock(spec=VectorSearchRepository)

        mock_repository.has_embedding.return_value = True

        service = EmbeddingDomainService(
            embedding_provider=mock_provider,
            vector_search_repository=mock_repository,
        )

        result = await service.has_embedding(1, EmbeddingType.CODE)

        assert result is True
        mock_repository.has_embedding.assert_called_once_with(1, EmbeddingType.CODE)

    @pytest.mark.asyncio
    async def test_has_embedding_invalid_snippet_id(self) -> None:
        """Test has_embedding with invalid snippet_id."""
        mock_provider = MagicMock(spec=EmbeddingProvider)
        mock_repository = MagicMock(spec=VectorSearchRepository)

        service = EmbeddingDomainService(
            embedding_provider=mock_provider,
            vector_search_repository=mock_repository,
        )

        # Test with negative snippet_id
        with pytest.raises(ValueError, match="Snippet ID must be positive"):
            await service.has_embedding(-1, EmbeddingType.CODE)

        # Test with zero snippet_id
        with pytest.raises(ValueError, match="Snippet ID must be positive"):
            await service.has_embedding(0, EmbeddingType.CODE)

        mock_repository.has_embedding.assert_not_called()

    @pytest.mark.asyncio
    async def test_has_embedding_returns_false(self) -> None:
        """Test has_embedding when repository returns False."""
        mock_provider = MagicMock(spec=EmbeddingProvider)
        mock_repository = MagicMock(spec=VectorSearchRepository)

        mock_repository.has_embedding.return_value = False

        service = EmbeddingDomainService(
            embedding_provider=mock_provider,
            vector_search_repository=mock_repository,
        )

        result = await service.has_embedding(1, EmbeddingType.TEXT)

        assert result is False
        mock_repository.has_embedding.assert_called_once_with(1, EmbeddingType.TEXT)
