"""Tests for MultiSearchResult.to_markdown() method."""

from datetime import UTC, datetime

from kodit.application.services.code_search_application_service import (
    MultiSearchResult,
)
from kodit.domain.entities.git import SnippetV2
from kodit.domain.value_objects import Enrichment


class TestMultiSearchResultToMarkdown:
    """Tests for the to_markdown() class method."""

    def test_empty_results_list(self) -> None:
        """Test that empty results list returns appropriate message."""
        result = MultiSearchResult.to_markdown([])

        assert result == "# Search Results (0 matches)\n\nNo results found."

    def test_single_result_without_enrichments(self) -> None:
        """Test single result without enrichments."""
        snippet = SnippetV2(
            sha="abc123",
            content="def hello():\n    return 'world'",
            extension="py",
            derives_from=[],
            enrichments=[],
            created_at=datetime.now(UTC),
            updated_at=datetime.now(UTC),
        )

        result = MultiSearchResult(
            snippet=snippet,
            original_scores=[0.95],
            enrichment_type="development",
            enrichment_subtype="snippet",
        )

        markdown = MultiSearchResult.to_markdown([result])

        # Verify structure
        assert "# Search Results (1 matches)" in markdown
        assert "## development/snippet" in markdown
        assert "**Metadata:**" in markdown
        assert "- Type: development" in markdown
        assert "- Subtype: snippet" in markdown
        assert "- Language: python" in markdown
        assert "- Score: 0.9500" in markdown
        assert "```python" in markdown
        assert "def hello():" in markdown
        assert "return 'world'" in markdown
        # Should not have enrichments section
        assert "**Enrichments:**" not in markdown

    def test_single_result_with_enrichments(self) -> None:
        """Test single result with enrichments."""
        enrichments = [
            Enrichment(
                type="summary",
                content="This function returns a greeting",
            ),
            Enrichment(
                type="documentation",
                content="Returns the string 'world'",
            ),
        ]

        snippet = SnippetV2(
            sha="def456",
            content="def greet():\n    pass",
            extension="py",
            derives_from=[],
            enrichments=enrichments,
            created_at=datetime.now(UTC),
            updated_at=datetime.now(UTC),
        )

        result = MultiSearchResult(
            snippet=snippet,
            original_scores=[0.88, 0.76],
            enrichment_type="development",
            enrichment_subtype="snippet",
        )

        markdown = MultiSearchResult.to_markdown([result])

        # Verify enrichments section
        assert "**Enrichments:**" in markdown
        assert "- **summary:**" in markdown
        assert "This function returns a greeting" in markdown
        assert "- **documentation:**" in markdown
        assert "Returns the string 'world'" in markdown

    def test_multiple_results(self) -> None:
        """Test multiple results with separator."""
        snippet1 = SnippetV2(
            sha="aaa111",
            content="class Foo:\n    pass",
            extension="py",
            derives_from=[],
            enrichments=[],
            created_at=datetime.now(UTC),
            updated_at=datetime.now(UTC),
        )

        snippet2 = SnippetV2(
            sha="bbb222",
            content="function bar() {}",
            extension="js",
            derives_from=[],
            enrichments=[],
            created_at=datetime.now(UTC),
            updated_at=datetime.now(UTC),
        )

        results = [
            MultiSearchResult(
                snippet=snippet1,
                original_scores=[0.95],
                enrichment_type="development",
                enrichment_subtype="snippet",
            ),
            MultiSearchResult(
                snippet=snippet2,
                original_scores=[0.85],
                enrichment_type="usage",
                enrichment_subtype="example",
            ),
        ]

        markdown = MultiSearchResult.to_markdown(results)

        # Verify structure
        assert "# Search Results (2 matches)" in markdown
        assert "## development/snippet" in markdown
        assert "## usage/example" in markdown
        # Verify separator between results
        assert "\n---\n" in markdown
        # Verify both code blocks
        assert "```python" in markdown
        assert "class Foo:" in markdown
        assert "```javascript" in markdown
        assert "function bar()" in markdown

    def test_result_without_subtype(self) -> None:
        """Test result without enrichment subtype."""
        snippet = SnippetV2(
            sha="xyz789",
            content="SELECT * FROM users;",
            extension="sql",
            derives_from=[],
            enrichments=[],
            created_at=datetime.now(UTC),
            updated_at=datetime.now(UTC),
        )

        result = MultiSearchResult(
            snippet=snippet,
            original_scores=[0.75],
            enrichment_type="database",
            enrichment_subtype=None,
        )

        markdown = MultiSearchResult.to_markdown([result])

        # Verify heading uses only type when no subtype
        assert "## database\n" in markdown
        # Verify no subtype in metadata
        assert "- Subtype:" not in markdown

    def test_result_with_empty_extension(self) -> None:
        """Test result with empty extension."""
        snippet = SnippetV2(
            sha="empty123",
            content="some code without extension",
            extension="",
            derives_from=[],
            enrichments=[],
            created_at=datetime.now(UTC),
            updated_at=datetime.now(UTC),
        )

        result = MultiSearchResult(
            snippet=snippet,
            original_scores=[0.60],
            enrichment_type="development",
            enrichment_subtype="snippet",
        )

        markdown = MultiSearchResult.to_markdown([result])

        # Should not show language when extension is empty
        assert "- Language:" not in markdown
        # Code fence should have empty language
        assert "```\n" in markdown or "```some" in markdown

    def test_result_with_unknown_extension(self) -> None:
        """Test result with unknown file extension."""
        snippet = SnippetV2(
            sha="unknown456",
            content="some unknown code",
            extension="xyz",
            derives_from=[],
            enrichments=[],
            created_at=datetime.now(UTC),
            updated_at=datetime.now(UTC),
        )

        result = MultiSearchResult(
            snippet=snippet,
            original_scores=[0.55],
            enrichment_type="development",
            enrichment_subtype="snippet",
        )

        markdown = MultiSearchResult.to_markdown([result])

        # Should show the extension as-is when unknown
        assert "- Language: xyz" in markdown
        assert "```xyz" in markdown

    def test_result_with_multiple_scores(self) -> None:
        """Test result with multiple original scores."""
        snippet = SnippetV2(
            sha="multi789",
            content="package main",
            extension="go",
            derives_from=[],
            enrichments=[],
            created_at=datetime.now(UTC),
            updated_at=datetime.now(UTC),
        )

        result = MultiSearchResult(
            snippet=snippet,
            original_scores=[0.95, 0.88, 0.76],
            enrichment_type="development",
            enrichment_subtype="snippet",
        )

        markdown = MultiSearchResult.to_markdown([result])

        # Verify all scores are shown
        assert "- Score: 0.9500, 0.8800, 0.7600" in markdown

    def test_result_with_no_scores(self) -> None:
        """Test result with empty scores list."""
        snippet = SnippetV2(
            sha="noscore123",
            content="print('test')",
            extension="py",
            derives_from=[],
            enrichments=[],
            created_at=datetime.now(UTC),
            updated_at=datetime.now(UTC),
        )

        result = MultiSearchResult(
            snippet=snippet,
            original_scores=[],
            enrichment_type="development",
            enrichment_subtype="snippet",
        )

        markdown = MultiSearchResult.to_markdown([result])

        # Should not show score line when no scores
        assert "- Score:" not in markdown

    def test_language_mapping_for_common_extensions(self) -> None:
        """Test that common extensions map to correct language identifiers."""
        test_cases = [
            ("py", "python"),
            ("js", "javascript"),
            ("jsx", "javascript"),
            ("ts", "typescript"),
            ("tsx", "typescript"),
            ("go", "go"),
            ("rs", "rust"),
            ("java", "java"),
            ("cpp", "cpp"),
            ("rb", "ruby"),
        ]

        for ext, expected_lang in test_cases:
            snippet = SnippetV2(
                sha=f"test_{ext}",
                content="test code",
                extension=ext,
                derives_from=[],
                enrichments=[],
                created_at=datetime.now(UTC),
                updated_at=datetime.now(UTC),
            )

            result = MultiSearchResult(
                snippet=snippet,
                original_scores=[0.5],
                enrichment_type="test",
                enrichment_subtype=None,
            )

            markdown = MultiSearchResult.to_markdown([result])

            assert f"- Language: {expected_lang}" in markdown
            assert f"```{expected_lang}" in markdown
