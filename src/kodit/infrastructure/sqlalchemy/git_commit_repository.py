"""SQLAlchemy implementation of GitCommitRepository."""

from collections.abc import Callable
from typing import Any

from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities.git import GitCommit
from kodit.domain.protocols import GitCommitRepository
from kodit.infrastructure.sqlalchemy import entities as db_entities
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

    @property
    def db_entity_type(self) -> type[db_entities.GitCommit]:
        """The SQLAlchemy model type."""
        return db_entities.GitCommit

    def _get_id(self, entity: GitCommit) -> Any:
        """Extract ID from domain entity."""
        return entity.commit_sha

    @staticmethod
    def to_domain(db_entity: db_entities.GitCommit) -> GitCommit:
        """Map database entity to domain entity."""
        return GitCommit(
            commit_sha=db_entity.commit_sha,
            repo_id=db_entity.repo_id,
            date=db_entity.date,
            message=db_entity.message,
            parent_commit_sha=db_entity.parent_commit_sha,
            author=db_entity.author,
        )

    @staticmethod
    def to_db(domain_entity: GitCommit) -> db_entities.GitCommit:
        """Map domain entity to database entity."""
        return db_entities.GitCommit(
            commit_sha=domain_entity.commit_sha,
            date=domain_entity.date,
            message=domain_entity.message,
            parent_commit_sha=domain_entity.parent_commit_sha,
            author=domain_entity.author,
            repo_id=domain_entity.repo_id,
        )
