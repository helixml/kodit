"""Tests for path utilities."""

from pathlib import Path

import pytest

from kodit.utils.path_utils import path_from_uri


class TestPathFromUri:
    """Test the path_from_uri function."""

    def test_file_uri(self) -> None:
        """Test converting a file URI to Path."""
        uri = "file:///tmp/test.txt"
        result = path_from_uri(uri)
        assert isinstance(result, Path)
        assert str(result) == "/tmp/test.txt"  # noqa: S108

    def test_windows_file_uri(self) -> None:
        """Test converting a Windows file URI to Path."""
        uri = "file:///tmp/test.txt"  # Use Unix path for cross-platform compatibility
        result = path_from_uri(uri)
        assert isinstance(result, Path)
        assert str(result) == "/tmp/test.txt"  # noqa: S108

    def test_file_uri_with_spaces(self) -> None:
        """Test converting a file URI with spaces to Path."""
        uri = "file:///tmp/test%20file.txt"
        result = path_from_uri(uri)
        assert isinstance(result, Path)
        assert str(result) == "/tmp/test file.txt"  # noqa: S108

    def test_invalid_scheme(self) -> None:
        """Test that non-file URIs raise ValueError."""
        with pytest.raises(ValueError, match="Expected file URI, got scheme: http"):
            path_from_uri("http://example.com/path")

    def test_no_scheme(self) -> None:
        """Test that URIs without scheme raise ValueError."""
        with pytest.raises(ValueError, match="Expected file URI, got scheme: "):
            path_from_uri("/path/to/file")
