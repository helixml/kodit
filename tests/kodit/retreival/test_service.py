"""Tests for the retrieval service module."""

from datetime import UTC, datetime

import pytest
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.indexing.models import File, Snippet
from kodit.indexing.models import Index as IndexModel
from kodit.retreival.repository import RetrievalRepository
from kodit.retreival.service import RetrievalRequest, RetrievalService
from kodit.sources.models import Source


@pytest.fixture
def repository(session: AsyncSession) -> RetrievalRepository:
    """Create a repository instance with a real database session."""
    return RetrievalRepository(session)


@pytest.fixture
def service(repository: RetrievalRepository) -> RetrievalService:
    """Create a service instance with a real repository."""
    return RetrievalService(repository)


@pytest.mark.asyncio
async def test_retrieve_snippets(
    service: RetrievalService, repository: RetrievalRepository, session: AsyncSession
) -> None:
    """Test retrieving snippets through the service."""
    # Create test data
    source = Source(created_at=datetime(2024, 1, 1, 0, 0, tzinfo=UTC))
    session.add(source)
    await session.commit()

    folder_source = Source(source_id=source.id, path="test_folder")
    session.add(folder_source)
    await session.commit()

    # Create index
    index = IndexModel(
        source_id=source.id, created_at=datetime(2024, 1, 1, 0, 0, tzinfo=UTC)
    )
    session.add(index)
    await session.commit()

    # Create test files and snippets
    file1 = File(
        index_id=index.id,
        source_id=source.id,
        mime_type="text/plain",
        path="test1.txt",
        sha256="hash1",
        size_bytes=100,
    )
    file2 = File(
        index_id=index.id,
        source_id=source.id,
        mime_type="text/plain",
        path="test2.txt",
        sha256="hash2",
        size_bytes=200,
    )
    session.add(file1)
    session.add(file2)
    await session.commit()

    snippet1 = Snippet(index_id=index.id, file_id=file1.id, content=b"hello world")
    snippet2 = Snippet(index_id=index.id, file_id=file2.id, content=b"goodbye world")
    session.add(snippet1)
    session.add(snippet2)
    await session.commit()

    # Test retrieving snippets
    results = await service.retrieve(RetrievalRequest(query="hello"))
    assert len(results) == 1
    assert results[0].file_path == "test1.txt"
    assert results[0].content == "hello world"

    # Test case-insensitive search
    results = await service.retrieve(RetrievalRequest(query="WORLD"))
    assert len(results) == 2
    assert {r.file_path for r in results} == {"test1.txt", "test2.txt"}

    # Test partial match
    results = await service.retrieve(RetrievalRequest(query="good"))
    assert len(results) == 1
    assert results[0].file_path == "test2.txt"
    assert results[0].content == "goodbye world"

    # Test no matches
    results = await service.retrieve(RetrievalRequest(query="nonexistent"))
    assert len(results) == 0
