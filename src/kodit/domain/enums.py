"""Domain enums."""

from enum import Enum


class SnippetExtractionStrategy(str, Enum):
    """Different strategies for extracting snippets from files."""

    METHOD_BASED = "method_based"
