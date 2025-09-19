"""Pure domain entities using Pydantic."""

from datetime import UTC, datetime
from pathlib import Path
from typing import Any, Protocol
from urllib.parse import urlparse, urlunparse

from pydantic import AnyUrl, BaseModel

from kodit.domain.value_objects import (
    ReportingState,
    TaskOperation,
    TrackableType,
)


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


class WorkingCopy(BaseModel):
    """Working copy value object representing cloned source location."""

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
        # Handle different URL types
        if not url:
            raise ValueError("URL is required")

        if url.startswith("git@"):
            return cls._handle_ssh_url(url)
        if url.startswith("ssh://"):
            return AnyUrl(url)
        if url.startswith("file://"):
            return AnyUrl(url)

        # Try local path conversion
        local_url = cls._try_local_path_conversion(url)
        if local_url:
            return local_url

        # Handle HTTPS URLs with credentials
        return cls._sanitize_https_url(url)

    @classmethod
    def _handle_ssh_url(cls, url: str) -> AnyUrl:
        """Handle SSH URL conversion."""
        if ":" in url and not url.startswith("ssh://"):
            host_path = url[4:]  # Remove "git@"
            if ":" in host_path:
                host, path = host_path.split(":", 1)
                return AnyUrl(f"ssh://git@{host}/{path}")
        return AnyUrl(url)

    @classmethod
    def _try_local_path_conversion(cls, url: str) -> AnyUrl | None:
        """Try to convert local paths to file:// URLs."""
        from pathlib import Path

        try:
            path = Path(url)
            if path.exists() or url.startswith(("/", "./", "../")) or url == ".":
                absolute_path = path.resolve()
                return AnyUrl(f"file://{absolute_path}")
        except OSError:
            # Path operations failed, not a local path
            pass
        return None

    @classmethod
    def _sanitize_https_url(cls, url: str) -> AnyUrl:
        """Remove credentials from HTTPS URLs."""
        try:
            parsed = urlparse(url)

            # If there are no credentials, return the URL as-is
            if not parsed.username:
                return AnyUrl(url)

            # Reconstruct the URL without credentials
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


class Source(BaseModel):
    """Source domain entity."""

    id: int | None = None  # Is populated by repository
    created_at: datetime | None = None  # Is populated by repository
    updated_at: datetime | None = None  # Is populated by repository
    working_copy: WorkingCopy


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
        first_id = next(iter(payload.values()), None)
        return f"{operation}:{first_id}"


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
