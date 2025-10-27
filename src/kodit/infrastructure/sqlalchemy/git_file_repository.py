"""SQLAlchemy implementation of GitFileRepository."""

from collections.abc import Callable
from typing import Any

from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities.git import GitFile
from kodit.domain.protocols import GitFileRepository
from kodit.infrastructure.sqlalchemy import entities as db_entities
from kodit.infrastructure.sqlalchemy.query import FilterOperator, QueryBuilder
from kodit.infrastructure.sqlalchemy.repository import SqlAlchemyRepository


def create_git_file_repository(
    session_factory: Callable[[], AsyncSession],
) -> GitFileRepository:
    """Create a git file repository."""
    return SqlAlchemyGitFileRepository(session_factory=session_factory)


class SqlAlchemyGitFileRepository(
    SqlAlchemyRepository[GitFile, db_entities.GitCommitFile], GitFileRepository
):
    """SQLAlchemy implementation of GitFileRepository."""

    def __init__(self, session_factory: Callable[[], AsyncSession]) -> None:
        """Initialize the repository."""
        super().__init__(session_factory)

    @property
    def db_entity_type(self) -> type[db_entities.GitCommitFile]:
        """The SQLAlchemy model type."""
        return db_entities.GitCommitFile

    def _get_id(self, entity: GitFile) -> Any:
        """Extract ID from domain entity."""
        return (entity.commit_sha, entity.path)

    @staticmethod
    def to_domain(db_entity: db_entities.GitCommitFile) -> GitFile:
        """Map database entity to domain entity."""
        return GitFile(
            commit_sha=db_entity.commit_sha,
            created_at=db_entity.created_at,
            blob_sha=db_entity.blob_sha,
            path=db_entity.path,
            mime_type=db_entity.mime_type,
            size=db_entity.size,
            extension=db_entity.extension,
        )

    @staticmethod
    def to_db(domain_entity: GitFile) -> db_entities.GitCommitFile:
        """Map domain entity to database entity."""
        return db_entities.GitCommitFile(
            commit_sha=domain_entity.commit_sha,
            blob_sha=domain_entity.blob_sha,
            path=domain_entity.path,
            mime_type=domain_entity.mime_type,
            size=domain_entity.size,
            extension=domain_entity.extension,
            created_at=domain_entity.created_at,
        )

    async def delete_by_commit_sha(self, commit_sha: str) -> None:
        """Delete all files for a repository."""
        await self.delete_by_query(
            QueryBuilder().filter("commit_sha", FilterOperator.EQ, commit_sha)
        )
