"""Legacy enrichment models for backward compatibility."""

from abc import ABC, abstractmethod
from collections.abc import AsyncGenerator
from dataclasses import dataclass


@dataclass
class EnrichmentRequest:
    """Legacy enrichment request model."""

    snippet_id: int
    text: str


@dataclass
class EnrichmentResponse:
    """Legacy enrichment response model."""

    snippet_id: int
    text: str


class EnrichmentService(ABC):
    """Legacy enrichment service interface."""

    @abstractmethod
    def enrich(
        self, data: list[EnrichmentRequest]
    ) -> AsyncGenerator[EnrichmentResponse, None]:
        """Enrich a list of requests."""


class NullEnrichmentService(EnrichmentService):
    """Null enrichment service for testing."""

    async def enrich(
        self, data: list[EnrichmentRequest]
    ) -> AsyncGenerator[EnrichmentResponse, None]:
        """Return empty responses for all requests."""
        for request in data:
            yield EnrichmentResponse(snippet_id=request.snippet_id, text="")
