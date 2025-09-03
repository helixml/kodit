"""Reporting factory."""

from kodit.application.services.reporting import OperationType, Step
from kodit.infrastructure.reporting.log_progress import LoggingReportingModule
from kodit.infrastructure.reporting.progress import ProgressConfig
from kodit.infrastructure.reporting.tdqm_progress import TQDMReportingModule


def create_noop_operation() -> Step:
    """Create a noop reporter."""
    return Step(OperationType.ROOT.value)


def create_cli_operation(config: ProgressConfig | None = None) -> Step:
    """Create a CLI reporter."""
    shared_config = config or ProgressConfig()
    s = Step(OperationType.ROOT.value)
    s.subscribe(TQDMReportingModule(shared_config))
    return s


def create_server_operation(config: ProgressConfig | None = None) -> Step:
    """Create a server reporter."""
    shared_config = config or ProgressConfig()
    s = Step(OperationType.ROOT.value)
    s.subscribe(LoggingReportingModule(shared_config))
    return s
