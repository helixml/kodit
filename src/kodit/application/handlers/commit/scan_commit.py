"""Handler for scanning a specific commit."""

from datetime import UTC, datetime
from typing import Any

import structlog

from kodit.application.services.reporting import ProgressTracker
from kodit.domain.protocols import (
    GitCommitRepository,
    GitFileRepository,
    GitRepoRepository,
)
from kodit.domain.services.git_repository_service import GitRepositoryScanner
from kodit.domain.value_objects import TaskOperation, TrackableType
from kodit.infrastructure.sqlalchemy.query import FilterOperator, QueryBuilder


class ScanCommitHandler:
    """Handler for scanning a specific commit."""

    def __init__(
        self,
        repo_repository: GitRepoRepository,
        git_commit_repository: GitCommitRepository,
        git_file_repository: GitFileRepository,
        scanner: GitRepositoryScanner,
        operation: ProgressTracker,
    ) -> None:
        """Initialize the scan commit handler."""
        self.repo_repository = repo_repository
        self.git_commit_repository = git_commit_repository
        self.git_file_repository = git_file_repository
        self.scanner = scanner
        self.operation = operation
        self._log = structlog.get_logger(__name__)

    async def execute(self, payload: dict[str, Any]) -> None:
        """Execute scan commit operation."""
        repository_id = payload["repository_id"]
        commit_sha = payload["commit_sha"]

        async with self.operation.create_child(
            TaskOperation.SCAN_COMMIT,
            trackable_type=TrackableType.KODIT_REPOSITORY,
            trackable_id=repository_id,
        ) as step:
            # Check if we've already scanned this commit
            existing_commit = await self.git_commit_repository.find(
                QueryBuilder().filter("commit_sha", FilterOperator.EQ, commit_sha)
            )

            if existing_commit:
                await step.skip("Commit already scanned")
                return

            # Get repository
            repo = await self.repo_repository.get(repository_id)
            if not repo.cloned_path:
                raise ValueError(f"Repository {repository_id} has never been cloned")

            # Scan the specific commit
            commit, files = await self.scanner.scan_commit(
                repo.cloned_path, commit_sha, repository_id
            )

            # Save commit and files
            await self.git_commit_repository.save(commit)
            if files:
                await self.git_file_repository.save_bulk(files)
            self._log.info(
                f"Scanned and saved commit {commit_sha[:8]} with {len(files)} files"
            )

            # Update repository metadata
            repo.last_scanned_at = datetime.now(UTC)
            repo.num_commits = 1  # We only scanned one commit
            await self.repo_repository.save(repo)
