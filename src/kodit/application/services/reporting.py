"""Reporting."""

from enum import StrEnum
from types import TracebackType
from typing import TYPE_CHECKING

import structlog

from kodit.domain.value_objects import Progress, ReportingState

if TYPE_CHECKING:
    from kodit.domain.protocols import ReportingModule


class OperationType(StrEnum):
    """Operation type."""

    ROOT = "kodit.root"
    CREATE_INDEX = "kodit.index.create"
    RUN_INDEX = "kodit.index.run"


class ProgressTracker:
    """Progress tracker."""

    def __init__(self, name: str, parent: "ProgressTracker | None" = None) -> None:
        """Initialize the progress tracker."""
        self.parent: ProgressTracker | None = parent
        self._children: list[ProgressTracker] = []
        self._log = structlog.get_logger(__name__)
        self._subscribers: list[ReportingModule] = []
        self._snapshot: Progress = Progress(name=name, state=ReportingState.IN_PROGRESS)
        self._index_id: int | None = None

    async def __aenter__(self) -> "ProgressTracker":
        """Enter the operation."""
        await self._notify_subscribers()
        return self

    async def __aexit__(
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
        await self._notify_subscribers()

    def create_child(self, name: str) -> "ProgressTracker":
        """Create a child step."""
        s = ProgressTracker(name, self)
        self._children.append(s)
        for subscriber in self._subscribers:
            s.subscribe(subscriber)
        return s

    async def skip(self, reason: str | None = None) -> None:
        """Skip the step."""
        self._snapshot = self._snapshot.with_state(ReportingState.SKIPPED, reason or "")

    def subscribe(self, subscriber: "ReportingModule") -> None:
        """Subscribe to the step."""
        self._subscribers.append(subscriber)

    async def set_total(self, total: int) -> None:
        """Set the total for the step."""
        self._snapshot = self._snapshot.with_total(total)
        await self._notify_subscribers()

    async def set_current(self, current: int) -> None:
        """Progress the step."""
        self._snapshot = self._snapshot.with_progress(current)
        await self._notify_subscribers()

    async def _notify_subscribers(self) -> None:
        """Notify the subscribers."""
        for subscriber in self._subscribers:
            await subscriber.on_change(self)

    async def status(self) -> Progress:
        """Get the state of the step."""
        return self._snapshot

    async def set_index_id(self, index_id: int) -> None:
        """Set the index id."""
        self._index_id = index_id
        await self._notify_subscribers()

    @property
    def index_id(self) -> int | None:
        """Get the index id."""
        if self._index_id:
            return self._index_id
        if self.parent:
            return self.parent.index_id
        return None
