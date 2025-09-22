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
    GitBranch,
    GitCommit,
    GitRepo,
    GitTag,
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

    async def next(self) -> Task | None:
        """Take a task for processing."""
        ...

    async def remove(self, task: Task) -> None:
        """Remove a task."""
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

    This is now a lightweight repository that handles only repository metadata,
    not the full graph of commits, branches, and tags.
    """

    @abstractmethod
    async def save(self, repo: GitRepo) -> GitRepo:
        """Save or update a repository (metadata only)."""

    @abstractmethod
    async def get_by_id(self, repo_id: int) -> GitRepo:
        """Get repository by ID (metadata only)."""

    @abstractmethod
    async def get_by_uri(self, sanitized_uri: AnyUrl) -> GitRepo:
        """Get repository by sanitized URI (metadata only)."""

    @abstractmethod
    async def get_all(self) -> list[GitRepo]:
        """Get all repositories (metadata only)."""

    @abstractmethod
    async def exists_by_uri(self, sanitized_uri: AnyUrl) -> bool:
        """Check if repository exists by URI."""

    @abstractmethod
    async def delete(self, sanitized_uri: AnyUrl) -> bool:
        """Delete a repository and all its associated data."""


class GitCommitRepository(ABC):
    """Repository pattern for GitCommit aggregate."""

    @abstractmethod
    async def save_commits_bulk(self, commits: list[GitCommit]) -> None:
        """Bulk save commits for efficiency."""

    @abstractmethod
    async def get_by_sha(self, commit_sha: str) -> GitCommit:
        """Get a specific commit by its SHA with files."""

    @abstractmethod
    async def get_commits_for_repo(
        self, repo_id: int, limit: int | None = None, offset: int = 0
    ) -> list[GitCommit]:
        """Get commits for a repository with optional pagination."""

    @abstractmethod
    async def get_commits_by_shas(self, commit_shas: list[str]) -> list[GitCommit]:
        """Get multiple commits by their SHAs."""

    @abstractmethod
    async def get_commit_count_for_repo(self, repo_id: int) -> int:
        """Get total count of commits for a repository."""

    @abstractmethod
    async def delete_commits_for_repo(self, repo_id: int) -> int:
        """Delete all commits for a repository, return count deleted."""

    @abstractmethod
    async def commit_exists(self, commit_sha: str) -> bool:
        """Check if a commit exists."""

    @abstractmethod
    async def get_repo_id_by_commit(self, commit_sha: str) -> int:
        """Get the repository ID that contains a specific commit."""


class GitBranchRepository(ABC):
    """Repository pattern for GitBranch aggregate."""

    @abstractmethod
    async def save_branches_bulk(self, branches: list[GitBranch]) -> None:
        """Bulk save branches for efficiency."""

    @abstractmethod
    async def get_branches_for_repo(self, repo_id: int) -> list[GitBranch]:
        """Get all branches for a repository."""

    @abstractmethod
    async def get_branch_by_name(self, repo_id: int, name: str) -> GitBranch:
        """Get a specific branch by repository ID and name."""

    @abstractmethod
    async def get_tracking_branch(self, repo_id: int) -> GitBranch | None:
        """Get the tracking branch for a repository (main/master)."""

    @abstractmethod
    async def set_tracking_branch(self, repo_id: int, branch_name: str) -> None:
        """Set the tracking branch for a repository."""

    @abstractmethod
    async def delete_branches_for_repo(self, repo_id: int) -> int:
        """Delete all branches for a repository, return count deleted."""


class GitTagRepository(ABC):
    """Repository pattern for GitTag aggregate."""

    @abstractmethod
    async def save_tags_bulk(self, tags: list[GitTag]) -> None:
        """Bulk save tags for efficiency."""

    @abstractmethod
    async def get_tags_for_repo(self, repo_id: int) -> list[GitTag]:
        """Get all tags for a repository."""

    @abstractmethod
    async def get_tag_by_name(self, repo_id: int, name: str) -> GitTag:
        """Get a specific tag by repository ID and name."""

    @abstractmethod
    async def get_version_tags_for_repo(self, repo_id: int) -> list[GitTag]:
        """Get only version tags for a repository."""

    @abstractmethod
    async def delete_tags_for_repo(self, repo_id: int) -> int:
        """Delete all tags for a repository, return count deleted."""


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

    @abstractmethod
    async def get_all_commits_bulk(self, local_path: Path) -> dict[str, dict[str, Any]]:
        """Get all commits from all branches in bulk for efficiency."""

    @abstractmethod
    async def get_branch_commit_shas(
        self, local_path: Path, branch_name: str
    ) -> list[str]:
        """Get only commit SHAs for a branch (much faster than full commit data)."""


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
