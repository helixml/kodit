"""Tests for the search service module."""

from typing import AsyncGenerator, Generator
import pytest
from sqlalchemy.ext.asyncio import AsyncSession
from unittest.mock import Mock

from kodit.bm25.bm25 import BM25Service
from kodit.config import AppContext
from kodit.embedding.embedding import Embedder, EmbeddingInput, EmbeddingOutput
from kodit.embedding.embedding_models import EmbeddingType
from kodit.indexing.indexing_models import Index, Snippet
from kodit.search.search_repository import SearchRepository
from kodit.search.search_service import (
    SearchRequest,
    SearchService,
    reciprocal_rank_fusion,
)
from kodit.source.source_models import File, Source


@pytest.fixture
def repository(session: AsyncSession) -> SearchRepository:
    """Create a repository instance with a real database session."""
    return SearchRepository(session)


@pytest.fixture
def service(app_context: AppContext, repository: SearchRepository) -> SearchService:
    """Create a service instance with a real repository."""

    # Mock embedding service
    async def mock_embed(
        snippets: list[EmbeddingInput],
    ) -> AsyncGenerator[EmbeddingOutput, None]:
        # Return a simple mock embedding for testing
        for _ in snippets:
            yield EmbeddingOutput(0, [0.1, 0.2, 0.3])

    async def mock_query(
        data: list[str],
    ) -> AsyncGenerator[list[float], None]:
        # Return a simple mock embedding for testing
        for _ in data:
            yield [0.1, 0.2, 0.3]

    mock_embedding = Mock(spec=Embedder)
    mock_embedding.embed = mock_embed
    mock_embedding.query = mock_query

    service = SearchService(
        repository,
        app_context.get_data_dir(),
        embedding_service=mock_embedding,
    )
    mock_bm25 = Mock(spec=BM25Service)

    def mock_search(
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

    mock_bm25.retrieve.side_effect = mock_search
    service.bm25 = mock_bm25

    return service


@pytest.mark.asyncio
async def test_search_snippets_bm25(
    service: SearchService, session: AsyncSession
) -> None:
    """Test searching for snippets through the service."""
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

    # Test searching for snippets
    results = await service.search(SearchRequest(keywords=["hello"]))
    assert len(results) == 1
    assert results[0].uri == "test1.txt"
    assert results[0].content == "hello world"

    # Test case-insensitive search
    results = await service.search(SearchRequest(keywords=["WORLD"]))
    assert len(results) == 2
    assert {r.uri for r in results} == {"test1.txt", "test2.txt"}

    # Test partial match
    results = await service.search(SearchRequest(keywords=["good"]))
    assert len(results) == 1
    assert results[0].uri == "test2.txt"
    assert results[0].content == "goodbye world"

    # Test no matches
    results = await service.search(SearchRequest(keywords=["nonexistent"]))
    assert len(results) == 0


def test_reciprocal_rank_fusion() -> None:
    """Test the reciprocal rank fusion function."""
    # Test case 1: Multiple rankings with overlapping documents
    rankings = [
        [1, 2, 3],  # First ranking
        [2, 1, 4],  # Second ranking
        [3, 2, 1],  # Third ranking
    ]
    results = reciprocal_rank_fusion(rankings, k=60)

    # Document 2 appears in all rankings and high up, should be first
    # Document 1 appears in all rankings but lower in some, should be second
    # Document 3 appears in two rankings, should be third
    # Document 4 appears in only one ranking, should be last
    assert len(results) == 4
    assert results[0][0] == 2  # Document 2 should be first
    assert results[1][0] == 1  # Document 1 should be second
    assert results[2][0] == 3  # Document 3 should be third
    assert results[3][0] == 4  # Document 4 should be last

    # Verify scores are in descending order
    assert results[0][1] > results[1][1] > results[2][1] > results[3][1]

    # Test case 2: Empty rankings
    results = reciprocal_rank_fusion([], k=60)
    assert len(results) == 0

    # Test case 3: Single ranking
    results = reciprocal_rank_fusion([[1, 2, 3]], k=60)
    assert len(results) == 3
    assert [r[0] for r in results] == [1, 2, 3]

    # Test case 4: Rankings with different lengths
    rankings = [
        [1, 2, 3],
        [2, 1],
        [3, 2, 1, 4],
    ]
    results = reciprocal_rank_fusion(rankings, k=60)
    assert len(results) == 4
    assert results[0][0] == 2  # Document 2 appears in all rankings
    assert results[1][0] == 1  # Document 1 appears in two rankings
    assert results[2][0] == 3  # Document 3 appears in two rankings
    assert results[3][0] == 4  # Document 4 appears in only one ranking

    # Test case 5: Verify RRF formula (1/(k + rank))
    # For k=60, first position should have score ~1/60, second ~1/61, etc.
    rankings = [[1, 2, 3]]
    results = reciprocal_rank_fusion(rankings, k=60)
    assert abs(results[0][1] - 1 / 60) < 0.0001  # First position
    assert abs(results[1][1] - 1 / 61) < 0.0001  # Second position
    assert abs(results[2][1] - 1 / 62) < 0.0001  # Third position


def test_reciprocal_rank_fusion_single_ranking() -> None:
    """Test the reciprocal rank fusion function with a single ranking."""
    rankings = [[1, 2, 3]]
    results = reciprocal_rank_fusion(rankings, k=60)
    assert len(results) == 3
    assert [r[0] for r in results] == [1, 2, 3]


@pytest.mark.asyncio
async def test_search_snippets_semantic(
    service: SearchService, session: AsyncSession
) -> None:
    """Test searching for snippets through semantic search."""
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

    # Mock repository's semantic search
    async def mock_list_semantic_results(
        embedding_type: EmbeddingType, embedding: list[float], top_k: int = 10
    ):
        # Return mock semantic search results
        return [(1, 0.8), (2, 0.6)]  # First snippet is more semantically similar

    service.repository.list_semantic_results = mock_list_semantic_results

    # Test semantic search
    results = await service.search(SearchRequest(code_query="greeting"))
    assert len(results) == 2
    assert (
        results[0].uri == "test1.txt"
    )  # First result should be the more semantically similar one
    assert results[0].content == "hello world"
    assert results[1].uri == "test2.txt"
    assert results[1].content == "goodbye world"

    # Test combined semantic and keyword search
    results = await service.search(
        SearchRequest(code_query="greeting", keywords=["hello"])
    )
    assert len(results) == 2
    # Results should be fused from both semantic and keyword search
    assert {r.uri for r in results} == {"test1.txt", "test2.txt"}

    # Test semantic search with no matches
    async def mock_empty_semantic_results(
        embedding_type: EmbeddingType, embedding: list[float], top_k: int = 10
    ):
        return []

    service.repository.list_semantic_results = mock_empty_semantic_results
    results = await service.search(SearchRequest(code_query="nonexistent"))
    assert len(results) == 0
