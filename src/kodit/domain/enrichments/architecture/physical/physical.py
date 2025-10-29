"""Physical architecture enrichment domain entity."""

from dataclasses import dataclass

from kodit.domain.enrichments.architecture.architecture import ArchitectureEnrichment

ENRICHMENT_SUBTYPE_PHYSICAL = "physical"


@dataclass(frozen=True)
class PhysicalArchitectureEnrichment(ArchitectureEnrichment):
    """Enrichment containing physical architecture discovery for a commit."""

    @property
    def subtype(self) -> str | None:
        """Return the enrichment subtype."""
        return ENRICHMENT_SUBTYPE_PHYSICAL
