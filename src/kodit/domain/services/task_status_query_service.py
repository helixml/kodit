"""Domain service for querying task status."""

from kodit.domain.entities import RepositoryStatusSummary, TaskStatus
from kodit.domain.protocols import TaskStatusRepository
from kodit.domain.value_objects import TrackableType


class TaskStatusQueryService:
    """Query service for task status information."""

    def __init__(self, repository: TaskStatusRepository) -> None:
        """Initialize the task status query service."""
        self._repository = repository

    async def get_index_status(self, repo_id: int) -> list[TaskStatus]:
        """Get the status of tasks for a specific index."""
        return await self._repository.load_with_hierarchy(
            trackable_type=TrackableType.KODIT_REPOSITORY.value, trackable_id=repo_id
        )

    async def get_status_summary(self, repo_id: int) -> RepositoryStatusSummary:
        """Get a summary of the repository indexing status."""
        tasks = await self.get_index_status(repo_id)
        return RepositoryStatusSummary.from_tasks(tasks)
