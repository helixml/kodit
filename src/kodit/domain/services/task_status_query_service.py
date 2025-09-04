"""Domain service for querying task status."""

from kodit.application.services.reporting import ProgressTracker
from kodit.domain.protocols import TaskStatusRepository
from kodit.domain.value_objects import TrackableType


class TaskStatusQueryService:
    """Query service for task status information."""

    def __init__(self, repository: TaskStatusRepository) -> None:
        """Initialize the task status query service."""
        self._repository = repository

    async def get_index_status(self, index_id: int) -> list[ProgressTracker]:
        """Get the status of tasks for a specific index.

        Args:
            index_id: ID of the index to query status for

        Returns:
            List of progress trackers for the index

        """
        return await self._repository.find(
            trackable_type=TrackableType.INDEX.value,
            trackable_id=index_id
        )
