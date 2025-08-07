"""Pure domain entities using Pydantic."""

import shutil
from dataclasses import dataclass
from datetime import datetime
from pathlib import Path
from typing import Any, Protocol
from urllib.parse import urlparse, urlunparse

from pydantic import AnyUrl, BaseModel

from kodit.domain.value_objects import (
    FileProcessingStatus,
    QueuePriority,
    SnippetContent,
    SnippetContentType,
    SourceType,
    TaskType,
)
from kodit.utils.path_utils import path_from_uri


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
    """Index domain entity."""

    id: int
    created_at: datetime
    updated_at: datetime
    source: Source
    snippets: list[Snippet]

    def delete_snippets_for_files(self, files: list[File]) -> None:
        """Delete the snippets that derive from a list of files."""
        self.snippets = [
            snippet
            for snippet in self.snippets
            if not any(file in snippet.derives_from for file in files)
        ]


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
    type: TaskType  # Task type
    priority: int  # Priority (higher number = higher priority)
    payload: dict[str, Any]  # Task-specific data

    created_at: datetime | None = None  # Is populated by repository
    updated_at: datetime | None = None  # Is populated by repository

    @staticmethod
    def create(task_type: TaskType, priority: int, payload: dict[str, Any]) -> "Task":
        """Create a task."""
        return Task(
            id=Task._create_id(task_type, payload),
            type=task_type,
            priority=priority,
            payload=payload,
        )

    @staticmethod
    def _create_id(task_type: TaskType, payload: dict[str, Any]) -> str:
        """Create a unique id for a task."""
        if task_type == TaskType.INDEX_UPDATE:
            return str(payload["index_id"])

        raise ValueError(f"Unknown task type: {task_type}")

    @staticmethod
    def create_index_update_task(
        index_id: int, priority: QueuePriority = QueuePriority.USER_INITIATED
    ) -> "Task":
        """Create an index update task."""
        return Task.create(
            task_type=TaskType.INDEX_UPDATE,
            priority=priority.value,
            payload={"index_id": index_id},
        )
