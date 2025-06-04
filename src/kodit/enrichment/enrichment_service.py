"""Enrichment service."""

from abc import ABC, abstractmethod

from kodit.enrichment.enrichment_provider.enrichment_provider import EnrichmentProvider


class EnrichmentService(ABC):
    """Enrichment service."""

    @abstractmethod
    async def enrich(self, data: list[str]) -> list[str]:
        """Enrich a list of strings."""


class NullEnrichmentService(EnrichmentService):
    """Null enrichment service."""

    async def enrich(self, data: list[str]) -> list[str]:
        """Enrich a list of strings."""
        return [""] * len(data)


class LLMEnrichmentService(EnrichmentService):
    """Enrichment service using an LLM."""

    def __init__(self, enrichment_provider: EnrichmentProvider) -> None:
        """Initialize the enrichment service."""
        self.enrichment_provider = enrichment_provider

    async def enrich(self, data: list[str]) -> list[str]:
        """Enrich a list of strings."""
        return await self.enrichment_provider.enrich(data)
