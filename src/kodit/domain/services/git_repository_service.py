"""Domain services for Git repository scanning and cloning operations."""

import asyncio
import shutil
from dataclasses import dataclass
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

import structlog
from pydantic import AnyUrl

from kodit.domain.entities import WorkingCopy
from kodit.domain.entities.git import (
    GitBranch,
    GitCommit,
    GitFile,
    GitRepo,
    GitTag,
    RepositoryScanResult,
)
from kodit.domain.protocols import GitAdapter


@dataclass(frozen=True)
class RepositoryInfo:
    """Immutable repository information needed for GitRepo construction."""

    remote_uri: AnyUrl
    sanitized_remote_uri: AnyUrl
    cloned_path: Path


class GitRepositoryScanner:
    """Pure scanner that extracts data without mutation."""

    def __init__(self, git_adapter: GitAdapter) -> None:
        """Initialize the Git repository scanner.

        Args:
            git_adapter: The Git adapter to use for Git operations.

        """
        self._log = structlog.getLogger(__name__)
        self.git_adapter = git_adapter

    async def scan_repository(self, cloned_path: Path) -> RepositoryScanResult:
        """Scan repository and return immutable result data."""
        self._log.info(f"Starting repository scan at: {cloned_path}")

        # Get all data in bulk for maximum efficiency
        branch_data = await self.git_adapter.get_all_branches(cloned_path)
        self._log.info(f"Found {len(branch_data)} branches")

        # Get all commits at once to avoid redundant processing
        all_commits_data = await self.git_adapter.get_all_commits_bulk(cloned_path)
        self._log.info(f"Found {len(all_commits_data)} unique commits")

        # Process branches efficiently using bulk commit data
        branches, commit_cache = await self._process_branches_bulk(
            cloned_path, branch_data, all_commits_data
        )
        self._log.info(f"Found {len(branches)} branches")
        tags = await self._process_tags(cloned_path, commit_cache)
        self._log.info(f"Found {len(tags)} tags")

        return self._create_scan_result(branches, commit_cache, tags)

    async def _process_commits_concurrently(
        self,
        cloned_path: Path,
        commits_batch: list[tuple[str, dict[str, Any]]],
    ) -> dict[str, GitCommit]:
        """Process a batch of commits concurrently."""
        batch_cache = {}

        async def process_single_commit(
            commit_sha: str, commit_data: dict[str, Any]
        ) -> tuple[str, GitCommit | None]:
            git_commit = await self._create_git_commit_from_data(
                cloned_path, commit_data
            )
            return commit_sha, git_commit

        # Process commits concurrently in smaller batches
        semaphore = asyncio.Semaphore(50)  # Limit concurrent operations

        async def bounded_process(
            item: tuple[str, dict[str, Any]]
        ) -> tuple[str, GitCommit | None]:
            async with semaphore:
                return await process_single_commit(item[0], item[1])

        # Process all commits concurrently
        results = await asyncio.gather(
            *[bounded_process(item) for item in commits_batch],
            return_exceptions=True,
        )

        # Collect successful results
        for result in results:
            if isinstance(result, tuple):
                # Type narrowing: result is now tuple[str, GitCommit | None]
                commit_sha, git_commit = result
                if git_commit is not None:
                    batch_cache[commit_sha] = git_commit

        return batch_cache

    async def _process_branches_bulk(
        self,
        cloned_path: Path,
        branch_data: list[dict],
        all_commits_data: dict[str, dict[str, Any]],
    ) -> tuple[list[GitBranch], dict[str, GitCommit]]:
        """Process branches efficiently using bulk commit data."""
        branches = []
        commit_cache: dict[str, GitCommit] = {}

        # Cache expensive operations
        current_time = datetime.now(UTC)

        # Create lightweight commits without file data (major optimization)
        self._log.info(f"Processing {len(all_commits_data)} commits (metadata only)")

        for commit_sha, commit_data in all_commits_data.items():
            git_commit = self._create_lightweight_git_commit(commit_data, current_time)
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
                    head_commit = commit_cache[commit_shas[0]]
                    branch = GitBranch(
                        created_at=current_time,
                        name=branch_info["name"],
                        head_commit=head_commit,
                    )
                    branches.append(branch)
                    self._log.debug(f"Processed branch: {branch_info['name']}")
                else:
                    self._log.warning(
                        "No commits found for branch %s", branch_info["name"]
                    )

            except Exception as e:  # noqa: BLE001
                self._log.warning(
                    "Failed to process branch %s: %s", branch_info["name"], e
                )
                continue

        return branches, commit_cache

    async def _create_git_commit_from_data(
        self, cloned_path: Path, commit_data: dict[str, Any]
    ) -> GitCommit | None:
        """Create GitCommit from pre-fetched commit data."""
        commit_sha = commit_data["sha"]

        # Get files for this commit
        files_data = await self.git_adapter.get_commit_files(cloned_path, commit_sha)
        files = self._create_git_files(cloned_path, files_data)
        author = self._format_author_from_data(commit_data)

        # Cache datetime creation
        created_at = datetime.now(UTC)

        return GitCommit(
            created_at=created_at,
            commit_sha=commit_sha,
            date=commit_data["date"],
            message=commit_data["message"],
            parent_commit_sha=commit_data["parent_sha"],
            files=files,
            author=author,
        )

    def _format_author_from_data(self, commit_data: dict[str, Any]) -> str:
        """Format author string from commit data."""
        author_name = commit_data.get("author_name", "")
        author_email = commit_data.get("author_email", "")
        if author_name and author_email:
            return f"{author_name} <{author_email}>"
        return author_name or "Unknown"

    def _create_lightweight_git_commit(
        self, commit_data: dict[str, Any], created_at: datetime
    ) -> GitCommit | None:
        """Create a GitCommit without expensive file data fetching."""
        try:
            commit_sha = commit_data["sha"]
            author = self._format_author_from_data(commit_data)

            # Create commit with empty files list for now
            # Files will be loaded lazily when actually needed (e.g., during indexing)
            return GitCommit(
                created_at=created_at,
                commit_sha=commit_sha,
                date=commit_data["date"],
                message=commit_data["message"],
                parent_commit_sha=commit_data["parent_sha"],
                files=[],  # Empty for performance - load on demand
                author=author,
            )
        except Exception as e:  # noqa: BLE001
            self._log.warning(f"Failed to create commit {commit_data.get('sha')}: {e}")
            return None

    async def _process_branches(
        self, cloned_path: Path, branch_data: list[dict]
    ) -> tuple[list[GitBranch], dict[str, GitCommit]]:
        """Process branches and return branches with commit cache."""
        branches = []
        commit_cache: dict[str, GitCommit] = {}

        for branch_info in branch_data:
            branch = await self._process_single_branch(
                cloned_path, branch_info, commit_cache
            )
            if branch:
                branches.append(branch)

        return branches, commit_cache

    async def _process_single_branch(
        self,
        cloned_path: Path,
        branch_info: dict,
        commit_cache: dict[str, GitCommit],
    ) -> GitBranch | None:
        """Process a single branch and return GitBranch or None."""
        self._log.info(f"Processing branch: {branch_info['name']}")

        commits_data = await self.git_adapter.get_branch_commits(
            cloned_path, branch_info["name"]
        )

        if not commits_data:
            self._log.warning(f"No commits found for branch {branch_info['name']}")
            return None

        head_commit = await self._process_branch_commits(
            cloned_path, commits_data, commit_cache
        )

        if head_commit:
            return GitBranch(
                created_at=datetime.now(UTC),
                name=branch_info["name"],
                head_commit=head_commit,
            )
        return None

    async def _process_branch_commits(
        self,
        cloned_path: Path,
        commits_data: list[dict],
        commit_cache: dict[str, GitCommit],
    ) -> GitCommit | None:
        """Process commits for a branch and return head commit."""
        head_commit = None

        for commit_data in commits_data:
            commit_sha = commit_data["sha"]

            # Use cached commit if already processed
            if commit_sha in commit_cache:
                if head_commit is None:
                    head_commit = commit_cache[commit_sha]
                continue

            git_commit = await self._create_git_commit(cloned_path, commit_data)
            if git_commit:
                commit_cache[commit_sha] = git_commit
                if head_commit is None:
                    head_commit = git_commit

        return head_commit

    async def _create_git_commit(
        self, cloned_path: Path, commit_data: dict
    ) -> GitCommit | None:
        """Create GitCommit from commit data."""
        commit_sha = commit_data["sha"]

        files_data = await self.git_adapter.get_commit_files(cloned_path, commit_sha)
        files = self._create_git_files(cloned_path, files_data)
        author = self._format_author(commit_data)

        return GitCommit(
            created_at=datetime.now(UTC),
            commit_sha=commit_sha,
            date=commit_data["date"],
            message=commit_data["message"],
            parent_commit_sha=commit_data["parent_sha"],
            files=files,
            author=author,
        )

    def _create_git_files(
        self, cloned_path: Path, files_data: list[dict]
    ) -> list[GitFile]:
        """Create GitFile entities from files data."""
        # Cache expensive path operations
        cloned_path_str = str(cloned_path)
        current_time = datetime.now(UTC)

        result = []
        for f in files_data:
            # Avoid expensive Path operations by doing string concatenation
            file_path = f["path"]
            full_path = f"{cloned_path_str}/{file_path}"

            result.append(GitFile(
                blob_sha=f["blob_sha"],
                path=full_path,
                mime_type=f.get("mime_type", "application/octet-stream"),
                size=f["size"],
                extension=GitFile.extension_from_path(file_path),
                created_at=f.get("created_at", current_time),
            ))
        return result

    def _format_author(self, commit_data: dict) -> str:
        """Format author string from commit data."""
        author_name = commit_data.get("author_name", "")
        author_email = commit_data.get("author_email", "")
        if author_name and author_email:
            return f"{author_name} <{author_email}>"
        return author_name or "Unknown"

    async def _process_tags(
        self, cloned_path: Path, commit_cache: dict[str, GitCommit]
    ) -> list[GitTag]:
        """Process repository tags."""
        tag_data = await self.git_adapter.get_all_tags(cloned_path)
        tags = []
        for tag_info in tag_data:
            try:
                target_commit = commit_cache[tag_info["target_commit_sha"]]
                git_tag = GitTag(
                    name=tag_info["name"],
                    target_commit=target_commit,
                    created_at=target_commit.created_at or datetime.now(UTC),
                    updated_at=target_commit.updated_at or datetime.now(UTC),
                )
                tags.append(git_tag)
            except (KeyError, ValueError) as e:
                self._log.warning(
                    f"Failed to process tag {tag_info.get('name', 'unknown')}: {e}"
                )
                continue

        self._log.info(f"Found {len(tags)} tags")
        return tags

    def _create_scan_result(
        self,
        branches: list[GitBranch],
        commit_cache: dict[str, GitCommit],
        tags: list[GitTag],
    ) -> RepositoryScanResult:
        """Create final scan result."""
        # Files are loaded on-demand for performance, so total_files is 0 during scan
        total_files = 0

        scan_result = RepositoryScanResult(
            branches=branches,
            all_commits=list(commit_cache.values()),
            scan_timestamp=datetime.now(UTC),
            total_files_across_commits=total_files,
            all_tags=tags,
        )

        self._log.info(
            f"Scan completed. Found {len(branches)} branches with "
            f"{len(commit_cache)} unique commits"
        )
        return scan_result


class RepositoryCloner:
    """Pure service for cloning repositories."""

    def __init__(self, git_adapter: GitAdapter, clone_dir: Path) -> None:
        """Initialize the repository cloner.

        Args:
            git_adapter: The Git adapter to use for Git operations.
            clone_dir: The directory where repositories will be cloned.

        """
        self.git_adapter = git_adapter
        self.clone_dir = clone_dir

    def _get_clone_path(self, sanitized_uri: AnyUrl) -> Path:
        """Get the clone path for a Git working copy."""
        dir_name = GitRepo.create_id(sanitized_uri)
        return self.clone_dir / dir_name

    async def clone_repository(self, remote_uri: AnyUrl) -> Path:
        """Clone repository and return repository info."""
        sanitized_uri = WorkingCopy.sanitize_git_url(str(remote_uri))
        clone_path = self._get_clone_path(sanitized_uri)

        try:
            await self.git_adapter.clone_repository(str(remote_uri), clone_path)
        except Exception:
            shutil.rmtree(clone_path)
            raise

        return clone_path

    async def pull_repository(self, repository: GitRepo) -> None:
        """Pull latest changes for existing repository."""
        if not repository.cloned_path:
            raise ValueError("Repository has never been cloned, please clone it first")
        if not repository.cloned_path.exists():
            await self.clone_repository(repository.remote_uri)
            return

        await self.git_adapter.pull_repository(repository.cloned_path)
