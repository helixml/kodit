"""History enrichment domain entity."""

from abc import ABC
from dataclasses import dataclass

from kodit.domain.enrichments.enrichment import CommitEnrichment

ENRICHMENT_TYPE_HISTORY = "history"


@dataclass(frozen=True)
class HistoryEnrichment(CommitEnrichment, ABC):
    """Enrichment containing historical information for a commit."""

    @property
    def type(self) -> str:
        """Return the enrichment type."""
        return ENRICHMENT_TYPE_HISTORY
