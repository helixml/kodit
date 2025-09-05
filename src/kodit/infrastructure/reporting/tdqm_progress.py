"""TQDM progress."""

from tqdm import tqdm

from kodit.config import ReportingConfig
from kodit.domain.entities import TaskStatus
from kodit.domain.protocols import ReportingModule
from kodit.domain.value_objects import ReportingState


class TQDMReportingModule(ReportingModule):
    """TQDM reporting module."""

    def __init__(self, config: ReportingConfig) -> None:
        """Initialize the TQDM reporting module."""
        self.config = config
        self.pbar = tqdm()

    async def on_change(self, progress: TaskStatus) -> None:
        """On step changed."""
        step = progress
        if step.state == ReportingState.COMPLETED:
            self.pbar.close()
            return

        self.pbar.set_description(step.operation)
        self.pbar.refresh()
        # Update description if message is provided
        if step.error:
            # Fix the event message to a specific size so it's not jumping around
            # If it's too small, add spaces
            # If it's too large, truncate
            if len(step.error) < 30:
                self.pbar.set_description(step.error + " " * (30 - len(step.error)))
            else:
                self.pbar.set_description(step.error[-30:])
        else:
            self.pbar.set_description(step.operation)
