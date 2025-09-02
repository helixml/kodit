"""Factory for creating the unified code indexing application service."""

from collections.abc import Callable

from sqlalchemy.ext.asyncio import AsyncSession

from kodit.application.services.code_indexing_application_service import (
    CodeIndexingApplicationService,
)
from kodit.config import AppContext
from kodit.domain.protocols import ReportingService, UnitOfWork
from kodit.domain.services.bm25_service import BM25DomainService
from kodit.domain.services.embedding_service import EmbeddingDomainService
from kodit.domain.services.enrichment_service import EnrichmentDomainService
from kodit.domain.services.index_query_service import IndexQueryService
from kodit.domain.services.index_service import (
    IndexDomainService,
)
from kodit.domain.value_objects import LanguageMapping
from kodit.infrastructure.bm25.bm25_factory import bm25_repository_factory
from kodit.infrastructure.embedding.embedding_factory import (
    embedding_domain_service_factory,
)
from kodit.infrastructure.embedding.embedding_providers.hash_embedding_provider import (
    HashEmbeddingProvider,
)
from kodit.infrastructure.embedding.local_vector_search_repository import (
    LocalVectorSearchRepository,
)
from kodit.infrastructure.enrichment.enrichment_factory import (
    enrichment_domain_service_factory,
)
from kodit.infrastructure.enrichment.null_enrichment_provider import (
    NullEnrichmentProvider,
)
from kodit.infrastructure.indexing.fusion_service import ReciprocalRankFusionService
from kodit.infrastructure.reporting.reporter import (
    create_cli_reporter,
    create_noop_reporter,
)
from kodit.infrastructure.slicing.language_detection_service import (
    FileSystemLanguageDetectionService,
)
from kodit.infrastructure.sqlalchemy.embedding_repository import (
    SqlAlchemyEmbeddingRepository,
)
from kodit.infrastructure.sqlalchemy.entities import EmbeddingType
from kodit.infrastructure.sqlalchemy.index_repository import SqlAlchemyIndexRepository
from kodit.infrastructure.sqlalchemy.unit_of_work import SqlAlchemyUnitOfWork


def create_code_indexing_application_service(
    app_context: AppContext,
    unit_of_work: UnitOfWork,
    reporter: ReportingService,
) -> CodeIndexingApplicationService:
    """Create a unified code indexing application service with all dependencies."""
    # Create domain services
    bm25_service = BM25DomainService(bm25_repository_factory(app_context, unit_of_work))
    code_search_service = embedding_domain_service_factory(
        "code", app_context, unit_of_work
    )
    text_search_service = embedding_domain_service_factory(
        "text", app_context, unit_of_work
    )
    enrichment_service = enrichment_domain_service_factory(app_context)

    # Create temporary index repository for services that still need it
    index_repository = SqlAlchemyIndexRepository(unit_of_work)

    # Use the unified language mapping from the domain layer
    language_map = LanguageMapping.get_extension_to_language_map()

    # Create infrastructure services
    language_detector = FileSystemLanguageDetectionService(language_map)

    index_domain_service = IndexDomainService(
        language_detector=language_detector,
        enrichment_service=enrichment_service,
        clone_dir=app_context.get_clone_dir(),
        reporter=reporter,
    )
    index_query_service = IndexQueryService(
        index_repository=index_repository,
        fusion_service=ReciprocalRankFusionService(),
    )

    # Create and return the unified application service
    return CodeIndexingApplicationService(
        indexing_domain_service=index_domain_service,
        index_query_service=index_query_service,
        bm25_service=bm25_service,
        code_search_service=code_search_service,
        text_search_service=text_search_service,
        enrichment_service=enrichment_service,
        unit_of_work=unit_of_work,
        reporter=reporter,
    )


def create_cli_code_indexing_application_service(
    app_context: AppContext,
    session_factory: Callable[[], AsyncSession],
) -> CodeIndexingApplicationService:
    """Create a CLI code indexing application service."""
    return create_code_indexing_application_service(
        app_context, session_factory, create_cli_reporter()
    )


def create_fast_test_code_indexing_application_service(
    app_context: AppContext,
    session_factory: Callable[[], AsyncSession],
) -> CodeIndexingApplicationService:
    """Create a fast test code indexing application service."""
    # Create a temporary session for services that still need it
    unit_of_work = SqlAlchemyUnitOfWork(session_factory)

    # Create domain services
    bm25_service = BM25DomainService(bm25_repository_factory(app_context, unit_of_work))
    embedding_repository = SqlAlchemyEmbeddingRepository(unit_of_work)
    reporter = create_noop_reporter()

    code_search_repository = LocalVectorSearchRepository(
        embedding_repository=embedding_repository,
        embedding_provider=HashEmbeddingProvider(),
        embedding_type=EmbeddingType.CODE,
    )
    code_search_service = EmbeddingDomainService(
        embedding_provider=HashEmbeddingProvider(),
        vector_search_repository=code_search_repository,
    )

    # Fast text search service
    text_search_repository = LocalVectorSearchRepository(
        embedding_repository=embedding_repository,
        embedding_provider=HashEmbeddingProvider(),
        embedding_type=EmbeddingType.TEXT,
    )
    text_search_service = EmbeddingDomainService(
        embedding_provider=HashEmbeddingProvider(),
        vector_search_repository=text_search_repository,
    )

    # Fast enrichment service using NullEnrichmentProvider
    enrichment_service = EnrichmentDomainService(
        enrichment_provider=NullEnrichmentProvider()
    )

    # Create Unit of Work
    unit_of_work = SqlAlchemyUnitOfWork(session_factory)

    # Create temporary index repository for services that still need it
    index_repository = SqlAlchemyIndexRepository(unit_of_work)

    # Use the unified language mapping from the domain layer
    language_map = LanguageMapping.get_extension_to_language_map()

    # Create infrastructure services
    language_detector = FileSystemLanguageDetectionService(language_map)

    index_domain_service = IndexDomainService(
        language_detector=language_detector,
        enrichment_service=enrichment_service,
        clone_dir=app_context.get_clone_dir(),
        reporter=reporter,
    )
    index_query_service = IndexQueryService(
        index_repository=index_repository,
        fusion_service=ReciprocalRankFusionService(),
    )

    # Create and return the unified application service
    return CodeIndexingApplicationService(
        indexing_domain_service=index_domain_service,
        index_query_service=index_query_service,
        bm25_service=bm25_service,
        code_search_service=code_search_service,
        text_search_service=text_search_service,
        enrichment_service=enrichment_service,
        unit_of_work=unit_of_work,
        reporter=reporter,
    )
