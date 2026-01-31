"""Domain service for querying task status."""

from kodit.domain.entities import RepositoryStatusSummary, TaskStatus
from kodit.domain.protocols import TaskRepository, TaskStatusRepository
from kodit.domain.value_objects import TrackableType
from kodit.infrastructure.sqlalchemy.query import TaskQueryBuilder


class TaskStatusQueryService:
    """Query service for task status information."""

    def __init__(
        self,
        repository: TaskStatusRepository,
        task_repository: TaskRepository | None = None,
    ) -> None:
        """Initialize the task status query service."""
        self._repository = repository
        self._task_repository = task_repository

    async def get_index_status(self, repo_id: int) -> list[TaskStatus]:
        """Get the status of tasks for a specific index."""
        return await self._repository.load_with_hierarchy(
            trackable_type=TrackableType.KODIT_REPOSITORY.value, trackable_id=repo_id
        )

    async def get_pending_task_count(self, repo_id: int) -> int:
        """Get the count of pending tasks in the queue for a repository."""
        if self._task_repository is None:
            return 0
        query = TaskQueryBuilder().for_repository(repo_id)
        return await self._task_repository.count(query)

    async def get_status_summary(self, repo_id: int) -> RepositoryStatusSummary:
        """Get a summary of the repository indexing status."""
        tasks = await self.get_index_status(repo_id)
        pending_task_count = await self.get_pending_task_count(repo_id)
        return RepositoryStatusSummary.from_tasks(tasks, pending_task_count)
