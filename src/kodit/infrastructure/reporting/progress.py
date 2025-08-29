"""Progress."""

from abc import ABC, abstractmethod
from dataclasses import dataclass
from datetime import timedelta

from kodit.domain.value_objects import ProgressState


@dataclass
class ProgressConfig:
    """Progress configuration."""

    log_time_interval: timedelta = timedelta(seconds=5)  # Log every N seconds


class Progress(ABC):
    """Progress."""

    @abstractmethod
    def on_update(self, state: ProgressState) -> None:
        """On update."""

    @abstractmethod
    def on_complete(self) -> None:
        """On complete."""
