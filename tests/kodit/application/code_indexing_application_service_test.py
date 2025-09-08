"""End-to-end tests for CodeIndexingApplicationService."""

from collections.abc import Callable
from pathlib import Path

import pytest
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.application.factories.code_indexing_factory import (
    create_fast_test_code_indexing_application_service,
)
from kodit.application.services.code_indexing_application_service import (
    CodeIndexingApplicationService,
)
from kodit.config import AppContext
from kodit.domain.protocols import IndexRepository
from kodit.domain.services.index_query_service import IndexQueryService
from kodit.infrastructure.indexing.fusion_service import ReciprocalRankFusionService
from kodit.infrastructure.sqlalchemy.index_repository import create_index_repository


@pytest.fixture
async def index_repository(
    session_factory: Callable[[], AsyncSession],
) -> IndexRepository:
    """Create a real CodeIndexingApplicationService with all dependencies."""
    return create_index_repository(session_factory=session_factory)


@pytest.fixture
async def code_indexing_service(
    app_context: AppContext,
    session_factory: Callable[[], AsyncSession],
) -> CodeIndexingApplicationService:
    """Create a real CodeIndexingApplicationService with all dependencies."""
    return create_fast_test_code_indexing_application_service(
        app_context=app_context,
        session_factory=session_factory,
    )


@pytest.fixture
async def indexing_query_service(
    index_repository: IndexRepository,
) -> IndexQueryService:
    """Create a real IndexQueryService with all dependencies."""
    return IndexQueryService(
        index_repository=index_repository,
        fusion_service=ReciprocalRankFusionService(),
    )


@pytest.mark.asyncio
async def test_run_index_with_empty_source_succeeds(
    code_indexing_service: CodeIndexingApplicationService,
    tmp_path: Path,
) -> None:
    """Test that create_index_from_uri succeeds with empty directory."""
    # The URL sanitization bug has been fixed, so empty directories should
    # successfully create an index with no files
    index = await code_indexing_service.create_index_from_uri(str(tmp_path))
    assert index is not None, "Index should be created for empty directory"

    # Run indexing on empty directory should complete without error
    await code_indexing_service.run_index(index)

    # Should have no snippets since there are no files
    assert len(index.snippets) == 0, "Empty directory should have no snippets"


@pytest.mark.asyncio
async def test_run_index_deletes_old_snippets(
    code_indexing_service: CodeIndexingApplicationService,
    indexing_query_service: IndexQueryService,
    tmp_path: Path,
) -> None:
    """Test that run_index processes only modified files in the new system."""
    # Create a temporary Python file
    test_file = tmp_path / "test.py"
    test_file.write_text("""
def old_function():
    return "old"
""")

    # Create initial index
    index = await code_indexing_service.create_index_from_uri(str(tmp_path))
    await code_indexing_service.run_index(index)

    # Verify snippets were created for the initial file
    created_index = await indexing_query_service.get_index_by_id(index.id)
    assert created_index is not None, "Index should be created"

    # In the new system, only files marked as ADDED/MODIFIED are processed
    # Since this is a new file, it should be processed and create snippets
    assert len(created_index.snippets) > 0, "Snippets should be created for new files"

    # Update the file content
    test_file.write_text("""
def new_function():
    return "new"
""")

    # In the new system, we need to refresh the working copy to detect file changes
    # The system should detect that the file has been modified and mark it accordingly
    # The existing index should be returned since it already exists for this URI
    existing_index = await code_indexing_service.create_index_from_uri(str(tmp_path))
    assert existing_index.id == index.id, "Should return same index for same URI"

    # Run indexing again to process the modified file
    await code_indexing_service.run_index(existing_index)

    # Verify the updated content is reflected
    updated_index = await indexing_query_service.get_index_by_id(existing_index.id)
    assert updated_index

    # In the current implementation, a new index is created, so we should have snippets
    assert len(updated_index.snippets) > 0, "Should have snippets after refresh"

    # Check that the content reflects the new function
    snippet_contents = [snippet.original_text() for snippet in updated_index.snippets]
    assert any("new_function" in content for content in snippet_contents), (
        "Should contain new function content"
    )


@pytest.mark.asyncio
async def test_file_deletion_after_refresh_handles_slicer_correctly(
    code_indexing_service: CodeIndexingApplicationService,
    indexing_query_service: IndexQueryService,
    tmp_path: Path,
) -> None:
    """Test that deleted files don't cause FileNotFoundError in slicer after refresh."""
    # Create a temporary Python file
    test_file = tmp_path / "calculator.py"
    test_file.write_text("""
def add(a: int, b: int) -> int:
    return a + b

def subtract(a: int, b: int) -> int:
    return a - b
""")

    # Create initial index
    index = await code_indexing_service.create_index_from_uri(str(tmp_path))
    await code_indexing_service.run_index(index)
    assert len(index.snippets) > 0, "Should have snippets for initial file"

    # Delete the file from filesystem (simulating git pull that removes files)
    test_file.unlink()
    assert not test_file.exists(), "File should be deleted"

    # Run indexing again - this should handle deleted files correctly
    # This is where the FileNotFoundError would occur if the bug exists
    await code_indexing_service.run_index(index)

    # The above should not raise an error
    final_index = await indexing_query_service.get_index_by_id(index.id)
    assert final_index is not None
