"""Application services for commit indexing operations."""

import structlog
from pydantic import AnyUrl

from kodit.application.handlers.registry import TaskHandlerRegistry
from kodit.application.services.queue_service import QueueService
from kodit.application.services.reporting import ProgressTracker
from kodit.domain.entities import Task, WorkingCopy
from kodit.domain.entities.git import GitRepo
from kodit.domain.factories.git_repo_factory import GitRepoFactory
from kodit.domain.protocols import GitRepoRepository
from kodit.domain.value_objects import (
    PrescribedOperations,
    QueuePriority,
    TaskOperation,
    TrackableType,
)
from kodit.infrastructure.sqlalchemy.query import FilterOperator, QueryBuilder


class CommitIndexingApplicationService:
    """Application service for commit indexing operations."""

    def __init__(
        self,
        repo_repository: GitRepoRepository,
        operation: ProgressTracker,
        queue: QueueService,
        handler_registry: TaskHandlerRegistry,
    ) -> None:
        """Initialize the commit indexing application service."""
        self.repo_repository = repo_repository
        self.operation = operation
        self.queue = queue
        self.handler_registry = handler_registry
        self._log = structlog.get_logger(__name__)

    async def create_git_repository(self, remote_uri: AnyUrl) -> tuple[GitRepo, bool]:
        """Create a new Git repository or get existing one.

        Returns tuple of (repository, created) where created is True if new.
        """
        # Check if repository already exists
        sanitized_uri = str(WorkingCopy.sanitize_git_url(str(remote_uri)))
        existing_repos = await self.repo_repository.find(
            QueryBuilder().filter(
                "sanitized_remote_uri", FilterOperator.EQ, sanitized_uri
            )
        )
        existing_repo = existing_repos[0] if existing_repos else None

        if existing_repo:
            # Repository exists, trigger re-indexing
            await self.queue.enqueue_tasks(
                tasks=PrescribedOperations.CREATE_NEW_REPOSITORY,
                base_priority=QueuePriority.USER_INITIATED,
                payload={"repository_id": existing_repo.id},
            )
            return existing_repo, False

        # Create new repository
        async with self.operation.create_child(
            TaskOperation.CREATE_REPOSITORY,
            trackable_type=TrackableType.KODIT_REPOSITORY,
        ):
            repo = GitRepoFactory.create_from_remote_uri(remote_uri)
            repo = await self.repo_repository.save(repo)
            await self.queue.enqueue_tasks(
                tasks=PrescribedOperations.CREATE_NEW_REPOSITORY,
                base_priority=QueuePriority.USER_INITIATED,
                payload={"repository_id": repo.id},
            )
            return repo, True

    async def delete_git_repository(self, repo_id: int) -> bool:
        """Delete a Git repository by ID."""
        repo = await self.repo_repository.get(repo_id)
        if not repo:
            return False

        # Use the handler to delete the repository
        handler = self.handler_registry.handler(TaskOperation.DELETE_REPOSITORY)
        await handler.execute({"repository_id": repo_id})
        return True

    async def run_task(self, task: Task) -> None:
        """Run a task by delegating to the appropriate handler."""
        handler = self.handler_registry.handler(task.type)
        await handler.execute(task.payload)
