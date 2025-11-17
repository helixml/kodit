"""Tests for APIDocExtractor."""

from datetime import UTC, datetime
from pathlib import Path
from typing import ClassVar

import pytest

from kodit.domain.entities.git import GitFile
from kodit.infrastructure.slicing.api_doc_extractor import APIDocExtractor


class TestAPIDocExtractor:
    """Test the APIDocExtractor functionality."""

    PositiveLanguageAssertions: ClassVar[dict[str, list[str]]] = {
        "go": [
            "## controller",
            "func (fs *FileStore) GetFileList(filter string) ([]*File, error)",
            """type File struct {
	Path string
}""",
            "File structure",
            "GetFile returns a file by path",
        ],
        "python": [
            "submodule_func",
            "Submodule provides submodules",
            "A point in 2D space.",
        ],
    }

    NegativeLanguageAssertions: ClassVar[dict[str, list[str]]] = {
        "python": [
            "__init__",
        ],
    }

    @pytest.mark.parametrize(
        ("language", "extension"),
        [
            ("python", ".py"),
            ("go", ".go"),
            ("c", ".c"),
            ("cpp", ".cpp"),
            ("csharp", ".cs"),
            ("java", ".java"),
            ("javascript", ".js"),
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
                commit_sha="abc123def456",
                path=str(f),
                mime_type="text/plain",
                size=f.stat().st_size,
                extension=extension,
            )
            for f in files
        ]

        extractor = APIDocExtractor()
        enrichments = extractor.extract_api_docs(
            git_files,
            language,
        )

        if language in self.PositiveLanguageAssertions:
            for assertion in self.PositiveLanguageAssertions[language]:
                assert assertion in enrichments[0].content, (
                    f"Assertion {assertion} not found in {enrichments[0].content}"
                )
        if language in self.NegativeLanguageAssertions:
            for assertion in self.NegativeLanguageAssertions[language]:
                assert assertion not in enrichments[0].content, (
                    f"Assertion {assertion} found in {enrichments[0].content}"
                )

        # Should generate exactly one enrichment per language
        assert len(enrichments) == 1

        enrichment = enrichments[0]
        content = enrichment.content

        # Check combined API doc format
        assert enrichment.type == "usage"
        assert enrichment.subtype == "api_docs"
        assert enrichment.language == language

        # Should have at least one module section
        # Module sections are now ## headers (not package headers)
        module_sections = [
            line for line in content.split("\n") if line.startswith("## ")
        ]
        # At least Index, and one module
        assert len(module_sections) >= 2

        # Should have at least one subsection (Functions, Types, or Constants)
        has_subsections = (
            "### Functions" in content
            or "### Types" in content
            or "### Constants" in content
        )
        assert has_subsections, f"No API subsections found for {language}"

        # Should have source files subsection
        assert "### Source Files" in content


def test_extract_api_docs_empty_result() -> None:
    """Test that files with only docstrings generate enrichments."""
    data_dir = Path(__file__).parent / "data" / "python"
    test_file = data_dir / "__init__.py"

    if not test_file.exists():
        pytest.skip("__init__.py not found in test data")

    git_file = GitFile(
        created_at=datetime.now(tz=UTC),
        blob_sha="test123",
        commit_sha="abc123def456",
        path=str(test_file),
        mime_type="text/x-python",
        size=test_file.stat().st_size if test_file.exists() else 0,
        extension=".py",
    )

    extractor = APIDocExtractor()
    enrichments = extractor.extract_api_docs(
        [git_file],
        "python",
    )

    # __init__.py has a module docstring, so should generate an enrichment
    # Module docstrings are considered content worth documenting
    assert isinstance(enrichments, list)
    assert len(enrichments) == 1
    assert "Python example project for testing" in enrichments[0].content


def test_constructor_params_in_api_docs() -> None:
    """Test that constructor parameters appear in API documentation."""
    # Create a simple Python file with a class that has constructor params
    import tempfile

    with tempfile.NamedTemporaryFile(mode="w", suffix=".py", delete=False) as f:
        f.write(
            '''class Calculator:
    """A simple calculator."""

    def __init__(self, precision: int, mode: str = "standard"):
        """Initialize calculator."""
        self.precision = precision
        self.mode = mode

    def add(self, a: float, b: float) -> float:
        """Add two numbers."""
        return a + b
'''
        )
        temp_path = f.name

    try:
        git_file = GitFile(
            created_at=datetime.now(tz=UTC),
            blob_sha="test123",
            commit_sha="abc123def456",
            path=temp_path,
            mime_type="text/x-python",
            size=Path(temp_path).stat().st_size,
            extension=".py",
        )

        extractor = APIDocExtractor()
        enrichments = extractor.extract_api_docs([git_file], "python")

        assert len(enrichments) == 1
        content = enrichments[0].content

        # Verify constructor parameters section exists
        assert "**Constructor Parameters:**" in content
        assert "precision: int" in content
        assert 'mode: str = "standard"' in content
    finally:
        Path(temp_path).unlink()
