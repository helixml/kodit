"""Factory for creating snippet domain service."""

from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.services.snippet_service import SnippetDomainService
from kodit.infrastructure.snippet_extraction.snippet_extraction_factory import (
    create_snippet_extraction_domain_service,
)
from kodit.infrastructure.sqlalchemy.file_repository import SqlAlchemyFileRepository
from kodit.infrastructure.sqlalchemy.snippet_repository import (
    SqlAlchemySnippetRepository,
)


def snippet_domain_service_factory(session: AsyncSession) -> SnippetDomainService:
    """Create a snippet domain service with all dependencies.

    Args:
        session: The database session

    Returns:
        Configured snippet domain service

    """
    # Create domain service for snippet extraction
    snippet_extraction_service = create_snippet_extraction_domain_service()

    # Create repositories
    snippet_repository = SqlAlchemySnippetRepository(session)
    file_repository = SqlAlchemyFileRepository(session)

    # Create and return the domain service
    return SnippetDomainService(
        snippet_extraction_service=snippet_extraction_service,
        snippet_repository=snippet_repository,
        file_repository=file_repository,
    )
