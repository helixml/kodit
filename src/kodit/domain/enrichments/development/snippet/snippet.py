"""Snippet enrichment domain entity."""

from dataclasses import dataclass

from kodit.domain.enrichments.development.development import DevelopmentEnrichment

ENRICHMENT_SUBTYPE_SNIPPET_SUMMARY = "snippet_summary"


@dataclass
class SnippetEnrichment(DevelopmentEnrichment):
    """Enrichment specific to code snippets."""

    @property
    def subtype(self) -> str | None:
        """Return the enrichment subtype."""
        return ENRICHMENT_SUBTYPE_SNIPPET_SUMMARY

    def entity_type_key(self) -> str:
        """Return the entity type key this enrichment is for."""
        return "snippet_v2"
