"""Log progress using structlog."""

import structlog

from kodit.domain.value_objects import ProgressState
from kodit.infrastructure.reporting.progress import Progress


class LogProgress(Progress):
    """Log progress using structlog."""

    def __init__(self) -> None:
        """Initialize the log progress."""
        self.log = structlog.get_logger()

    def on_update(self, state: ProgressState) -> None:
        """Log the progress."""
        self.log.info("Progress updated", state=state)

    def on_complete(self) -> None:
        """Log the completion."""
        self.log.info("Progress completed")
