"""Factory functions for creating SQLAlchemy repositories."""

from kodit.domain.protocols import (
    IndexRepository,
    OperationRepository,
    TaskRepository,
    UnitOfWork,
)
from kodit.infrastructure.sqlalchemy.embedding_repository import (
    SqlAlchemyEmbeddingRepository,
)
from kodit.infrastructure.sqlalchemy.index_repository import SqlAlchemyIndexRepository
from kodit.infrastructure.sqlalchemy.operation_repository import (
    SqlAlchemyOperationRepository,
)
from kodit.infrastructure.sqlalchemy.task_repository import SqlAlchemyTaskRepository


def create_index_repository(uow: UnitOfWork) -> IndexRepository:
    """Create an index repository using the UoW."""
    return SqlAlchemyIndexRepository(uow)


def create_task_repository(uow: UnitOfWork) -> TaskRepository:
    """Create a task repository using the UoW."""
    return SqlAlchemyTaskRepository(uow)


def create_operation_repository(uow: UnitOfWork) -> OperationRepository:
    """Create an operation repository using the UoW."""
    return SqlAlchemyOperationRepository(uow)


def create_embedding_repository(uow: UnitOfWork) -> SqlAlchemyEmbeddingRepository:
    """Create an embedding repository using the UoW."""
    return SqlAlchemyEmbeddingRepository(uow)
