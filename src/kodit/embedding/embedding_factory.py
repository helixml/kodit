"""Embedding service."""

from sqlalchemy.ext.asyncio import AsyncSession

from kodit.config import AppContext, Endpoint
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
        from openai import AsyncOpenAI

        embedding_provider = OpenAIEmbeddingProvider(
            openai_client=AsyncOpenAI(
                api_key=endpoint.api_key or "default",
                base_url=endpoint.base_url or "https://api.openai.com/v1",
            ),
            model_name=endpoint.model or "text-embedding-3-small",
        )
    else:
        embedding_provider = LocalEmbeddingProvider(CODE)

    if app_context.default_search.provider == "vectorchord":
        return VectorChordVectorSearchService(task_name, session, embedding_provider)
    if app_context.default_search.provider == "sqlite":
        return LocalVectorSearchService(
            embedding_repository=embedding_repository,
            embedding_provider=embedding_provider,
        )

    msg = f"Invalid semantic search provider: {app_context.default_search.provider}"
    raise ValueError(msg)
