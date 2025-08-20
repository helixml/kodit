"""Enrichment factory for creating enrichment domain services."""

import structlog

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
    log = structlog.get_logger(__name__)
    endpoint = _get_endpoint_configuration(app_context)

    enrichment_provider: EnrichmentProvider | None = None
    if endpoint:
        log_event("kodit.enrichment", {"provider": "litellm"})
        enrichment_provider = LiteLLMEnrichmentProvider(endpoint=endpoint)
        if not enrichment_provider.verify_provider():
            log.fatal(
                "Unable to verify enrichment provider, please check your settings",
                model=endpoint.model,
                base_url=endpoint.base_url,
                api_key=endpoint.api_key[:4] + "..." if endpoint.api_key else "None",
                num_parallel_tasks=endpoint.num_parallel_tasks,
                socket_path=endpoint.socket_path,
                timeout=endpoint.timeout,
                extra_params=endpoint.extra_params,
            )
    else:
        log_event("kodit.enrichment", {"provider": "local"})
        enrichment_provider = LocalEnrichmentProvider()

    return EnrichmentDomainService(enrichment_provider=enrichment_provider)
