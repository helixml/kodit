"""Embedding service."""

from sqlalchemy.ext.asyncio import AsyncSession

from kodit.config import AppContext
from kodit.embedding.embedding_repository import EmbeddingRepository
from kodit.embedding.embedding_service import (
    TINY,
    EmbeddingService,
    LocalEmbedder,
    OpenAIEmbedder,
)


def embedding_factory(
    app_context: AppContext, session: AsyncSession
) -> EmbeddingService:
    """Create an embedding service."""
    openai_client = app_context.get_default_openai_client()
    embedding_repository = EmbeddingRepository(session=session)
    if openai_client is not None:
        return OpenAIEmbedder(
            embedding_repository=embedding_repository, openai_client=openai_client
        )
    return LocalEmbedder(
        embedding_repository=embedding_repository,
        model_name=TINY,
    )
