"""Enrichment mapper."""

from kodit.domain.entities.enrichment import (
    CommitEnrichment,
    EnrichmentV2,
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
            content=domain_enrichment.content,
            created_at=domain_enrichment.created_at,
            updated_at=domain_enrichment.updated_at,
        )

    @staticmethod
    def to_domain(
        db_enrichment: db_entities.EnrichmentV2,
        entity_type: str,
        entity_id: str,
    ) -> EnrichmentV2:
        """Convert database enrichment to domain entity."""
        if entity_type == "snippet_v2":
            return SnippetEnrichment(
                id=db_enrichment.id,
                entity_id=entity_id,
                content=db_enrichment.content,
                created_at=db_enrichment.created_at,
                updated_at=db_enrichment.updated_at,
            )
        if entity_type == "git_commit":
            return CommitEnrichment(
                id=db_enrichment.id,
                entity_id=entity_id,
                content=db_enrichment.content,
                created_at=db_enrichment.created_at,
                updated_at=db_enrichment.updated_at,
            )
        raise ValueError(f"Unknown entity type: {entity_type}")
