"""Database progress implementation that persists to OperationRepository."""

import asyncio
from collections.abc import Callable

from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.value_objects import OperationAggregate, Step
from kodit.infrastructure.reporting.progress import Progress, ProgressConfig
from kodit.infrastructure.sqlalchemy.operation_repository import (
    SqlAlchemyOperationRepository,
)


class DatabaseProgress(Progress):
    """Progress implementation that persists operations to database."""

    def __init__(
        self,
        session_factory: Callable[[], AsyncSession],
        config: ProgressConfig | None = None,
    ) -> None:
        """Initialize the database progress."""
        self.session_factory = session_factory
        self.config = config or ProgressConfig()
        self.current_operation: OperationAggregate | None = None

    def on_operation_start(self, operation: OperationAggregate) -> None:
        """Persist when an operation starts."""
        self.current_operation = operation
        self._save_operation_async(operation)

    def on_step_update(self, step: Step) -> None:
        """Persist when a step is updated."""
        if self.current_operation:
            self.current_operation.current_step = step
            self._save_operation_async(self.current_operation)

    def on_operation_complete(self, operation: OperationAggregate) -> None:
        """Persist when an operation completes."""
        self._save_operation_async(operation)
        self.current_operation = None

    def on_operation_fail(self, operation: OperationAggregate) -> None:
        """Persist when an operation fails."""
        self._save_operation_async(operation)
        self.current_operation = None

    def _save_operation_async(self, operation: OperationAggregate) -> None:
        """Save operation using a new session in background task."""
        loop = asyncio.get_running_loop()
        loop.create_task(self._save_with_new_session(operation))  # noqa: RUF006

    async def _save_with_new_session(self, operation: OperationAggregate) -> None:
        """Save operation with a new session and commit."""
        try:
            async with self.session_factory() as session:
                repository = SqlAlchemyOperationRepository(session)
                await repository.save(operation)
                await session.commit()
        except Exception as e:
            # Log the error but don't crash the application
            import structlog

            log = structlog.get_logger(__name__)
            log.exception("Failed to save operation progress", error=str(e))
