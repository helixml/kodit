"""API documentation enrichment entity."""

from dataclasses import dataclass

from kodit.domain.enrichments.usage.usage import UsageEnrichment

ENRICHMENT_SUBTYPE_API_DOCS = "api_docs"


@dataclass(frozen=True)
class APIDocEnrichment(UsageEnrichment):
    """API documentation enrichment for a module."""

    language: str = ""

    @property
    def subtype(self) -> str | None:
        """Return the enrichment subtype."""
        return ENRICHMENT_SUBTYPE_API_DOCS
