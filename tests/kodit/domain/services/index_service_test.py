"""Unit tests for IndexDomainService."""

from pathlib import Path
from unittest.mock import AsyncMock, Mock

import pytest
from pydantic import AnyUrl

from kodit.domain.entities import Index, Snippet, Source, SourceType, WorkingCopy
from kodit.domain.services.enrichment_service import EnrichmentDomainService
from kodit.domain.services.index_service import (
    IndexDomainService,
    LanguageDetectionService,
)


class MockLanguageDetectionService(LanguageDetectionService):
    """Mock language detection service."""

    async def detect_language(self, file_path: Path) -> str:
        """Return a mock language based on file extension."""
        if file_path.suffix == ".py":
            return "python"
        return "unknown"


@pytest.fixture
def mock_enrichment_service() -> EnrichmentDomainService:
    """Create a mock enrichment service."""
    service = Mock(spec=EnrichmentDomainService)
    service.enrich_documents = AsyncMock()
    return service


@pytest.fixture
def index_domain_service(
    mock_enrichment_service: EnrichmentDomainService,
    tmp_path: Path,
) -> IndexDomainService:
    """Create an IndexDomainService for testing."""
    return IndexDomainService(
        language_detector=MockLanguageDetectionService(),
        enrichment_service=mock_enrichment_service,
        clone_dir=tmp_path / "clones",
    )


@pytest.mark.asyncio
async def test_prepare_index_creates_working_copy(
    index_domain_service: IndexDomainService, tmp_path: Path
) -> None:
    """Test that prepare_index creates a working copy without repository calls."""
    # Create a test file
    test_file = tmp_path / "test.py"
    test_file.write_text("def hello(): pass")

    # Prepare index (this only creates the working copy structure, no files scanned yet)
    working_copy = await index_domain_service.prepare_index(str(tmp_path))

    # Verify the working copy was created
    assert isinstance(working_copy, WorkingCopy)
    assert isinstance(working_copy.remote_uri, AnyUrl)
    assert working_copy.source_type == SourceType.FOLDER

    # In the new flow, files are not scanned during prepare_index
    # They are scanned during refresh_working_copy
    assert len(working_copy.files) == 0

    # Now refresh to actually scan the files
    refreshed_working_copy = await index_domain_service.refresh_working_copy(
        working_copy
    )
    assert len(refreshed_working_copy.files) == 1
    assert refreshed_working_copy.files[0].uri.path.endswith("test.py")  # type: ignore[union-attr]


@pytest.mark.asyncio
async def test_extract_snippets_from_index_returns_snippets(
    index_domain_service: IndexDomainService, tmp_path: Path
) -> None:
    """Test that extract_snippets_from_index returns snippets without persistence."""
    # Create a mock index with files
    test_file = tmp_path / "test.py"
    test_file.write_text("def hello(): pass")

    # Prepare working copy first
    working_copy = await index_domain_service.prepare_index(str(tmp_path))

    # Now refresh to scan the files
    working_copy = await index_domain_service.refresh_working_copy(working_copy)

    from datetime import UTC, datetime

    # Create a mock index
    source = Source(
        id=1,
        working_copy=working_copy,
    )
    index = Index(
        id=1,
        source=source,
        snippets=[],
        created_at=datetime.now(UTC),
        updated_at=datetime.now(UTC),
    )

    # Extract snippets - this method returns the updated Index, not just snippets
    updated_index = await index_domain_service.extract_snippets_from_index(
        index=index,
    )

    # Verify snippets were extracted
    assert len(updated_index.snippets) > 0
    assert isinstance(updated_index.snippets[0], Snippet)
    # The actual Slicer is used now, so we should check for the actual function
    assert "def hello" in updated_index.snippets[0].original_text()
    assert "pass" in updated_index.snippets[0].original_text()


@pytest.mark.asyncio
async def test_enrich_snippets_in_index_returns_enriched_snippets(
    tmp_path: Path,
) -> None:
    """Test enrich_snippets_in_index returns enriched snippets without persistence."""
    from kodit.domain.services.enrichment_service import EnrichmentDomainService
    from kodit.infrastructure.enrichment.null_enrichment_provider import (
        NullEnrichmentProvider,
    )

    # Create real enrichment service with null provider (fast)
    enrichment_service = EnrichmentDomainService(
        enrichment_provider=NullEnrichmentProvider()
    )

    # Create domain service with real enrichment service
    domain_service = IndexDomainService(
        language_detector=MockLanguageDetectionService(),
        enrichment_service=enrichment_service,
        clone_dir=tmp_path / "clones",
    )

    # Create mock snippets
    snippet = Snippet(derives_from=[])
    snippet.id = 1
    snippet.add_original_content("def test(): pass", "python")
    snippets = [snippet]

    # Enrich snippets
    enriched_snippets = await domain_service.enrich_snippets_in_index(snippets=snippets)

    # Verify snippets were returned (null provider doesn't actually enrich)
    assert len(enriched_snippets) == 1
    assert enriched_snippets[0].id == 1


@pytest.mark.asyncio
async def test_enrich_snippets_with_empty_list_returns_empty_list(
    index_domain_service: IndexDomainService,
) -> None:
    """Test that enriching an empty list returns an empty list."""
    enriched_snippets = await index_domain_service.enrich_snippets_in_index(snippets=[])
    assert enriched_snippets == []


@pytest.mark.asyncio
async def test_extract_snippets_only_processes_relevant_files(
    index_domain_service: IndexDomainService, tmp_path: Path
) -> None:
    """Test that extract_snippets only processes files relevant to their language."""
    # Create files of different types
    py_file = tmp_path / "test.py"
    py_file.write_text("def hello(): pass")

    js_file = tmp_path / "test.js"
    js_file.write_text("function hello() {}")

    png_file = tmp_path / "image.png"
    png_file.write_bytes(b"fake png data")

    txt_file = tmp_path / "readme.txt"
    txt_file.write_text("This is just text")

    # Prepare working copy
    working_copy = await index_domain_service.prepare_index(str(tmp_path))
    working_copy = await index_domain_service.refresh_working_copy(working_copy)

    # Verify all files were discovered
    assert len(working_copy.files) == 4

    from datetime import UTC, datetime

    from kodit.domain.entities import Index, Source

    # Create a mock index
    source = Source(id=1, working_copy=working_copy)
    index = Index(
        id=1,
        source=source,
        snippets=[],
        created_at=datetime.now(UTC),
        updated_at=datetime.now(UTC),
    )

    # Extract snippets - should process all supported languages found in files
    updated_index = await index_domain_service.extract_snippets_from_index(index)

    # Should have snippets from both Python and JavaScript files (2 supported languages)
    # PNG and TXT files should be ignored as they're not supported
    assert len(updated_index.snippets) == 2  # Python and JavaScript files

    # Get snippet texts
    snippet_texts = [s.original_text() for s in updated_index.snippets]

    # Should contain content from both supported languages
    assert any("def hello" in text for text in snippet_texts)  # Python file
    assert any("function hello" in text for text in snippet_texts)  # JavaScript file

    # Verify the snippets derive from the correct files
    source_files = [s.derives_from[0].uri.path for s in updated_index.snippets]
    assert any(path.endswith("test.py") for path in source_files)  # type: ignore[union-attr]
    assert any(path.endswith("test.js") for path in source_files)  # type: ignore[union-attr]


@pytest.mark.asyncio
async def test_extract_snippets_filters_by_language_mapping(
    index_domain_service: IndexDomainService, tmp_path: Path
) -> None:
    """Test that extract_snippets correctly filters files by language mapping."""
    # Create files for different languages
    py_file = tmp_path / "script.py"
    py_file.write_text("def python_func(): return 'python'")

    js_file = tmp_path / "script.js"
    js_file.write_text("function jsFunc() { return 'javascript'; }")

    java_file = tmp_path / "Script.java"
    java_file.write_text("public class Script { public void javaMethod() {} }")

    # Create unsupported file type
    unknown_file = tmp_path / "script.unknown"
    unknown_file.write_text("some unknown content")

    # Prepare working copy
    working_copy = await index_domain_service.prepare_index(str(tmp_path))
    working_copy = await index_domain_service.refresh_working_copy(working_copy)

    # Should discover all files
    assert len(working_copy.files) == 4

    from datetime import UTC, datetime

    from kodit.domain.entities import Index, Source

    # Create a mock index
    source = Source(id=1, working_copy=working_copy)
    index = Index(
        id=1,
        source=source,
        snippets=[],
        created_at=datetime.now(UTC),
        updated_at=datetime.now(UTC),
    )

    # Extract snippets - should process supported languages only
    updated_index = await index_domain_service.extract_snippets_from_index(index)

    # Should have snippets from Python, JavaScript, and Java files
    # (unknown file should be ignored)
    assert len(updated_index.snippets) == 3

    # Get the snippet texts
    snippet_texts = [s.original_text() for s in updated_index.snippets]

    # Should contain content from all supported languages
    assert any("def python_func" in text for text in snippet_texts)
    assert any("function jsFunc" in text for text in snippet_texts)
    assert any("javaMethod" in text for text in snippet_texts)

    # Should not contain content from unsupported file
    assert not any("unknown content" in text for text in snippet_texts)


@pytest.mark.asyncio
async def test_extract_snippets_handles_no_matching_files(
    index_domain_service: IndexDomainService, tmp_path: Path
) -> None:
    """Test extract_snippets handles case where no files match supported languages."""
    # Create only unsupported file types
    img_file = tmp_path / "image.png"
    img_file.write_bytes(b"fake image data")

    doc_file = tmp_path / "document.pdf"
    doc_file.write_bytes(b"fake pdf data")

    # Prepare working copy
    working_copy = await index_domain_service.prepare_index(str(tmp_path))
    working_copy = await index_domain_service.refresh_working_copy(working_copy)

    # Should discover files
    assert len(working_copy.files) == 2

    from datetime import UTC, datetime

    from kodit.domain.entities import Index, Source

    # Create a mock index
    source = Source(id=1, working_copy=working_copy)
    index = Index(
        id=1,
        source=source,
        snippets=[],
        created_at=datetime.now(UTC),
        updated_at=datetime.now(UTC),
    )

    # Extract snippets - should return empty list since no supported files
    updated_index = await index_domain_service.extract_snippets_from_index(index)

    # Should have no snippets
    assert len(updated_index.snippets) == 0
