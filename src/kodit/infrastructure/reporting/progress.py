"""Progress."""

from abc import ABC, abstractmethod
from dataclasses import dataclass
from datetime import timedelta

from kodit.infrastructure.reporting.progress_state import ProgressState


@dataclass
class ProgressConfig:
    """Progress configuration."""

    log_interval: int = 10  # Log every N%
    min_update_interval: timedelta = timedelta(milliseconds=100)
    auto_complete: bool = True


class Progress(ABC):
    """Progress."""

    @abstractmethod
    def on_update(self, state: ProgressState) -> None:
        """On update."""

    @abstractmethod
    def on_complete(self) -> None:
        """On complete."""
