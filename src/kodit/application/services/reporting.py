"""Reporting."""

from enum import StrEnum
from types import TracebackType
from typing import TYPE_CHECKING

import structlog

from kodit.domain.value_objects import Progress, ReportingState, TrackableType

if TYPE_CHECKING:
    from kodit.domain.protocols import ReportingModule
else:
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
        progress: Progress,
        parent: "ProgressTracker | None" = None,
    ) -> None:
        """Initialize the progress tracker."""
        self.progress: Progress = progress
        self.parent: ProgressTracker | None = parent
        self._log = structlog.get_logger(__name__)
        self._subscribers: list[ReportingModule] = []

    @classmethod
    def create(
        cls,
        name: str,
        parent: "ProgressTracker | None" = None,
        trackable_id: int | None = None,
        trackable_type: TrackableType | None = None,
    ) -> "ProgressTracker":
        """Create a new tracker."""
        progress = Progress(
            name=name,
            state=ReportingState.STARTED,
            trackable_id=trackable_id,
            trackable_type=trackable_type,
        )
        return cls(progress=progress, parent=parent)

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
            self.progress = self.progress.with_error(exc_value)
            self.progress = self.progress.with_state(
                ReportingState.FAILED, str(exc_value)
            )
        # TODO(philwinder): Probably need some state machine here # noqa: TD003, FIX002
        elif not ReportingState.is_terminal(self.progress.state):
            self.progress = self.progress.with_progress(self.progress.total)
            self.progress = self.progress.with_state(ReportingState.COMPLETED)
        await self._notify_subscribers()

    def create_child(self, name: str) -> "ProgressTracker":
        """Create a child step."""
        s = ProgressTracker.create(
            name=name,
            parent=self,
            trackable_id=self.progress.trackable_id,
            trackable_type=self.progress.trackable_type,
        )
        for subscriber in self._subscribers:
            s.subscribe(subscriber)
        return s

    async def skip(self, reason: str | None = None) -> None:
        """Skip the step."""
        self.progress = self.progress.with_state(ReportingState.SKIPPED, reason or "")
        await self._notify_subscribers()

    def subscribe(self, subscriber: "ReportingModule") -> None:
        """Subscribe to the step."""
        self._subscribers.append(subscriber)

    async def set_total(self, total: int) -> None:
        """Set the total for the step."""
        self.progress = self.progress.with_total(total)
        await self._notify_subscribers()

    async def set_current(self, current: int) -> None:
        """Progress the step."""
        self.progress = self.progress.with_state(ReportingState.IN_PROGRESS)
        self.progress = self.progress.with_progress(current)
        await self._notify_subscribers()

    async def _notify_subscribers(self) -> None:
        """Notify the subscribers only if progress has changed."""
        for subscriber in self._subscribers:
            await subscriber.on_change(self)

    async def status(self) -> Progress:
        """Get the state of the step."""
        return self.progress

    async def set_tracking_info(
        self, trackable_id: int, trackable_type: TrackableType
    ) -> None:
        """Set the index id."""
        self.progress = self.progress.with_tracking(trackable_id, trackable_type)
        await self._notify_subscribers()


class ProgressTrackerFactory:
    """Factory for reconstructing ProgressTracker from persisted state."""

    @staticmethod
    def from_progress_with_hierarchy(
        progress_with_hierarchy: list[tuple[int, Progress, int | None]],
        subscribers: list[ReportingModule] | None = None,
    ) -> list[ProgressTracker]:
        """Reconstruct tracker tree from Progress objects with database IDs.

        Args:
            progress_with_hierarchy: List of (db_id, Progress, parent_db_id) tuples
            subscribers: Optional list of subscribers to attach

        Returns:
            List of root ProgressTrackers with children linked

        """
        # Build tracker map by database ID
        trackers_by_db_id: dict[int, ProgressTracker] = {}

        # First pass: create all trackers
        for db_id, progress, _ in progress_with_hierarchy:
            tracker = ProgressTracker(progress=progress)
            # Notification state is already initialized to None in constructor
            if subscribers:
                for sub in subscribers:
                    tracker.subscribe(sub)
            trackers_by_db_id[db_id] = tracker

        # Second pass: link parents using database IDs
        for db_id, _, parent_id in progress_with_hierarchy:
            if parent_id and parent_id in trackers_by_db_id:
                trackers_by_db_id[db_id].parent = trackers_by_db_id[parent_id]

        # Return root trackers (those without parents)
        return [t for t in trackers_by_db_id.values() if not t.parent]
