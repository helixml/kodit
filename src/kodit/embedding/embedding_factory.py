"""Embedding service."""

from sqlalchemy.ext.asyncio import AsyncSession

from kodit.config import AppContext, Endpoint
from kodit.embedding.embedding_models import EmbeddingType
from kodit.embedding.embedding_provider.local_embedding_provider import (
    CODE,
    LocalEmbeddingProvider,
)
from kodit.embedding.embedding_provider.openai_embedding_provider import (
    OpenAIEmbeddingProvider,
)
from kodit.embedding.embedding_repository import EmbeddingRepository
from kodit.embedding.local_vector_search_service import LocalVectorSearchService
from kodit.embedding.vector_search_service import (
    VectorSearchService,
)
from kodit.embedding.vectorchord_vector_search_service import (
    TaskName,
    VectorChordVectorSearchService,
)
from kodit.log import log_event


def _get_endpoint_configuration(app_context: AppContext) -> Endpoint | None:
    """Get the endpoint configuration for the embedding service."""
    return app_context.embedding_endpoint or app_context.default_endpoint or None


def embedding_factory(
    task_name: TaskName, app_context: AppContext, session: AsyncSession
) -> VectorSearchService:
    """Create an embedding service."""
    embedding_repository = EmbeddingRepository(session=session)
    endpoint = _get_endpoint_configuration(app_context)

    if endpoint and endpoint.type == "openai":
        log_event("kodit.embedding", {"provider": "openai"})
        from openai import AsyncOpenAI

        embedding_provider = OpenAIEmbeddingProvider(
            openai_client=AsyncOpenAI(
                api_key=endpoint.api_key or "default",
                base_url=endpoint.base_url or "https://api.openai.com/v1",
            ),
            model_name=endpoint.model or "text-embedding-3-small",
        )
    else:
        log_event("kodit.embedding", {"provider": "local"})
        embedding_provider = LocalEmbeddingProvider(CODE)

    if app_context.default_search.provider == "vectorchord":
        log_event("kodit.database", {"provider": "vectorchord"})
        return VectorChordVectorSearchService(task_name, session, embedding_provider)
    if app_context.default_search.provider == "sqlite":
        log_event("kodit.database", {"provider": "sqlite"})
        if task_name == "code":
            embedding_type = EmbeddingType.CODE
        elif task_name == "text":
            embedding_type = EmbeddingType.TEXT
        return LocalVectorSearchService(
            embedding_repository=embedding_repository,
            embedding_provider=embedding_provider,
            embedding_type=embedding_type,
        )

    msg = f"Invalid semantic search provider: {app_context.default_search.provider}"
    raise ValueError(msg)
