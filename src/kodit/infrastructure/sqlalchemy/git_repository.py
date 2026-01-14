"""SQLAlchemy implementation of GitRepoRepository."""

from collections.abc import Callable
from typing import Any

from pydantic import AnyUrl

from kodit.domain.entities.git import GitRepo, TrackingConfig, TrackingType
from kodit.domain.protocols import GitRepoRepository
from kodit.infrastructure.sqlalchemy import entities as db_entities
from kodit.infrastructure.sqlalchemy.repository import SqlAlchemyRepository


def create_git_repo_repository(
    session_factory: Callable[[], Any],
) -> GitRepoRepository:
    """Create a git repository."""
    return SqlAlchemyGitRepoRepository(session_factory=session_factory)


class SqlAlchemyGitRepoRepository(
    SqlAlchemyRepository[GitRepo, db_entities.GitRepo], GitRepoRepository
):
    """SQLAlchemy implementation of GitRepoRepository."""

    def _get_id(self, entity: GitRepo) -> Any:
        return entity.id

    @property
    def db_entity_type(self) -> type[db_entities.GitRepo]:
        """The SQLAlchemy model type."""
        return db_entities.GitRepo

    @staticmethod
    def to_domain(db_entity: db_entities.GitRepo) -> GitRepo:
        """Map database entity to domain entity."""
        tracking_config = None
        if db_entity.tracking_type:
            tracking_config = TrackingConfig(
                type=TrackingType(db_entity.tracking_type),
                name=db_entity.tracking_name or "",
            )

        return GitRepo(
            id=db_entity.id,
            created_at=db_entity.created_at,
            updated_at=db_entity.updated_at,
            sanitized_remote_uri=AnyUrl(db_entity.sanitized_remote_uri),
            remote_uri=AnyUrl(db_entity.remote_uri),
            cloned_path=db_entity.cloned_path,
            last_scanned_at=db_entity.last_scanned_at,
            num_commits=db_entity.num_commits,
            num_branches=db_entity.num_branches,
            num_tags=db_entity.num_tags,
            tracking_config=tracking_config,
        )

    @staticmethod
    def to_db(domain_entity: GitRepo) -> db_entities.GitRepo:
        """Map domain entity to database entity."""
        tracking_type = ""
        tracking_name = ""
        if domain_entity.tracking_config:
            tracking_type = domain_entity.tracking_config.type
            tracking_name = domain_entity.tracking_config.name

        return db_entities.GitRepo(
            sanitized_remote_uri=str(domain_entity.sanitized_remote_uri),
            remote_uri=str(domain_entity.remote_uri),
            cloned_path=domain_entity.cloned_path,
            last_scanned_at=domain_entity.last_scanned_at,
            num_commits=domain_entity.num_commits,
            num_branches=domain_entity.num_branches,
            num_tags=domain_entity.num_tags,
            tracking_type=tracking_type,
            tracking_name=tracking_name,
        )
