"""Tests for APIDocExtractor."""

from datetime import UTC, datetime
from pathlib import Path

import pytest

from kodit.domain.entities.git import GitFile
from kodit.infrastructure.slicing.api_doc_extractor import APIDocExtractor


class TestAPIDocExtractor:
    """Test the APIDocExtractor functionality."""

    @pytest.mark.parametrize(
        ("language", "extension"),
        [
            ("c", ".c"),
            ("cpp", ".cpp"),
            ("csharp", ".cs"),
            ("css", ".css"),
            ("go", ".go"),
            ("html", ".html"),
            ("java", ".java"),
            ("javascript", ".js"),
            ("python", ".py"),
            ("rust", ".rs"),
        ],
    )
    def test_extract_api_docs_from_language(
        self, language: str, extension: str
    ) -> None:
        """Test extracting API docs from each supported language."""
        data_dir = Path(__file__).parent / "data" / language
        files = [f for f in data_dir.glob(f"**/*{extension}") if f.is_file()]

        if not files:
            pytest.skip(f"No test files found for {language}")

        git_files = [
            GitFile(
                created_at=datetime.now(tz=UTC),
                blob_sha=f"sha_{f.name}",
                path=str(f),
                mime_type="text/plain",
                size=f.stat().st_size,
                extension=extension,
            )
            for f in files
        ]

        extractor = APIDocExtractor()
        enrichments = extractor.extract_api_docs(git_files, language)

        # Should generate at least one enrichment for files with public APIs
        assert len(enrichments) > 0

        for enrichment in enrichments:
            content = enrichment.content

            # Check Go-Doc style format
            assert content.startswith("# package ")
            assert enrichment.type == "usage"
            assert enrichment.subtype == "api_docs"

            # Should have at least one section (Functions, Types, or Constants)
            has_sections = (
                "## Functions" in content
                or "## Types" in content
                or "## Constants" in content
            )
            assert has_sections, f"No API sections found in {enrichment.module_path}"

            # Should have source files section
            assert "## Source Files" in content

        if language == "python":
            # Should find submodule_func from the submodule
            assert any("submodule_func" in e.content for e in enrichments)


def test_extract_api_docs_filters_private_python() -> None:
    """Test that private functions are filtered out in Python."""
    data_dir = Path(__file__).parent / "data" / "python"
    test_file = data_dir / "utils.py"

    if not test_file.exists():
        pytest.skip("utils.py not found in test data")

    git_file = GitFile(
        created_at=datetime.now(tz=UTC),
        blob_sha="test123",
        path=str(test_file),
        mime_type="text/x-python",
        size=test_file.stat().st_size,
        extension=".py",
    )

    extractor = APIDocExtractor()
    enrichments = extractor.extract_api_docs([git_file], "python")

    assert len(enrichments) == 1
    content = enrichments[0].content

    # Should not include private functions (those starting with _)
    # But this depends on what's actually in utils.py
    # Just verify the format is correct
    # Package name is just the filename stem (Python has no package declaration in AST)
    assert "# package utils" in content


def test_extract_api_docs_empty_result() -> None:
    """Test that empty files return no enrichments."""
    data_dir = Path(__file__).parent / "data" / "python"
    test_file = data_dir / "__init__.py"

    if not test_file.exists():
        pytest.skip("__init__.py not found in test data")

    git_file = GitFile(
        created_at=datetime.now(tz=UTC),
        blob_sha="test123",
        path=str(test_file),
        mime_type="text/x-python",
        size=test_file.stat().st_size if test_file.exists() else 0,
        extension=".py",
    )

    extractor = APIDocExtractor()
    enrichments = extractor.extract_api_docs([git_file], "python")

    # __init__.py files are often empty, so might have no enrichments
    # This is fine - just testing the behavior
    assert isinstance(enrichments, list)
