"""Task repository for the task queue."""

from collections.abc import Callable
from typing import Any

import structlog
from sqlalchemy.ext.asyncio import AsyncSession

import kodit.domain.entities as domain_entities
from kodit.domain.entities import Task
from kodit.domain.protocols import TaskRepository
from kodit.domain.value_objects import TaskOperation
from kodit.infrastructure.sqlalchemy import entities as db_entities
from kodit.infrastructure.sqlalchemy.repository import SqlAlchemyRepository


def create_task_repository(
    session_factory: Callable[[], AsyncSession],
) -> TaskRepository:
    """Create an index repository."""
    return SqlAlchemyTaskRepository(session_factory=session_factory)


class SqlAlchemyTaskRepository(
    SqlAlchemyRepository[domain_entities.Task, db_entities.Task], TaskRepository
):
    """Repository for task persistence using the existing Task entity."""

    def __init__(self, session_factory: Callable[[], AsyncSession]) -> None:
        """Initialize the repository."""
        super().__init__(session_factory)
        self.log = structlog.get_logger(__name__)

    @property
    def db_entity_type(self) -> type[db_entities.Task]:
        """The SQLAlchemy model type."""
        return db_entities.Task

    def _get_id(self, entity: domain_entities.Task) -> Any:
        """Extract ID from domain entity."""
        return entity.id

    @staticmethod
    def to_domain(db_entity: db_entities.Task) -> domain_entities.Task:
        """Map database entity to domain entity."""
        # The dedup_key becomes the id in the domain entity
        return Task(
            id=db_entity.id,
            dedup_key=db_entity.dedup_key,
            type=TaskOperation(db_entity.type),
            priority=db_entity.priority,
            payload=db_entity.payload or {},
            created_at=db_entity.created_at,
            updated_at=db_entity.updated_at,
        )

    @staticmethod
    def to_db(domain_entity: domain_entities.Task) -> db_entities.Task:
        """Map domain entity to database entity."""
        return db_entities.Task(
            dedup_key=domain_entity.dedup_key,
            type=domain_entity.type.value,
            payload=domain_entity.payload,
            priority=domain_entity.priority,
        )
