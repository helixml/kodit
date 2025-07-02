"""Domain interfaces."""

from abc import ABC, abstractmethod

from kodit.domain.value_objects import ProgressEvent


class ProgressCallback(ABC):
    """Abstract interface for progress callbacks."""

    @abstractmethod
    async def on_progress(self, event: ProgressEvent) -> None:
        """On progress hook."""

    @abstractmethod
    async def on_complete(self, operation: str) -> None:
        """On complete hook."""


class NullProgressCallback(ProgressCallback):
    """Null implementation of progress callback that does nothing."""

    async def on_progress(self, event: ProgressEvent) -> None:
        """Do nothing on progress."""

    async def on_complete(self, operation: str) -> None:
        """Do nothing on complete."""
