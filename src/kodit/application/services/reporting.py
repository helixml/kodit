"""Reporting."""

from enum import StrEnum
from types import TracebackType
from typing import TYPE_CHECKING

import structlog

from kodit.domain.value_objects import Progress, ReportingState, TrackableType

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
        self._snapshot: Progress = Progress(name=name, state=ReportingState.STARTED)
        self._trackable_id: int | None = None
        self._trackable_type: TrackableType | None = None

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
        else:
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
        self._snapshot = self._snapshot.with_state(ReportingState.IN_PROGRESS)
        self._snapshot = self._snapshot.with_progress(current)
        await self._notify_subscribers()

    async def _notify_subscribers(self) -> None:
        """Notify the subscribers."""
        for subscriber in self._subscribers:
            await subscriber.on_change(self)

    async def status(self) -> Progress:
        """Get the state of the step."""
        return self._snapshot

    async def set_tracking_info(
        self, trackable_id: int, trackable_type: TrackableType
    ) -> None:
        """Set the index id."""
        self._trackable_id = trackable_id
        self._trackable_type = trackable_type
        await self._notify_subscribers()

    @property
    def trackable_id(self) -> int | None:
        """Get the index id."""
        if self._trackable_id:
            return self._trackable_id
        if self.parent:
            return self.parent.trackable_id
        return None

    @property
    def trackable_type(self) -> str | None:
        """Get the index type."""
        if self._trackable_type:
            return self._trackable_type
        if self.parent:
            return self.parent.trackable_type
        return None

    @property
    def children(self) -> list["ProgressTracker"]:
        """Get the children."""
        return self._children
