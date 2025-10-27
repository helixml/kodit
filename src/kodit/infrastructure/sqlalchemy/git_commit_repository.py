"""SQLAlchemy implementation of GitCommitRepository."""

from collections.abc import Callable
from typing import Any

from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities.git import GitCommit
from kodit.domain.protocols import GitCommitRepository
from kodit.infrastructure.sqlalchemy import entities as db_entities
from kodit.infrastructure.sqlalchemy.query import FilterOperator, QueryBuilder
from kodit.infrastructure.sqlalchemy.repository import SqlAlchemyRepository


def create_git_commit_repository(
    session_factory: Callable[[], AsyncSession],
) -> GitCommitRepository:
    """Create a git commit repository."""
    return SqlAlchemyGitCommitRepository(session_factory=session_factory)


class SqlAlchemyGitCommitRepository(
    SqlAlchemyRepository[GitCommit, db_entities.GitCommit], GitCommitRepository
):
    """SQLAlchemy implementation of GitCommitRepository."""

    def __init__(self, session_factory: Callable[[], AsyncSession]) -> None:
        """Initialize the repository."""
        super().__init__(session_factory)
        self.session_factory = session_factory

    @property
    def db_entity_type(self) -> type[db_entities.GitCommit]:
        """The SQLAlchemy model type."""
        return db_entities.GitCommit

    def _get_id(self, entity: GitCommit) -> Any:
        """Extract ID from domain entity."""
        return entity.commit_sha

    def to_domain(self, db_entity: db_entities.GitCommit) -> GitCommit:
        """Map database entity to domain entity."""
        # This is a placeholder - we need files, which requires a separate query
        # The actual conversion happens in get_by_sha and get_by_repo_id
        return GitCommit(
            commit_sha=db_entity.commit_sha,
            repo_id=db_entity.repo_id,
            date=db_entity.date,
            message=db_entity.message,
            parent_commit_sha=db_entity.parent_commit_sha,
            files=[],
            author=db_entity.author,
        )

    def to_db(self, domain_entity: GitCommit) -> db_entities.GitCommit:
        """Map domain entity to database entity."""
        return db_entities.GitCommit(
            commit_sha=domain_entity.commit_sha,
            date=domain_entity.date,
            message=domain_entity.message,
            parent_commit_sha=domain_entity.parent_commit_sha,
            author=domain_entity.author,
            repo_id=domain_entity.repo_id,
        )

    async def get_by_repo_id(self, repo_id: int) -> list[GitCommit]:
        """Get all commits for a repository without files.

        Files are lazy-loaded via the SQLAlchemy relationship, so they won't
        be queried unless explicitly accessed.
        """
        query = QueryBuilder().filter("repo_id", FilterOperator.EQ, repo_id)
        return await self.find(query)

    async def delete_by_repo_id(self, repo_id: int) -> None:
        """Delete all commits for a repository."""
        # Find all commits for this repo
        query = QueryBuilder().filter("repo_id", FilterOperator.EQ, repo_id)
        commits = await self.find(query)

        if not commits:
            return

        await self.delete_bulk(commits)

    async def count_by_repo_id(self, repo_id: int) -> int:
        """Count the number of commits for a repository."""
        query = QueryBuilder().filter("repo_id", FilterOperator.EQ, repo_id)
        return await self.count(query)
