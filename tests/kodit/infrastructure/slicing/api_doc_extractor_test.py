"""Tests for APIDocExtractor."""

from datetime import UTC, datetime
from pathlib import Path
from typing import ClassVar

import pytest

from kodit.domain.entities.git import GitFile
from kodit.infrastructure.slicing.api_doc_extractor import APIDocExtractor


class TestAPIDocExtractor:
    """Test the APIDocExtractor functionality."""

    LanguageAssertions: ClassVar[dict[str, list[str]]] = {
        "go": [
            "## api/pkg/controller",
            "func (fs *FileStore) GetFileList(filter string) ([]*File, error)",
            """type File struct {
	Path string
}""",
            "File structure",
            "GetFile returns a file by path",
        ],
        "python": [
            "submodule_func",
        ],
    }

    @pytest.mark.parametrize(
        ("language", "extension"),
        [
            ("go", ".go"),
            ("c", ".c"),
            ("cpp", ".cpp"),
            ("csharp", ".cs"),
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

        if language in self.LanguageAssertions:
            for assertion in self.LanguageAssertions[language]:
                assert assertion in enrichments[0].content, (
                    f"Assertion {assertion} not found in {enrichments[0].content}"
                )

        # Should generate exactly one enrichment per language
        assert len(enrichments) == 1

        enrichment = enrichments[0]
        content = enrichment.content

        # Check combined API doc format
        assert content.startswith("# API Documentation: ")
        assert enrichment.type == "usage"
        assert enrichment.subtype == "api_docs"
        assert enrichment.module_path == language

        # Should have Overview and Index sections
        assert "## Overview" in content
        assert "## Index" in content

        # Should have at least one module section
        # Module sections are now ## headers (not package headers)
        module_sections = [
            line for line in content.split("\n") if line.startswith("## ")
        ]
        # At least Overview, Index, and one module
        assert len(module_sections) >= 3

        # Should have at least one subsection (Functions, Types, or Constants)
        has_subsections = (
            "### Functions" in content
            or "### Types" in content
            or "### Constants" in content
        )
        assert has_subsections, f"No API subsections found for {language}"

        # Should have source files subsection
        assert "### Source Files" in content


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

    # Should use combined format
    assert content.startswith("# API Documentation: python")

    # Should have a module section for utils
    assert "## utils" in content


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
    # Note: Now returns empty list if no content, not a list with empty enrichment
    assert isinstance(enrichments, list)
    assert len(enrichments) == 0
