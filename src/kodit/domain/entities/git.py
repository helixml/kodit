"""Git domain entities - Lightweight version for improved performance."""

from dataclasses import dataclass
from datetime import datetime
from hashlib import sha256
from pathlib import Path

from pydantic import AnyUrl, BaseModel

from kodit.domain.entities import WorkingCopy
from kodit.domain.value_objects import Enrichment, IndexStatus
from kodit.utils.path_utils import repo_id_from_uri


class GitFile(BaseModel):
    """File domain entity."""

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


class GitCommit(BaseModel):
    """Commit domain entity as its own aggregate root."""

    created_at: datetime | None = None
    updated_at: datetime | None = None
    commit_sha: str
    repo_id: int = 0  # Will be set when saving
    date: datetime
    message: str
    parent_commit_sha: str | None = None
    files: list[GitFile]
    author: str

    @property
    def id(self) -> str:
        """Get the unique id for a commit."""
        return self.commit_sha


class GitBranch(BaseModel):
    """Branch domain entity as its own aggregate root."""

    repo_id: int = 0  # Will be set when saving
    name: str
    created_at: datetime | None = None
    updated_at: datetime | None = None
    head_commit_sha: str

    @property
    def id(self) -> str:
        """Get the unique id for a branch."""
        return f"{self.repo_id}-{self.name}"


class GitTag(BaseModel):
    """Tag domain entity as its own aggregate root."""

    created_at: datetime | None = None
    updated_at: datetime | None = None
    repo_id: int = 0  # Will be set when saving
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


class GitRepo(BaseModel):
    """Lightweight repository domain entity containing only essential metadata.

    This replaces the old massive aggregate that contained all commits, branches,
    tags, and files. Now it only contains repository metadata.
    """

    id: int = 0  # Will be set by database
    created_at: datetime | None = None
    updated_at: datetime | None = None
    sanitized_remote_uri: AnyUrl
    remote_uri: AnyUrl
    cloned_path: Path | None = None
    tracking_branch_name: str | None = None
    last_scanned_at: datetime | None = None

    @staticmethod
    def from_remote_uri(remote_uri: AnyUrl) -> "GitRepo":
        """Create a new Git repository from a remote URI."""
        return GitRepo(
            remote_uri=remote_uri,
            sanitized_remote_uri=WorkingCopy.sanitize_git_url(str(remote_uri)),
        )

    @staticmethod
    def create_id(sanitized_remote_uri: AnyUrl) -> str:
        """Create a unique business key for a repository."""
        return repo_id_from_uri(sanitized_remote_uri)


@dataclass(frozen=True)
class RepositoryScanResult:
    """Immutable scan result containing all repository metadata."""

    commits: list[GitCommit]
    branches: list[GitBranch]
    tags: list[GitTag]
    scan_timestamp: datetime
    total_files_across_commits: int
    tracking_branch_name: str | None


class CommitIndex(BaseModel):
    """Aggregate root for indexed commit data."""

    commit_sha: str
    created_at: datetime | None = None
    updated_at: datetime | None = None
    snippets: list["SnippetV2"]
    status: IndexStatus
    indexed_at: datetime | None = None
    error_message: str | None = None
    files_processed: int = 0
    processing_time_seconds: float = 0.0

    def get_snippet_count(self) -> int:
        """Get total number of snippets."""
        return len(self.snippets)

    @property
    def id(self) -> str:
        """Get the unique id for a commit index."""
        return self.commit_sha


class SnippetV2(BaseModel):
    """Snippet domain entity."""

    sha: str
    created_at: datetime | None = None
    updated_at: datetime | None = None
    derives_from: list[GitFile]
    content: str
    enrichments: list[Enrichment] = []
    extension: str

    @property
    def id(self) -> str:
        """Get the unique id for a snippet."""
        return self.sha

    @staticmethod
    def compute_sha(content: str) -> str:
        """Compute the SHA for a snippet."""
        return sha256(content.encode()).hexdigest()
