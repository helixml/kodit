"""End-to-end tests for CodeIndexingApplicationService."""

from pathlib import Path
from typing import Any
from unittest.mock import patch

import pytest
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.application.factories.code_indexing_factory import (
    create_fast_test_code_indexing_application_service,
)
from kodit.application.services.code_indexing_application_service import (
    CodeIndexingApplicationService,
)
from kodit.config import AppContext
from kodit.domain.entities import SnippetWithContext
from kodit.domain.interfaces import ProgressCallback
from kodit.domain.protocols import IndexRepository
from kodit.domain.services.index_query_service import IndexQueryService
from kodit.domain.value_objects import (
    FusionRequest,
    FusionResult,
    MultiSearchRequest,
    ProgressEvent,
)
from kodit.infrastructure.indexing.fusion_service import ReciprocalRankFusionService
from kodit.infrastructure.sqlalchemy.index_repository import SqlAlchemyIndexRepository


class MockProgressCallback(ProgressCallback):
    """Mock implementation of ProgressCallback for testing."""

    def __init__(self) -> None:
        """Initialize the mock progress callback."""
        self.progress_calls: list[dict[str, Any]] = []
        self.complete_calls: list[str] = []

    async def on_progress(self, event: ProgressEvent) -> None:
        """Record progress events."""
        self.progress_calls.append(
            {
                "operation": event.operation,
                "current": event.current,
                "total": event.total,
                "message": event.message,
            }
        )

    async def on_complete(self, operation: str) -> None:
        """Record completion events."""
        self.complete_calls.append(operation)


@pytest.fixture
async def index_repository(
    session: AsyncSession,
) -> IndexRepository:
    """Create a real CodeIndexingApplicationService with all dependencies."""
    return SqlAlchemyIndexRepository(session=session)


@pytest.fixture
async def code_indexing_service(
    session: AsyncSession, app_context: AppContext
) -> CodeIndexingApplicationService:
    """Create a real CodeIndexingApplicationService with all dependencies."""
    return create_fast_test_code_indexing_application_service(
        app_context=app_context,
        session=session,
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
async def test_search_finds_relevant_snippets(
    code_indexing_service: CodeIndexingApplicationService,
    tmp_path: Path,
) -> None:
    """Test that search function finds relevant snippets using different search modes.

    This test verifies the new file processing behavior where only files with
    FileProcessingStatus != CLEAN are processed for snippet creation.
    """
    # Create a temporary Python file with diverse code content
    test_file = tmp_path / "calculator.py"
    test_file.write_text("""
class Calculator:
    \"\"\"A simple calculator class for mathematical operations.\"\"\"

    def add(self, a: int, b: int) -> int:
        \"\"\"Add two numbers together.\"\"\"
        return a + b

    def subtract(self, a: int, b: int) -> int:
        \"\"\"Subtract the second number from the first.\"\"\"
        return a - b

    def multiply(self, a: int, b: int) -> int:
        \"\"\"Multiply two numbers.\"\"\"
        return a * b

    def divide(self, a: int, b: int) -> float:
        \"\"\"Divide the first number by the second.\"\"\"
        if b == 0:
            raise ValueError("Cannot divide by zero")
        return a / b

def calculate_area(radius: float) -> float:
    \"\"\"Calculate the area of a circle.\"\"\"
    import math
    return math.pi * radius ** 2

def validate_input(value: str) -> bool:
    \"\"\"Validate that input is a positive number.\"\"\"
    try:
        num = float(value)
        return num > 0
    except ValueError:
        return False
""")

    # Create index using application service
    index = await code_indexing_service.create_index_from_uri(str(tmp_path))

    # Run indexing to create snippets and search indexes
    # In the new system, since this is a new file, it will be marked as ADDED
    # and processed to create snippets
    await code_indexing_service.run_index(index)

    # Ensure that the search indexes have been properly created by checking
    # that we can retrieve snippets by ID. This is crucial because the BM25 index
    # uses database IDs, so we need to ensure the snippets have been persisted
    # with their proper IDs before searching.
    from kodit.infrastructure.sqlalchemy.index_repository import (
        SqlAlchemyIndexRepository,
    )

    # Verify the index has been properly persisted with snippets
    index_repo = SqlAlchemyIndexRepository(session=code_indexing_service.session)
    persisted_index = await index_repo.get(index.id)
    assert persisted_index is not None, "Index should be persisted"
    assert len(persisted_index.snippets) > 0, "Index should have snippets"

    # Verify that snippets have proper IDs (not None)
    for snippet in persisted_index.snippets:
        snippet_preview = snippet.original_text()[:50]
        assert snippet.id is not None, f"Snippet should have ID: {snippet_preview}..."

    # Test keyword search - search for "add" which should find the add method
    keyword_results = await code_indexing_service.search(
        MultiSearchRequest(keywords=["add"], top_k=5)
    )
    assert len(keyword_results) > 0, "Keyword search should return results"

    # Verify results contain relevant content (should find the add method)
    result_contents = [result.content.lower() for result in keyword_results]
    assert any("add" in content for content in result_contents), (
        "Keyword search should find add-related content"
    )

    # Test semantic code search
    code_results = await code_indexing_service.search(
        MultiSearchRequest(code_query="function to add numbers", top_k=5)
    )
    assert len(code_results) > 0, "Code search should return results"

    # Test search with top_k limit
    limited_results = await code_indexing_service.search(
        MultiSearchRequest(keywords=["function"], top_k=2)
    )
    assert len(limited_results) <= 2, "Search should respect top_k limit"

    # Test search with no matching content
    no_match_results = await code_indexing_service.search(
        MultiSearchRequest(keywords=["nonexistentkeyword"], top_k=5)
    )
    assert len(no_match_results) == 0, (
        "Search should return empty results for no matches"
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


@pytest.mark.asyncio
async def test_vectorchord_bug_zip_mismatch(
    code_indexing_service: CodeIndexingApplicationService,
    tmp_path: Path,
) -> None:
    """Test that reproduces the vectorchord bug with zip() length mismatch.

    This happens when get_snippets_by_ids returns fewer snippets than the
    number of IDs in final_results, which can occur when some snippet IDs
    don't exist in the database or when related files/sources are missing.
    """
    # Create a temporary Python file
    test_file = tmp_path / "test_code.py"
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

    # Mock perform_fusion to always return some fake results
    # This ensures final_results is not empty
    async def mock_perform_fusion(
        rankings: list[list[FusionRequest]],  # noqa: ARG001
        k: float = 60.0,  # noqa: ARG001
    ) -> list[FusionResult]:
        # Always return some fake fusion results to ensure final_results is populated
        return [
            FusionResult(id=99999, score=1.0, original_scores=[1.0]),
            FusionResult(id=99998, score=0.8, original_scores=[0.8]),
        ]

    # Mock get_snippets_by_ids to return an empty list
    # This ensures search_results is empty while final_results is not
    async def mock_get_snippets_by_ids(ids: list[int]) -> list[SnippetWithContext]:  # noqa: ARG001
        return []

    # Apply the mocks using patch.object to avoid mypy errors
    with (
        patch.object(
            code_indexing_service.index_query_service,
            "perform_fusion",
            side_effect=mock_perform_fusion,
        ),
        patch.object(
            code_indexing_service.index_query_service,
            "get_snippets_by_ids",
            side_effect=mock_get_snippets_by_ids,
        ),
    ):
        # This search used to fail with ValueError: zip() argument 2 is longer
        await code_indexing_service.search(
            MultiSearchRequest(keywords=["add"], top_k=5)
        )
