"""Log progress using structlog."""

from datetime import UTC, datetime

import structlog

from kodit.config import ReportingConfig
from kodit.domain.entities import TaskStatus
from kodit.domain.protocols import ReportingModule
from kodit.domain.value_objects import ReportingState


class LoggingReportingModule(ReportingModule):
    """Logging reporting module."""

    def __init__(self, config: ReportingConfig) -> None:
        """Initialize the logging reporting module."""
        self.config = config
        self._log = structlog.get_logger(__name__)
        self._last_log_time: datetime = datetime.now(UTC)

    async def on_change(self, progress: TaskStatus) -> None:
        """On step changed."""
        current_time = datetime.now(UTC)
        time_since_last_log = current_time - self._last_log_time
        step = progress

        if (
            step.state != ReportingState.IN_PROGRESS
            or time_since_last_log >= self.config.log_time_interval
        ):
            self._log.info(
                step.operation,
                state=step.state,
                completion_percent=step.completion_percent,
            )
            self._last_log_time = current_time
