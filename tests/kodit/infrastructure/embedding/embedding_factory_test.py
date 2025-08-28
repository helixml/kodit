"""Test the embedding domain service factory."""

import pytest
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.config import AppContext, Endpoint, Search
from kodit.infrastructure.embedding.embedding_factory import (
    embedding_domain_service_factory,
)
from kodit.infrastructure.embedding.embedding_providers.litellm_embedding_provider import (  # noqa: E501
    LiteLLMEmbeddingProvider,
)
from kodit.infrastructure.embedding.embedding_providers.local_embedding_provider import (  # noqa: E501
    LocalEmbeddingProvider,
)
from kodit.infrastructure.embedding.local_vector_search_repository import (
    LocalVectorSearchRepository,
)


@pytest.mark.asyncio
async def test_embedding_domain_service_factory(
    app_context: AppContext, session: AsyncSession
) -> None:
    """Test the embedding domain service factory."""
    # Set search provider to sqlite to override environment variable
    app_context.default_search = Search(provider="sqlite")

    # With defaults, no settings
    app_context.embedding_endpoint = None
    service = embedding_domain_service_factory(
        "code", app_context=app_context, session=session
    )
    assert isinstance(service.vector_search_repository, LocalVectorSearchRepository)
    assert isinstance(service.embedding_provider, LocalEmbeddingProvider)

    # With empty default and embedding endpoint
    app_context.embedding_endpoint = Endpoint(
        base_url="https://api.openai.com/v1",
        model="gpt-4o-mini",
        api_key="default",
    )
    service = embedding_domain_service_factory(
        "code", app_context=app_context, session=session
    )
    assert isinstance(service.vector_search_repository, LocalVectorSearchRepository)
    assert isinstance(service.embedding_provider, LiteLLMEmbeddingProvider)
