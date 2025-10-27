"""SQLAlchemy implementation of GitBranchRepository."""

from collections.abc import Callable

from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities.git import GitBranch
from kodit.domain.protocols import GitBranchRepository
from kodit.infrastructure.sqlalchemy import entities as db_entities
from kodit.infrastructure.sqlalchemy.query import FilterOperator, QueryBuilder
from kodit.infrastructure.sqlalchemy.repository import SqlAlchemyRepository


def create_git_branch_repository(
    session_factory: Callable[[], AsyncSession],
) -> GitBranchRepository:
    """Create a git branch repository."""
    return SqlAlchemyGitBranchRepository(session_factory=session_factory)


class SqlAlchemyGitBranchRepository(
    SqlAlchemyRepository[GitBranch, db_entities.GitBranch], GitBranchRepository
):
    """SQLAlchemy implementation of GitBranchRepository."""

    def __init__(self, session_factory: Callable[[], AsyncSession]) -> None:
        """Initialize the repository."""
        super().__init__(session_factory)

    def _get_id(self, entity: GitBranch) -> tuple[int, str]:
        """Get the ID of a branch."""
        if entity.repo_id is None:
            raise ValueError("Repository ID is required")
        return (entity.repo_id, entity.name)

    @property
    def db_entity_type(self) -> type[db_entities.GitBranch]:
        """Get the type of the database entity."""
        return db_entities.GitBranch

    @staticmethod
    def to_domain(db_entity: db_entities.GitBranch) -> GitBranch:
        """Map database entity to domain entity."""
        return GitBranch(
            repo_id=db_entity.repo_id,
            name=db_entity.name,
            head_commit_sha=db_entity.head_commit_sha,
            created_at=db_entity.created_at,
            updated_at=db_entity.updated_at,
        )

    @staticmethod
    def to_db(domain_entity: GitBranch) -> db_entities.GitBranch:
        """Map domain entity to database entity."""
        return db_entities.GitBranch(
            repo_id=domain_entity.repo_id,
            name=domain_entity.name,
            head_commit_sha=domain_entity.head_commit_sha,
        )

    async def get_by_name(self, branch_name: str, repo_id: int) -> GitBranch:
        """Get a branch by name and repository ID."""
        query = (
            QueryBuilder()
            .filter("name", FilterOperator.EQ, branch_name)
            .filter("repo_id", FilterOperator.EQ, repo_id)
        )
        branches = await self.find(query)
        if not branches:
            raise ValueError(f"Branch {branch_name} not found in repo {repo_id}")
        return branches[0]

    async def get_by_repo_id(self, repo_id: int) -> list[GitBranch]:
        """Get all branches for a repository."""
        query = QueryBuilder().filter("repo_id", FilterOperator.EQ, repo_id)
        return await self.find(query)

    async def delete_by_repo_id(self, repo_id: int) -> None:
        """Delete all branches for a repository."""
        await self.delete_by_query(
            QueryBuilder().filter("repo_id", FilterOperator.EQ, repo_id)
        )
