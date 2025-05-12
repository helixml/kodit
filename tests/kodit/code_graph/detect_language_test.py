import pytest

from kodit.code_graph.detect_language import detect_language


def test_detect_language_common_extensions() -> None:
    """Test detection of common programming language extensions."""
    test_cases = [
        ("script.py", "python"),
        ("app.js", "javascript"),
        ("component.jsx", "javascript"),
        ("types.ts", "typescript"),
        ("react.tsx", "typescript"),
        ("main.rs", "rust"),
        ("server.go", "go"),
        ("program.cpp", "cpp"),
        ("header.hpp", "cpp"),
        ("main.c", "c"),
        ("header.h", "c"),
        ("app.cs", "csharp"),
        ("script.rb", "ruby"),
        ("Main.java", "java"),
        ("index.php", "php"),
        ("app.swift", "swift"),
        ("Main.kt", "kotlin"),
    ]

    for file_path, expected_language in test_cases:
        assert detect_language(file_path) == expected_language, (
            f"Failed to detect language for {file_path}"
        )


def test_detect_language_case_insensitive() -> None:
    """Test that file extension detection is case insensitive."""
    test_cases = [
        ("script.PY", "python"),
        ("app.JS", "javascript"),
        ("types.TS", "typescript"),
        ("main.RS", "rust"),
    ]

    for file_path, expected_language in test_cases:
        assert detect_language(file_path) == expected_language, (
            f"Failed to detect language for uppercase extension {file_path}"
        )


def test_detect_language_unsupported_extensions() -> None:
    """Test handling of unsupported file extensions."""
    test_cases = [
        "file.xyz",
        "document.txt",
        "data.csv",
        "image.png",
        "script.sh",
    ]

    for file_path in test_cases:
        assert detect_language(file_path) is None, (
            f"Should return None for unsupported extension {file_path}"
        )


def test_detect_language_no_extension() -> None:
    """Test handling of files without extensions."""
    assert detect_language("Makefile") is None
    assert detect_language("Dockerfile") is None
    assert detect_language("README") is None


def test_detect_language_empty_path() -> None:
    """Test handling of empty file path."""
    with pytest.raises(ValueError):
        detect_language("")


def test_detect_language_invalid_path() -> None:
    """Test handling of invalid file paths."""
    with pytest.raises(ValueError):
        detect_language("")  # Using empty string instead of None to match type hints
