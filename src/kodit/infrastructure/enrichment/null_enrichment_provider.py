"""Null enrichment provider for testing."""

import re
from collections.abc import AsyncGenerator

from kodit.domain.services.enrichment_service import EnrichmentProvider
from kodit.domain.value_objects import EnrichmentRequest, EnrichmentResponse


class NullEnrichmentProvider(EnrichmentProvider):
    """Null enrichment provider that returns empty responses."""

    async def enrich(
        self, requests: list[EnrichmentRequest]
    ) -> AsyncGenerator[EnrichmentResponse, None]:
        """Only keep alphabetic characters."""
        for request in requests:
            response = re.sub(r"[^a-zA-Z]", " ", request.text)
            yield EnrichmentResponse(snippet_id=request.snippet_id, text=response)
