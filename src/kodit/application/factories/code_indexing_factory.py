"""Factory for creating the unified code indexing application service."""

from sqlalchemy.ext.asyncio import AsyncSession

from kodit.application.services.code_indexing_application_service import (
    CodeIndexingApplicationService,
)
from kodit.config import AppContext
from kodit.domain.enums import SnippetExtractionStrategy
from kodit.domain.services.bm25_service import BM25DomainService
from kodit.domain.services.index_query_service import IndexQueryService
from kodit.domain.services.index_service import (
    IndexDomainService,
)
from kodit.domain.value_objects import LanguageMapping
from kodit.infrastructure.bm25.bm25_factory import bm25_repository_factory
from kodit.infrastructure.embedding.embedding_factory import (
    embedding_domain_service_factory,
)
from kodit.infrastructure.enrichment.enrichment_factory import (
    enrichment_domain_service_factory,
)
from kodit.infrastructure.indexing.fusion_service import ReciprocalRankFusionService
from kodit.infrastructure.snippet_extraction.factories import (
    create_snippet_query_provider,
)
from kodit.infrastructure.snippet_extraction.language_detection_service import (
    FileSystemLanguageDetectionService,
)
from kodit.infrastructure.snippet_extraction.tree_sitter_snippet_extractor import (
    TreeSitterSnippetExtractor,
)
from kodit.infrastructure.sqlalchemy.index_repository import SqlAlchemyIndexRepository


def create_code_indexing_application_service(
    app_context: AppContext,
    session: AsyncSession,
) -> CodeIndexingApplicationService:
    """Create a unified code indexing application service with all dependencies."""
    # Create domain services
    bm25_service = BM25DomainService(bm25_repository_factory(app_context, session))
    code_search_service = embedding_domain_service_factory("code", app_context, session)
    text_search_service = embedding_domain_service_factory("text", app_context, session)
    enrichment_service = enrichment_domain_service_factory(app_context)
    index_repository = SqlAlchemyIndexRepository(session=session)
    # Use the unified language mapping from the domain layer
    language_map = LanguageMapping.get_extension_to_language_map()

    # Create infrastructure services
    language_detector = FileSystemLanguageDetectionService(language_map)
    query_provider = create_snippet_query_provider()

    # Create snippet extractors
    method_extractor = TreeSitterSnippetExtractor(query_provider)

    snippet_extractors = {
        SnippetExtractionStrategy.METHOD_BASED: method_extractor,
    }
    index_domain_service = IndexDomainService(
        index_repository=index_repository,
        language_detector=language_detector,
        snippet_extractors=snippet_extractors,
        enrichment_service=enrichment_service,
        clone_dir=app_context.get_clone_dir(),
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
        session=session,
    )
