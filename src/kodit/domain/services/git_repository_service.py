"""Domain services for Git repository scanning and cloning operations."""

import shutil
from dataclasses import dataclass
from datetime import UTC, datetime
from pathlib import Path

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

        # Get all branches and process them
        branch_data = await self.git_adapter.get_all_branches(cloned_path)
        self._log.info(f"Found {len(branch_data)} branches")

        branches, commit_cache = await self._process_branches(cloned_path, branch_data)
        tags = await self._process_tags(cloned_path, commit_cache)

        return self._create_scan_result(branches, commit_cache, tags)

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
        return [
            GitFile(
                blob_sha=f["blob_sha"],
                path=str(cloned_path / f["path"]),
                mime_type=f.get("mime_type", "application/octet-stream"),
                size=f["size"],
                extension=GitFile.extension_from_path(f["path"]),
                created_at=f.get("created_at", datetime.now(UTC)),
            )
            for f in files_data
        ]

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
        total_files = sum(len(commit.files) for commit in commit_cache.values())

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
