"""Factory for creating indexing services."""

from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.services.indexing_service import IndexingDomainService
from kodit.infrastructure.indexing.fusion_service import ReciprocalRankFusionService
from kodit.infrastructure.indexing.index_repository import SQLAlchemyIndexRepository


def indexing_domain_service_factory(session: AsyncSession) -> IndexingDomainService:
    """Create an indexing domain service with all dependencies.

    Args:
        session: SQLAlchemy session

    Returns:
        Configured indexing domain service

    """
    # Create repositories
    index_repository = SQLAlchemyIndexRepository(session)

    # Create fusion service
    fusion_service = ReciprocalRankFusionService()

    # Create domain service
    return IndexingDomainService(
        index_repository=index_repository,
        fusion_service=fusion_service,
    )
