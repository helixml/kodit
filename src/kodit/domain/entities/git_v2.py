"""Lightweight Git domain entities for improved performance.

This module contains refactored Git entities that follow DDD principles
with proper aggregate boundaries to avoid the performance issues of the
monolithic GitRepo aggregate.
"""

from datetime import datetime
from pathlib import Path

from pydantic import AnyUrl, BaseModel

from kodit.utils.path_utils import repo_id_from_uri


class GitRepositoryV2(BaseModel):
    """Lightweight repository domain entity containing only essential metadata."""

    id: int = 0  # Will be set by database
    created_at: datetime | None = None
    updated_at: datetime | None = None
    sanitized_remote_uri: AnyUrl
    remote_uri: AnyUrl
    cloned_path: Path | None = None
    tracking_branch_name: str | None = None
    last_scanned_at: datetime | None = None

    @staticmethod
    def from_remote_uri(remote_uri: AnyUrl) -> "GitRepositoryV2":
        """Create a new Git repository from a remote URI."""
        from kodit.domain.entities import WorkingCopy

        return GitRepositoryV2(
            remote_uri=remote_uri,
            sanitized_remote_uri=WorkingCopy.sanitize_git_url(str(remote_uri)),
        )

    @staticmethod
    def create_id(sanitized_remote_uri: AnyUrl) -> str:
        """Create a unique business key for a repository."""
        return repo_id_from_uri(sanitized_remote_uri)


class GitFileV2(BaseModel):
    """File domain entity (unchanged from original)."""

    created_at: datetime
    blob_sha: str
    path: str
    mime_type: str
    size: int
    extension: str

    @property
    def id(self) -> str:
        """Get the unique id for a file."""
        return self.blob_sha

    @staticmethod
    def extension_from_path(path: str) -> str:
        """Get the extension from a path."""
        if not path or "." not in path:
            return "unknown"
        return path.split(".")[-1]


class GitCommitV2(BaseModel):
    """Commit domain entity as its own aggregate root."""

    created_at: datetime | None = None
    updated_at: datetime | None = None
    commit_sha: str
    repo_id: int
    date: datetime
    message: str
    parent_commit_sha: str | None = None
    files: list[GitFileV2]
    author: str

    @property
    def id(self) -> str:
        """Get the unique id for a commit."""
        return self.commit_sha


class GitBranchV2(BaseModel):
    """Branch domain entity as its own aggregate root."""

    repo_id: int
    name: str
    created_at: datetime | None = None
    updated_at: datetime | None = None
    head_commit_sha: str

    @property
    def id(self) -> str:
        """Get the unique id for a branch."""
        return f"{self.repo_id}-{self.name}"


class GitTagV2(BaseModel):
    """Tag domain entity as its own aggregate root."""

    created_at: datetime | None = None
    updated_at: datetime | None = None
    repo_id: int
    name: str
    target_commit_sha: str

    @property
    def id(self) -> str:
        """Get the unique id for a tag."""
        return f"{self.repo_id}-{self.name}"

    @property
    def is_version_tag(self) -> bool:
        """Check if this appears to be a version tag."""
        import re

        # Simple heuristic for version tags
        version_pattern = r"^v?\d+\.\d+(\.\d+)?(-\w+)?$"
        return bool(re.match(version_pattern, self.name))


class GitBranchHeadV2(BaseModel):
    """Minimal branch info for repository metadata."""

    name: str
    head_commit_sha: str
