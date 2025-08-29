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
        modules: list[Progress],
        config: ProgressConfig | None = None,
    ) -> None:
        """Initialize the reporter."""
        self.modules = modules
        self.config = config

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
    return Reporter(modules=[], config=None)


def create_cli_reporter() -> Reporter:
    """Create a CLI reporter."""
    return Reporter(modules=[TQDMProgress()], config=None)


def create_server_reporter() -> Reporter:
    """Create a server reporter."""
    return Reporter(modules=[LogProgress()], config=None)
