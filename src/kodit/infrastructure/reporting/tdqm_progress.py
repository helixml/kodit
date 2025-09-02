"""TQDM progress."""

from tqdm import tqdm

from kodit.domain.value_objects import OperationAggregate, Step
from kodit.infrastructure.reporting.progress import Progress, ProgressConfig


class TQDMProgress(Progress):
    """TQDM-based progress callback implementation."""

    def __init__(self, config: ProgressConfig | None = None) -> None:
        """Initialize with a TQDM progress bar."""
        self.config = config or ProgressConfig()
        self.pbar = tqdm()

    def on_operation_start(self, operation: OperationAggregate) -> None:
        """Display when an operation starts."""
        self.pbar.set_description(f"Starting {operation.type}")

    def on_step_update(self, operation: OperationAggregate, step: Step) -> None:  # noqa: ARG002
        """Update progress bar with step information."""
        # Update the progress bar description with step info
        desc = f"{step.name} [{step.state.value}]"
        if len(desc) < 30:
            self.pbar.set_description(desc + " " * (30 - len(desc)))
        else:
            self.pbar.set_description(desc[:30])

        # Update progress percentage
        self.pbar.n = int(step.progress_percentage)
        self.pbar.total = 100
        self.pbar.refresh()

    def on_operation_complete(self, operation: OperationAggregate) -> None:
        """Display when an operation completes."""
        self.pbar.set_description(f"Completed {operation.type}")
        self.pbar.n = self.pbar.total
        self.pbar.refresh()
        self.pbar.close()

    def on_operation_fail(self, operation: OperationAggregate) -> None:
        """Display when an operation fails."""
        self.pbar.set_description(f"Failed: {str(operation.error)[:25]}")
        self.pbar.close()
