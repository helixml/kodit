"""Progress."""

from abc import ABC, abstractmethod
from dataclasses import dataclass
from datetime import timedelta

from kodit.domain.value_objects import OperationAggregate, Step


@dataclass
class ProgressConfig:
    """Progress configuration."""

    log_time_interval: timedelta = timedelta(seconds=5)  # Log every N seconds


class Progress(ABC):
    """Progress."""

    @abstractmethod
    def on_operation_start(self, operation: OperationAggregate) -> None:
        """Handle when an operation starts."""

    @abstractmethod
    def on_step_update(self, operation: OperationAggregate, step: Step) -> None:
        """Handle when a step is updated."""

    @abstractmethod
    def on_operation_complete(self, operation: OperationAggregate) -> None:
        """Handle when an operation completes."""

    @abstractmethod
    def on_operation_fail(self, operation: OperationAggregate) -> None:
        """Handle when an operation fails."""
