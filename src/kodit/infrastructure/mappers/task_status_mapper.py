"""Task status mapper."""

from kodit.domain import entities as domain_entities
from kodit.domain.value_objects import ReportingState, TaskOperation, TrackableType
from kodit.infrastructure.sqlalchemy import entities as db_entities


class TaskStatusMapper:
    """Mapper for converting between domain TaskStatus and database entities."""

    @staticmethod
    def from_domain_task_status(
        task_status: domain_entities.TaskStatus,
    ) -> db_entities.TaskStatus:
        """Convert domain TaskStatus to database TaskStatus."""
        return db_entities.TaskStatus(
            id=task_status.id,
            operation=task_status.operation,
            created_at=task_status.created_at,
            updated_at=task_status.updated_at,
            trackable_id=task_status.trackable_id,
            trackable_type=(
                task_status.trackable_type.value if task_status.trackable_type else None
            ),
            parent=task_status.parent.id if task_status.parent else None,
            state=(
                task_status.state.value
                if isinstance(task_status.state, ReportingState)
                else task_status.state
            ),
            error=task_status.error,
            total=task_status.total,
            current=task_status.current,
            message=task_status.message,
        )

    @staticmethod
    def to_domain_task_status(
        db_status: db_entities.TaskStatus,
    ) -> domain_entities.TaskStatus:
        """Convert database TaskStatus to domain TaskStatus."""
        return domain_entities.TaskStatus(
            id=db_status.id,
            operation=TaskOperation(db_status.operation),
            state=ReportingState(db_status.state),
            created_at=db_status.created_at,
            updated_at=db_status.updated_at,
            trackable_id=db_status.trackable_id,
            trackable_type=(
                TrackableType(db_status.trackable_type)
                if db_status.trackable_type
                else None
            ),
            parent=None,  # Parent relationships need to be reconstructed separately
            error=db_status.error if db_status.error else None,
            total=db_status.total,
            current=db_status.current,
            message=db_status.message,
        )

    @staticmethod
    def to_domain_task_status_with_hierarchy(
        db_statuses: list[db_entities.TaskStatus],
    ) -> list[domain_entities.TaskStatus]:
        """Convert database TaskStatus list to domain with parent-child hierarchy.

        This method performs a two-pass conversion:
        1. First pass: Convert all DB entities to domain entities
        2. Second pass: Reconstruct parent-child relationships using ID mapping
        """
        # First pass: Convert all database entities to domain entities
        domain_statuses = [
            TaskStatusMapper.to_domain_task_status(db_status)
            for db_status in db_statuses
        ]

        # Create ID-to-entity mapping for efficient parent lookup
        id_to_entity = {status.id: status for status in domain_statuses}

        # Second pass: Reconstruct parent-child relationships
        for db_status, domain_status in zip(db_statuses, domain_statuses, strict=True):
            if db_status.parent and db_status.parent in id_to_entity:
                domain_status.parent = id_to_entity[db_status.parent]

        return domain_statuses
