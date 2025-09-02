"""Operation mapper for converting between domain and database objects."""

from kodit.domain.value_objects import (
    OperationAggregate,
    OperationState,
    Step,
    StepState,
)
from kodit.infrastructure.sqlalchemy import entities as db_entities


class OperationMapper:
    """Maps between domain Operation objects and SQLAlchemy entities."""

    async def to_domain_operation(
        self,
        db_operation: db_entities.Operation,
        db_current_step: db_entities.Step | None = None,
    ) -> OperationAggregate:
        """Convert SQLAlchemy Operation to domain OperationAggregate."""
        current_step = None
        if db_current_step:
            current_step = self.to_domain_step(db_current_step)

        return OperationAggregate(
            index_id=db_operation.index_id,
            type=db_operation.type,
            state=OperationState(db_operation.state),
            updated_at=db_operation.updated_at,
            progress_percentage=db_operation.progress_percentage,
            error=None,  # Errors are not persisted in DB
            current_step=current_step,
        )

    async def from_domain_operation(
        self, operation: OperationAggregate
    ) -> db_entities.Operation:
        """Convert domain OperationAggregate to SQLAlchemy Operation."""
        return db_entities.Operation(
            index_id=operation.index_id,
            operation_type=operation.type,
            state=operation.state.value,
            progress_percentage=operation.progress_percentage,
        )

    def to_domain_step(self, db_step: db_entities.Step) -> Step:
        """Convert SQLAlchemy Step to domain Step."""
        return Step(
            name=db_step.name,
            state=StepState(db_step.state),
            updated_at=db_step.updated_at,
            progress_percentage=db_step.progress_percentage,
            error=None,  # Errors are not persisted in DB
        )

    async def from_domain_step(self, step: Step, operation_id: int) -> db_entities.Step:
        """Convert domain Step to SQLAlchemy Step."""
        return db_entities.Step(
            operation_id=operation_id,
            name=step.name,
            state=step.state.value,
            progress_percentage=step.progress_percentage,
        )
