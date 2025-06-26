"""Tests for LanguageMapping value object."""

from datetime import datetime

import pytest

from kodit.domain.value_objects import LanguageMapping, SnippetSearchFilters


class TestLanguageMapping:
    """Test cases for LanguageMapping value object."""

    def test_get_extensions_for_language_python(self) -> None:
        """Test getting extensions for Python."""
        extensions = LanguageMapping.get_extensions_for_language("python")
        assert extensions == ["py", "pyw", "pyx", "pxd"]

    def test_get_extensions_for_language_javascript(self) -> None:
        """Test getting extensions for JavaScript."""
        extensions = LanguageMapping.get_extensions_for_language("javascript")
        assert extensions == ["js", "jsx", "mjs"]

    def test_get_extensions_for_language_case_insensitive(self) -> None:
        """Test that language names are case insensitive."""
        extensions = LanguageMapping.get_extensions_for_language("PYTHON")
        assert extensions == ["py", "pyw", "pyx", "pxd"]

    def test_get_extensions_for_unsupported_language(self) -> None:
        """Test that unsupported languages raise ValueError."""
        with pytest.raises(ValueError, match="Unsupported language: unsupported"):
            LanguageMapping.get_extensions_for_language("unsupported")

    def test_get_language_for_extension_py(self) -> None:
        """Test getting language for .py extension."""
        language = LanguageMapping.get_language_for_extension("py")
        assert language == "python"

    def test_get_language_for_extension_with_dot(self) -> None:
        """Test getting language for extension with leading dot."""
        language = LanguageMapping.get_language_for_extension(".py")
        assert language == "python"

    def test_get_language_for_extension_case_insensitive(self) -> None:
        """Test that extensions are case insensitive."""
        language = LanguageMapping.get_language_for_extension("PY")
        assert language == "python"

    def test_get_language_for_unsupported_extension(self) -> None:
        """Test that unsupported extensions raise ValueError."""
        with pytest.raises(ValueError, match="Unsupported file extension: unsupported"):
            LanguageMapping.get_language_for_extension("unsupported")

    def test_get_extension_to_language_map(self) -> None:
        """Test getting the extension to language mapping."""
        extension_map = LanguageMapping.get_extension_to_language_map()

        # Check a few key mappings
        assert extension_map["py"] == "python"
        assert extension_map["js"] == "javascript"
        assert extension_map["go"] == "go"

        # Check that all extensions are included
        assert "py" in extension_map
        assert "js" in extension_map
        assert "ts" in extension_map

    def test_get_supported_languages(self) -> None:
        """Test getting list of supported languages."""
        languages = LanguageMapping.get_supported_languages()

        # Check that key languages are included
        assert "python" in languages
        assert "javascript" in languages
        assert "go" in languages
        assert "rust" in languages

    def test_get_supported_extensions(self) -> None:
        """Test getting list of supported extensions."""
        extensions = LanguageMapping.get_supported_extensions()

        # Check that key extensions are included
        assert "py" in extensions
        assert "js" in extensions
        assert "go" in extensions
        assert "rs" in extensions

    def test_is_supported_language(self) -> None:
        """Test checking if a language is supported."""
        assert LanguageMapping.is_supported_language("python") is True
        assert LanguageMapping.is_supported_language("PYTHON") is True
        assert LanguageMapping.is_supported_language("unsupported") is False

    def test_is_supported_extension(self) -> None:
        """Test checking if an extension is supported."""
        assert LanguageMapping.is_supported_extension("py") is True
        assert LanguageMapping.is_supported_extension(".py") is True
        assert LanguageMapping.is_supported_extension("PY") is True
        assert LanguageMapping.is_supported_extension("unsupported") is False

    def test_bidirectional_mapping_consistency(self) -> None:
        """Test that bidirectional mapping is consistent."""
        # Test that extension -> language -> extension gives the same result
        for language in LanguageMapping.get_supported_languages():
            extensions = LanguageMapping.get_extensions_for_language(language)
            for extension in extensions:
                detected_language = LanguageMapping.get_language_for_extension(
                    extension
                )
                assert detected_language == language

    def test_extension_uniqueness(self) -> None:
        """Test that each extension maps to only one language."""
        extension_map = LanguageMapping.get_extension_to_language_map()

        # Check that there are no duplicate extensions
        assert len(extension_map) == len(set(extension_map.keys()))

    def test_get_extensions_with_fallback_supported_language(self) -> None:
        """Test fallback method returns extensions for supported language."""
        extensions = LanguageMapping.get_extensions_with_fallback("python")
        assert extensions == ["py", "pyw", "pyx", "pxd"]

    def test_get_extensions_with_fallback_unsupported_language(self) -> None:
        """Test fallback method returns [language.lower()] for unsupported language."""
        extensions = LanguageMapping.get_extensions_with_fallback("foobar")
        assert extensions == ["foobar"]


class TestSnippetSearchFilters:
    """Test cases for SnippetSearchFilters value object."""

    def test_from_cli_params_no_filters(self) -> None:
        """Test that None is returned when no filters are provided."""
        filters = SnippetSearchFilters.from_cli_params()
        assert filters is None

    def test_from_cli_params_with_language(self) -> None:
        """Test creating filters with language parameter."""
        filters = SnippetSearchFilters.from_cli_params(language="python")
        assert filters is not None
        assert filters.language == "python"
        assert filters.author is None
        assert filters.created_after is None
        assert filters.created_before is None
        assert filters.source_repo is None

    def test_from_cli_params_with_all_filters(self) -> None:
        """Test creating filters with all parameters."""
        filters = SnippetSearchFilters.from_cli_params(
            language="python",
            author="John Doe",
            created_after="2023-01-01",
            created_before="2023-12-31",
            source_repo="github.com/example/repo",
        )
        assert filters is not None
        assert filters.language == "python"
        assert filters.author == "John Doe"
        assert filters.created_after == datetime(2023, 1, 1)  # noqa: DTZ001
        assert filters.created_before == datetime(2023, 12, 31)  # noqa: DTZ001
        assert filters.source_repo == "github.com/example/repo"

    def test_from_cli_params_invalid_date_format(self) -> None:
        """Test that invalid date formats raise ValueError."""
        with pytest.raises(ValueError, match="Invalid date format for created_after"):
            SnippetSearchFilters.from_cli_params(created_after="invalid-date")

        with pytest.raises(ValueError, match="Invalid date format for created_before"):
            SnippetSearchFilters.from_cli_params(created_before="invalid-date")

    def test_from_cli_params_partial_filters(self) -> None:
        """Test creating filters with only some parameters."""
        filters = SnippetSearchFilters.from_cli_params(
            language="go", author="Jane Smith"
        )
        assert filters is not None
        assert filters.language == "go"
        assert filters.author == "Jane Smith"
        assert filters.created_after is None
        assert filters.created_before is None
        assert filters.source_repo is None
