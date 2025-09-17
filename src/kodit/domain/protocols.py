"""Repository protocol interfaces for the domain layer."""

from abc import ABC, abstractmethod
from pathlib import Path
from typing import Any, Protocol

from pydantic import AnyUrl

from kodit.domain.entities import (
    Task,
    TaskStatus,
)
from kodit.domain.entities.git import (
    CommitIndex,
    GitCommit,
    GitRepo,
    SnippetV2,
)
from kodit.domain.value_objects import (
    FusionRequest,
    FusionResult,
    MultiSearchRequest,
    TaskOperation,
)


class TaskRepository(Protocol):
    """Repository interface for Task entities."""

    async def add(
        self,
        task: Task,
    ) -> None:
        """Add a task."""
        ...

    async def get(self, task_id: str) -> Task | None:
        """Get a task by ID."""
        ...

    async def take(self) -> Task | None:
        """Take a task for processing."""
        ...

    async def update(self, task: Task) -> None:
        """Update a task."""
        ...

    async def list(self, task_operation: TaskOperation | None = None) -> list[Task]:
        """List tasks with optional status filter."""
        ...


class ReportingModule(Protocol):
    """Reporting module."""

    async def on_change(self, progress: TaskStatus) -> None:
        """On step changed."""
        ...


class TaskStatusRepository(Protocol):
    """Repository interface for persisting progress state only."""

    async def save(self, status: TaskStatus) -> None:
        """Save a progress state."""
        ...

    async def load_with_hierarchy(
        self, trackable_type: str, trackable_id: int
    ) -> list[TaskStatus]:
        """Load progress states with IDs and parent IDs from database."""
        ...

    async def delete(self, status: TaskStatus) -> None:
        """Delete a progress state."""
        ...


class GitRepoRepository(ABC):
    """Repository pattern for GitRepo aggregate.

    GitRepo is the aggregate root that owns branches, commits, and tags.
    This repository handles persistence of the entire aggregate.
    """

    @abstractmethod
    async def save(self, repo: GitRepo) -> GitRepo:
        """Save or update a repository with all its branches, commits, and tags.

        This method persists the entire aggregate:
        - The GitRepo entity itself
        - All associated branches
        - All associated commits
        - All associated tags
        """

    @abstractmethod
    async def get_by_id(self, repo_id: int) -> GitRepo:
        """Get repository by ID with all associated data."""

    @abstractmethod
    async def get_by_uri(self, sanitized_uri: AnyUrl) -> GitRepo:
        """Get repository by sanitized URI with all associated data."""

    @abstractmethod
    async def get_by_commit(self, commit_sha: str) -> GitRepo:
        """Get repository by commit SHA with all associated data."""

    @abstractmethod
    async def get_all(self) -> list[GitRepo]:
        """Get all repositories."""

    @abstractmethod
    async def delete(self, sanitized_uri: AnyUrl) -> bool:
        """Delete a repository."""

    @abstractmethod
    async def get_commit_by_sha(self, commit_sha: str) -> GitCommit:
        """Get a specific commit by its SHA across all repositories."""


class GitAdapter(ABC):
    """Abstract interface for Git operations."""

    @abstractmethod
    async def clone_repository(self, remote_uri: str, local_path: Path) -> None:
        """Clone a repository to local path."""

    @abstractmethod
    async def pull_repository(self, local_path: Path) -> None:
        """Pull latest changes for existing repository."""

    @abstractmethod
    async def get_all_branches(self, local_path: Path) -> list[dict[str, Any]]:
        """Get all branches in repository."""

    @abstractmethod
    async def get_branch_commits(
        self, local_path: Path, branch_name: str
    ) -> list[dict[str, Any]]:
        """Get commit history for a specific branch."""

    @abstractmethod
    async def get_commit_files(
        self, local_path: Path, commit_sha: str
    ) -> list[dict[str, Any]]:
        """Get all files in a specific commit."""

    @abstractmethod
    async def repository_exists(self, local_path: Path) -> bool:
        """Check if repository exists at local path."""

    @abstractmethod
    async def get_commit_details(
        self, local_path: Path, commit_sha: str
    ) -> dict[str, Any]:
        """Get details of a specific commit."""

    @abstractmethod
    async def ensure_repository(self, remote_uri: str, local_path: Path) -> None:
        """Ensure repository exists at local path."""

    @abstractmethod
    async def get_file_content(
        self, local_path: Path, commit_sha: str, file_path: str
    ) -> bytes:
        """Get file content at specific commit."""

    @abstractmethod
    async def get_latest_commit_sha(
        self, local_path: Path, branch_name: str = "HEAD"
    ) -> str:
        """Get the latest commit SHA for a branch."""

    @abstractmethod
    async def get_all_tags(self, local_path: Path) -> list[dict[str, Any]]:
        """Get all tags in repository."""


class CommitIndexRepository(ABC):
    """Repository for commit indexing operations."""

    @abstractmethod
    async def save(self, commit_index: CommitIndex) -> None:
        """Save or update a commit index."""

    @abstractmethod
    async def get_by_commit(self, commit_sha: str) -> CommitIndex | None:
        """Get index data for a specific commit."""

    @abstractmethod
    async def get_indexed_commits_for_repo(self, repo_uri: str) -> list[CommitIndex]:
        """Get all indexed commits for a repository."""

    @abstractmethod
    async def delete(self, commit_sha: str) -> bool:
        """Delete index data for a commit."""


class SnippetRepositoryV2(ABC):
    """Repository for snippet operations."""

    @abstractmethod
    async def save_snippets(self, commit_sha: str, snippets: list[SnippetV2]) -> None:
        """Batch save snippets for a commit."""

    @abstractmethod
    async def get_snippets_for_commit(self, commit_sha: str) -> list[SnippetV2]:
        """Get all snippets for a specific commit."""

    @abstractmethod
    async def search(self, request: MultiSearchRequest) -> list[SnippetV2]:
        """Search snippets with filters."""

    @abstractmethod
    async def get_by_ids(self, ids: list[str]) -> list[SnippetV2]:
        """Get snippets by their IDs."""


class FusionService(ABC):
    """Abstract fusion service interface."""

    @abstractmethod
    def reciprocal_rank_fusion(
        self, rankings: list[list[FusionRequest]], k: float = 60
    ) -> list[FusionResult]:
        """Perform reciprocal rank fusion on search results."""
