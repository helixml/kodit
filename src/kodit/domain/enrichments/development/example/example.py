"""Example enrichment domain entity."""

from dataclasses import dataclass

from kodit.domain.enrichments.development.development import DevelopmentEnrichment

ENRICHMENT_SUBTYPE_EXAMPLE_SUMMARY = "example_summary"
ENRICHMENT_SUBTYPE_EXAMPLE = "example"


@dataclass(frozen=True)
class ExampleSummaryEnrichment(DevelopmentEnrichment):
    """Enrichment containing AI-generated summary for an example."""

    @property
    def subtype(self) -> str | None:
        """Return the enrichment subtype."""
        return ENRICHMENT_SUBTYPE_EXAMPLE_SUMMARY


@dataclass(frozen=True)
class ExampleEnrichment(DevelopmentEnrichment):
    """Enrichment containing code examples from repositories."""

    @property
    def subtype(self) -> str | None:
        """Return the enrichment subtype."""
        return ENRICHMENT_SUBTYPE_EXAMPLE
