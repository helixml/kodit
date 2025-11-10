"""Service for managing repository lifecycle operations."""

from datetime import UTC, datetime
from typing import TYPE_CHECKING

import structlog
from pydantic import AnyUrl

from kodit.application.services.queue_service import QueueService
from kodit.application.services.reporting import ProgressTracker
from kodit.domain.entities import WorkingCopy
from kodit.domain.entities.git import GitBranch, GitRepo, GitTag
from kodit.domain.factories.git_repo_factory import GitRepoFactory
from kodit.domain.protocols import (
    GitBranchRepository,
    GitCommitRepository,
    GitRepoRepository,
    GitTagRepository,
)
from kodit.domain.services.git_repository_service import (
    GitRepositoryScanner,
    RepositoryCloner,
)
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


class RepositoryLifecycleService:
    """Manages repository lifecycle: creation, cloning, and synchronization."""

    def __init__(  # noqa: PLR0913
        self,
        repo_repository: GitRepoRepository,
        git_commit_repository: GitCommitRepository,
        git_branch_repository: GitBranchRepository,
        git_tag_repository: GitTagRepository,
        cloner: RepositoryCloner,
        scanner: GitRepositoryScanner,
        queue: QueueService,
        operation: ProgressTracker,
        repository_query_service: "RepositoryQueryService",
    ) -> None:
        """Initialize the repository lifecycle service."""
        self.repo_repository = repo_repository
        self.git_commit_repository = git_commit_repository
        self.git_branch_repository = git_branch_repository
        self.git_tag_repository = git_tag_repository
        self.cloner = cloner
        self.scanner = scanner
        self.queue = queue
        self.operation = operation
        self.repository_query_service = repository_query_service
        self._log = structlog.get_logger(__name__)

    async def create_or_get_repository(
        self, remote_uri: AnyUrl
    ) -> tuple[GitRepo, bool]:
        """Create a new Git repository or get existing one."""
        sanitized_uri = str(WorkingCopy.sanitize_git_url(str(remote_uri)))
        existing_repos = await self.repo_repository.find(
            QueryBuilder().filter(
                "sanitized_remote_uri", FilterOperator.EQ, sanitized_uri
            )
        )
        existing_repo = existing_repos[0] if existing_repos else None

        if existing_repo:
            await self.queue.enqueue_tasks(
                tasks=PrescribedOperations.CREATE_NEW_REPOSITORY,
                base_priority=QueuePriority.USER_INITIATED,
                payload={"repository_id": existing_repo.id},
            )
            return existing_repo, False

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

    async def clone_repository(self, repository_id: int) -> None:
        """Clone a repository and enqueue head commit scan."""
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
            await self.sync_branches_and_tags(repo)

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

    async def sync_repository(self, repository_id: int) -> None:
        """Sync a repository by pulling and scanning head commit if changed."""
        async with self.operation.create_child(
            TaskOperation.SYNC_REPOSITORY,
            trackable_type=TrackableType.KODIT_REPOSITORY,
            trackable_id=repository_id,
        ):
            repo = await self.repo_repository.get(repository_id)
            if not repo.cloned_path:
                raise ValueError(f"Repository {repository_id} has never been cloned")

            await self.cloner.pull_repository(repo)
            await self.sync_branches_and_tags(repo)

            commit_sha = (
                await self.repository_query_service.resolve_tracked_commit_from_git(
                    repo
                )
            )
            self._log.info(
                f"Syncing repository {repository_id}, head commit is {commit_sha[:8]}"
            )

            existing_commit = await self.git_commit_repository.find(
                QueryBuilder().filter("commit_sha", FilterOperator.EQ, commit_sha)
            )

            if existing_commit:
                self._log.info(
                    f"Commit {commit_sha[:8]} already scanned, sync complete"
                )
                return

            self._log.info(
                f"New commit {commit_sha[:8]} detected, enqueuing scan and indexing"
            )
            await self.queue.enqueue_tasks(
                tasks=PrescribedOperations.SCAN_AND_INDEX_COMMIT,
                base_priority=QueuePriority.BACKGROUND,
                payload={"commit_sha": commit_sha, "repository_id": repository_id},
            )

    async def sync_branches_and_tags(self, repo: GitRepo) -> None:
        """Sync all branches and tags from git to database."""
        if not repo.id:
            raise ValueError("Repository must have an ID")
        if not repo.cloned_path:
            raise ValueError(f"Repository {repo.id} has never been cloned")

        current_time = datetime.now(UTC)
        num_branches = await self._sync_branches(repo, current_time)
        num_tags = await self._sync_tags(repo, current_time)

        repo.num_branches = num_branches
        repo.num_tags = num_tags
        await self.repo_repository.save(repo)

    async def _sync_branches(self, repo: GitRepo, current_time: datetime) -> int:
        """Sync branches from git to database."""
        if not repo.id or not repo.cloned_path:
            raise ValueError("Repository must have ID and cloned_path")

        branch_data = await self.scanner.git_adapter.get_all_branches(repo.cloned_path)
        self._log.info(f"Found {len(branch_data)} branches in git")

        branch_names = [branch_info["name"] for branch_info in branch_data]
        branch_head_shas = await self.scanner.git_adapter.get_all_branch_head_shas(
            repo.cloned_path, branch_names
        )

        branches = []
        skipped = 0
        for branch_info in branch_data:
            branch_name = branch_info["name"]
            head_sha = branch_head_shas.get(branch_name)

            if not head_sha:
                self._log.warning(f"No head commit found for branch {branch_name}")
                continue

            try:
                await self.git_commit_repository.get(head_sha)
                branch = GitBranch(
                    repo_id=repo.id,
                    created_at=current_time,
                    name=branch_name,
                    head_commit_sha=head_sha,
                )
                branches.append(branch)
                self._log.debug(f"Processed branch: {branch_name}")
            except Exception:  # noqa: BLE001
                skipped += 1
                self._log.debug(
                    f"Skipping branch {branch_name} - "
                    f"commit {head_sha[:8]} not in database yet"
                )

        for branch in branches:
            await self.git_branch_repository.save(branch)

        if branches:
            self._log.info(f"Saved {len(branches)} branches to database")
        if skipped > 0:
            self._log.info(
                f"Skipped {skipped} branches - commits not in database yet"
            )

        existing_branches = await self.git_branch_repository.get_by_repo_id(repo.id)
        git_branch_names = {b.name for b in branches}
        for existing_branch in existing_branches:
            if existing_branch.name not in git_branch_names:
                await self.git_branch_repository.delete(existing_branch)
                self._log.info(
                    f"Deleted branch {existing_branch.name} (no longer in git)"
                )

        return len(branches)

    async def _sync_tags(self, repo: GitRepo, current_time: datetime) -> int:
        """Sync tags from git to database."""
        if not repo.id or not repo.cloned_path:
            raise ValueError("Repository must have ID and cloned_path")

        tag_data = await self.scanner.git_adapter.get_all_tags(repo.cloned_path)
        self._log.info(f"Found {len(tag_data)} tags in git")

        tags = []
        skipped = 0
        for tag_info in tag_data:
            try:
                target_sha = tag_info["target_commit_sha"]

                try:
                    await self.git_commit_repository.get(target_sha)
                    git_tag = GitTag(
                        repo_id=repo.id,
                        name=tag_info["name"],
                        target_commit_sha=target_sha,
                        created_at=current_time,
                        updated_at=current_time,
                    )
                    tags.append(git_tag)
                except Exception:  # noqa: BLE001
                    skipped += 1
                    self._log.debug(
                        f"Skipping tag {tag_info['name']} - "
                        f"commit {target_sha[:8]} not in database yet"
                    )
            except (KeyError, ValueError) as e:
                self._log.warning(
                    f"Failed to process tag {tag_info.get('name', 'unknown')}: {e}"
                )
                continue

        for tag in tags:
            await self.git_tag_repository.save(tag)

        if tags:
            self._log.info(f"Saved {len(tags)} tags to database")
        if skipped > 0:
            self._log.info(f"Skipped {skipped} tags - commits not in database yet")

        existing_tags = await self.git_tag_repository.get_by_repo_id(repo.id)
        git_tag_names = {t.name for t in tags}
        for existing_tag in existing_tags:
            if existing_tag.name not in git_tag_names:
                await self.git_tag_repository.delete(existing_tag)
                self._log.info(f"Deleted tag {existing_tag.name} (no longer in git)")

        return len(tags)
