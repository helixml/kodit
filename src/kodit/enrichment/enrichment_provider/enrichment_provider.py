"""Enrichment provider."""

from abc import ABC, abstractmethod
from collections.abc import AsyncGenerator
from dataclasses import dataclass

ENRICHMENT_SYSTEM_PROMPT = """
You are a professional software developer. You will be given a snippet of code.
Please provide a concise explanation of the code.
"""


@dataclass
class EnrichmentRequest:
    """Enrichment request."""

    snippet_id: int
    text: str


@dataclass
class EnrichmentResponse:
    """Enrichment response."""

    snippet_id: int
    text: str


class EnrichmentProvider(ABC):
    """Enrichment provider."""

    @abstractmethod
    def enrich(
        self, data: list[EnrichmentRequest]
    ) -> AsyncGenerator[EnrichmentResponse, None]:
        """Enrich a list of strings."""
