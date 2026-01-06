"""Repository protocol interfaces for the domain layer."""

from abc import ABC, abstractmethod
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

T = TypeVar("T")


class Repository[T](Protocol):
    """Abstract base classes for repositories."""

    async def get(self, entity_id: Any) -> T:
        """Get entity by primary key."""
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

    async def next(self) -> Task | None:
        """Take a task for processing."""


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
