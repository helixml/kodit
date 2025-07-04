"""End-to-end tests for CodeIndexingApplicationService."""

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
from kodit.domain.interfaces import ProgressCallback
from kodit.domain.protocols import IndexRepository
from kodit.domain.services.index_query_service import IndexQueryService
from kodit.domain.value_objects import (
    MultiSearchRequest,
    ProgressEvent,
    SnippetSearchFilters,
)
from kodit.infrastructure.indexing.fusion_service import ReciprocalRankFusionService
from kodit.infrastructure.sqlalchemy.index_repository import SqlAlchemyIndexRepository


class MockProgressCallback(ProgressCallback):
    """Mock implementation of ProgressCallback for testing."""

    def __init__(self) -> None:
        """Initialize the mock progress callback."""
        self.progress_calls = []
        self.complete_calls = []

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

    # Test keyword search
    keyword_results = await code_indexing_service.search(
        MultiSearchRequest(keywords=["calculator", "add"], top_k=5)
    )
    assert len(keyword_results) > 0, "Keyword search should return results"

    # Verify results contain relevant content
    result_contents = [result.content.lower() for result in keyword_results]
    assert any("calculator" in content for content in result_contents), (
        "Keyword search should find calculator-related content"
    )

    # Test semantic code search
    code_results = await code_indexing_service.search(
        MultiSearchRequest(code_query="function to add numbers", top_k=5)
    )
    assert len(code_results) > 0, "Code search should return results"

    # Verify results contain relevant code patterns
    result_contents = [result.content.lower() for result in code_results]
    assert any(
        "def add" in content or "add" in content for content in result_contents
    ), "Code search should find add-related functions"

    # Test semantic text search
    text_results = await code_indexing_service.search(
        MultiSearchRequest(text_query="multiply", top_k=5)
    )
    assert len(text_results) > 0, "Text search should return results"

    # Verify results contain relevant text content
    result_contents = [result.content.lower() for result in text_results]
    assert any(
        "calculator" in content or "mathematical" in content
        for content in result_contents
    ), "Text search should find mathematical operation content"

    # Test hybrid search (combining multiple search modes)
    hybrid_results = await code_indexing_service.search(
        MultiSearchRequest(
            keywords=["multiply"],
            code_query="multiply",
            text_query="multiply",
            top_k=5,
        )
    )
    assert len(hybrid_results) > 0, "Hybrid search should return results"

    # Verify hybrid search finds relevant content
    result_contents = [result.content.lower() for result in hybrid_results]
    assert any("multiply" in content for content in result_contents), (
        "Hybrid search should find multiplication-related content"
    )

    # Test search with filters
    filter_results = await code_indexing_service.search(
        MultiSearchRequest(
            code_query="multiply",
            top_k=5,
            filters=SnippetSearchFilters(language="python"),
        )
    )
    assert len(filter_results) > 0, "Filtered search should return results"

    # Verify filtered results contain validation content
    result_contents = [result.content.lower() for result in filter_results]
    assert any("multiply" in content for content in result_contents), (
        "Filtered search should find multiply-related content"
    )

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
