"""Pure domain entities using Pydantic."""

import shutil
from dataclasses import dataclass
from datetime import UTC, datetime
from enum import StrEnum
from pathlib import Path
from typing import Any, Protocol
from urllib.parse import urlparse, urlunparse

from pydantic import AnyUrl, BaseModel

from kodit.domain.value_objects import (
    FileProcessingStatus,
    ReportingState,
    SnippetContent,
    SnippetContentType,
    SourceType,
    TaskOperation,
    TrackableType,
)
from kodit.utils.path_utils import path_from_uri, repo_id_from_uri


class IgnorePatternProvider(Protocol):
    """Protocol for ignore pattern providers."""

    def should_ignore(self, path: Path) -> bool:
        """Check if a path should be ignored."""
        ...


class Author(BaseModel):
    """Author domain entity."""

    id: int | None = None
    name: str
    email: str


class File(BaseModel):
    """File domain entity."""

    id: int | None = None  # Is populated by repository
    created_at: datetime | None = None  # Is populated by repository
    updated_at: datetime | None = None  # Is populated by repository
    uri: AnyUrl
    sha256: str
    authors: list[Author]
    mime_type: str
    file_processing_status: FileProcessingStatus

    def as_path(self) -> Path:
        """Return the file as a path."""
        return path_from_uri(str(self.uri))

    def extension(self) -> str:
        """Return the file extension."""
        return Path(self.as_path()).suffix.lstrip(".")


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

    id: str  # Primary key
    sanitized_remote_uri: AnyUrl
    branches: list[GitBranch]
    commits: list[GitCommit]
    tags: list[GitTag] = []
    tracking_branch: GitBranch
    cloned_path: Path
    remote_uri: AnyUrl  # May include credentials
    last_scanned_at: datetime | None = None
    total_unique_commits: int = 0

    @staticmethod
    def create_id(sanitized_remote_uri: AnyUrl) -> str:
        """Create a unique id for a repository."""
        return repo_id_from_uri(sanitized_remote_uri)


class IndexStatus(StrEnum):
    """Status of commit indexing."""

    PENDING = "pending"
    IN_PROGRESS = "in_progress"
    COMPLETED = "completed"
    FAILED = "failed"


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


class WorkingCopy(BaseModel):
    """Working copy value object representing cloned source location."""

    created_at: datetime | None = None  # Is populated by repository
    updated_at: datetime | None = None  # Is populated by repository
    remote_uri: AnyUrl
    cloned_path: Path
    source_type: SourceType
    files: list[File]

    @classmethod
    def sanitize_local_path(cls, path: str) -> AnyUrl:
        """Sanitize a local path."""
        return AnyUrl(Path(path).resolve().absolute().as_uri())

    @classmethod
    def sanitize_git_url(cls, url: str) -> AnyUrl:
        """Remove credentials from a git URL while preserving the rest of the URL.

        This function handles various git URL formats:
        - HTTPS URLs with username:password@host
        - HTTPS URLs with username@host (no password)
        - SSH URLs (left unchanged)
        - File URLs (left unchanged)

        Args:
            url: The git URL that may contain credentials.

        Returns:
            The sanitized URL with credentials removed.

        Examples:
            >>> sanitize_git_url("https://phil:token@dev.azure.com/org/project/_git/repo")
            "https://dev.azure.com/org/project/_git/repo"
            >>> sanitize_git_url("https://username@github.com/user/repo.git")
            "https://github.com/user/repo.git"
            >>> sanitize_git_url("git@github.com:user/repo.git")
            "ssh://git@github.com/user/repo.git"

        """
        # Handle SSH URLs (they don't have credentials in the URL format)
        if url.startswith("git@"):
            # Convert git@host:path to ssh://git@host/path format for AnyUrl
            # This maintains the same semantic meaning while making it a valid URL
            if ":" in url and not url.startswith("ssh://"):
                host_path = url[4:]  # Remove "git@"
                if ":" in host_path:
                    host, path = host_path.split(":", 1)
                    ssh_url = f"ssh://git@{host}/{path}"
                    return AnyUrl(ssh_url)
            return AnyUrl(url)
        if url.startswith("ssh://"):
            return AnyUrl(url)

        # Handle file URLs
        if url.startswith("file://"):
            return AnyUrl(url)

        # Handle local paths by converting to file:// URLs
        from pathlib import Path

        try:
            path = Path(url)
            if path.exists() or url.startswith(("/", "./", "../")) or url == ".":
                # Convert local path to file:// URL
                absolute_path = path.resolve()
                file_url = f"file://{absolute_path}"
                return AnyUrl(file_url)
        except Exception:
            # If path conversion fails, continue with URL parsing
            pass

        try:
            # Parse the URL
            parsed = urlparse(url)

            # If there are no credentials, return the URL as-is
            if not parsed.username:
                return AnyUrl(url)

            # Reconstruct the URL without credentials
            # scheme, netloc (without username/password), path, params, query, fragment
            sanitized_netloc = parsed.hostname
            if parsed.port:
                sanitized_netloc = f"{parsed.hostname}:{parsed.port}"

            return AnyUrl(
                urlunparse(
                    (
                        parsed.scheme,
                        sanitized_netloc,
                        parsed.path,
                        parsed.params,
                        parsed.query,
                        parsed.fragment,
                    )
                )
            )

        except Exception as e:
            raise ValueError(f"Invalid URL: {url}") from e

    def modified_or_deleted_files(self) -> list[File]:
        """Return the modified or deleted files."""
        return [
            file
            for file in self.files
            if file.file_processing_status
            in (FileProcessingStatus.MODIFIED, FileProcessingStatus.DELETED)
        ]

    def list_filesystem_paths(
        self, ignore_provider: IgnorePatternProvider
    ) -> list[Path]:
        """List the filesystem paths of the files in the working copy."""
        if not self.cloned_path.exists():
            raise ValueError(f"Cloned path does not exist: {self.cloned_path}")

        return [
            f
            for f in self.cloned_path.rglob("*")
            if f.is_file() and not ignore_provider.should_ignore(f)
        ]

    def dirty_files(self) -> list[File]:
        """Return the dirty files."""
        return [
            file
            for file in self.files
            if file.file_processing_status
            in (FileProcessingStatus.MODIFIED, FileProcessingStatus.ADDED)
        ]

    def changed_files(self) -> list[File]:
        """Return the changed files."""
        return [
            file
            for file in self.files
            if file.file_processing_status != FileProcessingStatus.CLEAN
        ]

    def clear_file_processing_statuses(self) -> None:
        """Clear the file processing statuses."""
        # First remove any files that are marked for deletion
        self.files = [
            file
            for file in self.files
            if file.file_processing_status != FileProcessingStatus.DELETED
        ]
        # Then clear the statuses for the remaining files
        for file in self.files:
            file.file_processing_status = FileProcessingStatus.CLEAN

    def delete(self) -> None:
        """Delete the working copy."""
        shutil.rmtree(self.cloned_path)


class Source(BaseModel):
    """Source domain entity."""

    id: int | None = None  # Is populated by repository
    created_at: datetime | None = None  # Is populated by repository
    updated_at: datetime | None = None  # Is populated by repository
    working_copy: WorkingCopy


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


class Snippet(BaseModel):
    """Snippet domain entity."""

    id: int | None = None  # Is populated by repository
    created_at: datetime | None = None  # Is populated by repository
    updated_at: datetime | None = None  # Is populated by repository
    derives_from: list[File]
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


class Index(BaseModel):
    """Index domain entity.

    The snippets field is optional and only loaded when explicitly needed.
    Most operations should use the SnippetRepository directly instead of
    accessing this field.
    """

    id: int
    created_at: datetime
    updated_at: datetime
    source: Source
    snippets: list[Snippet] = []

    def delete_snippets_for_files(self, files: list[File]) -> None:
        """Delete the snippets that derive from a list of files.

        Note: This method only operates on loaded snippets. For persistence,
        use SnippetRepository.delete_by_file_ids() directly.
        """
        self.snippets = [
            snippet
            for snippet in self.snippets
            if not any(file in snippet.derives_from for file in files)
        ]

    def has_snippets_loaded(self) -> bool:
        """Check if snippets have been loaded into this entity."""
        return len(self.snippets) > 0


# FUTURE: Remove this type, use the domain to get the required information.
@dataclass(frozen=True)
class SnippetWithContext:
    """Domain model for snippet with associated context information."""

    source: Source
    file: File
    authors: list[Author]
    snippet: Snippet


class Task(BaseModel):
    """Represents an item in the queue waiting to be processed.

    If the item exists, that means it is in the queue and waiting to be processed. There
    is no status associated.
    """

    id: str  # Is a unique key to deduplicate items in the queue
    type: TaskOperation  # Task operation
    priority: int  # Priority (higher number = higher priority)
    payload: dict[str, Any]  # Task-specific data

    created_at: datetime | None = None  # Is populated by repository
    updated_at: datetime | None = None  # Is populated by repository

    @staticmethod
    def create(
        operation: TaskOperation, priority: int, payload: dict[str, Any]
    ) -> "Task":
        """Create a task."""
        return Task(
            id=Task.create_id(operation, payload),
            type=operation,
            priority=priority,
            payload=payload,
        )

    @staticmethod
    def create_id(operation: TaskOperation, payload: dict[str, Any]) -> str:
        """Create a unique id for a task."""
        if operation in (
            TaskOperation.REFRESH_WORKING_COPY,
            TaskOperation.EXTRACT_SNIPPETS,
            TaskOperation.CREATE_BM25_INDEX,
            TaskOperation.CREATE_CODE_EMBEDDINGS,
            TaskOperation.ENRICH_SNIPPETS,
        ):
            # Use a shortened name for the ID to keep it concise
            operation_short_names = {
                TaskOperation.REFRESH_WORKING_COPY: "SYNC",
                TaskOperation.EXTRACT_SNIPPETS: "EXTRACT",
                TaskOperation.CREATE_BM25_INDEX: "BM25_INDEX",
                TaskOperation.CREATE_CODE_EMBEDDINGS: "CODE_EMBEDDINGS",
                TaskOperation.ENRICH_SNIPPETS: "ENRICH",
            }
            return f"{operation_short_names[operation]}:{payload['index_id']}"

        raise ValueError(f"Unknown operation: {operation}")


class TaskStatus(BaseModel):
    """Task status domain entity."""

    id: str
    state: ReportingState
    operation: TaskOperation
    message: str = ""

    created_at: datetime = datetime.now(UTC)
    updated_at: datetime = datetime.now(UTC)
    total: int = 0
    current: int = 0

    error: str | None = None
    parent: "TaskStatus | None" = None
    trackable_id: int | None = None
    trackable_type: TrackableType | None = None

    @staticmethod
    def create(
        operation: TaskOperation,
        parent: "TaskStatus | None" = None,
        trackable_type: TrackableType | None = None,
        trackable_id: int | None = None,
    ) -> "TaskStatus":
        """Create a task status."""
        return TaskStatus(
            id=TaskStatus._create_id(operation, trackable_type, trackable_id),
            operation=operation,
            parent=parent,
            trackable_type=trackable_type,
            trackable_id=trackable_id,
            state=ReportingState.STARTED,
        )

    @staticmethod
    def _create_id(
        step: TaskOperation,
        trackable_type: TrackableType | None = None,
        trackable_id: int | None = None,
    ) -> str:
        """Create a unique id for a task."""
        result = []
        # Nice to be prefixed by tracking information if it exists
        if trackable_type:
            result.append(str(trackable_type))
        if trackable_id:
            result.append(str(trackable_id))
        result.append(str(step))
        return "-".join(result)

    @property
    def completion_percent(self) -> float:
        """Calculate the percentage of completion."""
        if self.total == 0:
            return 0.0
        return min(100.0, max(0.0, (self.current / self.total) * 100.0))

    def skip(self, message: str) -> None:
        """Skip the task."""
        self.state = ReportingState.SKIPPED
        self.message = message

    def fail(self, error: str) -> None:
        """Fail the task."""
        self.state = ReportingState.FAILED
        self.error = error

    def set_total(self, total: int) -> None:
        """Set the total for the step."""
        self.total = total

    def set_current(self, current: int, message: str | None = None) -> None:
        """Progress the step."""
        self.state = ReportingState.IN_PROGRESS
        self.current = current
        if message:
            self.message = message

    def set_tracking_info(
        self, trackable_id: int, trackable_type: TrackableType
    ) -> None:
        """Set the tracking info."""
        self.trackable_id = trackable_id
        self.trackable_type = trackable_type

    def complete(self) -> None:
        """Complete the task."""
        if ReportingState.is_terminal(self.state):
            return  # Already in terminal state

        self.state = ReportingState.COMPLETED
        self.current = self.total  # Ensure progress shows 100%
