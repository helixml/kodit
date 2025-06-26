"""Test the embedding domain service factory."""

import pytest
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.config import AppContext, Endpoint, Search
from kodit.infrastructure.embedding.embedding_factory import (
    embedding_domain_service_factory,
)
from kodit.infrastructure.embedding.embedding_providers.local_embedding_provider import (  # noqa: E501
    LocalEmbeddingProvider,
)
from kodit.infrastructure.embedding.embedding_providers.openai_embedding_provider import (  # noqa: E501
    OpenAIEmbeddingProvider,
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
    app_context.default_endpoint = None
    app_context.embedding_endpoint = None
    service = embedding_domain_service_factory(
        "code", app_context=app_context, session=session
    )
    assert isinstance(service.vector_search_repository, LocalVectorSearchRepository)
    assert isinstance(service.embedding_provider, LocalEmbeddingProvider)

    # With openai default endpoint
    app_context.default_endpoint = Endpoint(
        type="openai",
        base_url="https://api.openai.com/v1",
        model="gpt-4o-mini",
        api_key="default",
    )
    app_context.embedding_endpoint = None
    service = embedding_domain_service_factory(
        "code", app_context=app_context, session=session
    )
    assert isinstance(service.vector_search_repository, LocalVectorSearchRepository)
    assert isinstance(service.embedding_provider, OpenAIEmbeddingProvider)

    # With empty default and embedding endpoint
    app_context.default_endpoint = None
    app_context.embedding_endpoint = Endpoint(
        type="openai",
        base_url="https://api.openai.com/v1",
        model="gpt-4o-mini",
        api_key="default",
    )
    service = embedding_domain_service_factory(
        "code", app_context=app_context, session=session
    )
    assert isinstance(service.vector_search_repository, LocalVectorSearchRepository)
    assert isinstance(service.embedding_provider, OpenAIEmbeddingProvider)

    # With default and override embedding endpoint
    app_context.default_endpoint = Endpoint(
        type="openai",
        base_url="https://api.openai.com/v1",
        model="gpt-4o-mini",
        api_key="default",
    )
    test_base_url = "http://localhost:8000/v1/"
    app_context.embedding_endpoint = Endpoint(
        type="openai",
        base_url=test_base_url,
        model="qwen/qwen3-8b",
        api_key="default",
    )
    service = embedding_domain_service_factory(
        "code", app_context=app_context, session=session
    )
    assert isinstance(service.vector_search_repository, LocalVectorSearchRepository)
    assert isinstance(service.embedding_provider, OpenAIEmbeddingProvider)
    assert service.embedding_provider.openai_client.base_url == test_base_url
