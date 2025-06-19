"""Null enrichment provider for testing."""

from collections.abc import AsyncGenerator

from kodit.domain.services.enrichment_service import EnrichmentProvider
from kodit.domain.value_objects import EnrichmentRequest, EnrichmentResponse


class NullEnrichmentProvider(EnrichmentProvider):
    """Null enrichment provider that returns empty responses."""

    async def enrich(
        self, requests: list[EnrichmentRequest]
    ) -> AsyncGenerator[EnrichmentResponse, None]:
        """Return empty responses for all requests.

        Args:
            requests: List of enrichment requests.

        Yields:
            Empty enrichment responses.

        """
        for request in requests:
            yield EnrichmentResponse(snippet_id=request.snippet_id, text="")
