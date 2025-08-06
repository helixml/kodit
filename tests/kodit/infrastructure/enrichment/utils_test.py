"""Tests for enrichment utilities."""


from kodit.infrastructure.enrichment.utils import clean_thinking_tags


class TestCleanThinkingTags:
    """Test the clean_thinking_tags utility function."""

    def test_clean_thinking_tags_empty_content(self) -> None:
        """Test cleaning empty content."""
        assert clean_thinking_tags("") == ""
        assert clean_thinking_tags(None) is None  # type: ignore[arg-type]

    def test_clean_thinking_tags_no_tags(self) -> None:
        """Test cleaning content without thinking tags."""
        content = "This is a simple function that returns a value."
        assert clean_thinking_tags(content) == content

    def test_clean_thinking_tags_single_tag(self) -> None:
        """Test removing single thinking tag."""
        content = (
            "<think>\nLet me analyze this code...\n</think>\n"
            "This function performs addition."
        )
        expected = "This function performs addition."
        assert clean_thinking_tags(content) == expected

    def test_clean_thinking_tags_multiple_tags(self) -> None:
        """Test removing multiple thinking tags."""
        content = (
            "<think>\nFirst analysis\n</think>\n"
            "Some explanation here.\n"
            "<think>\nSecond analysis\n</think>\n"
            "Final conclusion."
        )
        expected = "Some explanation here.\n\nFinal conclusion."
        assert clean_thinking_tags(content) == expected

    def test_clean_thinking_tags_case_insensitive(self) -> None:
        """Test that tag removal is case insensitive."""
        content = (
            "<THINK>\nUppercase thinking\n</THINK>\n"
            "<Think>\nMixed case thinking\n</Think>\n"
            "Final response."
        )
        expected = "Final response."
        assert clean_thinking_tags(content) == expected

    def test_clean_thinking_tags_multiline_content(self) -> None:
        """Test cleaning multiline thinking content."""
        content = (
            "<think>\n"
            "This is a complex analysis\n"
            "with multiple lines\n"
            "and detailed reasoning\n"
            "</think>\n"
            "Simple response."
        )
        expected = "Simple response."
        assert clean_thinking_tags(content) == expected

    def test_clean_thinking_tags_nested_or_malformed(self) -> None:
        """Test handling malformed or unclosed thinking tags."""
        # Unclosed tag - should remain as is since regex uses non-greedy matching
        content = "<think>\nUnclosed thinking\nSome content here."
        expected = "<think>\nUnclosed thinking\nSome content here."
        assert clean_thinking_tags(content) == expected

    def test_clean_thinking_tags_whitespace_cleanup(self) -> None:
        """Test that excessive whitespace is cleaned up."""
        content = (
            "<think>\nThinking content\n</think>\n\n\n"
            "Response with extra newlines\n\n\n"
            "<think>\nMore thinking\n</think>\n\n"
            "Final content."
        )
        expected = "Response with extra newlines\n\nFinal content."
        assert clean_thinking_tags(content) == expected

    def test_clean_thinking_tags_with_other_tags(self) -> None:
        """Test that only thinking tags are removed, not other tags."""
        content = (
            "<think>\nAnalysis here\n</think>\n"
            "<code>function example() { return true; }</code>\n"
            "<strong>Important note</strong>"
        )
        expected = (
            "<code>function example() { return true; }</code>\n"
            "<strong>Important note</strong>"
        )
        assert clean_thinking_tags(content) == expected

    def test_clean_thinking_tags_preserves_content_structure(self) -> None:
        """Test that content structure is preserved after cleaning."""
        content = (
            "Function overview:\n"
            "<think>\nLet me think about this...\n</think>\n"
            "1. Takes input parameter\n"
            "2. Processes the data\n"
            "<think>\nHow does it process?\n</think>\n"
            "3. Returns result"
        )
        expected = (
            "Function overview:\n\n"
            "1. Takes input parameter\n"
            "2. Processes the data\n\n"
            "3. Returns result"
        )
        assert clean_thinking_tags(content) == expected
