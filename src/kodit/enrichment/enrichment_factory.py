"""Embedding service."""

from kodit.config import AppContext
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


def enrichment_factory(app_context: AppContext) -> EnrichmentService:
    """Create an embedding service."""
    openai_client = app_context.get_default_openai_client()
    if openai_client is not None:
        enrichment_provider = OpenAIEnrichmentProvider(openai_client=openai_client)
        return LLMEnrichmentService(enrichment_provider)

    return LLMEnrichmentService(LocalEnrichmentProvider())
