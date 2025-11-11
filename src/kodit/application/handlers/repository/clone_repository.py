"""Handler for cloning a repository."""

from typing import TYPE_CHECKING, Any

import structlog

from kodit.application.services.queue_service import QueueService
from kodit.application.services.reporting import ProgressTracker
from kodit.application.services.repository_sync_service import RepositorySyncService
from kodit.domain.protocols import GitRepoRepository
from kodit.domain.services.git_repository_service import RepositoryCloner
from kodit.domain.value_objects import (
    PrescribedOperations,
    QueuePriority,
    TaskOperation,
    TrackableType,
)

if TYPE_CHECKING:
    from kodit.application.services.repository_query_service import (
        RepositoryQueryService,
    )


class CloneRepositoryHandler:
    """Handler for cloning a repository."""

    def __init__(  # noqa: PLR0913
        self,
        repo_repository: GitRepoRepository,
        cloner: RepositoryCloner,
        repository_sync_service: RepositorySyncService,
        repository_query_service: "RepositoryQueryService",
        queue: QueueService,
        operation: ProgressTracker,
    ) -> None:
        """Initialize the clone repository handler."""
        self.repo_repository = repo_repository
        self.cloner = cloner
        self.repository_sync_service = repository_sync_service
        self.repository_query_service = repository_query_service
        self.queue = queue
        self.operation = operation
        self._log = structlog.get_logger(__name__)

    async def execute(self, payload: dict[str, Any]) -> None:
        """Execute clone repository operation."""
        repository_id = payload["repository_id"]

        async with self.operation.create_child(
            TaskOperation.CLONE_REPOSITORY,
            trackable_type=TrackableType.KODIT_REPOSITORY,
            trackable_id=repository_id,
        ):
            repo = await self.repo_repository.get(repository_id)
            repo.cloned_path = await self.cloner.clone_repository(repo.remote_uri)

            if not repo.tracking_config:
                repo.tracking_config = (
                    await self.repository_query_service.get_tracking_config(repo)
                )

            await self.repo_repository.save(repo)

            # Sync all branches and tags to database
            await self.repository_sync_service.sync_branches_and_tags(repo)

            # Resolve the head commit SHA and enqueue scan + indexing
            commit_sha = (
                await self.repository_query_service.resolve_tracked_commit_from_git(
                    repo
                )
            )
            self._log.info(
                f"Enqueuing scan for head commit {commit_sha[:8]} "
                f"of repository {repository_id}"
            )

            await self.queue.enqueue_tasks(
                tasks=PrescribedOperations.SCAN_AND_INDEX_COMMIT,
                base_priority=QueuePriority.USER_INITIATED,
                payload={"commit_sha": commit_sha, "repository_id": repository_id},
            )
