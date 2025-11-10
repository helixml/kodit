"""Service for scanning commits and processing files."""

from datetime import UTC, datetime
from pathlib import Path

import structlog

from kodit.application.services.reporting import ProgressTracker
from kodit.domain.entities.git import GitCommit
from kodit.domain.protocols import (
    GitCommitRepository,
    GitFileRepository,
    GitRepoRepository,
)
from kodit.domain.services.git_repository_service import GitRepositoryScanner
from kodit.domain.value_objects import TaskOperation, TrackableType
from kodit.infrastructure.sqlalchemy.query import FilterOperator, QueryBuilder


class CommitScanningService:
    """Handles commit scanning and file processing operations."""

    def __init__(
        self,
        repo_repository: GitRepoRepository,
        git_commit_repository: GitCommitRepository,
        git_file_repository: GitFileRepository,
        scanner: GitRepositoryScanner,
        operation: ProgressTracker,
    ) -> None:
        """Initialize the commit scanning service."""
        self.repo_repository = repo_repository
        self.git_commit_repository = git_commit_repository
        self.git_file_repository = git_file_repository
        self.scanner = scanner
        self.operation = operation
        self._log = structlog.get_logger(__name__)

    async def scan_commit(self, repository_id: int, commit_sha: str) -> None:
        """Scan a specific commit and save to database."""
        async with self.operation.create_child(
            TaskOperation.SCAN_COMMIT,
            trackable_type=TrackableType.KODIT_REPOSITORY,
            trackable_id=repository_id,
        ) as step:
            existing_commit = await self.git_commit_repository.find(
                QueryBuilder().filter("commit_sha", FilterOperator.EQ, commit_sha)
            )

            if existing_commit:
                await step.skip("Commit already scanned")
                return

            repo = await self.repo_repository.get(repository_id)
            if not repo.cloned_path:
                raise ValueError(f"Repository {repository_id} has never been cloned")

            commit, files = await self.scanner.scan_commit(
                repo.cloned_path, commit_sha, repository_id
            )

            await self.git_commit_repository.save(commit)
            if files:
                await self.git_file_repository.save_bulk(files)

            self._log.info(
                f"Scanned and saved commit {commit_sha[:8]} with {len(files)} files"
            )

            repo.last_scanned_at = datetime.now(UTC)
            repo.num_commits = 1
            await self.repo_repository.save(repo)

    async def process_files_in_batches(
        self,
        cloned_path: Path,
        all_commits: list[GitCommit],
        batch_size: int = 500,
        *,
        is_incremental: bool = False,
    ) -> int:
        """Process file metadata for all commits in batches."""
        total_files = 0
        commit_shas = [commit.commit_sha for commit in all_commits]
        total_batches = (len(commit_shas) + batch_size - 1) // batch_size

        self._log.info(
            f"Processing files for {len(commit_shas)} commits "
            f"in {total_batches} batches"
        )

        for i in range(0, len(commit_shas), batch_size):
            batch = commit_shas[i : i + batch_size]
            batch_num = i // batch_size + 1

            self._log.debug(
                f"Processing batch {batch_num}/{total_batches} ({len(batch)} commits)"
            )

            files = await self.scanner.process_files_for_commits_batch(
                cloned_path, batch
            )

            if files:
                await self.git_file_repository.save_bulk(
                    files, skip_existence_check=not is_incremental
                )
                total_files += len(files)
                self._log.debug(
                    f"Batch {batch_num}: Saved {len(files)} files "
                    f"(total so far: {total_files})"
                )

        return total_files
