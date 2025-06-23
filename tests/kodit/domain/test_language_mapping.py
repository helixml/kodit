"""Tests for LanguageMapping value object."""

import pytest

from kodit.domain.value_objects import LanguageMapping


class TestLanguageMapping:
    """Test cases for LanguageMapping value object."""

    def test_get_extensions_for_language_python(self):
        """Test getting extensions for Python."""
        extensions = LanguageMapping.get_extensions_for_language("python")
        assert extensions == ["py", "pyw", "pyx", "pxd"]

    def test_get_extensions_for_language_javascript(self):
        """Test getting extensions for JavaScript."""
        extensions = LanguageMapping.get_extensions_for_language("javascript")
        assert extensions == ["js", "jsx", "mjs"]

    def test_get_extensions_for_language_case_insensitive(self):
        """Test that language names are case insensitive."""
        extensions = LanguageMapping.get_extensions_for_language("PYTHON")
        assert extensions == ["py", "pyw", "pyx", "pxd"]

    def test_get_extensions_for_unsupported_language(self):
        """Test that unsupported languages raise ValueError."""
        with pytest.raises(ValueError, match="Unsupported language: unsupported"):
            LanguageMapping.get_extensions_for_language("unsupported")

    def test_get_language_for_extension_py(self):
        """Test getting language for .py extension."""
        language = LanguageMapping.get_language_for_extension("py")
        assert language == "python"

    def test_get_language_for_extension_with_dot(self):
        """Test getting language for extension with leading dot."""
        language = LanguageMapping.get_language_for_extension(".py")
        assert language == "python"

    def test_get_language_for_extension_case_insensitive(self):
        """Test that extensions are case insensitive."""
        language = LanguageMapping.get_language_for_extension("PY")
        assert language == "python"

    def test_get_language_for_unsupported_extension(self):
        """Test that unsupported extensions raise ValueError."""
        with pytest.raises(ValueError, match="Unsupported file extension: unsupported"):
            LanguageMapping.get_language_for_extension("unsupported")

    def test_get_extension_to_language_map(self):
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

    def test_get_supported_languages(self):
        """Test getting list of supported languages."""
        languages = LanguageMapping.get_supported_languages()

        # Check that key languages are included
        assert "python" in languages
        assert "javascript" in languages
        assert "go" in languages
        assert "rust" in languages

    def test_get_supported_extensions(self):
        """Test getting list of supported extensions."""
        extensions = LanguageMapping.get_supported_extensions()

        # Check that key extensions are included
        assert "py" in extensions
        assert "js" in extensions
        assert "go" in extensions
        assert "rs" in extensions

    def test_is_supported_language(self):
        """Test checking if a language is supported."""
        assert LanguageMapping.is_supported_language("python") is True
        assert LanguageMapping.is_supported_language("PYTHON") is True
        assert LanguageMapping.is_supported_language("unsupported") is False

    def test_is_supported_extension(self):
        """Test checking if an extension is supported."""
        assert LanguageMapping.is_supported_extension("py") is True
        assert LanguageMapping.is_supported_extension(".py") is True
        assert LanguageMapping.is_supported_extension("PY") is True
        assert LanguageMapping.is_supported_extension("unsupported") is False

    def test_bidirectional_mapping_consistency(self):
        """Test that bidirectional mapping is consistent."""
        # Test that extension -> language -> extension gives the same result
        for language in LanguageMapping.get_supported_languages():
            extensions = LanguageMapping.get_extensions_for_language(language)
            for extension in extensions:
                detected_language = LanguageMapping.get_language_for_extension(
                    extension
                )
                assert detected_language == language

    def test_extension_uniqueness(self):
        """Test that each extension maps to only one language."""
        extension_map = LanguageMapping.get_extension_to_language_map()
        extension_values = list(extension_map.values())

        # Check that there are no duplicate extensions
        assert len(extension_map) == len(set(extension_map.keys()))

    def test_get_extensions_with_fallback_supported_language(self):
        """Test fallback method returns extensions for supported language."""
        extensions = LanguageMapping.get_extensions_with_fallback("python")
        assert extensions == ["py", "pyw", "pyx", "pxd"]

    def test_get_extensions_with_fallback_unsupported_language(self):
        """Test fallback method returns [language.lower()] for unsupported language."""
        extensions = LanguageMapping.get_extensions_with_fallback("foobar")
        assert extensions == ["foobar"]
