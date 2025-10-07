"""Enricher factory for creating generic enricher domain services."""

from kodit.config import AppContext, Endpoint
from kodit.domain.enrichments.enricher import Enricher
from kodit.infrastructure.enricher.litellm_enricher import LiteLLMEnricher
from kodit.infrastructure.enricher.local_enricher import LocalEnricher
from kodit.infrastructure.enricher.null_enricher import NullEnricher
from kodit.log import log_event


def _get_endpoint_configuration(app_context: AppContext) -> Endpoint | None:
    """Get the endpoint configuration for the enricher service.

    Args:
        app_context: The application context.

    Returns:
        The endpoint configuration or None.

    """
    return app_context.enrichment_endpoint or None


def enricher_domain_service_factory(
    app_context: AppContext,
    *,
    use_null_enricher: bool = False,
) -> Enricher:
    """Create an enricher domain service.

    Args:
        app_context: The application context.
        use_null_enricher: Whether to use the null enricher instead.

    Returns:
        An enricher domain service instance.

    """
    enricher: Enricher

    if use_null_enricher:
        log_event("kodit.enricher", {"provider": "null"})
        enricher = NullEnricher()
    else:
        endpoint = _get_endpoint_configuration(app_context)
        if endpoint:
            log_event("kodit.enricher", {"provider": "litellm"})
            enricher = LiteLLMEnricher(endpoint=endpoint)
        else:
            log_event("kodit.enricher", {"provider": "local"})
            enricher = LocalEnricher()

    return enricher
