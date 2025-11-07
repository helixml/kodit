"""Git domain entities."""

from dataclasses import dataclass
from datetime import UTC, datetime
from enum import StrEnum
from hashlib import sha256
from pathlib import Path

from pydantic import AnyUrl, BaseModel, Field

from kodit.domain.value_objects import Enrichment, IndexStatus
from kodit.utils.path_utils import repo_id_from_uri


class TrackingType(StrEnum):
    """Tracking type."""

    BRANCH = "branch"
    TAG = "tag"
    COMMIT_SHA = "commit_sha"


DEFAULT_TRACKING_BRANCH = "main"


class TrackingConfig(BaseModel, frozen=True):
    """Tracking configuration for a repository."""

    type: str = Field(..., description="The type of tracking to use.")
    name: str = Field(..., description="The name of the tracking to use.")


class GitFile(BaseModel):
    """File domain entity."""

    created_at: datetime
    blob_sha: str
    commit_sha: str
    path: str
    mime_type: str
    size: int
    extension: str

    @property
    def id(self) -> str:
        """Get the unique id for a tag."""
        return self.blob_sha

    @staticmethod
    def extension_from_path(path: str) -> str:
        """Get the extension from a path."""
        if not path or "." not in path:
            return "unknown"
        return path.split(".")[-1]


class GitCommit(BaseModel):
    """Commit domain entity."""

    created_at: datetime | None = None  # Is populated by repository
    updated_at: datetime | None = None  # Is populated by repository
    commit_sha: str
    repo_id: int  # Repository this commit belongs to
    date: datetime
    message: str
    parent_commit_sha: str | None = None  # The first commit in the repo is None
    author: str

    @property
    def id(self) -> str:
        """Get the unique id for a tag."""
        return self.commit_sha


class GitTag(BaseModel):
    """Git tag domain entity."""

    created_at: datetime  # Is populated by repository
    updated_at: datetime | None = None  # Is populated by repository
    repo_id: int | None = None
    name: str  # e.g., "v1.0.0", "release-2023"
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


class GitBranch(BaseModel):
    """Branch domain entity."""

    repo_id: int
    name: str
    created_at: datetime | None = None  # Is populated by repository
    updated_at: datetime | None = None  # Is populated by repository
    head_commit_sha: str


@dataclass(frozen=True)
class RepositoryScanResult:
    """Immutable scan result containing all repository metadata."""

    branches: list[GitBranch]
    all_commits: list[GitCommit]
    all_files: list[GitFile]
    all_tags: list[GitTag]
    scan_timestamp: datetime
    total_files_across_commits: int


class GitRepo(BaseModel):
    """Repository domain entity."""

    id: int | None = None  # Database-generated surrogate key
    created_at: datetime | None = None  # Is populated by repository
    updated_at: datetime | None = None  # Is populated by repository
    sanitized_remote_uri: AnyUrl  # Business key for lookups
    remote_uri: AnyUrl  # May include credentials

    # The following may be empty when initially created
    cloned_path: Path | None = None
    last_scanned_at: datetime | None = None
    tracking_config: TrackingConfig | None = None
    num_commits: int = 0  # Total number of commits in this repository
    num_branches: int = 0  # Total number of branches in this repository
    num_tags: int = 0  # Total number of tags in this repository

    @staticmethod
    def create_id(sanitized_remote_uri: AnyUrl) -> str:
        """Create a unique business key for a repository (kept for compatibility)."""
        return repo_id_from_uri(sanitized_remote_uri)

    def update_with_scan_result(self, scan_result: RepositoryScanResult) -> None:
        """Update the GitRepo with a scan result."""
        self.last_scanned_at = datetime.now(UTC)
        self.num_commits = len(scan_result.all_commits)
        self.num_branches = len(scan_result.branches)
        self.num_tags = len(scan_result.all_tags)


class CommitIndex(BaseModel):
    """Aggregate root for indexed commit data."""

    commit_sha: str
    created_at: datetime | None = None  # Is populated by repository
    updated_at: datetime | None = None  # Is populated by repository
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
        """Get the unique id for a tag."""
        return self.commit_sha


class SnippetV2(BaseModel):
    """Snippet domain entity."""

    sha: str  # Content addressed ID to prevent duplicates and unnecessary updates
    created_at: datetime | None = None  # Is populated by repository
    updated_at: datetime | None = None  # Is populated by repository
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
