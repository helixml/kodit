"""Reporting."""

from enum import StrEnum
from types import TracebackType
from typing import TYPE_CHECKING

import structlog

from kodit.domain.entities import TaskStatus
from kodit.domain.value_objects import ReportingState, TaskStep, TrackableType

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
        task_status: TaskStatus,
    ) -> None:
        """Initialize the progress tracker."""
        self.task_status = task_status
        self._log = structlog.get_logger(__name__)
        self._subscribers: list[ReportingModule] = []

    @staticmethod
    def create(
        step: TaskStep,
        parent: "TaskStatus | None" = None,
        trackable_type: TrackableType | None = None,
        trackable_id: int | None = None,
    ) -> "ProgressTracker":
        """Create a progress tracker."""
        return ProgressTracker(
            TaskStatus.create(
                step=step,
                trackable_type=trackable_type,
                trackable_id=trackable_id,
                parent=parent,
            )
        )

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
            self.task_status.error = str(exc_value)
            self.task_status.state = ReportingState.FAILED
        # TODO(philwinder): Probably need some state machine here # noqa: TD003, FIX002
        elif not ReportingState.is_terminal(self.task_status.state):
            self.task_status.state = ReportingState.COMPLETED
            self.task_status.current = self.task_status.total
        await self._notify_subscribers()

    def create_child(self, name: str) -> "ProgressTracker":
        """Create a child step."""
        c = ProgressTracker.create(
            step=name,
            parent=self.task_status,
            trackable_type=self.task_status.trackable_type,
            trackable_id=self.task_status.trackable_id,
        )
        for subscriber in self._subscribers:
            c.subscribe(subscriber)
        return c

    async def skip(self, _reason: str) -> None:
        """Skip the step."""
        self.task_status.state = ReportingState.SKIPPED
        await self._notify_subscribers()

    def subscribe(self, subscriber: "ReportingModule") -> None:
        """Subscribe to the step."""
        self._subscribers.append(subscriber)

    async def set_total(self, total: int) -> None:
        """Set the total for the step."""
        self.task_status.total = total
        await self._notify_subscribers()

    async def set_current(self, current: int) -> None:
        """Progress the step."""
        self.task_status.state = ReportingState.IN_PROGRESS
        self.task_status.current = current
        await self._notify_subscribers()

    async def _notify_subscribers(self) -> None:
        """Notify the subscribers only if progress has changed."""
        for subscriber in self._subscribers:
            await subscriber.on_change(self.task_status)

    async def set_tracking_info(
        self, trackable_id: int, trackable_type: TrackableType
    ) -> None:
        """Set the index id."""
        self.task_status.trackable_id = trackable_id
        self.task_status.trackable_type = trackable_type
        await self._notify_subscribers()
