"""Factory for creating GitRepo domain entities."""

from datetime import datetime
from pathlib import Path

from pydantic import AnyUrl

from kodit.domain.entities import WorkingCopy
from kodit.domain.entities.git import GitBranch, GitCommit, GitRepo, GitTag


class GitRepoFactory:
    """Factory for creating GitRepo domain entities."""

    @staticmethod
    def create_from_remote_uri(remote_uri: AnyUrl) -> GitRepo:
        """Create a new Git repository from a remote URI."""
        return GitRepo(
            remote_uri=remote_uri,
            sanitized_remote_uri=WorkingCopy.sanitize_git_url(str(remote_uri)),
        )

    @staticmethod
    def create_from_components(  # noqa: PLR0913
        *,
        repo_id: int | None = None,
        created_at: datetime | None = None,
        updated_at: datetime | None = None,
        sanitized_remote_uri: AnyUrl,
        remote_uri: AnyUrl,
        branches: list[GitBranch] | None = None,
        commits: list[GitCommit] | None = None,
        tags: list[GitTag] | None = None,
        cloned_path: Path | None = None,
        tracking_branch: GitBranch | None = None,
        last_scanned_at: datetime | None = None,
    ) -> GitRepo:
        """Create a GitRepo from individual components."""
        repo = GitRepo(
            id=repo_id,
            created_at=created_at,
            updated_at=updated_at,
            sanitized_remote_uri=sanitized_remote_uri,
            remote_uri=remote_uri,
            branches=branches or [],
            _commits=[],  # Start with empty, will be set below
            tags=tags or [],
            cloned_path=cloned_path,
            tracking_branch=tracking_branch,
            last_scanned_at=last_scanned_at,
        )
        # Set commits through the private field to avoid property issues
        repo._commits = commits or []  # noqa: SLF001
        return repo

    @staticmethod
    def create_from_path_scan(  # noqa: PLR0913
        *,
        remote_uri: AnyUrl,
        sanitized_remote_uri: AnyUrl,
        repo_path: Path,
        branches: list[GitBranch],
        commits: list[GitCommit],
        tags: list[GitTag],
        tracking_branch: GitBranch | None = None,
        last_scanned_at: datetime | None = None,
    ) -> GitRepo:
        """Create a GitRepo from a scanned local repository path."""
        repo = GitRepo(
            id=None,  # Let repository assign database ID
            sanitized_remote_uri=sanitized_remote_uri,
            remote_uri=remote_uri,
            branches=branches,
            _commits=[],  # Start with empty, will be set below
            tags=tags,
            tracking_branch=tracking_branch,
            cloned_path=repo_path,
            last_scanned_at=last_scanned_at,
        )
        # Set commits through the private field to avoid property issues
        repo._commits = commits  # noqa: SLF001
        return repo
