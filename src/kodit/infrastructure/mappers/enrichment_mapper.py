"""Enrichment mapper."""

from kodit.domain.entities import enrichment
from kodit.domain.entities.enrichment import (
    EnrichmentV2,
    PhysicalArchitectureEnrichment,
    SnippetEnrichment,
)
from kodit.infrastructure.sqlalchemy import entities as db_entities


class EnrichmentMapper:
    """Maps between domain enrichment entities and database entities."""

    @staticmethod
    def to_database(domain_enrichment: EnrichmentV2) -> db_entities.EnrichmentV2:
        """Convert domain enrichment to database entity."""
        return db_entities.EnrichmentV2(
            id=domain_enrichment.id,
            type=domain_enrichment.type,
            subtype=domain_enrichment.subtype,
            content=domain_enrichment.content,
            created_at=domain_enrichment.created_at,
            updated_at=domain_enrichment.updated_at,
        )

    @staticmethod
    def to_domain(
        db_enrichment: db_entities.EnrichmentV2,
        entity_type: str,  # noqa: ARG004
        entity_id: str,
    ) -> EnrichmentV2:
        """Convert database enrichment to domain entity."""
        # Use the stored type and subtype to determine the correct domain class
        if (
            db_enrichment.type == enrichment.ENRICHMENT_TYPE_DEVELOPMENT
            and db_enrichment.subtype == enrichment.ENRICHMENT_SUBTYPE_SNIPPET
        ):
            return SnippetEnrichment(
                id=db_enrichment.id,
                entity_id=entity_id,
                content=db_enrichment.content,
                created_at=db_enrichment.created_at,
                updated_at=db_enrichment.updated_at,
            )
        if (
            db_enrichment.type == enrichment.ENRICHMENT_TYPE_ARCHITECTURE
            and db_enrichment.subtype == enrichment.ENRICHMENT_SUBTYPE_PHYSICAL
        ):
            return PhysicalArchitectureEnrichment(
                id=db_enrichment.id,
                entity_id=entity_id,
                content=db_enrichment.content,
                created_at=db_enrichment.created_at,
                updated_at=db_enrichment.updated_at,
            )

        raise ValueError(
            f"Unknown enrichment type: {db_enrichment.type}/{db_enrichment.subtype}"
        )
