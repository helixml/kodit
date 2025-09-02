"""SQLAlchemy implementation of OperationRepository."""

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.protocols import OperationRepository
from kodit.domain.value_objects import OperationAggregate
from kodit.infrastructure.mappers.operation_mapper import OperationMapper
from kodit.infrastructure.sqlalchemy import entities as db_entities


class SqlAlchemyOperationRepository(OperationRepository):
    """SQLAlchemy implementation of OperationRepository."""

    def __init__(self, session: AsyncSession) -> None:
        """Initialize the repository."""
        self._session = session
        self._mapper = OperationMapper()

    async def get_by_index_id(self, index_id: int) -> OperationAggregate:
        """Get an operation by index ID. Raises exception if not found."""
        stmt = select(db_entities.Operation).where(
            db_entities.Operation.index_id == index_id
        )
        db_operation = await self._session.scalar(stmt)

        if not db_operation:
            raise ValueError(f"Operation with index_id {index_id} not found")

        # Get current step if exists
        step_stmt = select(db_entities.Step).where(
            db_entities.Step.operation_id == db_operation.id
        ).order_by(db_entities.Step.updated_at.desc()).limit(1)
        current_step = await self._session.scalar(step_stmt)

        return await self._mapper.to_domain_operation(db_operation, current_step)

    async def save(self, operation: OperationAggregate) -> None:
        """Save an operation."""
        # Check if operation exists
        stmt = select(db_entities.Operation).where(
            db_entities.Operation.index_id == operation.index_id,
            db_entities.Operation.type == operation.type
        )
        db_operation = await self._session.scalar(stmt)

        if db_operation:
            # Update existing operation
            db_operation.state = operation.state.value
            db_operation.progress_percentage = self._calculate_progress(operation)
            db_operation.updated_at = operation.updated_at
        else:
            # Create new operation
            db_operation = await self._mapper.from_domain_operation(operation)
            self._session.add(db_operation)
            await self._session.flush()  # Get operation ID

        # Handle current step
        if operation.current_step:
            # Check if this step already exists
            step_stmt = select(db_entities.Step).where(
                db_entities.Step.operation_id == db_operation.id,
                db_entities.Step.name == operation.current_step.name
            )
            db_step = await self._session.scalar(step_stmt)

            if db_step:
                # Update existing step
                db_step.state = operation.current_step.state.value
                db_step.progress_percentage = operation.current_step.progress_percentage
                db_step.updated_at = operation.current_step.updated_at
            else:
                # Create new step
                db_step = await self._mapper.from_domain_step(
                    operation.current_step, db_operation.id
                )
                self._session.add(db_step)

    def _calculate_progress(self, operation: OperationAggregate) -> float:
        """Calculate overall progress percentage for an operation."""
        if operation.current_step:
            return operation.current_step.progress_percentage
        return 0.0
