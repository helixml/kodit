"""Enrichment factory for creating enrichment domain services."""

from kodit.config import AppContext, Endpoint
from kodit.domain.services.enrichment_service import (
    EnrichmentDomainService,
    EnrichmentProvider,
)
from kodit.infrastructure.enrichment.local_enrichment_provider import (
    LocalEnrichmentProvider,
)
from kodit.infrastructure.enrichment.openai_enrichment_provider import (
    OpenAIEnrichmentProvider,
)
from kodit.log import log_event


def _get_endpoint_configuration(app_context: AppContext) -> Endpoint | None:
    """Get the endpoint configuration for the enrichment service.

    Args:
        app_context: The application context.

    Returns:
        The endpoint configuration or None.

    """
    return app_context.enrichment_endpoint or app_context.default_endpoint or None


def enrichment_domain_service_factory(
    app_context: AppContext,
) -> EnrichmentDomainService:
    """Create an enrichment domain service.

    Args:
        app_context: The application context.

    Returns:
        An enrichment domain service instance.

    """
    endpoint = _get_endpoint_configuration(app_context)

    enrichment_provider: EnrichmentProvider | None = None
    if endpoint and endpoint.type == "openai":
        log_event("kodit.enrichment", {"provider": "openai"})
        from openai import AsyncOpenAI

        enrichment_provider = OpenAIEnrichmentProvider(
            openai_client=AsyncOpenAI(
                api_key=endpoint.api_key or "default",
                base_url=endpoint.base_url or "https://api.openai.com/v1",
                timeout=60,
                max_retries=2,
            ),
            model_name=endpoint.model or "gpt-4o-mini",
        )
    else:
        log_event("kodit.enrichment", {"provider": "local"})
        enrichment_provider = LocalEnrichmentProvider()

    return EnrichmentDomainService(enrichment_provider=enrichment_provider)
