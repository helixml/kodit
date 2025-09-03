"""Log progress using structlog."""

import structlog

from kodit.application.services.reporting import ProgressTracker
from kodit.config import ReportingConfig
from kodit.domain.protocols import ReportingModule, TaskStatusRepository


class DBProgressReportingModule(ReportingModule):
    """Database progress reporting module."""

    def __init__(
        self, task_status_repository: TaskStatusRepository, config: ReportingConfig
    ) -> None:
        """Initialize the database progress reporting module."""
        self.task_status_repository = task_status_repository
        self.config = config
        self._log = structlog.get_logger(__name__)

    async def on_change(self, progress: ProgressTracker) -> None:
        """On step changed - update task status in database."""
        await self.task_status_repository.update(progress)
