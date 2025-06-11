import pytest
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.config import AppContext, Endpoint
from kodit.embedding.embedding_factory import embedding_factory
from kodit.embedding.embedding_provider.local_embedding_provider import (
    LocalEmbeddingProvider,
)
from kodit.embedding.embedding_provider.openai_embedding_provider import (
    OpenAIEmbeddingProvider,
)
from kodit.embedding.local_vector_search_service import LocalVectorSearchService


@pytest.mark.asyncio
async def test_embedding_factory(
    app_context: AppContext, session: AsyncSession
) -> None:
    # With defaults, no settings
    app_context.default_endpoint = None
    app_context.embedding_endpoint = None
    e = embedding_factory("code", app_context=app_context, session=session)
    assert isinstance(e, LocalVectorSearchService)
    assert isinstance(e.embedding_provider, LocalEmbeddingProvider)

    # With openai default endpoint
    app_context.default_endpoint = Endpoint(
        type="openai",
        base_url="https://api.openai.com/v1",
        model="gpt-4o-mini",
        api_key="default",
    )
    app_context.embedding_endpoint = None
    e = embedding_factory("code", app_context=app_context, session=session)
    assert isinstance(e, LocalVectorSearchService)
    assert isinstance(e.embedding_provider, OpenAIEmbeddingProvider)

    # With empty default and embedding endpoint
    app_context.default_endpoint = None
    app_context.embedding_endpoint = Endpoint(
        type="openai",
        base_url="https://api.openai.com/v1",
        model="gpt-4o-mini",
        api_key="default",
    )
    e = embedding_factory("code", app_context=app_context, session=session)
    assert isinstance(e, LocalVectorSearchService)
    assert isinstance(e.embedding_provider, OpenAIEmbeddingProvider)

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
    e = embedding_factory("code", app_context=app_context, session=session)
    assert isinstance(e, LocalVectorSearchService)
    assert isinstance(e.embedding_provider, OpenAIEmbeddingProvider)
    assert e.embedding_provider.openai_client.base_url == test_base_url
