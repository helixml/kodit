"""Repository structure enrichment domain entity."""

from dataclasses import dataclass

from kodit.domain.enrichments.architecture.architecture import ArchitectureEnrichment

ENRICHMENT_SUBTYPE_REPOSITORY_STRUCTURE = "repository_structure"


@dataclass(frozen=True)
class RepositoryStructureEnrichment(ArchitectureEnrichment):
    """Enrichment containing intelligent repository structure tree for a commit."""

    @property
    def subtype(self) -> str | None:
        """Return the enrichment subtype."""
        return ENRICHMENT_SUBTYPE_REPOSITORY_STRUCTURE
