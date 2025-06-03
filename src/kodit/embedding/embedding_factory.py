"""Embedding service."""

from sqlalchemy.ext.asyncio import AsyncSession

from kodit.config import AppContext
from kodit.embedding.embedding_provider.embedding_provider import EmbeddingProvider
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


def embedding_factory(
    app_context: AppContext, session: AsyncSession
) -> VectorSearchService:
    """Create an embedding service."""
    embedding_repository = EmbeddingRepository(session=session)
    embedding_provider: EmbeddingProvider | None = None
    openai_client = app_context.get_default_openai_client()
    if openai_client is not None:
        embedding_provider = OpenAIEmbeddingProvider(openai_client=openai_client)
    else:
        embedding_provider = LocalEmbeddingProvider(CODE)

    return LocalVectorSearchService(
        embedding_repository=embedding_repository, embedding_provider=embedding_provider
    )
