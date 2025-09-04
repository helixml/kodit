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

    def __init__(
        self,
        name: str,
        parent: "ProgressTracker | None" = None,
        initial_progress: Progress | None = None,
    ) -> None:
        """Initialize the progress tracker."""
        self.parent: ProgressTracker | None = parent
        self._log = structlog.get_logger(__name__)
        self._subscribers: list[ReportingModule] = []
        self.snapshot: Progress = (
            initial_progress
            if initial_progress
            else Progress(name=name, state=ReportingState.STARTED)
        )
        self.trackable_id: int | None = None
        self.trackable_type: TrackableType | None = None

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
            self.snapshot = self.snapshot.with_error(exc_value)
            self.snapshot = self.snapshot.with_state(
                ReportingState.FAILED, str(exc_value)
            )
        # TODO(philwinder): Probably need some state machine here # noqa: TD003, FIX002
        elif not ReportingState.is_terminal(self.snapshot.state):
            self.snapshot = self.snapshot.with_progress(self.snapshot.total)
            self.snapshot = self.snapshot.with_state(ReportingState.COMPLETED)
        await self._notify_subscribers()

    def create_child(self, name: str) -> "ProgressTracker":
        """Create a child step."""
        s = ProgressTracker(name, self)
        s.parent = self
        if self.trackable_id:
            s.trackable_id = self.trackable_id
        if self.trackable_type:
            s.trackable_type = self.trackable_type
        for subscriber in self._subscribers:
            s.subscribe(subscriber)
        return s

    async def skip(self, reason: str | None = None) -> None:
        """Skip the step."""
        self.snapshot = self.snapshot.with_state(ReportingState.SKIPPED, reason or "")

    def subscribe(self, subscriber: "ReportingModule") -> None:
        """Subscribe to the step."""
        self._subscribers.append(subscriber)

    async def set_total(self, total: int) -> None:
        """Set the total for the step."""
        self.snapshot = self.snapshot.with_total(total)
        await self._notify_subscribers()

    async def set_current(self, current: int) -> None:
        """Progress the step."""
        self.snapshot = self.snapshot.with_state(ReportingState.IN_PROGRESS)
        self.snapshot = self.snapshot.with_progress(current)
        await self._notify_subscribers()

    async def _notify_subscribers(self) -> None:
        """Notify the subscribers."""
        for subscriber in self._subscribers:
            await subscriber.on_change(self)

    async def status(self) -> Progress:
        """Get the state of the step."""
        return self.snapshot

    async def set_tracking_info(
        self, trackable_id: int, trackable_type: TrackableType
    ) -> None:
        """Set the index id."""
        self.trackable_id = trackable_id
        self.trackable_type = trackable_type
        await self._notify_subscribers()
