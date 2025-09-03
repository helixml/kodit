"""Log progress using structlog."""

import time
from datetime import UTC, datetime

import structlog

from kodit.application.services.reporting import ProgressTracker
from kodit.config import ReportingConfig
from kodit.domain.protocols import ReportingModule
from kodit.domain.value_objects import Progress, ProgressState, ReportingState


class LoggingReportingModule(ReportingModule):
    """Logging reporting module."""

    def __init__(self, config: ReportingConfig) -> None:
        """Initialize the logging reporting module."""
        self.config = config
        self._log = structlog.get_logger(__name__)
        self._last_log_time: datetime = datetime.now(UTC)

    async def on_change(self, progress: ProgressTracker) -> None:
        """On step changed."""
        current_time = datetime.now(UTC)
        time_since_last_log = current_time - self._last_log_time
        step = await progress.status()

        if (
            step.state != ReportingState.IN_PROGRESS
            or time_since_last_log >= self.config.log_time_interval
        ):
            self._log.info(
                step.name,
                state=step.state,
                message=step.message,
                completion_percent=step.completion_percent,
            )
            self._last_log_time = current_time


class LogProgress(Progress):
    """Log progress using structlog with time-based throttling."""

    def __init__(self, config: ReportingConfig | None = None) -> None:
        """Initialize the log progress."""
        self.log = structlog.get_logger()
        self.config = config or ReportingConfig()
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
