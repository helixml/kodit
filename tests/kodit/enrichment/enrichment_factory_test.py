import pytest

from kodit.config import AppContext, Endpoint
from kodit.enrichment.enrichment_factory import enrichment_factory
from kodit.enrichment.enrichment_provider.local_enrichment_provider import (
    LocalEnrichmentProvider,
)
from kodit.enrichment.enrichment_provider.openai_enrichment_provider import (
    OpenAIEnrichmentProvider,
)
from kodit.enrichment.enrichment_service import LLMEnrichmentService


@pytest.mark.asyncio
async def test_enrichment_factory(app_context: AppContext):
    # With defaults, no settings
    app_context.default_endpoint = None
    app_context.enrichment_endpoint = None
    e = enrichment_factory(app_context=app_context)
    assert isinstance(e, LLMEnrichmentService)
    assert isinstance(e.enrichment_provider, LocalEnrichmentProvider)

    # With openai default endpoint
    app_context.default_endpoint = Endpoint(
        type="openai",
        base_url="https://api.openai.com/v1",
        model="gpt-4o-mini",
        api_key="default",
    )
    app_context.enrichment_endpoint = None
    e = enrichment_factory(app_context=app_context)
    assert isinstance(e, LLMEnrichmentService)
    assert isinstance(e.enrichment_provider, OpenAIEnrichmentProvider)

    # With empty default and enrichment endpoint
    app_context.default_endpoint = None
    app_context.enrichment_endpoint = Endpoint(
        type="openai",
        base_url="https://api.openai.com/v1",
        model="gpt-4o-mini",
        api_key="default",
    )
    e = enrichment_factory(app_context=app_context)
    assert isinstance(e, LLMEnrichmentService)
    assert isinstance(e.enrichment_provider, OpenAIEnrichmentProvider)

    # With default and override enrichment endpoint
    app_context.default_endpoint = Endpoint(
        type="openai",
        base_url="https://api.openai.com/v1",
        model="gpt-4o-mini",
        api_key="default",
    )
    test_base_url = "http://localhost:8000/v1/"
    app_context.enrichment_endpoint = Endpoint(
        type="openai",
        base_url=test_base_url,
        model="qwen/qwen3-8b",
        api_key="default",
    )
    e = enrichment_factory(app_context=app_context)
    assert isinstance(e, LLMEnrichmentService)
    assert isinstance(e.enrichment_provider, OpenAIEnrichmentProvider)
    assert e.enrichment_provider.openai_client.base_url == test_base_url
