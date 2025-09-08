"""Test the code search application service."""

from collections.abc import Callable
from pathlib import Path
from unittest.mock import patch

import pytest
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.application.factories.code_indexing_factory import (
    create_fast_test_code_indexing_application_service,
)
from kodit.application.factories.code_search_factory import (
    create_fast_test_code_search_application_service,
)
from kodit.application.services.code_indexing_application_service import (
    CodeIndexingApplicationService,
)
from kodit.application.services.code_search_application_service import (
    CodeSearchApplicationService,
)
from kodit.config import AppContext
from kodit.domain.entities import SnippetWithContext
from kodit.domain.protocols import IndexRepository
from kodit.domain.services.index_query_service import IndexQueryService
from kodit.domain.value_objects import (
    FusionRequest,
    FusionResult,
    MultiSearchRequest,
)
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
async def code_search_service(
    app_context: AppContext,
    session_factory: Callable[[], AsyncSession],
) -> CodeSearchApplicationService:
    """Create a real CodeIndexingApplicationService with all dependencies."""
    return create_fast_test_code_search_application_service(
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
async def test_search_finds_relevant_snippets(
    code_indexing_service: CodeIndexingApplicationService,
    code_search_service: CodeSearchApplicationService,
    tmp_path: Path,
    session_factory: Callable[[], AsyncSession],
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

    # Verify the index has been properly persisted with snippets
    index_repo = create_index_repository(session_factory=session_factory)
    persisted_index = await index_repo.get(index.id)
    assert persisted_index is not None, "Index should be persisted"
    assert len(persisted_index.snippets) > 0, "Index should have snippets"

    # Verify that snippets have proper IDs (not None)
    for snippet in persisted_index.snippets:
        snippet_preview = snippet.original_text()[:50]
        assert snippet.id is not None, f"Snippet should have ID: {snippet_preview}..."

    # Test keyword search - search for "add" which should find the add method
    keyword_results = await code_search_service.search(
        MultiSearchRequest(keywords=["add"], top_k=5)
    )
    assert len(keyword_results) > 0, "Keyword search should return results"

    # Verify results contain relevant content (should find the add method)
    result_contents = [result.content.lower() for result in keyword_results]
    assert any("add" in content for content in result_contents), (
        "Keyword search should find add-related content"
    )

    # Test semantic code search
    code_results = await code_search_service.search(
        MultiSearchRequest(code_query="function to add numbers", top_k=5)
    )
    assert len(code_results) > 0, "Code search should return results"

    # Test search with top_k limit
    limited_results = await code_search_service.search(
        MultiSearchRequest(keywords=["function"], top_k=2)
    )
    assert len(limited_results) <= 2, "Search should respect top_k limit"

    # Test search with no matching content
    no_match_results = await code_search_service.search(
        MultiSearchRequest(keywords=["nonexistentkeyword"], top_k=5)
    )
    assert len(no_match_results) == 0, (
        "Search should return empty results for no matches"
    )


@pytest.mark.asyncio
async def test_vectorchord_bug_zip_mismatch(
    code_indexing_service: CodeIndexingApplicationService,
    code_search_service: CodeSearchApplicationService,
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
            code_search_service.index_query_service,
            "perform_fusion",
            side_effect=mock_perform_fusion,
        ),
        patch.object(
            code_search_service.index_query_service,
            "get_snippets_by_ids",
            side_effect=mock_get_snippets_by_ids,
        ),
    ):
        # This search used to fail with ValueError: zip() argument 2 is longer
        await code_search_service.search(MultiSearchRequest(keywords=["add"], top_k=5))
