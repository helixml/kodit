"""Factory for creating the unified code indexing application service."""

from sqlalchemy.ext.asyncio import AsyncSession

from kodit.application.services.code_indexing_application_service import (
    CodeIndexingApplicationService,
)
from kodit.config import AppContext
from kodit.domain.services.bm25_service import BM25DomainService
from kodit.domain.services.source_service import SourceService
from kodit.infrastructure.bm25.bm25_factory import bm25_repository_factory
from kodit.infrastructure.embedding.embedding_factory import (
    embedding_domain_service_factory,
)
from kodit.infrastructure.enrichment.enrichment_factory import (
    enrichment_domain_service_factory,
)
from kodit.infrastructure.indexing.indexing_factory import (
    indexing_domain_service_factory,
)
from kodit.infrastructure.indexing.snippet_domain_service_factory import (
    snippet_domain_service_factory,
)


def create_code_indexing_application_service(
    app_context: AppContext,
    session: AsyncSession,
    source_service: SourceService,
) -> CodeIndexingApplicationService:
    """Create a unified code indexing application service with all dependencies.

    This factory creates the new unified service that replaces the previously
    separate IndexingApplicationService and SnippetApplicationService.

    Args:
        app_context: The application context
        session: The database session
        source_service: The source service

    Returns:
        A fully configured CodeIndexingApplicationService instance

    """
    # Create domain services
    indexing_domain_service = indexing_domain_service_factory(session)
    snippet_domain_service = snippet_domain_service_factory(session)
    bm25_service = BM25DomainService(bm25_repository_factory(app_context, session))
    code_search_service = embedding_domain_service_factory("code", app_context, session)
    text_search_service = embedding_domain_service_factory("text", app_context, session)
    enrichment_service = enrichment_domain_service_factory(app_context)

    # Create and return the unified application service
    return CodeIndexingApplicationService(
        indexing_domain_service=indexing_domain_service,
        snippet_domain_service=snippet_domain_service,
        source_service=source_service,
        bm25_service=bm25_service,
        code_search_service=code_search_service,
        text_search_service=text_search_service,
        enrichment_service=enrichment_service,
        session=session,
    )
