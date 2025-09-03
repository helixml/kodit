"""TQDM progress."""

from tqdm import tqdm

from kodit.application.services.reporting import ProgressTracker
from kodit.config import ReportingConfig
from kodit.domain.protocols import ReportingModule
from kodit.domain.value_objects import Progress, ProgressState, ReportingState


class TQDMReportingModule(ReportingModule):
    """TQDM reporting module."""

    def __init__(self, config: ReportingConfig) -> None:
        """Initialize the TQDM reporting module."""
        self.config = config
        self.pbar = tqdm()

    async def on_change(self, progress: ProgressTracker) -> None:
        """On step changed."""
        step = await progress.status()
        if step.state == ReportingState.COMPLETED:
            self.pbar.close()
            return

        self.pbar.set_description(step.message)
        self.pbar.refresh()
        # Update description if message is provided
        if step.message:
            # Fix the event message to a specific size so it's not jumping around
            # If it's too small, add spaces
            # If it's too large, truncate
            if len(step.message) < 30:
                self.pbar.set_description(step.message + " " * (30 - len(step.message)))
            else:
                self.pbar.set_description(step.message[-30:])
        else:
            self.pbar.set_description(step.name)


class TQDMProgress(Progress):
    """TQDM-based progress callback implementation."""

    def __init__(self, config: ReportingConfig | None = None) -> None:
        """Initialize with a TQDM progress bar."""
        self.config = config or ReportingConfig()
        self.pbar = tqdm()

    def on_update(self, state: ProgressState) -> None:
        """Update the TQDM progress bar."""
        # Update total if it changes
        if state.total != self.pbar.total:
            self.pbar.total = state.total

        # Update the progress bar
        self.pbar.n = state.current
        self.pbar.refresh()

        # Update description if message is provided
        if state.message:
            # Fix the event message to a specific size so it's not jumping around
            # If it's too small, add spaces
            # If it's too large, truncate
            if len(state.message) < 30:
                self.pbar.set_description(
                    state.message + " " * (30 - len(state.message))
                )
            else:
                self.pbar.set_description(state.message[-30:])
        else:
            self.pbar.set_description(state.operation)

    def on_complete(self) -> None:
        """Complete the progress bar."""
        self.pbar.close()
