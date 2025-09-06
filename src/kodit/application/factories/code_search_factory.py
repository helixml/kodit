"""Factory for creating the code search application service."""

from collections.abc import Callable

from sqlalchemy.ext.asyncio import AsyncSession

from kodit.application.factories.reporting_factory import (
    create_cli_operation,
    create_noop_operation,
    create_server_operation,
)
from kodit.application.services.code_search_application_service import (
    CodeSearchApplicationService,
)
from kodit.application.services.reporting import (
    ProgressTracker,
)
from kodit.config import AppContext
from kodit.domain.services.bm25_service import BM25DomainService
from kodit.domain.services.embedding_service import EmbeddingDomainService
from kodit.domain.services.index_query_service import IndexQueryService
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
from kodit.infrastructure.indexing.fusion_service import ReciprocalRankFusionService
from kodit.infrastructure.sqlalchemy.embedding_repository import (
    create_embedding_repository,
)
from kodit.infrastructure.sqlalchemy.entities import EmbeddingType
from kodit.infrastructure.sqlalchemy.index_repository import (
    create_index_repository,
)
from kodit.infrastructure.sqlalchemy.task_status_repository import (
    create_task_status_repository,
)


def create_code_search_application_service(
    app_context: AppContext,
    session_factory: Callable[[], AsyncSession],
    progress_tracker: ProgressTracker | None = None,
) -> CodeSearchApplicationService:
    """Create a unified code indexing application service with all dependencies."""
    if not progress_tracker:
        progress_tracker = create_noop_operation()
    # Create domain services
    bm25_service = BM25DomainService(
        bm25_repository_factory(app_context, session_factory())
    )
    code_search_service = embedding_domain_service_factory(
        "code", app_context, session_factory(), session_factory
    )
    text_search_service = embedding_domain_service_factory(
        "text", app_context, session_factory(), session_factory
    )
    index_repository = create_index_repository(session_factory=session_factory)
    # Use the unified language mapping from the domain layer

    index_query_service = IndexQueryService(
        index_repository=index_repository,
        fusion_service=ReciprocalRankFusionService(),
    )

    # Create and return the unified application service
    return CodeSearchApplicationService(
        index_query_service=index_query_service,
        bm25_service=bm25_service,
        code_search_service=code_search_service,
        text_search_service=text_search_service,
        progress_tracker=progress_tracker,
    )


def create_cli_code_search_application_service(
    app_context: AppContext,
    session_factory: Callable[[], AsyncSession],
) -> CodeSearchApplicationService:
    """Create a CLI code indexing application service."""
    return create_code_search_application_service(
        app_context,
        session_factory,
        create_cli_operation(),
    )


def create_server_code_search_application_service(
    app_context: AppContext,
    session_factory: Callable[[], AsyncSession],
) -> CodeSearchApplicationService:
    """Create a server code indexing application service."""
    return create_code_search_application_service(
        app_context,
        session_factory,
        create_server_operation(create_task_status_repository(session_factory)),
    )


def create_fast_test_code_search_application_service(
    app_context: AppContext,
    session_factory: Callable[[], AsyncSession],
) -> CodeSearchApplicationService:
    """Create a fast test code indexing application service."""
    progress_tracker = create_noop_operation()
    # Create domain services
    bm25_service = BM25DomainService(
        bm25_repository_factory(app_context, session_factory())
    )
    embedding_repository = create_embedding_repository(session_factory=session_factory)

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
    index_repository = create_index_repository(session_factory=session_factory)
    index_query_service = IndexQueryService(
        index_repository=index_repository,
        fusion_service=ReciprocalRankFusionService(),
    )

    # Create and return the unified application service
    return CodeSearchApplicationService(
        index_query_service=index_query_service,
        bm25_service=bm25_service,
        code_search_service=code_search_service,
        text_search_service=text_search_service,
        progress_tracker=progress_tracker,
    )
