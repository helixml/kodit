"""Domain services for Git repository scanning and cloning operations (V2).

This version works with the refactored V2 entities and repositories
for improved performance.
"""

import shutil
from dataclasses import dataclass
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

import structlog
from pydantic import AnyUrl

from kodit.domain.entities import WorkingCopy
from kodit.domain.entities.git_v2 import GitBranchV2, GitCommitV2, GitFileV2, GitTagV2
from kodit.domain.protocols import GitAdapter


@dataclass(frozen=True)
class RepositoryInfoV2:
    """Immutable repository information needed for GitRepositoryV2 construction."""

    remote_uri: AnyUrl
    sanitized_remote_uri: AnyUrl
    cloned_path: Path


@dataclass(frozen=True)
class RepositoryScanResultV2:
    """Immutable scan result containing all repository metadata (V2)."""

    commits: list[GitCommitV2]
    branches: list[GitBranchV2]
    tags: list[GitTagV2]
    scan_timestamp: datetime
    total_files_across_commits: int
    tracking_branch_name: str | None


class GitRepositoryScannerV2:
    """Pure scanner that extracts data without mutation (V2)."""

    def __init__(self, git_adapter: GitAdapter, repo_id: int) -> None:
        """Initialize the Git repository scanner.

        Args:
            git_adapter: The Git adapter to use for Git operations.
            repo_id: The repository ID to associate with scanned data.

        """
        self._log = structlog.getLogger(__name__)
        self.git_adapter = git_adapter
        self.repo_id = repo_id

    async def scan_repository(self, cloned_path: Path) -> RepositoryScanResultV2:
        """Scan repository and return immutable result data."""
        self._log.info(f"Starting repository scan at: {cloned_path}")

        # Get all data in bulk for maximum efficiency
        branch_data = await self.git_adapter.get_all_branches(cloned_path)
        self._log.info(f"Found {len(branch_data)} branches")

        # Get all commits at once to avoid redundant processing
        all_commits_data = await self.git_adapter.get_all_commits_bulk(cloned_path)
        self._log.info(f"Found {len(all_commits_data)} unique commits")

        # Process branches efficiently using bulk commit data
        branches, commits = await self._process_branches_and_commits_bulk(
            cloned_path, branch_data, all_commits_data
        )
        tags = await self._process_tags(cloned_path, all_commits_data)

        # Determine tracking branch
        tracking_branch_name = self._determine_tracking_branch(branches)

        total_files = sum(len(commit.files) for commit in commits)

        return RepositoryScanResultV2(
            commits=commits,
            branches=branches,
            tags=tags,
            scan_timestamp=datetime.now(UTC),
            total_files_across_commits=total_files,
            tracking_branch_name=tracking_branch_name,
        )

    async def _process_branches_and_commits_bulk(
        self,
        cloned_path: Path,
        branch_data: list[dict],
        all_commits_data: dict[str, dict[str, Any]],
    ) -> tuple[list[GitBranchV2], list[GitCommitV2]]:
        """Process branches and commits efficiently using bulk commit data."""
        branches = []
        commit_cache: dict[str, GitCommitV2] = {}

        # First, convert all commit data to GitCommitV2 objects with files
        for commit_sha, commit_data in all_commits_data.items():
            if commit_sha not in commit_cache:
                git_commit = await self._create_git_commit_from_data(
                    cloned_path, commit_data
                )
                if git_commit:
                    commit_cache[commit_sha] = git_commit

        # Now process branches using the pre-built commit cache
        for branch_info in branch_data:
            # Get commit SHAs for this branch (much faster than full commit data)
            try:
                commit_shas = await self.git_adapter.get_branch_commit_shas(
                    cloned_path, branch_info["name"]
                )

                if commit_shas and commit_shas[0] in commit_cache:
                    head_commit_sha = commit_shas[0]
                    branch = GitBranchV2(
                        repo_id=self.repo_id,
                        name=branch_info["name"],
                        head_commit_sha=head_commit_sha,
                        created_at=datetime.now(UTC),
                    )
                    branches.append(branch)
                    self._log.debug(f"Added branch: {branch_info['name']}")
            except Exception as e:
                self._log.error(
                    f"Failed to process branch {branch_info['name']}: {e}"
                )
                continue

        # Convert cache to list
        commits = list(commit_cache.values())
        return branches, commits

    async def _create_git_commit_from_data(
        self, cloned_path: Path, commit_data: dict[str, Any]
    ) -> GitCommitV2 | None:
        """Create a GitCommitV2 object from commit data."""
        try:
            commit_sha = commit_data["commit"]
            files_data = await self.git_adapter.get_commit_files(
                cloned_path, commit_sha
            )

            files = []
            for file_data in files_data:
                try:
                    git_file = GitFileV2(
                        created_at=datetime.now(UTC),
                        blob_sha=file_data["blob_sha"],
                        path=file_data["path"],
                        mime_type=file_data["mime_type"],
                        size=file_data["size"],
                        extension=GitFileV2.extension_from_path(file_data["path"]),
                    )
                    files.append(git_file)
                except Exception as e:
                    self._log.error(
                        f"Failed to create GitFile from {file_data}: {e}"
                    )
                    continue

            return GitCommitV2(
                created_at=datetime.now(UTC),
                commit_sha=commit_sha,
                repo_id=self.repo_id,
                date=commit_data["date"],
                message=commit_data["message"],
                parent_commit_sha=commit_data.get("parent_commit"),
                files=files,
                author=commit_data["author"],
            )
        except Exception as e:
            self._log.error(f"Failed to create GitCommit from {commit_data}: {e}")
            return None

    async def _process_tags(
        self, cloned_path: Path, all_commits_data: dict[str, dict[str, Any]]
    ) -> list[GitTagV2]:
        """Process tags and associate them with commits."""
        tags = []
        try:
            tags_data = await self.git_adapter.get_all_tags(cloned_path)
            self._log.info(f"Found {len(tags_data)} tags")

            for tag_info in tags_data:
                target_commit_sha = tag_info["target_commit"]
                if target_commit_sha in all_commits_data:
                    tag = GitTagV2(
                        created_at=datetime.now(UTC),
                        repo_id=self.repo_id,
                        name=tag_info["name"],
                        target_commit_sha=target_commit_sha,
                    )
                    tags.append(tag)
                    self._log.debug(f"Added tag: {tag_info['name']}")
        except Exception as e:
            self._log.error(f"Failed to process tags: {e}")

        return tags

    def _determine_tracking_branch(self, branches: list[GitBranchV2]) -> str | None:
        """Determine the tracking branch (prefer main, then master, then first)."""
        if not branches:
            return None

        branch_names = [branch.name for branch in branches]

        # Prefer main, then master
        for preferred_name in ["main", "master"]:
            if preferred_name in branch_names:
                return preferred_name

        # Return first available branch
        return branches[0].name


class RepositoryClonerV2:
    """Service for cloning Git repositories (V2)."""

    def __init__(self, git_adapter: GitAdapter, clone_base_path: Path) -> None:
        """Initialize the repository cloner.

        Args:
            git_adapter: The Git adapter to use for Git operations.
            clone_base_path: Base directory where repositories will be cloned.

        """
        self._log = structlog.get_logger(__name__)
        self.git_adapter = git_adapter
        self.clone_base_path = clone_base_path

    async def clone_repository(self, remote_uri: AnyUrl) -> Path:
        """Clone a repository and return the local path."""
        self._log.info(f"Cloning repository: {remote_uri}")

        # Create local path for the repository
        working_copy = WorkingCopy(remote_uri)
        clone_path = self.clone_base_path / working_copy.local_directory_name

        # Ensure the clone directory exists
        self.clone_base_path.mkdir(parents=True, exist_ok=True)

        # Remove existing directory if it exists
        if clone_path.exists():
            self._log.info(f"Removing existing clone at: {clone_path}")
            shutil.rmtree(clone_path)

        # Clone the repository
        await self.git_adapter.clone_repository(str(remote_uri), clone_path)

        self._log.info(f"Successfully cloned repository to: {clone_path}")
        return clone_path

    async def pull_repository(self, cloned_path: Path) -> None:
        """Pull latest changes for existing repository."""
        if not cloned_path.exists():
            raise ValueError(f"Repository at {cloned_path} does not exist")

        await self.git_adapter.pull_repository(cloned_path)
        self._log.info(f"Successfully pulled latest changes for: {cloned_path}")
