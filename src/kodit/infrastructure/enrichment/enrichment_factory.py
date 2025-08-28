"""Enrichment factory for creating enrichment domain services."""

from kodit.config import AppContext, Endpoint
from kodit.domain.services.enrichment_service import (
    EnrichmentDomainService,
    EnrichmentProvider,
)
from kodit.infrastructure.enrichment.litellm_enrichment_provider import (
    LiteLLMEnrichmentProvider,
)
from kodit.infrastructure.enrichment.local_enrichment_provider import (
    LocalEnrichmentProvider,
)
from kodit.log import log_event


def _get_endpoint_configuration(app_context: AppContext) -> Endpoint | None:
    """Get the endpoint configuration for the enrichment service.

    Args:
        app_context: The application context.

    Returns:
        The endpoint configuration or None.

    """
    return app_context.enrichment_endpoint or None


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
    if endpoint:
        log_event("kodit.enrichment", {"provider": "litellm"})
        enrichment_provider = LiteLLMEnrichmentProvider(endpoint=endpoint)
    else:
        log_event("kodit.enrichment", {"provider": "local"})
        enrichment_provider = LocalEnrichmentProvider()

    return EnrichmentDomainService(enrichment_provider=enrichment_provider)
