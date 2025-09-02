"""Database progress implementation that persists to OperationRepository."""

import asyncio
from collections.abc import Coroutine

from kodit.domain.protocols import OperationRepository
from kodit.domain.value_objects import OperationAggregate, Step
from kodit.infrastructure.reporting.progress import Progress, ProgressConfig


class DatabaseProgress(Progress):
    """Progress implementation that persists operations to database."""

    def __init__(
        self,
        operation_repository: OperationRepository,
        config: ProgressConfig | None = None,
    ) -> None:
        """Initialize the database progress."""
        self.operation_repository = operation_repository
        self.config = config or ProgressConfig()
        self.current_operation: OperationAggregate | None = None

    def on_operation_start(self, operation: OperationAggregate) -> None:
        """Persist when an operation starts."""
        self.current_operation = operation
        self._run_async_in_background(self.operation_repository.save(operation))

    def on_step_update(self, step: Step) -> None:
        """Persist when a step is updated."""
        if self.current_operation:
            self.current_operation.current_step = step
            self._run_async_in_background(
                self.operation_repository.save(self.current_operation)
            )

    def on_operation_complete(self, operation: OperationAggregate) -> None:
        """Persist when an operation completes."""
        self._run_async_in_background(self.operation_repository.save(operation))
        self.current_operation = None

    def on_operation_fail(self, operation: OperationAggregate) -> None:
        """Persist when an operation fails."""
        self._run_async_in_background(self.operation_repository.save(operation))
        self.current_operation = None

    def _run_async_in_background(self, co: Coroutine) -> None:
        """Schedule async coroutine in the existing event loop."""
        loop = asyncio.get_running_loop()
        loop.create_task(co)  # noqa: RUF006
