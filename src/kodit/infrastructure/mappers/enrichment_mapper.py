"""Enrichment mapper."""

from kodit.domain.enrichments.architecture.architecture import (
    ENRICHMENT_TYPE_ARCHITECTURE,
)
from kodit.domain.enrichments.architecture.physical.physical import (
    ENRICHMENT_SUBTYPE_PHYSICAL,
    PhysicalArchitectureEnrichment,
)
from kodit.domain.enrichments.development.development import ENRICHMENT_TYPE_DEVELOPMENT
from kodit.domain.enrichments.development.snippet.snippet import (
    ENRICHMENT_SUBTYPE_SNIPPET_SUMMARY,
    SnippetEnrichment,
)
from kodit.domain.enrichments.enrichment import EnrichmentV2
from kodit.domain.enrichments.usage.api_docs import (
    ENRICHMENT_SUBTYPE_API_DOCS,
    APIDocEnrichment,
)
from kodit.domain.enrichments.usage.usage import ENRICHMENT_TYPE_USAGE
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
            db_enrichment.type == ENRICHMENT_TYPE_DEVELOPMENT
            and db_enrichment.subtype == ENRICHMENT_SUBTYPE_SNIPPET_SUMMARY
        ):
            return SnippetEnrichment(
                id=db_enrichment.id,
                entity_id=entity_id,
                content=db_enrichment.content,
                created_at=db_enrichment.created_at,
                updated_at=db_enrichment.updated_at,
            )
        if (
            db_enrichment.type == ENRICHMENT_TYPE_USAGE
            and db_enrichment.subtype == ENRICHMENT_SUBTYPE_API_DOCS
        ):
            return APIDocEnrichment(
                id=db_enrichment.id,
                entity_id=entity_id,
                content=db_enrichment.content,
                created_at=db_enrichment.created_at,
                updated_at=db_enrichment.updated_at,
            )
        if (
            db_enrichment.type == ENRICHMENT_TYPE_ARCHITECTURE
            and db_enrichment.subtype == ENRICHMENT_SUBTYPE_PHYSICAL
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
