"""Tests for the snippet domain service."""

from unittest.mock import AsyncMock, MagicMock

import pytest

from kodit.domain.entities import File, Snippet
from kodit.domain.enums import SnippetExtractionStrategy
from kodit.domain.repositories import FileRepository, SnippetRepository
from kodit.domain.services.snippet_extraction_service import (
    SnippetExtractionDomainService,
)
from kodit.domain.services.snippet_service import SnippetDomainService
from kodit.domain.value_objects import (
    MultiSearchRequest,
    MultiSearchResult,
    SnippetExtractionResult,
    SnippetListItem,
)


@pytest.fixture
def mock_snippet_extraction_service() -> MagicMock:
    """Create a mock snippet extraction service."""
    service = MagicMock(spec=SnippetExtractionDomainService)
    service.extract_snippets = AsyncMock()
    return service


@pytest.fixture
def mock_snippet_repository() -> MagicMock:
    """Create a mock snippet repository."""
    repository = MagicMock(spec=SnippetRepository)
    repository.save = AsyncMock()
    repository.get = AsyncMock()
    repository.get_by_index = AsyncMock()
    repository.delete_by_index = AsyncMock()
    repository.search = AsyncMock()
    repository.list_snippets = AsyncMock()
    return repository


@pytest.fixture
def mock_file_repository() -> MagicMock:
    """Create a mock file repository."""
    repository = MagicMock(spec=FileRepository)
    repository.get_files_for_index = AsyncMock()
    return repository


@pytest.fixture
def snippet_domain_service(
    mock_snippet_extraction_service: MagicMock,
    mock_snippet_repository: MagicMock,
    mock_file_repository: MagicMock,
) -> SnippetDomainService:
    """Create a snippet domain service with mocked dependencies."""
    return SnippetDomainService(
        snippet_extraction_service=mock_snippet_extraction_service,
        snippet_repository=mock_snippet_repository,
        file_repository=mock_file_repository,
    )


@pytest.mark.asyncio
async def test_extract_and_create_snippets_success(
    snippet_domain_service: SnippetDomainService,
    mock_snippet_extraction_service: MagicMock,
    mock_snippet_repository: MagicMock,
    mock_file_repository: MagicMock,
) -> None:
    """Test successful snippet extraction and creation."""
    # Setup
    index_id = 1
    file1 = MagicMock(spec=File)
    file1.id = 1
    file1.cloned_path = "/test/file1.py"
    file1.mime_type = "text/x-python"

    file2 = MagicMock(spec=File)
    file2.id = 2
    file2.cloned_path = "/test/file2.py"
    file2.mime_type = "text/x-python"

    mock_file_repository.get_files_for_index.return_value = [file1, file2]

    # Mock extraction results
    mock_snippet_extraction_service.extract_snippets.side_effect = [
        SnippetExtractionResult(
            snippets=["def func1(): pass", "class Test: pass"], language="python"
        ),
        SnippetExtractionResult(snippets=["def func2(): pass"], language="python"),
    ]

    # Mock saved snippets
    saved_snippets = []

    async def mock_save(snippet: Snippet) -> Snippet:
        """Mock the save method."""
        saved = MagicMock(spec=Snippet)
        saved.id = len(saved_snippets) + 1
        saved.file_id = snippet.file_id
        saved.index_id = snippet.index_id
        saved.content = snippet.content
        saved_snippets.append(saved)
        return saved

    mock_snippet_repository.save.side_effect = mock_save

    # Execute
    result = await snippet_domain_service.extract_and_create_snippets(
        index_id=index_id,
        strategy=SnippetExtractionStrategy.METHOD_BASED,
    )

    # Verify
    assert len(result) == 3
    assert all(isinstance(s, MagicMock) for s in result)
    mock_file_repository.get_files_for_index.assert_called_once_with(index_id)
    assert mock_snippet_extraction_service.extract_snippets.call_count == 2
    assert mock_snippet_repository.save.call_count == 3


@pytest.mark.asyncio
async def test_extract_and_create_snippets_skips_unsupported_files(
    snippet_domain_service: SnippetDomainService,
    mock_file_repository: MagicMock,
    mock_snippet_extraction_service: MagicMock,
) -> None:
    """Test that unsupported files are skipped."""
    # Setup
    index_id = 1
    unsupported_file = MagicMock(spec=File)
    unsupported_file.cloned_path = "/test/binary.bin"
    unsupported_file.mime_type = "unknown/unknown"

    mock_file_repository.get_files_for_index.return_value = [unsupported_file]

    # Execute
    result = await snippet_domain_service.extract_and_create_snippets(
        index_id=index_id,
        strategy=SnippetExtractionStrategy.METHOD_BASED,
    )

    # Verify
    assert len(result) == 0
    mock_snippet_extraction_service.extract_snippets.assert_not_called()


@pytest.mark.asyncio
async def test_get_snippets_for_index(
    snippet_domain_service: SnippetDomainService,
    mock_snippet_repository: MagicMock,
) -> None:
    """Test getting snippets for an index."""
    # Setup
    index_id = 1
    mock_snippets = [MagicMock(spec=Snippet), MagicMock(spec=Snippet)]
    mock_snippet_repository.get_by_index.return_value = mock_snippets

    # Execute
    result = await snippet_domain_service.get_snippets_for_index(index_id)

    # Verify
    assert result == mock_snippets
    mock_snippet_repository.get_by_index.assert_called_once_with(index_id)


@pytest.mark.asyncio
async def test_update_snippet_content_success(
    snippet_domain_service: SnippetDomainService,
    mock_snippet_repository: MagicMock,
) -> None:
    """Test successful snippet content update."""
    # Setup
    snippet_id = 1
    new_content = "updated content"
    mock_snippet = MagicMock(spec=Snippet)
    mock_snippet.id = snippet_id
    mock_snippet.content = "old content"

    mock_snippet_repository.get.return_value = mock_snippet
    mock_snippet_repository.save.return_value = mock_snippet

    # Execute
    await snippet_domain_service.update_snippet_content(snippet_id, new_content)

    # Verify
    assert mock_snippet.content == new_content
    mock_snippet_repository.get.assert_called_once_with(snippet_id)
    mock_snippet_repository.save.assert_called_once_with(mock_snippet)


@pytest.mark.asyncio
async def test_update_snippet_content_not_found(
    snippet_domain_service: SnippetDomainService,
    mock_snippet_repository: MagicMock,
) -> None:
    """Test updating non-existent snippet raises ValueError."""
    # Setup
    snippet_id = 999
    mock_snippet_repository.get.return_value = None

    # Execute and verify
    with pytest.raises(ValueError, match="Snippet not found: 999"):
        await snippet_domain_service.update_snippet_content(snippet_id, "new content")


@pytest.mark.asyncio
async def test_delete_snippets_for_index(
    snippet_domain_service: SnippetDomainService,
    mock_snippet_repository: MagicMock,
) -> None:
    """Test deleting snippets for an index."""
    # Setup
    index_id = 1

    # Execute
    await snippet_domain_service.delete_snippets_for_index(index_id)

    # Verify
    mock_snippet_repository.delete_by_index.assert_called_once_with(index_id)


@pytest.mark.asyncio
async def test_search_snippets(
    snippet_domain_service: SnippetDomainService,
    mock_snippet_repository: MagicMock,
) -> None:
    """Test searching snippets."""
    # Setup
    request = MultiSearchRequest(keywords=["test"], top_k=10)
    mock_results = [
        MagicMock(spec=SnippetListItem),
        MagicMock(spec=SnippetListItem),
    ]
    mock_snippet_repository.search.return_value = mock_results

    # Execute
    result = await snippet_domain_service.search_snippets(request)

    # Verify
    assert result == mock_results
    mock_snippet_repository.search.assert_called_once_with(request)


@pytest.mark.asyncio
async def test_list_snippets(
    snippet_domain_service: SnippetDomainService,
    mock_snippet_repository: MagicMock,
) -> None:
    """Test listing snippets with filters."""
    # Setup
    file_path = "/test/path"
    source_uri = "https://github.com/test/repo"
    mock_snippet_items = [
        SnippetListItem(
            id=1,
            file_path="test.py",
            content="test content 1",
            source_uri=source_uri
        ),
        SnippetListItem(
            id=2,
            file_path="test2.py",
            content="test content 2",
            source_uri=source_uri
        ),
    ]
    mock_snippet_repository.list_snippets.return_value = mock_snippet_items

    # Execute
    result = await snippet_domain_service.list_snippets(file_path, source_uri)

    # Verify - should return MultiSearchResult objects
    assert len(result) == 2
    assert all(isinstance(item, MultiSearchResult) for item in result)
    assert result[0].id == 1
    assert result[0].content == "test content 1"
    assert result[1].id == 2
    assert result[1].content == "test content 2"
    mock_snippet_repository.list_snippets.assert_called_once_with(file_path, source_uri)
