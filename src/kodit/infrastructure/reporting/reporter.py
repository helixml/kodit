"""Progress reporter."""

from kodit.domain.protocols import ReportingService
from kodit.domain.value_objects import ProgressState
from kodit.infrastructure.reporting.log_progress import LogProgress
from kodit.infrastructure.reporting.progress import Progress, ProgressConfig
from kodit.infrastructure.reporting.tdqm_progress import TQDMProgress


class Reporter(ReportingService):
    """Reporter reports on progress."""

    def __init__(
        self,
        modules: list[Progress] | None = None,
    ) -> None:
        """Initialize the reporter."""
        self.modules = modules or []

    def update(self, state: ProgressState) -> None:
        """Update the reporter."""
        for module in self.modules:
            module.on_update(state)

    def complete(self) -> None:
        """Complete the reporter."""
        for module in self.modules:
            module.on_complete()


def create_noop_reporter() -> Reporter:
    """Create a noop reporter."""
    return Reporter(modules=[])


def create_cli_reporter(config: ProgressConfig | None = None) -> Reporter:
    """Create a CLI reporter."""
    shared_config = config or ProgressConfig()
    return Reporter(modules=[TQDMProgress(shared_config)])


def create_server_reporter(config: ProgressConfig | None = None) -> Reporter:
    """Create a server reporter."""
    shared_config = config or ProgressConfig()
    return Reporter(modules=[LogProgress(shared_config)])
