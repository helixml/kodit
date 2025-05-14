"""Detect the language of a file."""

from pathlib import Path
from typing import cast

from tree_sitter_language_pack import SupportedLanguage

# Mapping of file extensions to programming languages
LANGUAGE_MAP: dict[str, str] = {
    # JavaScript/TypeScript
    "js": "javascript",
    "jsx": "javascript",
    "ts": "typescript",
    "tsx": "typescript",
    # Python
    "py": "python",
    # Rust
    "rs": "rust",
    # Go
    "go": "go",
    # C/C++
    "cpp": "cpp",
    "hpp": "cpp",
    "c": "c",
    "h": "c",
    # C#
    "cs": "csharp",
    # Ruby
    "rb": "ruby",
    # Java
    "java": "java",
    # PHP
    "php": "php",
    # Swift
    "swift": "swift",
    # Kotlin
    "kt": "kotlin",
}


def detect_language(file_path: Path) -> SupportedLanguage:
    """Detect the language of a file."""
    suffix = file_path.suffix.removeprefix(".").lower()
    msg = f"Unsupported language for file suffix: {suffix}"
    lang = LANGUAGE_MAP.get(suffix)
    if lang is None:
        raise ValueError(msg)

    # Try to cast the language to a SupportedLanguage
    try:
        return cast("SupportedLanguage", lang)
    except Exception as e:
        raise ValueError(msg) from e
