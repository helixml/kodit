"""Reporting."""

from enum import StrEnum
from types import TracebackType
from typing import TYPE_CHECKING

import structlog

from kodit.domain.value_objects import ReportingState, StepSnapshot

if TYPE_CHECKING:
    from kodit.domain.protocols import ReportingModule


class OperationType(StrEnum):
    """Operation type."""

    ROOT = "kodit.root"
    CREATE_INDEX = "kodit.index.create"
    RUN_INDEX = "kodit.index.run"


class Step:
    """Step."""

    def __init__(self, name: str, parent: "Step | None" = None) -> None:
        """Initialize the step."""
        self._parent: Step | None = parent
        self._children: list[Step] = []
        self._log = structlog.get_logger(__name__)
        self._subscribers: list[ReportingModule] = []
        self._snapshot: StepSnapshot = StepSnapshot(
            name=name, state=ReportingState.IN_PROGRESS
        )

    def __enter__(self) -> "Step":
        """Enter the operation."""
        self._notify_subscribers()
        return self

    def __exit__(
        self,
        exc_type: type[BaseException] | None,
        exc_value: BaseException | None,
        traceback: TracebackType | None,
    ) -> None:
        """Exit the operation."""
        if exc_value:
            self._snapshot = self._snapshot.with_error(exc_value)
            self._snapshot = self._snapshot.with_state(
                ReportingState.FAILED, str(exc_value)
            )

        if self._snapshot.state == ReportingState.IN_PROGRESS:
            self._snapshot = self._snapshot.with_progress(100)
            self._snapshot = self._snapshot.with_state(ReportingState.COMPLETED)
        self._notify_subscribers()

    def create_child(self, name: str) -> "Step":
        """Create a child step."""
        s = Step(name, self)
        self._children.append(s)
        for subscriber in self._subscribers:
            s.subscribe(subscriber)
        return s

    def skip(self, reason: str | None = None) -> None:
        """Skip the step."""
        self._snapshot = self._snapshot.with_state(ReportingState.SKIPPED, reason or "")

    def subscribe(self, subscriber: "ReportingModule") -> None:
        """Subscribe to the step."""
        self._subscribers.append(subscriber)

    def set_total(self, total: int) -> None:
        """Set the total for the step."""
        self._snapshot = self._snapshot.with_total(total)
        self._notify_subscribers()

    def set_current(self, current: int) -> None:
        """Progress the step."""
        self._snapshot = self._snapshot.with_progress(current)
        self._notify_subscribers()

    def _notify_subscribers(self) -> None:
        """Notify the subscribers."""
        for subscriber in self._subscribers:
            subscriber.on_change(self._snapshot)
