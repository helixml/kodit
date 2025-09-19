"""Factory for creating embedding services with DDD architecture."""

from collections.abc import Callable

import structlog
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.config import AppContext, Endpoint
from kodit.domain.services.embedding_service import (
    EmbeddingDomainService,
    EmbeddingProvider,
    VectorSearchRepository,
)
from kodit.infrastructure.embedding.embedding_providers.litellm_embedding_provider import (  # noqa: E501
    LiteLLMEmbeddingProvider,
)
from kodit.infrastructure.embedding.embedding_providers.local_embedding_provider import (  # noqa: E501
    CODE,
    LocalEmbeddingProvider,
)
from kodit.infrastructure.embedding.local_vector_search_repository import (
    LocalVectorSearchRepository,
)
from kodit.infrastructure.embedding.vectorchord_vector_search_repository import (
    TaskName,
    VectorChordVectorSearchRepository,
)
from kodit.infrastructure.sqlalchemy.embedding_repository import (
    create_embedding_repository,
)
from kodit.infrastructure.sqlalchemy.entities import EmbeddingType
from kodit.log import log_event


def _get_endpoint_configuration(app_context: AppContext) -> Endpoint | None:
    """Get the endpoint configuration for the embedding service."""
    return app_context.embedding_endpoint or None


def embedding_domain_service_factory(
    task_name: TaskName,
    app_context: AppContext,
    session_factory: Callable[[], AsyncSession],
) -> EmbeddingDomainService:
    """Create an embedding domain service."""
    structlog.get_logger(__name__)
    # Create embedding repository
    embedding_repository = create_embedding_repository(session_factory=session_factory)

    # Create embedding provider
    embedding_provider: EmbeddingProvider | None = None
    endpoint = _get_endpoint_configuration(app_context)

    if endpoint:
        log_event("kodit.embedding", {"provider": "litellm"})
        embedding_provider = LiteLLMEmbeddingProvider(endpoint=endpoint)
    else:
        log_event("kodit.embedding", {"provider": "local"})
        embedding_provider = LocalEmbeddingProvider(CODE)

    # Create vector search repository based on configuration
    vector_search_repository: VectorSearchRepository | None = None
    if app_context.default_search.provider == "vectorchord":
        log_event("kodit.database", {"provider": "vectorchord"})
        vector_search_repository = VectorChordVectorSearchRepository(
            session_factory=session_factory,
            task_name=task_name,
            embedding_provider=embedding_provider,
        )
    elif app_context.default_search.provider == "sqlite":
        log_event("kodit.database", {"provider": "sqlite"})
        if task_name == "code":
            embedding_type = EmbeddingType.CODE
        elif task_name == "text":
            embedding_type = EmbeddingType.TEXT
        else:
            raise ValueError(f"Invalid task name: {task_name}")

        vector_search_repository = LocalVectorSearchRepository(
            embedding_repository=embedding_repository,
            embedding_provider=embedding_provider,
            embedding_type=embedding_type,
        )
    else:
        msg = f"Invalid semantic search provider: {app_context.default_search.provider}"
        raise ValueError(msg)

    # Create and return domain service
    return EmbeddingDomainService(
        embedding_provider=embedding_provider,
        vector_search_repository=vector_search_repository,
    )
