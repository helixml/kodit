"""Generic enrichment request value object."""

from dataclasses import dataclass


@dataclass
class EnrichmentRequest:
    """Domain model for generic enrichment request with custom prompt."""

    id: str
    text: str
    system_prompt: str
