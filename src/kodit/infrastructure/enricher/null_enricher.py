"""Null enricher implementation."""

from collections.abc import AsyncGenerator

import structlog

from kodit.domain.enrichments.enricher import Enricher
from kodit.domain.value_objects import (
    GenericEnrichmentRequest,
    GenericEnrichmentResponse,
)


class NullEnricher(Enricher):
    """Null enricher that returns empty responses."""

    def __init__(self) -> None:
        """Initialize the null enricher."""
        self.log = structlog.get_logger(__name__)

    async def enrich(
        self, requests: list[GenericEnrichmentRequest]
    ) -> AsyncGenerator[GenericEnrichmentResponse, None]:
        """Return empty responses for all requests.

        Args:
            requests: List of generic enrichment requests.

        Yields:
            Empty generic enrichment responses.

        """
        self.log.info("NullEnricher: returning empty responses", count=len(requests))
        for request in requests:
            yield GenericEnrichmentResponse(
                id=request.id,
                text="",
            )
