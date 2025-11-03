"""Cookbook enrichment entity."""

from dataclasses import dataclass

from kodit.domain.enrichments.usage.usage import UsageEnrichment

ENRICHMENT_SUBTYPE_COOKBOOK = "cookbook"


@dataclass(frozen=True)
class CookbookEnrichment(UsageEnrichment):
    """Cookbook enrichment containing usage examples for a repository."""

    @property
    def subtype(self) -> str | None:
        """Return the enrichment subtype."""
        return ENRICHMENT_SUBTYPE_COOKBOOK
