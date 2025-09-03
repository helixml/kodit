"""Reporting factory."""

from kodit.application.services.reporting import OperationType, ProgressTracker
from kodit.infrastructure.reporting.log_progress import LoggingReportingModule
from kodit.infrastructure.reporting.progress import ProgressConfig
from kodit.infrastructure.reporting.tdqm_progress import TQDMReportingModule


def create_noop_operation() -> ProgressTracker:
    """Create a noop reporter."""
    return ProgressTracker(OperationType.ROOT.value)


def create_cli_operation(config: ProgressConfig | None = None) -> ProgressTracker:
    """Create a CLI reporter."""
    shared_config = config or ProgressConfig()
    s = ProgressTracker(OperationType.ROOT.value)
    s.subscribe(TQDMReportingModule(shared_config))
    return s


def create_server_operation(config: ProgressConfig | None = None) -> ProgressTracker:
    """Create a server reporter."""
    shared_config = config or ProgressConfig()
    s = ProgressTracker(OperationType.ROOT.value)
    s.subscribe(LoggingReportingModule(shared_config))
    return s
