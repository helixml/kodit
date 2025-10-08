"""Domain service for enrichment operations."""

from collections.abc import AsyncGenerator

from kodit.domain.enrichments.enricher import Enricher
from kodit.domain.enrichments.request import EnrichmentRequest
from kodit.domain.enrichments.response import EnrichmentResponse


class EnrichmentDomainService:
    """Domain service for enrichment operations."""

    def __init__(self, enricher: Enricher) -> None:
        """Initialize the enrichment domain service."""
        self.enricher = enricher

    async def enrich_documents(
        self, requests: list[EnrichmentRequest]
    ) -> AsyncGenerator[EnrichmentResponse, None]:
        """Enrich documents using the enricher.

        Yields:
            Enrichment responses as they are processed.

        """
        async for response in self.enricher.enrich(requests):
            yield response
