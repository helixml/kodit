"""Git domain entities."""

from dataclasses import dataclass
from datetime import UTC, datetime
from hashlib import sha256
from pathlib import Path

from pydantic import AnyUrl, BaseModel

from kodit.domain.entities import WorkingCopy
from kodit.domain.value_objects import Enrichment, IndexStatus
from kodit.utils.path_utils import repo_id_from_uri


class GitFile(BaseModel):
    """File domain entity."""

    created_at: datetime | None = None  # Is populated by repository
    updated_at: datetime | None = None  # Is populated by repository
    blob_sha: str
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
    date: datetime
    message: str
    parent_commit_sha: str | None = None  # The first commit in the repo is None
    files: list[GitFile]
    author: str

    @property
    def id(self) -> str:
        """Get the unique id for a tag."""
        return self.commit_sha


class GitTag(BaseModel):
    """Git tag domain entity."""

    created_at: datetime | None = None  # Is populated by repository
    updated_at: datetime | None = None  # Is populated by repository
    name: str  # e.g., "v1.0.0", "release-2023"
    target_commit_sha: str  # The commit this tag points to

    @property
    def id(self) -> str:
        """Get the unique id for a tag."""
        return self.target_commit_sha

    @property
    def is_version_tag(self) -> bool:
        """Check if this appears to be a version tag."""
        import re

        # Simple heuristic for version tags
        version_pattern = r"^v?\d+\.\d+(\.\d+)?(-\w+)?$"
        return bool(re.match(version_pattern, self.name))


class GitBranch(BaseModel):
    """Branch domain entity."""

    id: int | None = None  # Is populated by repository
    created_at: datetime | None = None  # Is populated by repository
    updated_at: datetime | None = None  # Is populated by repository
    name: str
    head_commit: GitCommit


@dataclass(frozen=True)
class RepositoryScanResult:
    """Immutable scan result containing all repository metadata."""

    branches: list[GitBranch]
    all_commits: list[GitCommit]
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
    branches: list[GitBranch] = []
    commits: list[GitCommit] = []
    tags: list[GitTag] = []
    cloned_path: Path | None = None
    tracking_branch: GitBranch | None = None
    last_scanned_at: datetime | None = None

    @staticmethod
    def create_id(sanitized_remote_uri: AnyUrl) -> str:
        """Create a unique business key for a repository (kept for compatibility)."""
        return repo_id_from_uri(sanitized_remote_uri)

    @staticmethod
    def from_remote_uri(remote_uri: AnyUrl) -> "GitRepo":
        """Create a new Git repository from a remote URI."""
        return GitRepo(
            remote_uri=remote_uri,
            sanitized_remote_uri=WorkingCopy.sanitize_git_url(str(remote_uri)),
        )

    def update_with_scan_result(self, scan_result: RepositoryScanResult) -> None:
        """Update the GitRepo with a scan result."""
        # Determine tracking branch (prefer main, then master, then first available)
        if not self.tracking_branch:
            tracking_branch = None
            for preferred_name in ["main", "master"]:
                tracking_branch = next(
                    (b for b in scan_result.branches if b.name == preferred_name), None
                )
                if tracking_branch:
                    break

            if not tracking_branch and scan_result.branches:
                tracking_branch = scan_result.branches[0]

            if not tracking_branch:
                raise ValueError("No tracking branch found")

            self.tracking_branch = tracking_branch

        self.branches = scan_result.branches
        self.last_scanned_at = datetime.now(UTC)
        self.commits = scan_result.all_commits
        self.tags = scan_result.all_tags


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
