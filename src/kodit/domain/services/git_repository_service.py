"""Domain services for Git repository scanning and cloning operations."""

import shutil
from dataclasses import dataclass
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

import structlog
from pydantic import AnyUrl

from kodit.domain.entities import WorkingCopy
from kodit.domain.entities.git import (
    GitCommit,
    GitFile,
    GitRepo,
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

    async def scan_commit(
        self,
        cloned_path: Path,
        commit_sha: str,
        repo_id: int,
    ) -> tuple[GitCommit, list[GitFile]]:
        """Scan a specific commit and return commit with its files.

        Args:
            cloned_path: Path to the cloned repository
            commit_sha: SHA of the commit to scan
            repo_id: Repository ID

        Returns:
            Tuple of (commit, files) for the scanned commit

        """
        self._log.info(f"Scanning commit {commit_sha[:8]} at: {cloned_path}")

        # Get commit details
        commit_data = await self.git_adapter.get_commit_details(cloned_path, commit_sha)

        # Create GitCommit entity
        git_commit = self._create_lightweight_git_commit(
            commit_data, datetime.now(UTC), repo_id
        )
        if not git_commit:
            raise ValueError(f"Failed to create commit object for {commit_sha}")

        # Get files for this commit
        files_data = await self.git_adapter.get_commit_file_data(
            cloned_path, commit_sha
        )
        files = self._create_git_files(cloned_path, files_data, commit_sha)

        self._log.info(
            f"Scanned commit {commit_sha[:8]}: found {len(files)} files"
        )

        return git_commit, files


    def _format_author_from_data(self, commit_data: dict[str, Any]) -> str:
        """Format author string from commit data."""
        author_name = commit_data.get("author_name", "")
        author_email = commit_data.get("author_email", "")
        if author_name and author_email:
            return f"{author_name} <{author_email}>"
        return author_name or "Unknown"

    def _create_lightweight_git_commit(
        self, commit_data: dict[str, Any], created_at: datetime, repo_id: int | None
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
                repo_id=repo_id or 0,  # Use 0 as default if not provided
                date=commit_data["date"],
                message=commit_data["message"],
                parent_commit_sha=commit_data["parent_sha"],
                author=author,
            )
        except Exception as e:  # noqa: BLE001
            self._log.warning(f"Failed to create commit {commit_data.get('sha')}: {e}")
            return None


    def _create_git_files(
        self, cloned_path: Path, files_data: list[dict], commit_sha: str
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

            result.append(
                GitFile(
                    blob_sha=f["blob_sha"],
                    commit_sha=commit_sha,
                    path=full_path,
                    mime_type=f.get("mime_type", "application/octet-stream"),
                    size=f["size"],
                    extension=GitFile.extension_from_path(file_path),
                    created_at=f.get("created_at", current_time),
                )
            )
        return result


    async def process_files_for_commits_batch(
        self, cloned_path: Path, commit_shas: list[str]
    ) -> list[GitFile]:
        """Process files for a batch of commits.

        This allows the application service to process files in batches
        to avoid loading millions of files into memory at once.

        CRITICAL: Reuses a single Repo object to avoid creating 32K+ Repo instances
        which would consume massive memory (1-2 MB each).
        """
        from git import Repo

        # Open repo once and reuse for all commits in this batch
        repo = Repo(cloned_path)
        files = []

        try:
            for commit_sha in commit_shas:
                files_data = await self.git_adapter.get_commit_files(
                    cloned_path, commit_sha, repo=repo
                )
                files.extend(
                    self._create_git_files(cloned_path, files_data, commit_sha)
                )
        finally:
            # Explicitly close the repo to free resources
            repo.close()

        return files


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
