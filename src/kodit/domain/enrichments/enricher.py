"""Enricher interface."""

from collections.abc import AsyncGenerator
from typing import Protocol

from kodit.domain.enrichments.request import EnrichmentRequest
from kodit.domain.enrichments.response import EnrichmentResponse


class Enricher(Protocol):
    """Interface for text enrichment with custom prompts."""

    def enrich(
        self, requests: list[EnrichmentRequest]
    ) -> AsyncGenerator[EnrichmentResponse, None]:
        """Enrich a list of requests with custom system prompts."""
        ...
