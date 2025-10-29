"""Usage enrichment domain entity."""

from abc import ABC
from dataclasses import dataclass

from kodit.domain.enrichments.enrichment import CommitEnrichment

ENRICHMENT_TYPE_USAGE = "usage"


@dataclass(frozen=True)
class UsageEnrichment(CommitEnrichment, ABC):
    """Enrichment containing development discovery for a commit."""

    @property
    def type(self) -> str:
        """Return the enrichment type."""
        return ENRICHMENT_TYPE_USAGE
