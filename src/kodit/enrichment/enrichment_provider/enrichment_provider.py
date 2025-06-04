"""Enrichment provider."""

from abc import ABC, abstractmethod

ENRICHMENT_SYSTEM_PROMPT = """
You are a professional software developer. You will be given a snippet of code.
Please provide a concise explanation of the code.
"""


class EnrichmentProvider(ABC):
    """Enrichment provider."""

    @abstractmethod
    async def enrich(self, data: list[str]) -> list[str]:
        """Enrich a list of strings."""
