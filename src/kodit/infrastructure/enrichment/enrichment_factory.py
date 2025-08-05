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
    OPENAI_NUM_PARALLEL_TASKS,
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
        # Use new httpx-based provider with socket support
        enrichment_provider = OpenAIEnrichmentProvider(
            api_key=endpoint.api_key,
            base_url=endpoint.base_url or "https://api.openai.com/v1",
            model_name=endpoint.model or "gpt-4o-mini",
            num_parallel_tasks=endpoint.num_parallel_tasks or OPENAI_NUM_PARALLEL_TASKS,
            socket_path=endpoint.socket_path,
            timeout=endpoint.timeout or 30.0,
        )
    else:
        log_event("kodit.enrichment", {"provider": "local"})
        enrichment_provider = LocalEnrichmentProvider()

    return EnrichmentDomainService(enrichment_provider=enrichment_provider)
