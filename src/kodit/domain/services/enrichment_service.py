"""Domain service for enrichment operations."""

from abc import ABC, abstractmethod
from collections.abc import AsyncGenerator

from kodit.domain.value_objects import (
    EnrichmentIndexRequest,
    EnrichmentRequest,
    EnrichmentResponse,
)


class EnrichmentProvider(ABC):
    """Abstract enrichment provider interface."""

    @abstractmethod
    def enrich(
        self, requests: list[EnrichmentRequest]
    ) -> AsyncGenerator[EnrichmentResponse, None]:
        """Enrich a list of requests."""


class EnrichmentDomainService:
    """Domain service for enrichment operations."""

    def __init__(self, enrichment_provider: EnrichmentProvider) -> None:
        """Initialize the enrichment domain service.

        Args:
            enrichment_provider: The enrichment provider to use.

        """
        self.enrichment_provider = enrichment_provider

    async def enrich_documents(
        self, request: EnrichmentIndexRequest
    ) -> AsyncGenerator[EnrichmentResponse, None]:
        """Enrich documents using the enrichment provider.

        Args:
            request: The enrichment index request.

        Yields:
            Enrichment responses as they are processed.

        """
        async for response in self.enrichment_provider.enrich(request.requests):
            yield response
