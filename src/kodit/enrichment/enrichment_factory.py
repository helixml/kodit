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
    """Create an embedding service."""
    from openai import AsyncOpenAI

    endpoint = _get_endpoint_configuration(app_context)
    endpoint = app_context.enrichment_endpoint or app_context.default_endpoint or None

    if endpoint is None or endpoint.type != "openai":
        return LLMEnrichmentService(LocalEnrichmentProvider())

    return LLMEnrichmentService(
        OpenAIEnrichmentProvider(
            openai_client=AsyncOpenAI(
                api_key=endpoint.api_key,
                base_url=endpoint.base_url,
            ),
            model_name=endpoint.model,
        )
    )
