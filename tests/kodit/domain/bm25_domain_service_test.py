"""Tests for the BM25 domain service."""

from unittest.mock import AsyncMock, MagicMock

import pytest

from kodit.domain.services.bm25_service import BM25DomainService, BM25Repository
from kodit.domain.value_objects import (
    DeleteRequest,
    Document,
    IndexRequest,
    SearchRequest,
    SearchResult,
)


class MockBM25Repository(MagicMock):
    """Mock BM25 repository for testing."""

    def __init__(self) -> None:
        """Initialize the mock BM25 repository."""
        super().__init__(spec=BM25Repository)
        self.index_documents = AsyncMock()
        self.search = AsyncMock()
        self.delete_documents = AsyncMock()


@pytest.fixture
def mock_repository() -> MockBM25Repository:
    """Create a mock BM25 repository."""
    return MockBM25Repository()


@pytest.fixture
def bm25_domain_service(mock_repository: MockBM25Repository) -> BM25DomainService:
    """Create a BM25 domain service with a mock repository."""
    return BM25DomainService(repository=mock_repository)


@pytest.mark.asyncio
async def test_index_documents_success(
    bm25_domain_service: BM25DomainService, mock_repository: MockBM25Repository
) -> None:
    """Test successful document indexing."""
    # Setup
    documents = [
        Document(snippet_id=1, text="test content 1"),
        Document(snippet_id=2, text="test content 2"),
    ]
    request = IndexRequest(documents=documents)

    # Execute
    await bm25_domain_service.index_documents(request)

    # Verify
    mock_repository.index_documents.assert_called_once()
    call_args = mock_repository.index_documents.call_args[0][0]
    assert len(call_args.documents) == 2
    assert call_args.documents[0].snippet_id == 1
    assert call_args.documents[0].text == "test content 1"


@pytest.mark.asyncio
async def test_index_documents_empty_list(
    bm25_domain_service: BM25DomainService,
) -> None:
    """Test indexing with empty document list."""
    # Setup
    request = IndexRequest(documents=[])

    # Execute and verify
    with pytest.raises(ValueError, match="Cannot index empty document list"):
        await bm25_domain_service.index_documents(request)


@pytest.mark.asyncio
async def test_index_documents_invalid_documents(
    bm25_domain_service: BM25DomainService, mock_repository: MockBM25Repository
) -> None:
    """Test indexing with invalid documents."""
    # Setup
    documents = [
        Document(snippet_id=1, text="valid content"),
        Document(snippet_id=2, text=""),  # Empty text
    ]
    request = IndexRequest(documents=documents)

    # Execute
    await bm25_domain_service.index_documents(request)

    # Verify - only valid documents should be passed to repository
    call_args = mock_repository.index_documents.call_args[0][0]
    assert len(call_args.documents) == 1
    assert call_args.documents[0].snippet_id == 1
    assert call_args.documents[0].text == "valid content"


@pytest.mark.asyncio
async def test_search_success(
    bm25_domain_service: BM25DomainService, mock_repository: MockBM25Repository
) -> None:
    """Test successful search."""
    # Setup
    expected_results = [
        SearchResult(snippet_id=1, score=0.8),
        SearchResult(snippet_id=2, score=0.6),
    ]
    mock_repository.search.return_value = expected_results

    request = SearchRequest(query="test query", top_k=10)

    # Execute
    results = await bm25_domain_service.search(request)

    # Verify
    assert results == expected_results
    mock_repository.search.assert_called_once()
    call_args = mock_repository.search.call_args[0][0]
    assert call_args.query == "test query"
    assert call_args.top_k == 10


@pytest.mark.asyncio
async def test_search_empty_query(bm25_domain_service: BM25DomainService) -> None:
    """Test search with empty query."""
    # Setup
    request = SearchRequest(query="", top_k=10)

    # Execute and verify
    with pytest.raises(ValueError, match="Search query cannot be empty"):
        await bm25_domain_service.search(request)


@pytest.mark.asyncio
async def test_search_invalid_top_k(bm25_domain_service: BM25DomainService) -> None:
    """Test search with invalid top_k."""
    # Setup
    request = SearchRequest(query="test query", top_k=0)

    # Execute and verify
    with pytest.raises(ValueError, match="Top-k must be positive"):
        await bm25_domain_service.search(request)


@pytest.mark.asyncio
async def test_delete_documents_success(
    bm25_domain_service: BM25DomainService, mock_repository: MockBM25Repository
) -> None:
    """Test successful document deletion."""
    # Setup
    request = DeleteRequest(snippet_ids=[1, 2, 3])

    # Execute
    await bm25_domain_service.delete_documents(request)

    # Verify
    mock_repository.delete_documents.assert_called_once()
    call_args = mock_repository.delete_documents.call_args[0][0]
    assert call_args.snippet_ids == [1, 2, 3]


@pytest.mark.asyncio
async def test_delete_documents_empty_list(
    bm25_domain_service: BM25DomainService,
) -> None:
    """Test deletion with empty snippet ID list."""
    # Setup
    request = DeleteRequest(snippet_ids=[])

    # Execute and verify
    with pytest.raises(ValueError, match="Cannot delete empty snippet ID list"):
        await bm25_domain_service.delete_documents(request)


@pytest.mark.asyncio
async def test_delete_documents_invalid_ids(
    bm25_domain_service: BM25DomainService, mock_repository: MockBM25Repository
) -> None:
    """Test deletion with invalid snippet IDs."""
    # Setup
    request = DeleteRequest(snippet_ids=[1, 0, -1, 3])

    # Execute
    await bm25_domain_service.delete_documents(request)

    # Verify - only valid IDs should be passed to repository
    call_args = mock_repository.delete_documents.call_args[0][0]
    assert call_args.snippet_ids == [1, 3]
