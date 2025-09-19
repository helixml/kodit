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
        step = progress

        if step.state == ReportingState.FAILED:
            self._log.exception(
                step.operation,
                state=step.state,
                completion_percent=step.completion_percent,
                error=step.error,
            )
        else:
            self._log.info(
                step.operation,
                state=step.state,
                completion_percent=step.completion_percent,
            )
            self._last_log_time = current_time
