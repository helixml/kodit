"""Generic enrichment response value object."""

from dataclasses import dataclass


@dataclass
class EnrichmentResponse:
    """Domain model for generic enrichment response."""

    id: str
    text: str
