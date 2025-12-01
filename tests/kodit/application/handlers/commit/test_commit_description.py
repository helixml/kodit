"""Tests for commit description handler."""

from kodit.application.handlers.commit.commit_description import truncate_diff


def test_truncate_diff_truncates_large_diffs() -> None:
    """Test that large diffs get truncated."""
    # Create a diff larger than the max length (100k chars)
    large_diff = "a" * 150_000

    result = truncate_diff(large_diff)

    assert len(result) <= 100_000
    assert result.endswith("\n\n[diff truncated due to size]")


def test_truncate_diff_preserves_small_diffs() -> None:
    """Test that small diffs are not truncated."""
    small_diff = "small diff content"

    result = truncate_diff(small_diff)

    assert result == small_diff
