"""Bundle of infrastructure services."""

from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from kodit.application.services.queue_service import QueueService
    from kodit.application.services.reporting import ProgressTracker


class InfrastructureServices:
    """Bundles infrastructure services.

    This is a Parameter Object pattern to reduce constructor complexity.
    """

    def __init__(
        self,
        operation: "ProgressTracker",
        queue: "QueueService",
    ) -> None:
        """Initialize infrastructure services bundle."""
        self.operation = operation
        self.queue = queue
