"""Tests for the snippet application service."""

from pathlib import Path
from unittest.mock import AsyncMock, MagicMock

import pytest

from kodit.application.commands.snippet_commands import (
    CreateIndexSnippetsCommand,
    ExtractSnippetsCommand,
    ListSnippetsCommand,
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
from kodit.domain.value_objects import SnippetExtractionResult, SnippetListItem


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


@pytest.mark.asyncio
async def test_list_snippets():
    """Test listing snippets with optional filtering."""
    # Mock dependencies
    mock_snippet_extraction_service = AsyncMock()
    mock_snippet_repository = AsyncMock()
    mock_file_repository = AsyncMock()
    mock_session = AsyncMock()

    # Create test data
    test_snippets = [
        SnippetListItem(
            id=1,
            file_path="test.py",
            content="test snippet 1",
            source_uri="https://github.com/test/repo.git",
        ),
        SnippetListItem(
            id=2,
            file_path="test2.py",
            content="test snippet 2",
            source_uri="https://github.com/test/repo.git",
        ),
    ]

    # Mock the repository to return test data
    mock_snippet_repository.list_snippets.return_value = test_snippets

    # Create service
    service = SnippetApplicationService(
        snippet_extraction_service=mock_snippet_extraction_service,
        snippet_repository=mock_snippet_repository,
        file_repository=mock_file_repository,
        session=mock_session,
    )

    # Test listing all snippets
    command = ListSnippetsCommand()
    result = await service.list_snippets(command)

    assert len(result) == 2
    assert result[0].id == 1
    assert result[0].file_path == "test.py"
    assert result[0].content == "test snippet 1"
    assert result[0].source_uri == "https://github.com/test/repo.git"
    assert result[1].id == 2
    assert result[1].file_path == "test2.py"
    assert result[1].content == "test snippet 2"
    assert result[1].source_uri == "https://github.com/test/repo.git"

    # Verify the repository was called correctly
    mock_snippet_repository.list_snippets.assert_called_once_with(None, None)

    # Test filtering by file path
    command = ListSnippetsCommand(file_path="/tmp/test.py")
    await service.list_snippets(command)
    mock_snippet_repository.list_snippets.assert_called_with("/tmp/test.py", None)

    # Test filtering by source URI
    command = ListSnippetsCommand(source_uri="https://github.com/test/repo.git")
    await service.list_snippets(command)
    mock_snippet_repository.list_snippets.assert_called_with(
        None, "https://github.com/test/repo.git"
    )

    # Test filtering by both
    command = ListSnippetsCommand(
        file_path="/tmp/test.py", source_uri="https://github.com/test/repo.git"
    )
    await service.list_snippets(command)
    mock_snippet_repository.list_snippets.assert_called_with(
        "/tmp/test.py", "https://github.com/test/repo.git"
    )
