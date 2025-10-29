"""Snippet enrichment domain entity."""

from dataclasses import dataclass

from kodit.domain.enrichments.development.development import DevelopmentEnrichment

ENRICHMENT_SUBTYPE_SNIPPET_SUMMARY = "snippet_summary"
ENRICHMENT_SUBTYPE_SNIPPET = "snippet"


@dataclass(frozen=True)
class SnippetEnrichmentSummary(DevelopmentEnrichment):
    """Enrichment specific to code snippets."""

    @property
    def subtype(self) -> str | None:
        """Return the enrichment subtype."""
        return ENRICHMENT_SUBTYPE_SNIPPET_SUMMARY


@dataclass(frozen=True)
class SnippetEnrichment(DevelopmentEnrichment):
    """Enrichment specific to code snippets."""

    @property
    def subtype(self) -> str | None:
        """Return the enrichment subtype."""
        return ENRICHMENT_SUBTYPE_SNIPPET
