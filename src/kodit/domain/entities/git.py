"""Git domain entities."""

from datetime import datetime
from enum import StrEnum
from pathlib import Path

from pydantic import AnyUrl, BaseModel

from kodit.domain.value_objects import SnippetContent, SnippetContentType
from kodit.utils.path_utils import repo_id_from_uri


class IndexStatus(StrEnum):
    """Status of commit indexing."""

    PENDING = "pending"
    IN_PROGRESS = "in_progress"
    COMPLETED = "completed"
    FAILED = "failed"


class GitFile(BaseModel):
    """File domain entity."""

    blob_sha: str  # Primary key
    path: str
    mime_type: str
    size: int

    def extension(self) -> str:
        """Return the file extension."""
        return Path(self.path).suffix.lstrip(".")


class GitCommit(BaseModel):
    """Commit domain entity."""

    commit_sha: str  # Primary key
    date: datetime
    message: str
    parent_commit_sha: str
    files: list[GitFile]
    author: str


class GitTag(BaseModel):
    """Git tag domain entity."""

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
    name: str
    head_commit: GitCommit


class GitRepo(BaseModel):
    """Repository domain entity."""

    id: int | None = None  # Database-generated surrogate key
    sanitized_remote_uri: AnyUrl  # Business key for lookups
    branches: list[GitBranch]
    commits: list[GitCommit]
    tags: list[GitTag] = []
    tracking_branch: GitBranch
    cloned_path: Path
    remote_uri: AnyUrl  # May include credentials
    last_scanned_at: datetime | None = None
    total_unique_commits: int = 0

    @property
    def business_key(self) -> str:
        """Get the business identifier for this repository."""
        return repo_id_from_uri(self.sanitized_remote_uri)

    @staticmethod
    def create_id(sanitized_remote_uri: AnyUrl) -> str:
        """Create a unique business key for a repository (kept for compatibility)."""
        return repo_id_from_uri(sanitized_remote_uri)


class CommitIndex(BaseModel):
    """Aggregate root for indexed commit data."""

    commit_sha: str  # Primary key
    snippets: list["SnippetV2"]
    status: IndexStatus
    indexed_at: datetime | None = None
    error_message: str | None = None
    files_processed: int = 0
    processing_time_seconds: float = 0.0

    def get_snippet_count(self) -> int:
        """Get total number of snippets."""
        return len(self.snippets)


class SnippetV2(BaseModel):
    """Snippet domain entity."""

    id: int | None = None  # Is populated by repository
    created_at: datetime | None = None  # Is populated by repository
    updated_at: datetime | None = None  # Is populated by repository
    derives_from: list[GitFile]
    original_content: SnippetContent | None = None
    summary_content: SnippetContent | None = None

    def original_text(self) -> str:
        """Return the original content of the snippet."""
        if self.original_content is None:
            return ""
        return self.original_content.value

    def summary_text(self) -> str:
        """Return the summary content of the snippet."""
        if self.summary_content is None:
            return ""
        return self.summary_content.value

    def add_original_content(self, content: str, language: str) -> None:
        """Add an original content to the snippet."""
        self.original_content = SnippetContent(
            type=SnippetContentType.ORIGINAL,
            value=content,
            language=language,
        )

    def add_summary(self, summary: str) -> None:
        """Add a summary to the snippet."""
        self.summary_content = SnippetContent(
            type=SnippetContentType.SUMMARY,
            value=summary,
            language="markdown",
        )
