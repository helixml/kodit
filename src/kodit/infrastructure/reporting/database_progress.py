"""Database progress implementation that persists to OperationRepository."""

import asyncio
import concurrent.futures

import structlog

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
        # Thread pool for running async operations synchronously
        self._executor = concurrent.futures.ThreadPoolExecutor(max_workers=1)
        self.log = structlog.get_logger(__name__)

    def on_operation_start(self, operation: OperationAggregate) -> None:
        """Persist when an operation starts."""
        self._save_operation_blocking(operation)

    def on_step_update(self, operation: OperationAggregate, step: Step) -> None:
        """Persist when a step is updated."""
        operation.current_step = step
        self._save_operation_blocking(operation)

    def on_operation_complete(self, operation: OperationAggregate) -> None:
        """Persist when an operation completes."""
        self._save_operation_blocking(operation)

    def on_operation_fail(self, operation: OperationAggregate) -> None:
        """Persist when an operation fails."""
        self._save_operation_blocking(operation)

    def _save_operation_blocking(self, operation: OperationAggregate) -> None:
        """Save operation synchronously by blocking until complete."""
        # Run the async save operation in a thread pool and wait for completion
        future = self._executor.submit(
            asyncio.run, self.operation_repository.save(operation)
        )
        try:
            # Block until the save completes (with a timeout)
            future.result(timeout=5.0)
        except Exception as e:
            self.log.exception("Failed to save operation progress", error=str(e))
