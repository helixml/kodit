"""Task mapper for the task queue."""

from kodit.domain.entities import Task
from kodit.domain.value_objects import TaskOperation
from kodit.infrastructure.sqlalchemy import entities as db_entities


class TaskMapper:
    """Maps between domain QueuedTask and SQLAlchemy Task entities.

    This mapper handles the conversion between the existing domain and
    persistence layers without creating any new entities.
    """

    @staticmethod
    def to_domain_task(record: db_entities.Task) -> Task:
        """Convert SQLAlchemy Task record to domain QueuedTask.

        Since QueuedTask doesn't have status fields, we store processing
        state in the payload.
        """
        if record.type not in TaskOperation.__members__.values():
            raise ValueError(f"Unknown operation: {record.type}")
        # The dedup_key becomes the id in the domain entity
        return Task(
            id=record.dedup_key,  # Use dedup_key as the unique identifier
            type=TaskOperation(record.type),
            priority=record.priority,
            payload=record.payload or {},
            created_at=record.created_at,
            updated_at=record.updated_at,
        )

    @staticmethod
    def from_domain_task(task: Task) -> db_entities.Task:
        """Convert domain QueuedTask to SQLAlchemy Task record."""
        return db_entities.Task(
            dedup_key=task.id,
            type=task.type.value,
            payload=task.payload,
            priority=task.priority,
        )
