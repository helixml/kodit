"""Tests for the snippet application service."""

from pathlib import Path
from unittest.mock import AsyncMock, MagicMock

import pytest

from kodit.application.commands.snippet_commands import (
    CreateIndexSnippetsCommand,
    ExtractSnippetsCommand,
)
from kodit.application.services.snippet_application_service import (
    SnippetApplicationService,
)
from kodit.domain.entities import Snippet
from kodit.domain.enums import SnippetExtractionStrategy
from kodit.domain.repositories import FileRepository, SnippetRepository
from kodit.domain.services.snippet_extraction_service import (
    SnippetExtractionDomainService,
)
from kodit.domain.value_objects import SnippetExtractionResult


@pytest.fixture
def mock_snippet_extraction_service() -> MagicMock:
    """Create a mock snippet extraction domain service."""
    service = MagicMock(spec=SnippetExtractionDomainService)
    service.extract_snippets = AsyncMock()
    return service


@pytest.fixture
def mock_snippet_repository() -> MagicMock:
    """Create a mock snippet repository."""
    repository = MagicMock(spec=SnippetRepository)
    repository.create = AsyncMock()
    repository.get_snippets_for_index = AsyncMock()
    return repository


@pytest.fixture
def mock_file_repository() -> MagicMock:
    """Create a mock file repository."""
    repository = MagicMock(spec=FileRepository)
    repository.get_files_for_index = AsyncMock()
    return repository


@pytest.fixture
def mock_session() -> MagicMock:
    """Create a mock session."""
    session = MagicMock()
    session.commit = AsyncMock()
    return session


@pytest.fixture
def snippet_application_service(
    mock_snippet_extraction_service: MagicMock,
    mock_snippet_repository: MagicMock,
    mock_file_repository: MagicMock,
    mock_session: MagicMock,
) -> SnippetApplicationService:
    """Create a snippet application service with mocked dependencies."""
    return SnippetApplicationService(
        snippet_extraction_service=mock_snippet_extraction_service,
        snippet_repository=mock_snippet_repository,
        file_repository=mock_file_repository,
        session=mock_session,
    )


@pytest.mark.asyncio
async def test_extract_snippets_from_file_success(
    snippet_application_service: SnippetApplicationService,
    mock_snippet_extraction_service: MagicMock,
) -> None:
    """Test extracting snippets from a single file."""
    # Setup
    file_path = Path("test.py")
    strategy = SnippetExtractionStrategy.METHOD_BASED
    command = ExtractSnippetsCommand(file_path=file_path, strategy=strategy)

    mock_result = SnippetExtractionResult(
        snippets=["def hello(): pass", "def world(): pass"], language="python"
    )
    mock_snippet_extraction_service.extract_snippets.return_value = mock_result

    # Execute
    result = await snippet_application_service.extract_snippets_from_file(command)

    # Verify
    assert len(result) == 2
    assert all(isinstance(snippet, Snippet) for snippet in result)
    assert result[0].content == "def hello(): pass"
    assert result[1].content == "def world(): pass"
    mock_snippet_extraction_service.extract_snippets.assert_called_once()


@pytest.mark.asyncio
async def test_create_snippets_for_index_success(
    snippet_application_service: SnippetApplicationService,
    mock_file_repository: MagicMock,
    mock_snippet_repository: MagicMock,
    mock_snippet_extraction_service: MagicMock,
    mock_session: MagicMock,
) -> None:
    """Test creating snippets for all files in an index."""
    # Setup
    index_id = 1
    command = CreateIndexSnippetsCommand(
        index_id=index_id, strategy=SnippetExtractionStrategy.METHOD_BASED
    )

    # Use a mock file object with a mime_type attribute
    class MockFile:
        def __init__(self, id, cloned_path, mime_type="text/plain"):
            self.id = id
            self.cloned_path = cloned_path
            self.mime_type = mime_type

    mock_files = [
        MockFile(1, "file1.py"),
        MockFile(2, "file2.py"),
    ]
    mock_file_repository.get_files_for_index.return_value = mock_files

    mock_result = SnippetExtractionResult(
        snippets=["def test(): pass"], language="python"
    )
    mock_snippet_extraction_service.extract_snippets.return_value = mock_result

    # Execute
    await snippet_application_service.create_snippets_for_index(command)

    # Verify
    mock_file_repository.get_files_for_index.assert_called_once_with(index_id)
    assert mock_snippet_extraction_service.extract_snippets.call_count == 2
    assert mock_snippet_repository.save.call_count == 2
    # Verify that commit is called at the application service level
    mock_session.commit.assert_called_once()


@pytest.mark.asyncio
async def test_create_snippets_for_index_no_files(
    snippet_application_service: SnippetApplicationService,
    mock_file_repository: MagicMock,
) -> None:
    """Test creating snippets when no files are found."""
    # Setup
    index_id = 1
    command = CreateIndexSnippetsCommand(
        index_id=index_id, strategy=SnippetExtractionStrategy.METHOD_BASED
    )
    mock_file_repository.get_files_for_index.return_value = []

    # Execute
    await snippet_application_service.create_snippets_for_index(command)

    # Verify
    mock_file_repository.get_files_for_index.assert_called_once_with(index_id)
