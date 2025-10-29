"""Architecture enrichment domain entity."""

from abc import ABC
from dataclasses import dataclass

from kodit.domain.enrichments.enrichment import (
    CommitEnrichment,
)

ENRICHMENT_TYPE_ARCHITECTURE = "architecture"


@dataclass(frozen=True)
class ArchitectureEnrichment(CommitEnrichment, ABC):
    """Enrichment containing physical architecture discovery for a commit."""

    @property
    def type(self) -> str:
        """Return the enrichment type."""
        return ENRICHMENT_TYPE_ARCHITECTURE
