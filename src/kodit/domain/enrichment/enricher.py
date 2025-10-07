"""Enricher interface."""

from collections.abc import AsyncGenerator
from typing import Protocol

from kodit.domain.value_objects import (
    GenericEnrichmentRequest,
    GenericEnrichmentResponse,
)


class Enricher(Protocol):
    """Interface for text enrichment with custom prompts."""

    def enrich(
        self, requests: list[GenericEnrichmentRequest]
    ) -> AsyncGenerator[GenericEnrichmentResponse, None]:
        """Enrich a list of requests with custom system prompts."""
        ...
