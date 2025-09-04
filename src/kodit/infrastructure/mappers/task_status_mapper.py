"""Task status mapper."""

from kodit.domain import entities as domain_entities
from kodit.domain.value_objects import ReportingState, TrackableType
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
            step=task_status.step,
            created_at=task_status.created_at,
            updated_at=task_status.updated_at,
            trackable_id=task_status.trackable_id,
            trackable_type=(
                task_status.trackable_type.value
                if task_status.trackable_type
                else None
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
        )
    @staticmethod
    def to_domain_task_status(
        db_status: db_entities.TaskStatus,
    ) -> domain_entities.TaskStatus:
        """Convert database TaskStatus to domain TaskStatus."""
        return domain_entities.TaskStatus(
            id=db_status.id,
            step=db_status.step,
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
        )
