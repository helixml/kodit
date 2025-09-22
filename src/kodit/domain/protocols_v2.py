"""Repository protocol interfaces for lightweight Git aggregates.

This module contains new repository protocols that follow DDD principles
with proper aggregate boundaries for improved performance.
"""

from abc import ABC, abstractmethod

from pydantic import AnyUrl

from kodit.domain.entities.git_v2 import (
    GitBranchV2,
    GitCommitV2,
    GitRepositoryV2,
    GitTagV2,
)


class GitRepositoryRepositoryV2(ABC):
    """Repository pattern for GitRepositoryV2 aggregate.

    This repository handles only the basic repository metadata,
    not the full graph of commits, branches, and tags.
    """

    @abstractmethod
    async def save(self, repo: GitRepositoryV2) -> GitRepositoryV2:
        """Save or update a repository (metadata only)."""

    @abstractmethod
    async def get_by_id(self, repo_id: int) -> GitRepositoryV2:
        """Get repository by ID (metadata only)."""

    @abstractmethod
    async def get_by_uri(self, sanitized_uri: AnyUrl) -> GitRepositoryV2:
        """Get repository by sanitized URI (metadata only)."""

    @abstractmethod
    async def get_all(self) -> list[GitRepositoryV2]:
        """Get all repositories (metadata only)."""

    @abstractmethod
    async def exists_by_uri(self, sanitized_uri: AnyUrl) -> bool:
        """Check if repository exists by URI."""

    @abstractmethod
    async def delete(self, sanitized_uri: AnyUrl) -> bool:
        """Delete a repository and all its associated data."""


class GitCommitRepositoryV2(ABC):
    """Repository pattern for GitCommitV2 aggregate."""

    @abstractmethod
    async def save_commits_bulk(self, commits: list[GitCommitV2]) -> None:
        """Bulk save commits for efficiency."""

    @abstractmethod
    async def get_by_sha(self, commit_sha: str) -> GitCommitV2:
        """Get a specific commit by its SHA with files."""

    @abstractmethod
    async def get_commits_for_repo(
        self, repo_id: int, limit: int | None = None, offset: int = 0
    ) -> list[GitCommitV2]:
        """Get commits for a repository with optional pagination."""

    @abstractmethod
    async def get_commits_by_shas(self, commit_shas: list[str]) -> list[GitCommitV2]:
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


class GitBranchRepositoryV2(ABC):
    """Repository pattern for GitBranchV2 aggregate."""

    @abstractmethod
    async def save_branches_bulk(self, branches: list[GitBranchV2]) -> None:
        """Bulk save branches for efficiency."""

    @abstractmethod
    async def get_branches_for_repo(self, repo_id: int) -> list[GitBranchV2]:
        """Get all branches for a repository."""

    @abstractmethod
    async def get_branch_by_name(self, repo_id: int, name: str) -> GitBranchV2:
        """Get a specific branch by repository ID and name."""

    @abstractmethod
    async def get_tracking_branch(self, repo_id: int) -> GitBranchV2 | None:
        """Get the tracking branch for a repository (main/master)."""

    @abstractmethod
    async def set_tracking_branch(self, repo_id: int, branch_name: str) -> None:
        """Set the tracking branch for a repository."""

    @abstractmethod
    async def delete_branches_for_repo(self, repo_id: int) -> int:
        """Delete all branches for a repository, return count deleted."""


class GitTagRepositoryV2(ABC):
    """Repository pattern for GitTagV2 aggregate."""

    @abstractmethod
    async def save_tags_bulk(self, tags: list[GitTagV2]) -> None:
        """Bulk save tags for efficiency."""

    @abstractmethod
    async def get_tags_for_repo(self, repo_id: int) -> list[GitTagV2]:
        """Get all tags for a repository."""

    @abstractmethod
    async def get_tag_by_name(self, repo_id: int, name: str) -> GitTagV2:
        """Get a specific tag by repository ID and name."""

    @abstractmethod
    async def get_version_tags_for_repo(self, repo_id: int) -> list[GitTagV2]:
        """Get only version tags for a repository."""

    @abstractmethod
    async def delete_tags_for_repo(self, repo_id: int) -> int:
        """Delete all tags for a repository, return count deleted."""


class GitAggregateRepositoryV2(ABC):
    """Composite repository for operations that need multiple aggregates.

    This interface provides methods that require coordination between
    multiple Git aggregates while maintaining their boundaries.
    """

    @abstractmethod
    async def get_repository_with_tracking_branch(
        self, repo_id: int
    ) -> tuple[GitRepositoryV2, GitBranchV2 | None]:
        """Get repository with its tracking branch in a single operation."""

    @abstractmethod
    async def get_commit_with_repository(
        self, commit_sha: str
    ) -> tuple[GitCommitV2, GitRepositoryV2]:
        """Get commit with its repository metadata."""

    @abstractmethod
    async def scan_and_save_repository(
        self,
        repo: GitRepositoryV2,
        commits: list[GitCommitV2],
        branches: list[GitBranchV2],
        tags: list[GitTagV2],
        tracking_branch_name: str,
    ) -> GitRepositoryV2:
        """Save complete repository scan results across all aggregates."""
