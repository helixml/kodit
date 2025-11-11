"""Handler for syncing a repository."""

from typing import TYPE_CHECKING, Any

import structlog

from kodit.application.services.queue_service import QueueService
from kodit.application.services.reporting import ProgressTracker
from kodit.application.services.repository_sync_service import RepositorySyncService
from kodit.domain.protocols import GitCommitRepository, GitRepoRepository
from kodit.domain.services.git_repository_service import RepositoryCloner
from kodit.domain.value_objects import (
    PrescribedOperations,
    QueuePriority,
    TaskOperation,
    TrackableType,
)
from kodit.infrastructure.sqlalchemy.query import FilterOperator, QueryBuilder

if TYPE_CHECKING:
    from kodit.application.services.repository_query_service import (
        RepositoryQueryService,
    )


class SyncRepositoryHandler:
    """Handler for syncing a repository."""

    def __init__(  # noqa: PLR0913
        self,
        repo_repository: GitRepoRepository,
        git_commit_repository: GitCommitRepository,
        cloner: RepositoryCloner,
        repository_sync_service: RepositorySyncService,
        repository_query_service: "RepositoryQueryService",
        queue: QueueService,
        operation: ProgressTracker,
    ) -> None:
        """Initialize the sync repository handler."""
        self.repo_repository = repo_repository
        self.git_commit_repository = git_commit_repository
        self.cloner = cloner
        self.repository_sync_service = repository_sync_service
        self.repository_query_service = repository_query_service
        self.queue = queue
        self.operation = operation
        self._log = structlog.get_logger(__name__)

    async def execute(self, payload: dict[str, Any]) -> None:
        """Execute sync repository operation."""
        repository_id = payload["repository_id"]

        async with self.operation.create_child(
            TaskOperation.SYNC_REPOSITORY,
            trackable_type=TrackableType.KODIT_REPOSITORY,
            trackable_id=repository_id,
        ):
            repo = await self.repo_repository.get(repository_id)
            if not repo.cloned_path:
                raise ValueError(f"Repository {repository_id} has never been cloned")

            # Pull latest changes from remote
            await self.cloner.pull_repository(repo)

            # Sync all branches and tags to database
            await self.repository_sync_service.sync_branches_and_tags(repo)

            # Resolve the head commit SHA
            commit_sha = (
                await self.repository_query_service.resolve_tracked_commit_from_git(
                    repo
                )
            )
            self._log.info(
                f"Syncing repository {repository_id}, head commit is {commit_sha[:8]}"
            )

            # Check if we've already scanned this commit
            existing_commit = await self.git_commit_repository.find(
                QueryBuilder().filter("commit_sha", FilterOperator.EQ, commit_sha)
            )

            if existing_commit:
                self._log.info(
                    f"Commit {commit_sha[:8]} already scanned, sync complete"
                )
                return

            # New commit detected, enqueue scan and indexing
            self._log.info(
                f"New commit {commit_sha[:8]} detected, enqueuing scan and indexing"
            )
            await self.queue.enqueue_tasks(
                tasks=PrescribedOperations.SCAN_AND_INDEX_COMMIT,
                base_priority=QueuePriority.BACKGROUND,
                payload={"commit_sha": commit_sha, "repository_id": repository_id},
            )
