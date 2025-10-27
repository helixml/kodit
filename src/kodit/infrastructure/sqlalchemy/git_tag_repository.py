"""SQLAlchemy implementation of GitTagRepository."""

from collections.abc import Callable

from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities.git import GitTag
from kodit.domain.protocols import GitTagRepository
from kodit.infrastructure.sqlalchemy import entities as db_entities
from kodit.infrastructure.sqlalchemy.query import FilterOperator, QueryBuilder
from kodit.infrastructure.sqlalchemy.repository import SqlAlchemyRepository


def create_git_tag_repository(
    session_factory: Callable[[], AsyncSession],
) -> GitTagRepository:
    """Create a git tag repository."""
    return SqlAlchemyGitTagRepository(session_factory=session_factory)


class SqlAlchemyGitTagRepository(
    SqlAlchemyRepository[GitTag, db_entities.GitTag], GitTagRepository
):
    """SQLAlchemy implementation of GitTagRepository."""

    def _get_id(self, entity: GitTag) -> tuple[int, str]:
        """Get the ID of a tag."""
        if entity.repo_id is None:
            raise ValueError("Repository ID is required")
        return (entity.repo_id, entity.name)

    @property
    def db_entity_type(self) -> type[db_entities.GitTag]:
        """The SQLAlchemy model type."""
        return db_entities.GitTag

    @staticmethod
    def to_domain(db_entity: db_entities.GitTag) -> GitTag:
        """Map a SQLAlchemy GitTag to a domain GitTag."""
        return GitTag(
            created_at=db_entity.created_at,
            updated_at=db_entity.updated_at,
            repo_id=db_entity.repo_id,
            name=db_entity.name,
            target_commit_sha=db_entity.target_commit_sha,
        )

    @staticmethod
    def to_db(domain_entity: GitTag) -> db_entities.GitTag:
        """Map a domain GitTag to a SQLAlchemy GitTag."""
        if domain_entity.repo_id is None:
            raise ValueError("Repository ID is required")
        return db_entities.GitTag(
            repo_id=domain_entity.repo_id,
            name=domain_entity.name,
            target_commit_sha=domain_entity.target_commit_sha,
        )

    async def get_by_name(self, tag_name: str, repo_id: int) -> GitTag:
        """Get a tag by name and repository ID."""
        query = (
            QueryBuilder()
            .filter("name", FilterOperator.EQ, tag_name)
            .filter("repo_id", FilterOperator.EQ, repo_id)
        )
        tags = await self.find(query)
        if not tags:
            raise ValueError(f"Tag {tag_name} not found in repo {repo_id}")
        return tags[0]

    async def get_by_repo_id(self, repo_id: int) -> list[GitTag]:
        """Get all tags for a repository."""
        return await self.find(
            QueryBuilder().filter("repo_id", FilterOperator.EQ, repo_id)
        )

    async def delete_by_repo_id(self, repo_id: int) -> None:
        """Delete all tags for a repository."""
        await self.delete_by_query(
            QueryBuilder().filter("repo_id", FilterOperator.EQ, repo_id)
        )
