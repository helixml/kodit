"""Log progress using telemetry."""

import structlog

from kodit.domain.entities import TaskStatus
from kodit.domain.protocols import ReportingModule
from kodit.log import log_event


class TelemetryProgressReportingModule(ReportingModule):
    """Database progress reporting module."""

    def __init__(self) -> None:
        """Initialize the logging reporting module."""
        self._log = structlog.get_logger(__name__)

    async def on_change(self, progress: TaskStatus) -> None:
        """On step changed."""
        log_event(
            progress.operation,
        )
