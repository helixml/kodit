"""Service for syncing repository branches and tags."""

from datetime import UTC, datetime

import structlog

from kodit.domain.entities.git import GitBranch, GitRepo, GitTag
from kodit.domain.protocols import (
    GitBranchRepository,
    GitCommitRepository,
    GitTagRepository,
)
from kodit.domain.services.git_repository_service import GitRepositoryScanner


class RepositorySyncService:
    """Service for syncing repository branches and tags."""

    def __init__(
        self,
        scanner: GitRepositoryScanner,
        git_commit_repository: GitCommitRepository,
        git_branch_repository: GitBranchRepository,
        git_tag_repository: GitTagRepository,
    ) -> None:
        """Initialize the repository sync service."""
        self.scanner = scanner
        self.git_commit_repository = git_commit_repository
        self.git_branch_repository = git_branch_repository
        self.git_tag_repository = git_tag_repository
        self._log = structlog.get_logger(__name__)

    async def sync_branches_and_tags(self, repo: GitRepo) -> None:
        """Sync all branches and tags from git to database."""
        if not repo.id:
            raise ValueError("Repository must have an ID")
        if not repo.cloned_path:
            raise ValueError(f"Repository {repo.id} has never been cloned")

        current_time = datetime.now(UTC)

        # Sync branches and tags
        await self._sync_branches(repo, current_time)
        await self._sync_tags(repo, current_time)

    async def _sync_branches(self, repo: GitRepo, current_time: datetime) -> int:
        """Sync branches from git to database."""
        if not repo.id or not repo.cloned_path:
            raise ValueError("Repository must have ID and cloned_path")

        # Get all branches from git
        branch_data = await self.scanner.git_adapter.get_all_branches(repo.cloned_path)
        self._log.info(f"Found {len(branch_data)} branches in git")

        # Get all branch head SHAs efficiently
        branch_names = [branch_info["name"] for branch_info in branch_data]
        branch_head_shas = await self.scanner.git_adapter.get_all_branch_head_shas(
            repo.cloned_path, branch_names
        )

        # Create branches only for commits that exist in database
        branches = []
        skipped = 0
        for branch_info in branch_data:
            branch_name = branch_info["name"]
            head_sha = branch_head_shas.get(branch_name)

            if not head_sha:
                self._log.warning(f"No head commit found for branch {branch_name}")
                continue

            branch = GitBranch(
                repo_id=repo.id,
                created_at=current_time,
                name=branch_name,
                head_commit_sha=head_sha,
            )
            branches.append(branch)
            self._log.debug(f"Processed branch: {branch_name}")

        # Save branches individually (handles upsert)
        for branch in branches:
            await self.git_branch_repository.save(branch)

        if branches:
            self._log.info(f"Saved {len(branches)} branches to database")
        if skipped > 0:
            self._log.info(f"Skipped {skipped} branches - commits not in database yet")

        # Delete branches that no longer exist in git
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

        # Get all tags from git
        tag_data = await self.scanner.git_adapter.get_all_tags(repo.cloned_path)
        self._log.info(f"Found {len(tag_data)} tags in git")

        # Create tags only for commits that exist in database
        tags = []
        skipped = 0
        for tag_info in tag_data:
            try:
                target_sha = tag_info["target_commit_sha"]
                git_tag = GitTag(
                    repo_id=repo.id,
                    name=tag_info["name"],
                    target_commit_sha=target_sha,
                    created_at=current_time,
                    updated_at=current_time,
                )
                tags.append(git_tag)
                self._log.debug(f"Processed tag: {tag_info['name']}")
            except (KeyError, ValueError) as e:
                self._log.warning(
                    f"Failed to process tag {tag_info.get('name', 'unknown')}: {e}"
                )
                continue

        # Save tags individually (handles upsert)
        for tag in tags:
            await self.git_tag_repository.save(tag)

        if tags:
            self._log.info(f"Saved {len(tags)} tags to database")
        if skipped > 0:
            self._log.info(f"Skipped {skipped} tags - commits not in database yet")

        # Delete tags that no longer exist in git
        existing_tags = await self.git_tag_repository.get_by_repo_id(repo.id)
        git_tag_names = {t.name for t in tags}
        for existing_tag in existing_tags:
            if existing_tag.name not in git_tag_names:
                await self.git_tag_repository.delete(existing_tag)
                self._log.info(f"Deleted tag {existing_tag.name} (no longer in git)")

        return len(tags)
