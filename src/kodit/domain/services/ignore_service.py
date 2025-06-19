"""Domain service for ignore patterns."""

from abc import ABC, abstractmethod
from pathlib import Path


class IgnorePatternProvider(ABC):
    """Abstract interface for ignore pattern providers."""

    @abstractmethod
    def should_ignore(self, path: Path) -> bool:
        """Check if a path should be ignored.

        Args:
            path: The path to check.

        Returns:
            True if the path should be ignored, False otherwise.

        """


class IgnoreService:
    """Domain service for managing ignore patterns."""

    def __init__(self, ignore_pattern_provider: IgnorePatternProvider) -> None:
        """Initialize the ignore service.

        Args:
            ignore_pattern_provider: The ignore pattern provider to use.

        """
        self.ignore_pattern_provider = ignore_pattern_provider

    def should_ignore(self, path: Path) -> bool:
        """Check if a path should be ignored.

        Args:
            path: The path to check.

        Returns:
            True if the path should be ignored, False otherwise.

        """
        return self.ignore_pattern_provider.should_ignore(path)
