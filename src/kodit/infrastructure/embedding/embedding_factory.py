"""Factory for creating embedding services with DDD architecture."""

from sqlalchemy.ext.asyncio import AsyncSession

from kodit.config import AppContext, Endpoint
from kodit.domain.services.embedding_service import EmbeddingDomainService
from kodit.infrastructure.embedding.embedding_providers.local_embedding_provider import (  # noqa: E501
    CODE,
    LocalEmbeddingProvider,
)
from kodit.infrastructure.embedding.embedding_providers.openai_embedding_provider import (  # noqa: E501
    OpenAIEmbeddingProvider,
)
from kodit.infrastructure.embedding.local_vector_search_repository import (
    LocalVectorSearchRepository,
)
from kodit.infrastructure.embedding.vectorchord_vector_search_repository import (
    TaskName,
    VectorChordVectorSearchRepository,
)
from kodit.infrastructure.sqlalchemy.embedding_repository import (
    SqlAlchemyEmbeddingRepository,
)
from kodit.infrastructure.sqlalchemy.entities import EmbeddingType
from kodit.log import log_event


def _get_endpoint_configuration(app_context: AppContext) -> Endpoint | None:
    """Get the endpoint configuration for the embedding service."""
    return app_context.embedding_endpoint or app_context.default_endpoint or None


def embedding_domain_service_factory(
    task_name: TaskName, app_context: AppContext, session: AsyncSession
) -> EmbeddingDomainService:
    """Create an embedding domain service."""
    # Create embedding repository
    embedding_repository = SqlAlchemyEmbeddingRepository(session=session)

    # Create embedding provider
    endpoint = _get_endpoint_configuration(app_context)
    if endpoint and endpoint.type == "openai":
        log_event("kodit.embedding", {"provider": "openai"})
        from openai import AsyncOpenAI

        embedding_provider = OpenAIEmbeddingProvider(
            openai_client=AsyncOpenAI(
                api_key=endpoint.api_key or "default",
                base_url=endpoint.base_url or "https://api.openai.com/v1",
                timeout=10,
                max_retries=2,
            ),
            model_name=endpoint.model or "text-embedding-3-small",
        )
    else:
        log_event("kodit.embedding", {"provider": "local"})
        embedding_provider = LocalEmbeddingProvider(CODE)

    # Create vector search repository based on configuration
    if app_context.default_search.provider == "vectorchord":
        log_event("kodit.database", {"provider": "vectorchord"})
        vector_search_repository = VectorChordVectorSearchRepository(
            task_name, session, embedding_provider
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
