"""Enrichment service."""

from abc import ABC, abstractmethod
from collections.abc import AsyncGenerator

from kodit.enrichment.enrichment_provider.enrichment_provider import (
    EnrichmentProvider,
    EnrichmentRequest,
    EnrichmentResponse,
)


class EnrichmentService(ABC):
    """Enrichment service."""

    @abstractmethod
    def enrich(
        self, data: list[EnrichmentRequest]
    ) -> AsyncGenerator[EnrichmentResponse, None]:
        """Enrich a list of strings."""


class NullEnrichmentService(EnrichmentService):
    """Null enrichment service."""

    async def enrich(
        self, data: list[EnrichmentRequest]
    ) -> AsyncGenerator[EnrichmentResponse, None]:
        """Enrich a list of strings."""
        for request in data:
            yield EnrichmentResponse(snippet_id=request.snippet_id, text="")


class LLMEnrichmentService(EnrichmentService):
    """Enrichment service using an LLM."""

    def __init__(self, enrichment_provider: EnrichmentProvider) -> None:
        """Initialize the enrichment service."""
        self.enrichment_provider = enrichment_provider

    def enrich(
        self, data: list[EnrichmentRequest]
    ) -> AsyncGenerator[EnrichmentResponse, None]:
        """Enrich a list of snippets."""
        return self.enrichment_provider.enrich(data)
