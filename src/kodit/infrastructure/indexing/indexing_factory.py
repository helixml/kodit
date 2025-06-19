"""Factory for creating indexing services."""

from typing import Any

from sqlalchemy.ext.asyncio import AsyncSession

from kodit.application.services.indexing_application_service import (
    IndexingApplicationService,
)
from kodit.application.services.snippet_application_service import (
    SnippetApplicationService,
)
from kodit.domain.services.bm25_service import BM25DomainService
from kodit.domain.services.indexing_service import IndexingDomainService
from kodit.domain.services.source_service import SourceService
from kodit.infrastructure.bm25.bm25_factory import bm25_repository_factory
from kodit.infrastructure.embedding.embedding_factory import (
    embedding_domain_service_factory,
)
from kodit.infrastructure.enrichment.enrichment_factory import (
    create_enrichment_domain_service,
)
from kodit.infrastructure.indexing.fusion_service import ReciprocalRankFusionService
from kodit.infrastructure.indexing.index_repository import SQLAlchemyIndexRepository
from kodit.infrastructure.snippet_extraction.snippet_extraction_factory import (
    create_snippet_extraction_domain_service,
)
from kodit.infrastructure.sqlalchemy.file_repository import (
    SqlAlchemyFileRepository,
)
from kodit.infrastructure.sqlalchemy.snippet_repository import (
    SqlAlchemySnippetRepository,
)


def create_snippet_application_service(
    session: AsyncSession,
) -> SnippetApplicationService:
    """Create a snippet application service with all dependencies."""
    # Create domain service
    snippet_extraction_service = create_snippet_extraction_domain_service()

    # Create repositories
    snippet_repository = SqlAlchemySnippetRepository(session)
    file_repository = SqlAlchemyFileRepository(session)

    # Create application service
    from kodit.application.services.snippet_application_service import (
        SnippetApplicationService,
    )

    return SnippetApplicationService(
        snippet_extraction_service=snippet_extraction_service,
        snippet_repository=snippet_repository,
        file_repository=file_repository,
    )


def create_indexing_domain_service(session: AsyncSession) -> IndexingDomainService:
    """Create an indexing domain service.

    Args:
        session: The database session.

    Returns:
        An indexing domain service instance.

    """
    index_repository = SQLAlchemyIndexRepository(session)
    fusion_service = ReciprocalRankFusionService()

    return IndexingDomainService(
        index_repository=index_repository,
        fusion_service=fusion_service,
    )


def create_indexing_application_service(
    app_context: Any,
    session: AsyncSession,
    source_service: SourceService,
    snippet_application_service: SnippetApplicationService,
) -> IndexingApplicationService:
    """Create an indexing application service.

    Args:
        app_context: The application context.
        session: The database session.
        source_service: The source service.
        snippet_application_service: The snippet application service.

    Returns:
        An indexing application service instance.

    """
    # Create domain services
    indexing_domain_service = create_indexing_domain_service(session)
    bm25_service = BM25DomainService(bm25_repository_factory(app_context, session))
    code_search_service = embedding_domain_service_factory("code", app_context, session)
    text_search_service = embedding_domain_service_factory("text", app_context, session)
    enrichment_service = create_enrichment_domain_service(app_context)

    return IndexingApplicationService(
        indexing_domain_service=indexing_domain_service,
        source_service=source_service,
        bm25_service=bm25_service,
        code_search_service=code_search_service,
        text_search_service=text_search_service,
        enrichment_service=enrichment_service,
        snippet_application_service=snippet_application_service,
    )
