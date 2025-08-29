"""Log progress using structlog."""

import time

import structlog

from kodit.domain.value_objects import ProgressState
from kodit.infrastructure.reporting.progress import Progress, ProgressConfig


class LogProgress(Progress):
    """Log progress using structlog with time-based throttling."""

    def __init__(self, config: ProgressConfig | None = None) -> None:
        """Initialize the log progress."""
        self.log = structlog.get_logger()
        self.config = config or ProgressConfig()
        self.last_log_time: float = 0

    def on_update(self, state: ProgressState) -> None:
        """Log the progress with time-based throttling."""
        current_time = time.time()
        time_since_last_log = current_time - self.last_log_time

        if time_since_last_log >= self.config.log_time_interval.total_seconds():
            self.log.info(
                "Progress...",
                operation=state.operation,
                percentage=state.percentage,
                message=state.message,
            )
            self.last_log_time = current_time

    def on_complete(self) -> None:
        """Log the completion."""
        self.log.info("Completed")
