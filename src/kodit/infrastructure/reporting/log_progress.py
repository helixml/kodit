"""Log progress using structlog."""

import structlog

from kodit.domain.value_objects import OperationAggregate, Step
from kodit.infrastructure.reporting.progress import Progress, ProgressConfig


class LogProgress(Progress):
    """Log progress using structlog with time-based throttling."""

    def __init__(self, config: ProgressConfig | None = None) -> None:
        """Initialize the log progress."""
        self.log = structlog.get_logger()
        self.config = config or ProgressConfig()
        self.last_log_time: float = 0

    def on_operation_start(self, operation: OperationAggregate) -> None:
        """Log when an operation starts."""
        self.log.info(
            "Operation started",
            index_id=operation.index_id,
            type=operation.type,
            state=operation.state.value,
        )

    def on_step_update(self, operation: OperationAggregate, step: Step) -> None:
        """Log when a step is updated."""
        self.log.info(
            "Step update",
            index_id=operation.index_id,
            operation_type=operation.type,
            step=step.name,
            state=step.state.value,
            progress=f"{step.progress_percentage:.1f}%",
        )

    def on_operation_complete(self, operation: OperationAggregate) -> None:
        """Log when an operation completes."""
        self.log.info(
            "Operation completed",
            index_id=operation.index_id,
            type=operation.type,
        )

    def on_operation_fail(self, operation: OperationAggregate) -> None:
        """Log when an operation fails."""
        self.log.error(
            "Operation failed",
            index_id=operation.index_id,
            type=operation.type,
            error=str(operation.error),
        )
