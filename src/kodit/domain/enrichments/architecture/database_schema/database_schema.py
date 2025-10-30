"""Database schema enrichment domain entity."""

from dataclasses import dataclass

from kodit.domain.enrichments.architecture.architecture import ArchitectureEnrichment

ENRICHMENT_SUBTYPE_DATABASE_SCHEMA = "database_schema"


@dataclass(frozen=True)
class DatabaseSchemaEnrichment(ArchitectureEnrichment):
    """Enrichment containing database schema information for a commit."""

    @property
    def subtype(self) -> str | None:
        """Return the enrichment subtype."""
        return ENRICHMENT_SUBTYPE_DATABASE_SCHEMA
