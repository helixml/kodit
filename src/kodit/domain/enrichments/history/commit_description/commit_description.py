"""Commit description enrichment domain entity."""

from dataclasses import dataclass

from kodit.domain.enrichments.history.history import HistoryEnrichment

ENRICHMENT_SUBTYPE_COMMIT_DESCRIPTION = "commit_description"


@dataclass(frozen=True)
class CommitDescriptionEnrichment(HistoryEnrichment):
    """Enrichment containing a description of what a commit did."""

    @property
    def subtype(self) -> str | None:
        """Return the enrichment subtype."""
        return ENRICHMENT_SUBTYPE_COMMIT_DESCRIPTION
