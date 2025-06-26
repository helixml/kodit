"""Tests for the snippet extraction domain service."""

from pathlib import Path
from unittest.mock import AsyncMock, MagicMock

import pytest

from kodit.domain.enums import SnippetExtractionStrategy
from kodit.domain.services.snippet_extraction_service import (
    LanguageDetectionService,
    SnippetExtractionDomainService,
    SnippetExtractor,
)
from kodit.domain.value_objects import SnippetExtractionRequest, SnippetExtractionResult


class MockLanguageDetectionService(MagicMock):
    """Mock language detection service for testing."""

    def __init__(self) -> None:
        """Initialize the mock language detection service."""
        super().__init__(spec=LanguageDetectionService)
        self.detect_language = AsyncMock()


class MockSnippetExtractor(MagicMock):
    """Mock snippet extractor for testing."""

    def __init__(self) -> None:
        """Initialize the mock snippet extractor."""
        super().__init__(spec=SnippetExtractor)
        self.extract = AsyncMock()


@pytest.fixture
def mock_language_detector() -> MockLanguageDetectionService:
    """Create a mock language detection service."""
    return MockLanguageDetectionService()


@pytest.fixture
def mock_method_extractor() -> MockSnippetExtractor:
    """Create a mock method-based snippet extractor."""
    return MockSnippetExtractor()


@pytest.fixture
def snippet_extraction_domain_service(
    mock_language_detector: MockLanguageDetectionService,
    mock_method_extractor: MockSnippetExtractor,
) -> SnippetExtractionDomainService:
    """Create a snippet extraction domain service with mocked dependencies."""
    extractors = {SnippetExtractionStrategy.METHOD_BASED: mock_method_extractor}
    return SnippetExtractionDomainService(mock_language_detector, extractors)


@pytest.mark.asyncio
async def test_extract_snippets_success(
    snippet_extraction_domain_service: SnippetExtractionDomainService,
    mock_language_detector: MockLanguageDetectionService,
    mock_method_extractor: MockSnippetExtractor,
    tmp_path: Path,
) -> None:
    """Test successful snippet extraction."""
    # Setup
    test_file = tmp_path / "test.py"
    test_file.write_text("def hello(): pass")

    request = SnippetExtractionRequest(
        file_path=test_file, strategy=SnippetExtractionStrategy.METHOD_BASED
    )

    mock_language_detector.detect_language.return_value = "python"
    mock_method_extractor.extract.return_value = [
        "def hello(): pass",
        "def world(): pass",
    ]

    # Execute
    result = await snippet_extraction_domain_service.extract_snippets(request)

    # Verify
    assert isinstance(result, SnippetExtractionResult)
    assert result.language == "python"
    assert result.snippets == ["def hello(): pass", "def world(): pass"]
    mock_language_detector.detect_language.assert_called_once_with(test_file)
    mock_method_extractor.extract.assert_called_once_with(test_file, "python")


@pytest.mark.asyncio
async def test_extract_snippets_file_not_exists(
    snippet_extraction_domain_service: SnippetExtractionDomainService,
) -> None:
    """Test snippet extraction with non-existent file."""
    # Setup
    non_existent_file = Path("non_existent.py")
    request = SnippetExtractionRequest(
        file_path=non_existent_file, strategy=SnippetExtractionStrategy.METHOD_BASED
    )

    # Execute and verify
    with pytest.raises(ValueError, match="File does not exist"):
        await snippet_extraction_domain_service.extract_snippets(request)


@pytest.mark.asyncio
async def test_extract_snippets_unsupported_strategy(
    snippet_extraction_domain_service: SnippetExtractionDomainService,
    tmp_path: Path,
) -> None:
    """Test snippet extraction with unsupported strategy."""
    # Setup
    test_file = tmp_path / "test.py"
    test_file.write_text("def hello(): pass")

    # Create request with unsupported strategy
    request = SnippetExtractionRequest(
        file_path=test_file,
        strategy="unsupported_strategy",  # type: ignore[arg-type]
    )

    # Execute and verify
    with pytest.raises(ValueError, match="Unsupported extraction strategy"):
        await snippet_extraction_domain_service.extract_snippets(request)


@pytest.mark.asyncio
async def test_extract_snippets_filters_empty_snippets(
    snippet_extraction_domain_service: SnippetExtractionDomainService,
    mock_language_detector: MockLanguageDetectionService,
    mock_method_extractor: MockSnippetExtractor,
    tmp_path: Path,
) -> None:
    """Test that empty snippets are filtered out."""
    # Setup
    test_file = tmp_path / "test.py"
    test_file.write_text("def hello(): pass")

    request = SnippetExtractionRequest(
        file_path=test_file, strategy=SnippetExtractionStrategy.METHOD_BASED
    )

    mock_language_detector.detect_language.return_value = "python"
    # Return snippets with empty ones that should be filtered
    mock_method_extractor.extract.return_value = [
        "def hello(): pass",
        "",  # Empty snippet
        "   ",  # Whitespace-only snippet
        "def world(): pass",
    ]

    # Execute
    result = await snippet_extraction_domain_service.extract_snippets(request)

    # Verify
    assert result.snippets == ["def hello(): pass", "def world(): pass"]
    # Empty and whitespace-only snippets should be filtered out


@pytest.mark.asyncio
async def test_extract_snippets_detects_language(
    snippet_extraction_domain_service: SnippetExtractionDomainService,
    mock_language_detector: MockLanguageDetectionService,
    mock_method_extractor: MockSnippetExtractor,
    tmp_path: Path,
) -> None:
    """Test that language detection is called correctly."""
    # Setup
    test_file = tmp_path / "test.js"
    test_file.write_text("function hello() {}")

    request = SnippetExtractionRequest(
        file_path=test_file, strategy=SnippetExtractionStrategy.METHOD_BASED
    )

    mock_language_detector.detect_language.return_value = "javascript"
    mock_method_extractor.extract.return_value = ["function hello() {}"]

    # Execute
    result = await snippet_extraction_domain_service.extract_snippets(request)

    # Verify
    assert result.language == "javascript"
    mock_language_detector.detect_language.assert_called_once_with(test_file)
    mock_method_extractor.extract.assert_called_once_with(test_file, "javascript")
