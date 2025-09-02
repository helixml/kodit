"""Progress reporter."""

from collections.abc import Callable, Generator
from contextlib import contextmanager
from datetime import UTC, datetime

from kodit.domain.protocols import (
    OperationRepository,
    ReportingService,
    ReportingStep,
)
from kodit.domain.value_objects import (
    OperationAggregate,
    OperationState,
    Step,
    StepState,
)
from kodit.infrastructure.reporting.database_progress import DatabaseProgress
from kodit.infrastructure.reporting.log_progress import LogProgress
from kodit.infrastructure.reporting.progress import Progress, ProgressConfig
from kodit.infrastructure.reporting.tdqm_progress import TQDMProgress


class Reporter(ReportingService):
    """Reporter reports on progress."""

    def __init__(
        self,
        modules: list[Progress] | None = None,
        operation_repository: OperationRepository | None = None,
    ) -> None:
        """Initialize the reporter."""
        self.modules = modules or []
        self.operation_repository = operation_repository

    def start_operation(self, operation: OperationAggregate) -> None:
        """Start tracking a new operation with steps."""
        operation.state = OperationState.IN_PROGRESS
        operation.updated_at = datetime.now(UTC)

        # Notify progress modules
        for module in self.modules:
            module.on_operation_start(operation)

    def update_step(self, operation: OperationAggregate, step: Step) -> None:
        """Update the current step of an operation."""
        operation.current_step = step
        operation.updated_at = datetime.now(UTC)

        # Notify progress modules
        for module in self.modules:
            module.on_step_update(operation, step)

    @contextmanager
    def reporting_step_context(
        self, operation: OperationAggregate
    ) -> Generator[ReportingStep, None, None]:
        """Context manager for a reporting step."""

        def on_update(step: Step) -> None:
            self.update_step(operation, step)

        yield StepReporter(on_update)

    def complete_operation(self, operation: OperationAggregate) -> None:
        """Mark the current operation as completed."""
        operation.state = OperationState.COMPLETED
        operation.updated_at = datetime.now(UTC)
        operation.current_step = None

        # Notify progress modules
        for module in self.modules:
            module.on_operation_complete(operation)

    def fail_operation(self, operation: OperationAggregate, error: Exception) -> None:
        """Mark the current operation as failed."""
        operation.state = OperationState.FAILED
        operation.error = error
        operation.updated_at = datetime.now(UTC)

        if operation.current_step:
            operation.current_step.state = StepState.FAILED
            operation.current_step.error = error

        # Notify progress modules
        for module in self.modules:
            module.on_operation_fail(operation)


class StepReporter(ReportingStep):
    """Step."""

    def __init__(self, on_update: Callable[[Step], None]) -> None:
        """Initialize the step."""
        self.on_update = on_update

    def update_step_progress(self, step: Step) -> None:
        """Update the progress of the current step."""
        self.on_update(step)


def create_noop_reporter() -> Reporter:
    """Create a noop reporter."""
    return Reporter(modules=[])


def create_cli_reporter(config: ProgressConfig | None = None) -> Reporter:
    """Create a CLI reporter."""
    shared_config = config or ProgressConfig()
    return Reporter(modules=[TQDMProgress(shared_config)])


def create_server_reporter(
    operation_repository: OperationRepository, config: ProgressConfig | None = None
) -> Reporter:
    """Create a server reporter."""
    shared_config = config or ProgressConfig()
    return Reporter(
        modules=[
            LogProgress(shared_config),
            DatabaseProgress(
                operation_repository=operation_repository, config=shared_config
            ),
        ]
    )


def create_index_operation(index_id: int, operation_type: str) -> OperationAggregate:
    """Create a new operation aggregate for tracking."""
    return OperationAggregate(
        index_id=index_id,
        type=operation_type,
        state=OperationState.PENDING,
        updated_at=datetime.now(UTC),
        progress_percentage=0.0,
    )


def create_step(name: str, progress: float = 0.0) -> Step:
    """Create a new step in running state."""
    return Step(
        name=name,
        state=StepState.RUNNING,
        updated_at=datetime.now(UTC),
        progress_percentage=progress,
    )


def complete_step(step: Step) -> Step:
    """Mark a step as completed with 100% progress."""
    step.state = StepState.COMPLETED
    step.progress_percentage = 100.0
    step.updated_at = datetime.now(UTC)
    return step


def fail_step(step: Step, error: Exception) -> Step:
    """Mark a step as failed with error."""
    step.state = StepState.FAILED
    step.error = error
    step.updated_at = datetime.now(UTC)
    return step


def update_step_progress(step: Step, current: int, total: int) -> Step:
    """Update step progress percentage based on current/total."""
    step.progress_percentage = (current / total) * 100 if total > 0 else 0.0
    step.updated_at = datetime.now(UTC)
    return step
