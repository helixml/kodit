"""Tests for the retrieval service module."""

import pytest
from sqlalchemy.ext.asyncio import AsyncSession
from unittest.mock import Mock

from kodit.bm25.bm25 import BM25Service
from kodit.config import AppContext
from kodit.indexing.models import Index, Snippet
from kodit.retreival.repository import RetrievalRepository
from kodit.retreival.service import RetrievalRequest, RetrievalService
from kodit.sources.models import File, Source


@pytest.fixture
def repository(session: AsyncSession) -> RetrievalRepository:
    """Create a repository instance with a real database session."""
    return RetrievalRepository(session)


@pytest.fixture
def service(
    app_context: AppContext, repository: RetrievalRepository
) -> RetrievalService:
    """Create a service instance with a real repository."""
    service = RetrievalService(repository, app_context.get_data_dir())
    mock_bm25 = Mock(spec=BM25Service)

    def mock_retrieve(
        doc_ids: list[int], query: str, top_k: int = 2
    ) -> list[tuple[int, float]]:
        # Mock behavior based on test cases
        if query.lower() == "hello":
            return [(1, 0.5)]  # Return first snippet for "hello"
        elif query.lower() == "world":
            return [(1, 0.5), (2, 0.4)]  # Return both snippets for "world"
        elif query.lower() == "good":
            return [(2, 0.4)]  # Return second snippet for "good"
        return []  # Return empty list for no matches

    mock_bm25.retrieve.side_effect = mock_retrieve
    service.bm25 = mock_bm25
    return service


@pytest.mark.asyncio
async def test_retrieve_snippets(
    service: RetrievalService, session: AsyncSession
) -> None:
    """Test retrieving snippets through the service."""
    # Create test source
    source = Source(uri="test_source", cloned_path="test_source")
    session.add(source)
    await session.commit()

    # Create test index
    index = Index(source_id=source.id)
    session.add(index)
    await session.commit()

    # Create test files and snippets
    file1 = File(
        source_id=source.id,
        cloned_path="test1.txt",
        mime_type="text/plain",
        uri="test1.txt",
        sha256="hash1",
        size_bytes=100,
    )
    file2 = File(
        source_id=source.id,
        cloned_path="test2.txt",
        mime_type="text/plain",
        sha256="hash2",
        size_bytes=200,
        uri="test2.txt",
    )
    session.add(file1)
    session.add(file2)
    await session.commit()

    snippet1 = Snippet(index_id=1, file_id=file1.id, content="hello world")
    snippet2 = Snippet(index_id=1, file_id=file2.id, content="goodbye world")
    session.add(snippet1)
    session.add(snippet2)
    await session.commit()

    # Test retrieving snippets
    results = await service.retrieve(RetrievalRequest(keywords=["hello"]))
    assert len(results) == 1
    assert results[0].uri == "test1.txt"
    assert results[0].content == "hello world"

    # Test case-insensitive search
    results = await service.retrieve(RetrievalRequest(keywords=["WORLD"]))
    assert len(results) == 2
    assert {r.uri for r in results} == {"test1.txt", "test2.txt"}

    # Test partial match
    results = await service.retrieve(RetrievalRequest(keywords=["good"]))
    assert len(results) == 1
    assert results[0].uri == "test2.txt"
    assert results[0].content == "goodbye world"

    # Test no matches
    results = await service.retrieve(RetrievalRequest(keywords=["nonexistent"]))
    assert len(results) == 0
