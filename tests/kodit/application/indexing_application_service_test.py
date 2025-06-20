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
from kodit.domain.value_objects import IndexView
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
    """Create a mock enrichment domain service."""
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
def indexing_application_service(
    mock_indexing_domain_service: MagicMock,
    mock_source_service: MagicMock,
    mock_bm25_service: MagicMock,
    mock_code_search_service: MagicMock,
    mock_text_search_service: MagicMock,
    mock_enrichment_service: MagicMock,
    mock_snippet_application_service: MagicMock,
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
    )


@pytest.mark.asyncio
async def test_create_index_success(
    indexing_application_service: IndexingApplicationService,
    mock_source_service: MagicMock,
    mock_indexing_domain_service: MagicMock,
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
