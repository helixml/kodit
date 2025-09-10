"""Reporting factory."""

from kodit.application.services.reporting import ProgressTracker, TaskOperation
from kodit.config import ReportingConfig
from kodit.domain.protocols import TaskStatusRepository
from kodit.infrastructure.reporting.db_progress import DBProgressReportingModule
from kodit.infrastructure.reporting.log_progress import LoggingReportingModule
from kodit.infrastructure.reporting.telemetry_progress import (
    TelemetryProgressReportingModule,
)


def create_noop_operation() -> ProgressTracker:
    """Create a noop reporter."""
    return ProgressTracker.create(TaskOperation.ROOT)


def create_cli_operation(config: ReportingConfig | None = None) -> ProgressTracker:
    """Create a CLI reporter."""
    shared_config = config or ReportingConfig()
    s = ProgressTracker.create(TaskOperation.ROOT)
    s.subscribe(TelemetryProgressReportingModule())
    s.subscribe(LoggingReportingModule(shared_config))
    return s


def create_server_operation(
    task_status_repository: TaskStatusRepository, config: ReportingConfig | None = None
) -> ProgressTracker:
    """Create a server reporter."""
    shared_config = config or ReportingConfig()
    s = ProgressTracker.create(TaskOperation.ROOT)
    s.subscribe(TelemetryProgressReportingModule())
    s.subscribe(LoggingReportingModule(shared_config))
    s.subscribe(DBProgressReportingModule(task_status_repository, shared_config))
    return s
