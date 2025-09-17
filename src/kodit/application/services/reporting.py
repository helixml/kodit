"""Reporting."""

from collections.abc import AsyncGenerator
from contextlib import asynccontextmanager
from typing import TYPE_CHECKING

import structlog

from kodit.domain.entities import TaskStatus
from kodit.domain.value_objects import TaskOperation, TrackableType

if TYPE_CHECKING:
    from kodit.domain.protocols import ReportingModule


class ProgressTracker:
    """Progress tracker.

    Provides a reactive wrapper around TaskStatus domain entities that automatically
    propagates state changes to the database and reporting modules. This pattern was
    chosen over a traditional service-repository approach because:
    - State changes must trigger immediate side effects (database writes, notifications)
    - Multiple consumers need real-time updates without polling
    - The wrapper pattern allows transparent interception of all state mutations

    The tracker monitors all modifications to the underlying TaskStatus and ensures
    consistency across all downstream systems.
    """

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
        operation: TaskOperation,
        parent: "TaskStatus | None" = None,
        trackable_type: TrackableType | None = None,
        trackable_id: int | None = None,
    ) -> "ProgressTracker":
        """Create a progress tracker."""
        return ProgressTracker(
            TaskStatus.create(
                operation=operation,
                trackable_type=trackable_type,
                trackable_id=trackable_id,
                parent=parent,
            )
        )

    @asynccontextmanager
    async def create_child(
        self,
        operation: TaskOperation,
        trackable_type: TrackableType | None = None,
        trackable_id: int | None = None,
    ) -> AsyncGenerator["ProgressTracker", None]:
        """Create a child step."""
        c = ProgressTracker.create(
            operation=operation,
            parent=self.task_status,
            trackable_type=trackable_type or self.task_status.trackable_type,
            trackable_id=trackable_id or self.task_status.trackable_id,
        )
        try:
            for subscriber in self._subscribers:
                c.subscribe(subscriber)

            await c.notify_subscribers()
            yield c
        except Exception as e:  # noqa: BLE001
            c.task_status.fail(str(e))
        finally:
            c.task_status.complete()
            await c.notify_subscribers()

    async def skip(self, reason: str) -> None:
        """Skip the step."""
        self.task_status.skip(reason)

    def subscribe(self, subscriber: "ReportingModule") -> None:
        """Subscribe to the step."""
        self._subscribers.append(subscriber)

    async def set_total(self, total: int) -> None:
        """Set the total for the step."""
        self.task_status.set_total(total)

    async def set_current(self, current: int, message: str | None = None) -> None:
        """Progress the step."""
        self.task_status.set_current(current, message)
        await self.notify_subscribers()

    async def notify_subscribers(self) -> None:
        """Notify the subscribers only if progress has changed."""
        for subscriber in self._subscribers:
            await subscriber.on_change(self.task_status)
