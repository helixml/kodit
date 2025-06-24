"""Tests for the indexing application service module."""

from datetime import datetime, UTC
from pathlib import Path
import tempfile
from typing import Any, Generator
from unittest.mock import AsyncMock, MagicMock

import pytest
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.application.services.indexing_application_service import (
    IndexingApplicationService,
)
from kodit.domain.entities import Snippet, Source, SourceType
from kodit.domain.errors import EmptySourceError
from kodit.domain.value_objects import (
    IndexView,
    MultiSearchRequest,
    SnippetSearchFilters,
    BM25SearchResult,
    FusionRequest,
    FusionResult,
)
from kodit.domain.services.bm25_service import BM25DomainService
from kodit.domain.services.embedding_service import EmbeddingDomainService
from kodit.domain.services.enrichment_service import EnrichmentDomainService
from kodit.domain.services.indexing_service import IndexingDomainService
from kodit.domain.services.source_service import SourceService
from kodit.application.services.snippet_application_service import (
    SnippetApplicationService,
)
from kodit.domain.value_objects import EnrichmentResponse


@pytest.fixture
def mock_indexing_domain_service() -> MagicMock:
    """Create a mock indexing domain service."""
    service = MagicMock(spec=IndexingDomainService)
    service.create_index = AsyncMock()
    service.list_indexes = AsyncMock()
    service.get_index = AsyncMock()
    service.delete_all_snippets = AsyncMock()
    service.get_snippets_for_index = AsyncMock()
    service.add_snippet = AsyncMock()
    return service


@pytest.fixture
def mock_source_service() -> MagicMock:
    """Create a mock source service."""
    service = MagicMock(spec=SourceService)
    service.get = AsyncMock()
    return service


@pytest.fixture
def mock_bm25_service() -> MagicMock:
    """Create a mock BM25 domain service."""
    service = MagicMock(spec=BM25DomainService)
    service.index_documents = AsyncMock()
    return service


@pytest.fixture
def mock_code_search_service() -> MagicMock:
    """Create a mock code search domain service."""
    service = MagicMock(spec=EmbeddingDomainService)
    service.index_documents = AsyncMock()
    return service


@pytest.fixture
def mock_text_search_service() -> MagicMock:
    """Create a mock text search domain service."""
    service = MagicMock(spec=EmbeddingDomainService)
    service.index_documents = AsyncMock()
    return service


@pytest.fixture
def mock_enrichment_service() -> MagicMock:
    """Create a mock enrichment service."""
    service = MagicMock(spec=EnrichmentDomainService)
    service.enrich_documents = AsyncMock()
    return service


@pytest.fixture
def mock_snippet_application_service() -> MagicMock:
    """Create a mock snippet application service."""
    service = MagicMock(spec=SnippetApplicationService)
    service.create_snippets_for_index = AsyncMock()
    return service


@pytest.fixture
def mock_session() -> MagicMock:
    """Create a mock session."""
    session = MagicMock()
    session.commit = AsyncMock()
    return session


@pytest.fixture
def indexing_application_service(
    mock_indexing_domain_service: MagicMock,
    mock_source_service: MagicMock,
    mock_bm25_service: MagicMock,
    mock_code_search_service: MagicMock,
    mock_text_search_service: MagicMock,
    mock_enrichment_service: MagicMock,
    mock_snippet_application_service: MagicMock,
    mock_session: MagicMock,
) -> IndexingApplicationService:
    """Create an indexing application service with mocked dependencies."""
    return IndexingApplicationService(
        indexing_domain_service=mock_indexing_domain_service,
        source_service=mock_source_service,
        bm25_service=mock_bm25_service,
        code_search_service=mock_code_search_service,
        text_search_service=mock_text_search_service,
        enrichment_service=mock_enrichment_service,
        snippet_application_service=mock_snippet_application_service,
        session=mock_session,
    )


@pytest.mark.asyncio
async def test_create_index_success(
    indexing_application_service: IndexingApplicationService,
    mock_source_service: MagicMock,
    mock_indexing_domain_service: MagicMock,
    mock_session: MagicMock,
) -> None:
    """Test creating a new index through the application service."""
    # Setup mocks
    source = Source(
        uri="test_folder", cloned_path="test_folder", source_type=SourceType.FOLDER
    )
    source.id = 1
    mock_source_service.get.return_value = source

    expected_index_view = IndexView(id=1, created_at=datetime.now(UTC), num_snippets=0)
    mock_indexing_domain_service.create_index.return_value = expected_index_view

    # Execute
    result = await indexing_application_service.create_index(source.id)

    # Verify
    assert result == expected_index_view
    mock_source_service.get.assert_called_once_with(source.id)
    mock_indexing_domain_service.create_index.assert_called_once()
    # Verify that commit is called at the application service level
    mock_session.commit.assert_called_once()


@pytest.mark.asyncio
async def test_create_index_source_not_found(
    indexing_application_service: IndexingApplicationService,
    mock_source_service: MagicMock,
) -> None:
    """Test creating an index for a non-existent source."""
    # Setup mocks
    mock_source_service.get.side_effect = ValueError("Source not found: 999")

    # Execute and verify
    with pytest.raises(ValueError, match="Source not found: 999"):
        await indexing_application_service.create_index(999)


@pytest.mark.asyncio
async def test_list_indexes(
    indexing_application_service: IndexingApplicationService,
    mock_indexing_domain_service: MagicMock,
) -> None:
    """Test listing indexes through the application service."""
    # Setup mocks
    expected_indexes = [
        IndexView(id=1, created_at=datetime.now(UTC), num_snippets=5),
        IndexView(id=2, created_at=datetime.now(UTC), num_snippets=10),
    ]
    mock_indexing_domain_service.list_indexes.return_value = expected_indexes

    # Execute
    result = await indexing_application_service.list_indexes()

    # Verify
    assert result == expected_indexes
    mock_indexing_domain_service.list_indexes.assert_called_once()


@pytest.mark.asyncio
async def test_run_index_success(
    indexing_application_service: IndexingApplicationService,
    mock_indexing_domain_service: MagicMock,
    mock_snippet_application_service: MagicMock,
    mock_bm25_service: MagicMock,
    mock_code_search_service: MagicMock,
    mock_text_search_service: MagicMock,
    mock_enrichment_service: MagicMock,
    mock_session: MagicMock,
) -> None:
    """Test running an index through the application service."""
    # Setup mocks
    index_id = 1
    mock_index = MagicMock()
    mock_index.id = index_id
    mock_indexing_domain_service.get_index.return_value = mock_index

    # Create mock Snippet entities
    mock_snippet1 = MagicMock(spec=Snippet)
    mock_snippet1.id = 1
    mock_snippet1.content = "def hello(): pass"
    mock_snippet2 = MagicMock(spec=Snippet)
    mock_snippet2.id = 2
    mock_snippet2.content = "def world(): pass"

    mock_snippets = [mock_snippet1, mock_snippet2]
    mock_indexing_domain_service.get_snippets_for_index.return_value = mock_snippets

    # Mock enrichment responses
    async def mock_enrichment(*args, **kwargs):
        yield EnrichmentResponse(snippet_id=1, text="enriched content")
        yield EnrichmentResponse(snippet_id=2, text="enriched content")

    mock_enrichment_service.enrich_documents = mock_enrichment

    # Mock code search responses
    async def mock_index_documents(*args, **kwargs):
        yield []

    mock_code_search_service.index_documents = mock_index_documents

    # Mock text search responses
    async def mock_text_index_documents(*args, **kwargs):
        yield []

    mock_text_search_service.index_documents = mock_text_index_documents

    # Execute
    await indexing_application_service.run_index(index_id)

    # Verify
    mock_indexing_domain_service.get_index.assert_called_once_with(index_id)
    mock_indexing_domain_service.delete_all_snippets.assert_called_once_with(index_id)
    mock_snippet_application_service.create_snippets_for_index.assert_called_once()
    mock_bm25_service.index_documents.assert_called_once()
    # Verify that commits are called at the application service level (3 times total:
    # 1. After delete_all_snippets, 2. After enrichment updates, 3. After timestamp update)
    assert mock_session.commit.call_count == 3


@pytest.mark.asyncio
async def test_run_index_not_found(
    indexing_application_service: IndexingApplicationService,
    mock_indexing_domain_service: MagicMock,
) -> None:
    """Test running an index that doesn't exist."""
    # Setup mocks
    mock_indexing_domain_service.get_index.return_value = None

    # Execute and verify
    with pytest.raises(ValueError, match="Index not found: 999"):
        await indexing_application_service.run_index(999)


@pytest.mark.asyncio
async def test_enrichment_duplicate_bug_with_database_simulation(
    indexing_application_service: IndexingApplicationService,
    mock_indexing_domain_service: MagicMock,
    mock_snippet_application_service: MagicMock,
    mock_bm25_service: MagicMock,
    mock_code_search_service: MagicMock,
    mock_text_search_service: MagicMock,
    mock_enrichment_service: MagicMock,
) -> None:
    """Regression test to ensure enrichment updates existing snippets instead of creating duplicates.

    This test verifies that the enrichment process correctly updates existing snippets
    rather than creating duplicate entries in the database.
    """
    # Setup mocks
    index_id = 1
    mock_index = MagicMock()
    mock_index.id = index_id
    mock_indexing_domain_service.get_index.return_value = mock_index

    # Simulate a database that tracks all snippets (original + any duplicates)
    database_snippets = []

    # Create mock Snippet entities
    mock_snippet1 = MagicMock(spec=Snippet)
    mock_snippet1.id = 1
    mock_snippet1.file_id = 1
    mock_snippet1.index_id = 1
    mock_snippet1.content = "def hello(): pass"

    mock_snippet2 = MagicMock(spec=Snippet)
    mock_snippet2.id = 2
    mock_snippet2.file_id = 1
    mock_snippet2.index_id = 1
    mock_snippet2.content = "def world(): pass"

    original_snippets = [mock_snippet1, mock_snippet2]

    # Original snippets as dicts for database simulation
    original_snippets_dict = [
        {"id": 1, "file_id": 1, "index_id": 1, "content": "def hello(): pass"},
        {"id": 2, "file_id": 1, "index_id": 1, "content": "def world(): pass"},
    ]

    # Add original snippets to our simulated database
    database_snippets.extend(original_snippets_dict.copy())

    mock_indexing_domain_service.get_snippets_for_index.return_value = original_snippets

    # Track update_snippet_content calls instead of add_snippet
    update_calls = []

    async def track_update_snippet_content(snippet_id: int, content: str):
        """Track snippet content updates (proper behavior)."""
        # Find the snippet in the database and update it (simulating SQLAlchemy behavior)
        for snippet in database_snippets:
            if snippet["id"] == snippet_id:
                snippet["content"] = content
                break
        update_calls.append((snippet_id, content))

    mock_indexing_domain_service.update_snippet_content.side_effect = (
        track_update_snippet_content
    )

    # Mock enrichment responses
    async def mock_enrichment(*args, **kwargs):
        yield EnrichmentResponse(snippet_id=1, text="This function says hello")
        yield EnrichmentResponse(snippet_id=2, text="This function says world")

    mock_enrichment_service.enrich_documents = mock_enrichment

    # Mock search services
    async def mock_index_documents(*args, **kwargs):
        yield []

    mock_code_search_service.index_documents = mock_index_documents
    mock_text_search_service.index_documents = mock_index_documents

    # Execute the enrichment process
    await indexing_application_service.run_index(index_id)

    # VERIFICATION: Check that enrichment properly updates without creating duplicates
    print(f"Total snippets in database after enrichment: {len(database_snippets)}")
    print("Database contents:")
    for i, snippet in enumerate(database_snippets):
        print(
            f"  Snippet {i}: id={snippet['id']}, content={snippet['content'][:50]}..."
        )

    # Verify we have exactly 2 snippets (no duplicates)
    assert len(database_snippets) == 2, (
        f"Expected 2 snippets (updated originals), but found {len(database_snippets)}."
    )

    # Verify that update_snippet_content was called instead of add_snippet
    assert mock_indexing_domain_service.update_snippet_content.call_count == 2
    assert (
        mock_indexing_domain_service.add_snippet.call_count == 0
    )  # Should not be called

    # Verify the content was properly enriched in place
    for snippet in database_snippets:
        assert "This function says" in snippet["content"]
        assert "```\ndef " in snippet["content"]
        assert snippet["content"].endswith("\n```")

    # Verify no duplicate IDs exist
    snippet_ids = [s["id"] for s in database_snippets]
    unique_ids = set(snippet_ids)
    assert len(snippet_ids) == len(unique_ids), "Found duplicate snippet IDs"

    # Verify the correct snippet IDs are present
    assert unique_ids == {1, 2}, f"Expected snippet IDs {{1, 2}}, got {unique_ids}"


@pytest.mark.asyncio
async def test_run_index_with_empty_snippets_list(
    indexing_application_service: IndexingApplicationService,
    mock_indexing_domain_service: MagicMock,
    mock_snippet_application_service: MagicMock,
    mock_bm25_service: MagicMock,
    mock_code_search_service: MagicMock,
    mock_text_search_service: MagicMock,
    mock_enrichment_service: MagicMock,
) -> None:
    """Test running an index with an empty repository (no indexable snippets).

    The system should detect when no snippets are found and provide a helpful
    error message to the user instead of crashing with internal errors.
    """
    # Setup mocks
    index_id = 1
    mock_index = MagicMock()
    mock_index.id = index_id
    mock_indexing_domain_service.get_index.return_value = mock_index

    # Simulate an empty repository - no snippets found after extraction
    empty_snippets = []
    mock_indexing_domain_service.get_snippets_for_index.return_value = empty_snippets

    # Execute and verify that the system detects empty repositories and provides a helpful error
    with pytest.raises(EmptySourceError) as exc_info:
        await indexing_application_service.run_index(index_id)

    # Verify the sequence of calls that should occur before the early failure
    mock_indexing_domain_service.get_index.assert_called_once_with(index_id)
    mock_indexing_domain_service.delete_all_snippets.assert_called_once_with(index_id)
    mock_snippet_application_service.create_snippets_for_index.assert_called_once()

    # Verify that the indexing services were not called due to the early failure
    mock_bm25_service.index_documents.assert_not_called()
    mock_code_search_service.index_documents.assert_not_called()
    mock_text_search_service.index_documents.assert_not_called()
    mock_enrichment_service.enrich_documents.assert_not_called()


@pytest.mark.asyncio
def test_keyword_search_with_filters_calls_bm25(
    indexing_application_service,
    mock_bm25_service,
    mock_snippet_application_service,
):
    request = MultiSearchRequest(
        keywords=["test"], top_k=10, filters=SnippetSearchFilters(language="python")
    )
    mock_bm25_service.search.return_value = []
    mock_snippet_application_service.search.return_value = []

    # Run
    import asyncio

    asyncio.run(indexing_application_service.search(request))
    # Assert
    mock_bm25_service.search.assert_called()


@pytest.mark.asyncio
def test_code_search_with_filters_calls_code_search(
    indexing_application_service,
    mock_code_search_service,
    mock_snippet_application_service,
):
    request = MultiSearchRequest(
        code_query="def foo(): pass",
        top_k=10,
        filters=SnippetSearchFilters(language="python"),
    )
    mock_code_search_service.search.return_value = []
    mock_snippet_application_service.search.return_value = []
    import asyncio

    asyncio.run(indexing_application_service.search(request))
    mock_code_search_service.search.assert_called()


@pytest.mark.asyncio
def test_text_search_with_filters_calls_text_search(
    indexing_application_service,
    mock_text_search_service,
    mock_snippet_application_service,
):
    request = MultiSearchRequest(
        text_query="find something",
        top_k=10,
        filters=SnippetSearchFilters(language="python"),
    )
    mock_text_search_service.search.return_value = []
    mock_snippet_application_service.search.return_value = []
    import asyncio

    asyncio.run(indexing_application_service.search(request))
    mock_text_search_service.search.assert_called()


@pytest.mark.asyncio
def test_hybrid_search_with_filters_calls_all_searches(
    indexing_application_service,
    mock_bm25_service,
    mock_code_search_service,
    mock_text_search_service,
    mock_snippet_application_service,
):
    request = MultiSearchRequest(
        keywords=["test"],
        code_query="def foo(): pass",
        text_query="find something",
        top_k=10,
        filters=SnippetSearchFilters(language="python"),
    )
    mock_bm25_service.search.return_value = []
    mock_code_search_service.search.return_value = []
    mock_text_search_service.search.return_value = []
    mock_snippet_application_service.search.return_value = []
    import asyncio

    asyncio.run(indexing_application_service.search(request))
    mock_bm25_service.search.assert_called()
    mock_code_search_service.search.assert_called()
    mock_text_search_service.search.assert_called()


@pytest.mark.asyncio
def test_filter_should_pre_filter_before_top_k(
    indexing_application_service,
    mock_bm25_service,
    mock_snippet_application_service,
    mock_indexing_domain_service,
):
    """
    Test that filtering is now applied before top_k, so Python snippets can be returned
    even if Java snippets would have higher scores in an unfiltered search.
    """
    # 10 java snippets (ids 1-10) match the keyword well, 10 python snippets (ids 11-20) do not match at all
    # Only python snippets should be returned if filtering is correct
    request = MultiSearchRequest(
        keywords=["foobar"], top_k=10, filters=SnippetSearchFilters(language="python")
    )

    # Mock the snippet application service to return filtered snippet IDs
    # This simulates what happens when language filtering is applied
    mock_snippet_application_service.search.return_value = [
        MagicMock(id=i)
        for i in range(11, 21)  # Python snippets (ids 11-20)
    ]

    # BM25 now receives filtered snippet_ids and should return only python snippet ids (11-20) as top_k
    mock_bm25_service.search.return_value = [
        MagicMock(snippet_id=i, score=1.0) for i in range(11, 21)
    ]

    # Mock the fusion results to return the filtered snippets
    mock_fusion_results = [
        MagicMock(id=i, score=1.0, original_scores=[1.0]) for i in range(11, 21)
    ]
    mock_indexing_domain_service.perform_fusion.return_value = mock_fusion_results

    # Mock the DB to return the filtered snippets (10 Python snippets with IDs 11-20)
    mock_indexing_domain_service.get_snippets_by_ids.return_value = [
        (
            {
                "id": i,
                "source_id": 1,
                "mime_type": "text/plain",
                "uri": f"test{i}.py",
                "cloned_path": f"/tmp/test_repo/test{i}.py",
                "sha256": "abc123",
                "size_bytes": 100,
                "extension": "py",
                "created_at": "2023-01-01",
                "updated_at": "2023-01-01",
                "source_uri": "https://github.com/test/repo.git",
                "source_cloned_path": "/tmp/test_repo",
            },
            {
                "id": i,
                "file_id": 1,
                "index_id": 1,
                "content": f"def test{i}(): pass",
                "created_at": "2023-01-01",
                "updated_at": "2023-01-01",
            },
        )
        for i in range(11, 21)  # Python snippets (ids 11-20)
    ]

    # Run
    import asyncio

    result = asyncio.run(indexing_application_service.search(request))

    # Should return 10 Python snippets (ids 11-20)
    assert len(result) == 10
    assert all(snippet.id >= 11 and snippet.id <= 20 for snippet in result)

    # Verify that BM25 was called with filtered snippet_ids
    mock_bm25_service.search.assert_called_once()
    call_args = mock_bm25_service.search.call_args[0][0]
    assert call_args.snippet_ids == list(range(11, 21))  # Python snippet IDs (11-20)


@pytest.mark.asyncio
def test_vector_search_with_language_filtering_bug(
    indexing_application_service,
    mock_text_search_service,
    mock_snippet_application_service,
    mock_indexing_domain_service,
):
    """
    Test that demonstrates the bug where language filtering changes vector search results
    even when the original result matches the language filter.
    This test should fail because the current logic excludes the best match.
    """
    # Create a request with text query and language filter
    request = MultiSearchRequest(
        text_query="data science",  # This should match snippet with ID 4 best
        top_k=3,
        filters=SnippetSearchFilters(language="python"),
    )

    # Mock the snippet application service to return a limited set of snippets
    # This simulates what happens when language filtering is applied
    # The snippet service returns snippets 1, 2, 3, 4 (all Python files)
    # Snippet 4 should be the best match for "data science" and should be included
    mock_snippet_application_service.search.return_value = [
        MagicMock(id=1),  # Python snippet 1
        MagicMock(id=2),  # Python snippet 2
        MagicMock(id=3),  # Python snippet 3
        MagicMock(
            id=4
        ),  # Python snippet 4 - should be the best match for "data science"
    ]

    # Mock the vector search service to return results based on the filtered snippet IDs
    # Snippet 4 should be the best match for "data science" and should be ranked first
    mock_text_search_service.search.return_value = [
        MagicMock(snippet_id=4, score=0.95),  # Best match for "data science"
        MagicMock(snippet_id=2, score=0.8),  # Second best match
        MagicMock(snippet_id=1, score=0.6),  # Third best match
        MagicMock(snippet_id=3, score=0.4),  # Fourth best match
    ]

    # Mock the fusion results to match what the vector search returns
    mock_fusion_results = [
        MagicMock(id=4, score=0.95, original_scores=[0.95]),
        MagicMock(id=2, score=0.8, original_scores=[0.8]),
        MagicMock(id=1, score=0.6, original_scores=[0.6]),
        MagicMock(id=3, score=0.4, original_scores=[0.4]),
    ]
    mock_indexing_domain_service.perform_fusion.return_value = mock_fusion_results

    # Mock the DB to return only the top 3 snippets (matching top_k=3)
    mock_indexing_domain_service.get_snippets_by_ids.return_value = [
        (
            {
                "id": 4,
                "uri": "test4.py",
                "content": "Python supports multiple programming paradigms",
            },
            {"id": 4, "content": "Python supports multiple programming paradigms"},
        ),
        (
            {
                "id": 2,
                "uri": "test2.py",
                "content": "Python is a high-level programming language",
            },
            {"id": 2, "content": "Python is a high-level programming language"},
        ),
        (
            {
                "id": 1,
                "uri": "test1.py",
                "content": "The Python programming language was created by Guido van Rossum",
            },
            {
                "id": 1,
                "content": "The Python programming language was created by Guido van Rossum",
            },
        ),
    ]

    # Run the search
    import asyncio

    result = asyncio.run(indexing_application_service.search(request))

    # Verify that the vector search was called with the filtered snippet IDs
    mock_text_search_service.search.assert_called_once()
    call_args = mock_text_search_service.search.call_args[0][0]
    assert call_args.snippet_ids == [1, 2, 3, 4]  # All snippet IDs

    # The fix: snippet 4 (which is the best match for "data science")
    # should now be included in the search because it's in the filtered snippet IDs
    # and it IS a Python file

    # This assertion should PASS because the fix works correctly:
    # We're getting snippet 4 as the top result, which is the best match for "data science"
    assert result[0].id == 4, (
        f"Expected snippet 4 to be the top result for 'data science', "
        f"but got snippet {result[0].id}. "
        f"This demonstrates that the filtering fix is working correctly."
    )
