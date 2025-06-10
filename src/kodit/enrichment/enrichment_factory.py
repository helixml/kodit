"""Embedding service."""

from kodit.config import AppContext, Endpoint
from kodit.enrichment.enrichment_provider.local_enrichment_provider import (
    LocalEnrichmentProvider,
)
from kodit.enrichment.enrichment_provider.openai_enrichment_provider import (
    OpenAIEnrichmentProvider,
)
from kodit.enrichment.enrichment_service import (
    EnrichmentService,
    LLMEnrichmentService,
)


def _get_endpoint_configuration(app_context: AppContext) -> Endpoint | None:
    """Get the endpoint configuration for the enrichment service."""
    return app_context.enrichment_endpoint or app_context.default_endpoint or None


def enrichment_factory(app_context: AppContext) -> EnrichmentService:
    """Create an enrichment service."""
    endpoint = _get_endpoint_configuration(app_context)
    endpoint = app_context.enrichment_endpoint or app_context.default_endpoint or None

    if endpoint and endpoint.type == "openai":
        from openai import AsyncOpenAI

        enrichment_provider = OpenAIEnrichmentProvider(
            openai_client=AsyncOpenAI(
                api_key=endpoint.api_key or "default",
                base_url=endpoint.base_url or "https://api.openai.com/v1",
            ),
            model_name=endpoint.model or "gpt-4o-mini",
        )
    else:
        enrichment_provider = LocalEnrichmentProvider()

    return LLMEnrichmentService(enrichment_provider=enrichment_provider)
