"""Task mapper for the task queue."""

from typing import ClassVar

from kodit.domain.entities import Task
from kodit.domain.value_objects import TaskType
from kodit.infrastructure.sqlalchemy import entities as db_entities


class TaskTypeMapper:
    """Maps between domain QueuedTaskType and SQLAlchemy TaskType."""

    # Map TaskType enum to QueuedTaskType
    TASK_TYPE_MAPPING: ClassVar[dict[db_entities.TaskType, TaskType]] = {
        db_entities.TaskType.INDEX_UPDATE: TaskType.INDEX_UPDATE,
    }

    @staticmethod
    def to_domain_type(task_type: db_entities.TaskType) -> TaskType:
        """Convert SQLAlchemy TaskType to domain QueuedTaskType."""
        if task_type not in TaskTypeMapper.TASK_TYPE_MAPPING:
            raise ValueError(f"Unknown task type: {task_type}")
        return TaskTypeMapper.TASK_TYPE_MAPPING[task_type]

    @staticmethod
    def from_domain_type(task_type: TaskType) -> db_entities.TaskType:
        """Convert domain QueuedTaskType to SQLAlchemy TaskType."""
        if task_type not in TaskTypeMapper.TASK_TYPE_MAPPING.values():
            raise ValueError(f"Unknown task type: {task_type}")

        # Find value in TASK_TYPE_MAPPING
        return next(
            (
                task_type
                for task_type, value in TaskTypeMapper.TASK_TYPE_MAPPING.items()
                if value == task_type
            )
        )


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
        # Get the task type
        if record.type not in TaskMapper.TASK_TYPE_MAPPING:
            raise ValueError(f"Unknown task type: {record.type}")
        task_type = TaskMapper.TASK_TYPE_MAPPING[record.type]

        # The dedup_key becomes the id in the domain entity
        return Task(
            id=record.dedup_key,  # Use dedup_key as the unique identifier
            type=task_type,
            priority=record.priority,
            payload=record.payload or {},
            created_at=record.created_at,
            updated_at=record.updated_at,
        )

    @staticmethod
    def from_domain_task(task: Task) -> db_entities.Task:
        """Convert domain QueuedTask to SQLAlchemy Task record."""
        if task.type not in TaskMapper.TASK_TYPE_MAPPING.values():
            raise ValueError(f"Unknown task type: {task.type}")

        # Find value in TASK_TYPE_MAPPING
        task_type = next(
            (
                db_entities.TaskType(task_type)
                for task_type, value in TaskMapper.TASK_TYPE_MAPPING.items()
                if value == task.type
            ),
        )
        return db_entities.Task(
            dedup_key=task.id,
            type=task_type,
            payload=task.payload,
            priority=task.priority,
        )
