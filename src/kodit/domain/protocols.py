"""Repository protocol interfaces for the domain layer."""

from abc import ABC, abstractmethod
from datetime import datetime
from pathlib import Path
from typing import Any, Protocol, TypeVar

from kodit.domain.enrichments.enrichment import EnrichmentAssociation, EnrichmentV2
from kodit.domain.entities import (
    Task,
    TaskStatus,
)
from kodit.domain.entities.git import (
    GitBranch,
    GitCommit,
    GitFile,
    GitRepo,
    GitTag,
    SnippetV2,
)
from kodit.domain.value_objects import (
    FusionRequest,
    FusionResult,
    MultiSearchRequest,
)
from kodit.infrastructure.sqlalchemy.query import Query


class GitAdapter(Protocol):
    """Protocol for Git repository operations."""

    async def clone_repository(self, remote_uri: str, local_path: Path) -> None:
        """Clone a repository to local path."""
        ...

    async def checkout_commit(self, local_path: Path, commit_sha: str) -> None:
        """Checkout a specific commit."""
        ...

    async def checkout_branch(self, local_path: Path, branch_name: str) -> None:
        """Checkout a specific branch."""
        ...

    async def fetch_repository(self, local_path: Path) -> None:
        """Fetch latest changes for existing repository."""
        ...

    async def pull_repository(self, local_path: Path) -> None:
        """Pull latest changes for existing repository."""
        ...

    async def get_all_branches(self, local_path: Path) -> list[dict[str, Any]]:
        """Get all branches in repository."""
        ...

    async def get_branch_commits(
        self, local_path: Path, branch_name: str
    ) -> list[dict[str, Any]]:
        """Get commit history for a specific branch."""
        ...

    async def get_all_commits_bulk(
        self, local_path: Path, since_date: datetime | None = None
    ) -> dict[str, dict[str, Any]]:
        """Get all commits from all branches in bulk for efficiency."""
        ...

    async def get_branch_commit_shas(
        self, local_path: Path, branch_name: str
    ) -> list[str]:
        """Get only commit SHAs for a branch."""
        ...

    async def get_all_branch_head_shas(
        self, local_path: Path, branch_names: list[str]
    ) -> dict[str, str]:
        """Get head commit SHAs for all branches in one operation."""
        ...

    async def get_commit_files(
        self, local_path: Path, commit_sha: str
    ) -> list[dict[str, Any]]:
        """Get all files in a specific commit from the git tree."""
        ...

    async def repository_exists(self, local_path: Path) -> bool:
        """Check if repository exists at local path."""
        ...

    async def get_commit_details(
        self, local_path: Path, commit_sha: str
    ) -> dict[str, Any]:
        """Get detailed information about a specific commit."""
        ...

    async def ensure_repository(self, remote_uri: str, local_path: Path) -> None:
        """Clone repository if it doesn't exist, otherwise pull latest changes."""
        ...

    async def get_file_content(
        self, local_path: Path, commit_sha: str, file_path: str
    ) -> bytes:
        """Get file content at specific commit."""
        ...

    async def get_default_branch(self, local_path: Path) -> str:
        """Get the default branch name with fallback strategies."""
        ...

    async def get_latest_commit_sha(
        self, local_path: Path, branch_name: str = "HEAD"
    ) -> str:
        """Get the latest commit SHA for a branch."""
        ...

    async def get_all_tags(self, local_path: Path) -> list[dict[str, Any]]:
        """Get all tags in repository."""
        ...

    async def get_commit_diff(self, local_path: Path, commit_sha: str) -> str:
        """Get the diff for a specific commit."""
        ...


T = TypeVar("T")


class Repository[T](Protocol):
    """Abstract base classes for repositories."""

    async def get(self, entity_id: Any) -> T:
        """Get entity by primary key."""
        ...

    async def get_or_create(self, entity: T, unique_field: str) -> tuple[T, bool]:
        """Get existing entity or create new one atomically.

        Handles race conditions by catching unique constraint violations.
        Returns (entity, created) where created is True if new.
        """
        ...

    async def find(self, query: Query) -> list[T]:
        """Find all entities matching query."""
        ...

    async def save(self, entity: T) -> T:
        """Save entity (create new or update existing)."""
        ...

    async def save_bulk(
        self, entities: list[T], *, skip_existence_check: bool = False
    ) -> list[T]:
        """Save multiple entities in bulk (create new or update existing)."""
        ...

    async def exists(self, entity_id: Any) -> bool:
        """Check if entity exists by primary key."""
        ...

    async def delete(self, entity: T) -> None:
        """Remove entity."""
        ...

    async def delete_by_query(self, query: Query) -> None:
        """Remove entities by query."""
        ...

    async def count(self, query: Query) -> int:
        """Count the number of entities matching query."""
        ...


class TaskRepository(Repository[Task], Protocol):
    """Repository interface for Task entities."""


class ReportingModule(Protocol):
    """Reporting module."""

    async def on_change(self, progress: TaskStatus) -> None:
        """On step changed."""
        ...


class TaskStatusRepository(Repository[TaskStatus]):
    """Repository interface for persisting progress state only."""

    @abstractmethod
    async def load_with_hierarchy(
        self, trackable_type: str, trackable_id: int
    ) -> list[TaskStatus]:
        """Load progress states with IDs and parent IDs from database."""

    @abstractmethod
    async def delete(self, entity: TaskStatus) -> None:
        """Delete a progress state."""


class GitCommitRepository(Repository[GitCommit]):
    """Repository for Git commits."""


class GitFileRepository(Repository[GitFile]):
    """Repository for Git files."""

    @abstractmethod
    async def delete_by_commit_sha(self, commit_sha: str) -> None:
        """Delete all files for a commit."""


class GitBranchRepository(Repository[GitBranch]):
    """Repository for Git branches."""

    @abstractmethod
    async def get_by_name(self, branch_name: str, repo_id: int) -> GitBranch:
        """Get a branch by name and repository ID."""

    @abstractmethod
    async def get_by_repo_id(self, repo_id: int) -> list[GitBranch]:
        """Get all branches for a repository."""

    @abstractmethod
    async def delete_by_repo_id(self, repo_id: int) -> None:
        """Delete all branches for a repository."""


class GitTagRepository(Repository[GitTag]):
    """Repository for Git tags."""

    @abstractmethod
    async def get_by_name(self, tag_name: str, repo_id: int) -> GitTag:
        """Get a tag by name and repository ID."""

    @abstractmethod
    async def get_by_repo_id(self, repo_id: int) -> list[GitTag]:
        """Get all tags for a repository."""

    @abstractmethod
    async def delete_by_repo_id(self, repo_id: int) -> None:
        """Delete all tags for a repository."""


class GitRepoRepository(Repository[GitRepo]):
    """Repository pattern for GitRepo aggregate."""


class SnippetRepositoryV2(ABC):
    """Repository for snippet operations."""

    @abstractmethod
    async def save_snippets(self, commit_sha: str, snippets: list[SnippetV2]) -> None:
        """Batch save snippets for a commit."""

    @abstractmethod
    async def get_snippets_for_commit(self, commit_sha: str) -> list[SnippetV2]:
        """Get all snippets for a specific commit."""

    @abstractmethod
    async def delete_snippets_for_commit(self, commit_sha: str) -> None:
        """Delete all snippet associations for a commit."""

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


class EnrichmentV2Repository(Repository[EnrichmentV2]):
    """Repository for enrichment operations."""


class EnrichmentAssociationRepository(Repository[EnrichmentAssociation]):
    """Repository for enrichment association operations."""
