"""Tests for keyword extraction service."""

from kodit.domain.services.keyword_extraction_service import extract_keywords


def test_extract_keywords_removes_stop_words() -> None:
    """Test that common stop words are removed from queries."""
    query = "How do I find the user authentication code?"
    keywords = extract_keywords(query)

    # Stop words should be removed
    assert "how" not in keywords
    assert "do" not in keywords
    assert "the" not in keywords

    # Meaningful words should remain
    assert "user" in keywords
    assert "authentication" in keywords
    assert "code" in keywords


def test_extract_keywords_returns_unique() -> None:
    """Test that duplicate words are removed."""
    query = "search search database database"
    keywords = extract_keywords(query)

    assert keywords.count("search") == 1
    assert keywords.count("database") == 1


def test_extract_keywords_handles_empty_query() -> None:
    """Test that empty queries return empty list."""
    assert extract_keywords("") == []
    assert extract_keywords("   ") == []


def test_extract_keywords_handles_only_stop_words() -> None:
    """Test that queries with only stop words return empty list."""
    query = "the and or but if then"
    keywords = extract_keywords(query)
    assert keywords == []


def test_extract_keywords_preserves_order() -> None:
    """Test that keyword order is preserved."""
    query = "python async database connection"
    keywords = extract_keywords(query)

    assert keywords == ["python", "async", "database", "connection"]


def test_extract_keywords_lowercases() -> None:
    """Test that keywords are lowercased."""
    query = "Python AsyncIO DATABASE"
    keywords = extract_keywords(query)

    assert "python" in keywords
    assert "asyncio" in keywords
    assert "database" in keywords
