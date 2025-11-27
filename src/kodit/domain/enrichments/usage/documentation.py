"""Documentation enrichment entity."""

from dataclasses import dataclass

from kodit.domain.enrichments.usage.usage import UsageEnrichment

ENRICHMENT_SUBTYPE_DOCUMENTATION = "documentation"


@dataclass(frozen=True)
class DocumentationEnrichment(UsageEnrichment):
    """Documentation enrichment containing chunked documentation content."""

    @property
    def subtype(self) -> str | None:
        """Return the enrichment subtype."""
        return ENRICHMENT_SUBTYPE_DOCUMENTATION
