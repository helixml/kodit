"""Tests for the BM25 domain service."""

from unittest.mock import AsyncMock, MagicMock

import pytest

from kodit.domain.value_objects import (
    BM25DeleteRequest,
    BM25Document,
    BM25IndexRequest,
    BM25SearchRequest,
    BM25SearchResult,
)
from kodit.domain.entities import Snippet
from kodit.domain.services.bm25_service import BM25DomainService, BM25Repository


class MockBM25Repository(MagicMock):
    """Mock BM25 repository for testing."""

    def __init__(self):
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
    """Create a BM25 domain service with mocked repository."""
    return BM25DomainService(mock_repository)


@pytest.mark.asyncio
async def test_index_documents_success(
    bm25_domain_service: BM25DomainService, mock_repository: MockBM25Repository
) -> None:
    """Test successful document indexing."""
    # Setup
    documents = [
        BM25Document(snippet_id=1, text="test document 1"),
        BM25Document(snippet_id=2, text="test document 2"),
    ]
    request = BM25IndexRequest(documents=documents)

    # Execute
    await bm25_domain_service.index_documents(request)

    # Verify
    mock_repository.index_documents.assert_called_once()
    called_request = mock_repository.index_documents.call_args[0][0]
    assert len(called_request.documents) == 2
    assert called_request.documents[0].snippet_id == 1
    assert called_request.documents[1].snippet_id == 2


@pytest.mark.asyncio
async def test_index_documents_empty_list(
    bm25_domain_service: BM25DomainService,
) -> None:
    """Test indexing with empty document list."""
    # Setup
    request = BM25IndexRequest(documents=[])

    # Execute and verify
    with pytest.raises(ValueError, match="Cannot index empty document list"):
        await bm25_domain_service.index_documents(request)


@pytest.mark.asyncio
async def test_index_documents_invalid_documents(
    bm25_domain_service: BM25DomainService,
) -> None:
    """Test indexing with invalid documents."""
    # Setup
    documents = [
        BM25Document(snippet_id=1, text=""),  # Invalid: empty text
        BM25Document(snippet_id=2, text="   "),  # Invalid: whitespace-only text
    ]
    request = BM25IndexRequest(documents=documents)

    # Execute and verify
    with pytest.raises(ValueError, match="No valid documents to index"):
        await bm25_domain_service.index_documents(request)


@pytest.mark.asyncio
async def test_search_success(
    bm25_domain_service: BM25DomainService, mock_repository: MockBM25Repository
) -> None:
    """Test successful search."""
    # Setup
    query = "test query"
    top_k = 5
    request = BM25SearchRequest(query=query, top_k=top_k)

    expected_results = [
        BM25SearchResult(snippet_id=1, score=0.8),
        BM25SearchResult(snippet_id=2, score=0.6),
    ]
    mock_repository.search.return_value = expected_results

    # Execute
    result = await bm25_domain_service.search(request)

    # Verify
    assert result == expected_results
    mock_repository.search.assert_called_once()
    called_request = mock_repository.search.call_args[0][0]
    assert called_request.query == query
    assert called_request.top_k == top_k


@pytest.mark.asyncio
async def test_search_empty_query(bm25_domain_service: BM25DomainService) -> None:
    """Test search with empty query."""
    # Setup
    request = BM25SearchRequest(query="", top_k=10)

    # Execute and verify
    with pytest.raises(ValueError, match="Search query cannot be empty"):
        await bm25_domain_service.search(request)


@pytest.mark.asyncio
async def test_search_whitespace_query(bm25_domain_service: BM25DomainService) -> None:
    """Test search with whitespace-only query."""
    # Setup
    request = BM25SearchRequest(query="   ", top_k=10)

    # Execute and verify
    with pytest.raises(ValueError, match="Search query cannot be empty"):
        await bm25_domain_service.search(request)


@pytest.mark.asyncio
async def test_search_invalid_top_k(bm25_domain_service: BM25DomainService) -> None:
    """Test search with invalid top_k."""
    # Setup
    request = BM25SearchRequest(query="test", top_k=0)

    # Execute and verify
    with pytest.raises(ValueError, match="Top-k must be positive"):
        await bm25_domain_service.search(request)


@pytest.mark.asyncio
async def test_search_normalizes_query(
    bm25_domain_service: BM25DomainService, mock_repository: MockBM25Repository
) -> None:
    """Test that search normalizes the query."""
    # Setup
    request = BM25SearchRequest(query="  test query  ", top_k=10)
    mock_repository.search.return_value = []

    # Execute
    await bm25_domain_service.search(request)

    # Verify
    called_request = mock_repository.search.call_args[0][0]
    assert called_request.query == "test query"  # Should be trimmed


@pytest.mark.asyncio
async def test_delete_documents_success(
    bm25_domain_service: BM25DomainService, mock_repository: MockBM25Repository
) -> None:
    """Test successful document deletion."""
    # Setup
    snippet_ids = [1, 2, 3]
    request = BM25DeleteRequest(snippet_ids=snippet_ids)

    # Execute
    await bm25_domain_service.delete_documents(request)

    # Verify
    mock_repository.delete_documents.assert_called_once()
    called_request = mock_repository.delete_documents.call_args[0][0]
    assert called_request.snippet_ids == snippet_ids


@pytest.mark.asyncio
async def test_delete_documents_empty_list(
    bm25_domain_service: BM25DomainService,
) -> None:
    """Test deletion with empty snippet ID list."""
    # Setup
    request = BM25DeleteRequest(snippet_ids=[])

    # Execute and verify
    with pytest.raises(ValueError, match="Cannot delete empty snippet ID list"):
        await bm25_domain_service.delete_documents(request)


@pytest.mark.asyncio
async def test_delete_documents_invalid_ids(
    bm25_domain_service: BM25DomainService,
) -> None:
    """Test deletion with invalid snippet IDs."""
    # Setup
    snippet_ids = [None, 0, -1]  # Invalid IDs
    request = BM25DeleteRequest(snippet_ids=snippet_ids)

    # Execute and verify
    with pytest.raises(ValueError, match="No valid snippet IDs to delete"):
        await bm25_domain_service.delete_documents(request)


@pytest.mark.asyncio
async def test_delete_documents_filters_invalid_ids(
    bm25_domain_service: BM25DomainService, mock_repository: MockBM25Repository
) -> None:
    """Test that deletion filters out invalid IDs."""
    # Setup
    snippet_ids = [1, None, 2, 0, 3, -1]  # Mix of valid and invalid
    request = BM25DeleteRequest(snippet_ids=snippet_ids)

    # Execute
    await bm25_domain_service.delete_documents(request)

    # Verify
    called_request = mock_repository.delete_documents.call_args[0][0]
    assert called_request.snippet_ids == [1, 2, 3]  # Only valid IDs should remain
