"""End-to-end tests for CodeIndexingApplicationService."""

from datetime import UTC, datetime
from pathlib import Path

import pytest
from sqlalchemy import func, select
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.application.factories.code_indexing_factory import (
    create_fast_test_code_indexing_application_service,
)
from kodit.application.services.code_indexing_application_service import (
    CodeIndexingApplicationService,
)
from kodit.config import AppContext
from kodit.domain.entities import (
    Author,
    File,
    Index,
    Snippet,
    Source,
    SourceType,
)
from kodit.domain.errors import EmptySourceError
from kodit.domain.interfaces import ProgressCallback
from kodit.domain.services.source_service import SourceService
from kodit.domain.value_objects import (
    MultiSearchRequest,
    ProgressEvent,
    SnippetSearchFilters,
)


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
async def sample_source(session: AsyncSession, tmp_path: Path) -> Source:
    """Create a sample source for testing."""
    source = Source(
        uri=f"file://{tmp_path}/test-repo",
        cloned_path=str(tmp_path / "test-repo"),
        source_type=SourceType.FOLDER,
    )
    session.add(source)
    await session.commit()
    await session.refresh(source)
    return source


@pytest.fixture
async def sample_file(
    session: AsyncSession, sample_source: Source, tmp_path: Path
) -> File:
    """Create a sample file for testing."""
    now = datetime.now(UTC)
    file = File(
        created_at=now,
        updated_at=now,
        source_id=sample_source.id,
        mime_type="text/plain",
        uri=f"file://{tmp_path}/test-repo/test.py",
        cloned_path=str(tmp_path / "test-repo/test.py"),
        sha256="",
        size_bytes=0,
        extension="py",
    )
    session.add(file)
    await session.commit()
    await session.refresh(file)
    return file


@pytest.fixture
async def sample_author(session: AsyncSession) -> Author:
    """Create a sample author for testing."""
    author = Author(
        name="Test Author",
        email="test@example.com",
    )
    session.add(author)
    await session.commit()
    await session.refresh(author)
    return author


@pytest.fixture
async def sample_index(session: AsyncSession, sample_source: Source) -> Index:
    """Create a sample index for testing."""
    index = Index(
        source_id=sample_source.id,
    )
    session.add(index)
    await session.commit()
    await session.refresh(index)
    return index


@pytest.fixture
async def code_indexing_service(
    session: AsyncSession, app_context: AppContext
) -> CodeIndexingApplicationService:
    """Create a real CodeIndexingApplicationService with all dependencies."""
    source_service = SourceService(
        clone_dir=app_context.get_clone_dir(),
        session_factory=lambda: session,
    )

    return create_fast_test_code_indexing_application_service(
        app_context=app_context,
        session=session,
        source_service=source_service,
    )


@pytest.mark.asyncio
async def test_run_index_with_empty_source_raises_error(
    session: AsyncSession,
    code_indexing_service: CodeIndexingApplicationService,
    tmp_path: Path,
) -> None:
    """Test that run_index raises EmptySourceError when no indexable files are.

    found.
    """
    # Create a valid source with no files
    source = Source(
        uri=f"file://{tmp_path}/empty-repo",
        cloned_path=str(tmp_path / "empty-repo"),
        source_type=SourceType.FOLDER,
    )
    session.add(source)
    await session.commit()
    await session.refresh(source)

    # Create an index for the valid but empty source
    index = Index(
        source_id=source.id,
    )
    session.add(index)
    await session.commit()
    await session.refresh(index)

    # Run indexing should fail
    with pytest.raises(EmptySourceError, match="No indexable snippets found"):
        await code_indexing_service.run_index(index.id)


@pytest.mark.asyncio
async def test_run_index_with_nonexistent_index_raises_error(
    code_indexing_service: CodeIndexingApplicationService,
) -> None:
    """Test that run_index raises ValueError for non-existent index."""
    with pytest.raises(ValueError, match="Index not found"):
        await code_indexing_service.run_index(99999)


@pytest.mark.asyncio
async def test_run_index_deletes_old_snippets(
    session: AsyncSession,
    code_indexing_service: CodeIndexingApplicationService,
    tmp_path: Path,
) -> None:
    """Test that run_index deletes old snippets before creating new ones."""
    # Create a temporary Python file
    test_file = tmp_path / "test.py"
    test_file.write_text("""
def old_function():
    return "old"
""")

    # Create source and index
    source = Source(
        uri=f"file://{tmp_path}",
        cloned_path=str(tmp_path),
        source_type=SourceType.FOLDER,
    )
    session.add(source)
    await session.commit()
    await session.refresh(source)

    index = Index(
        source_id=source.id,
    )
    session.add(index)
    await session.commit()
    await session.refresh(index)

    now = datetime.now(UTC)
    file = File(
        created_at=now,
        updated_at=now,
        source_id=source.id,
        mime_type="text/plain",
        uri=f"file://{tmp_path}/test.py",
        cloned_path=str(test_file),
        sha256="",
        size_bytes=test_file.stat().st_size,
        extension="py",
    )
    session.add(file)
    await session.commit()
    await session.refresh(file)

    # Run indexing first time
    await code_indexing_service.run_index(index.id)

    # Verify snippets were created
    result = await session.execute(
        select(func.count(Snippet.id)).where(Snippet.file_id == file.id)
    )
    first_count = result.scalar()
    assert first_count is not None, "First count should not be None"
    assert first_count > 0, "Snippets should be created in first run"

    # Update the file content
    test_file.write_text("""
def new_function():
    return "new"
""")

    # Run indexing again
    await code_indexing_service.run_index(index.id)

    # Verify old snippets were deleted and new ones created
    result = await session.execute(
        select(func.count(Snippet.id)).where(Snippet.file_id == file.id)
    )
    second_count = result.scalar()
    assert second_count is not None, "Second count should not be None"
    assert second_count > 0, "New snippets should be created"
    assert second_count == first_count, (
        "Should have same number of snippets (one function each)"
    )

    # Verify the content changed
    result = await session.execute(
        select(Snippet.content).where(Snippet.file_id == file.id).limit(1)
    )
    snippet_content = result.scalar()
    assert snippet_content is not None, "Snippet content should exist"
    assert "new_function" in snippet_content, "Should contain new function"
    assert "old_function" not in snippet_content, "Should not contain old function"


@pytest.mark.asyncio
async def test_run_index_with_progress_callback(
    session: AsyncSession,
    code_indexing_service: CodeIndexingApplicationService,
    tmp_path: Path,
) -> None:
    """Test that run_index calls progress callback during execution."""
    # Create a temporary Python file
    test_file = tmp_path / "test.py"
    test_file.write_text("""
def test_function():
    return "test"
""")

    # Create source and index
    source = Source(
        uri=f"file://{tmp_path}",
        cloned_path=str(tmp_path),
        source_type=SourceType.FOLDER,
    )
    session.add(source)
    await session.commit()
    await session.refresh(source)

    index = Index(
        source_id=source.id,
    )
    session.add(index)
    await session.commit()
    await session.refresh(index)

    now = datetime.now(UTC)
    file = File(
        created_at=now,
        updated_at=now,
        source_id=source.id,
        mime_type="text/plain",
        uri=f"file://{tmp_path}/test.py",
        cloned_path=str(test_file),
        sha256="",
        size_bytes=test_file.stat().st_size,
        extension="py",
    )
    session.add(file)
    await session.commit()
    await session.refresh(file)

    # Create progress callback
    progress_callback = MockProgressCallback()

    # Run indexing with progress callback
    await code_indexing_service.run_index(index.id, progress_callback)

    # Verify progress was reported
    assert len(progress_callback.progress_calls) > 0, "Progress should be reported"

    # Verify we have progress for different stages
    operations = [call["operation"] for call in progress_callback.progress_calls]
    assert "bm25_index" in operations, "Should report BM25 indexing progress"
    assert "code_embeddings" in operations, "Should report code embedding progress"
    assert "enrichment" in operations, "Should report enrichment progress"
    assert "text_embeddings" in operations, "Should report text embedding progress"


@pytest.mark.asyncio
async def test_search_finds_relevant_snippets(
    session: AsyncSession,
    code_indexing_service: CodeIndexingApplicationService,
    tmp_path: Path,
) -> None:
    """Test that search function finds relevant snippets using different.

    search modes.
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

    # Create source and index
    source = Source(
        uri=f"file://{tmp_path}",
        cloned_path=str(tmp_path),
        source_type=SourceType.FOLDER,
    )
    session.add(source)
    await session.commit()
    await session.refresh(source)

    index = Index(
        source_id=source.id,
    )
    session.add(index)
    await session.commit()
    await session.refresh(index)

    # Create file record
    now = datetime.now(UTC)
    file = File(
        created_at=now,
        updated_at=now,
        source_id=source.id,
        mime_type="text/plain",
        uri=f"file://{tmp_path}/calculator.py",
        cloned_path=str(test_file),
        sha256="",
        size_bytes=test_file.stat().st_size,
        extension="py",
    )
    session.add(file)
    await session.commit()
    await session.refresh(file)

    # Run indexing to create snippets and search indexes
    await code_indexing_service.run_index(index.id)

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
        MultiSearchRequest(text_query="mathematical operations", top_k=5)
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
            code_query="multiplication function",
            text_query="mathematical calculation",
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
            code_query="validation function",
            top_k=5,
            filters=SnippetSearchFilters(language="python"),
        )
    )
    assert len(filter_results) > 0, "Filtered search should return results"

    # Verify filtered results contain validation content
    result_contents = [result.content.lower() for result in filter_results]
    assert any("validate" in content for content in result_contents), (
        "Filtered search should find validation-related content"
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
