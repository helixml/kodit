"""Enrichment association repository."""

from collections.abc import Callable

import structlog
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.enrichments.enrichment import (
    EnrichmentAssociation,
)
from kodit.domain.protocols import EnrichmentAssociationRepository
from kodit.infrastructure.sqlalchemy import entities as db_entities
from kodit.infrastructure.sqlalchemy.query import (
    EnrichmentAssociationQueryBuilder,
    FilterOperator,
    QueryBuilder,
)
from kodit.infrastructure.sqlalchemy.repository import SqlAlchemyRepository


def create_enrichment_association_repository(
    session_factory: Callable[[], AsyncSession],
) -> EnrichmentAssociationRepository:
    """Create a enrichment association repository."""
    return SQLAlchemyEnrichmentAssociationRepository(session_factory=session_factory)


class SQLAlchemyEnrichmentAssociationRepository(
    SqlAlchemyRepository[EnrichmentAssociation, db_entities.EnrichmentAssociation],
    EnrichmentAssociationRepository,
):
    """Repository for managing enrichment associations."""

    def __init__(self, session_factory: Callable[[], AsyncSession]) -> None:
        """Initialize the repository."""
        super().__init__(session_factory=session_factory)
        self._log = structlog.get_logger(__name__)

    def _get_id(self, entity: EnrichmentAssociation) -> int | None:
        """Get the ID of an enrichment association."""
        return entity.id

    @property
    def db_entity_type(self) -> type[db_entities.EnrichmentAssociation]:
        """The SQLAlchemy model type."""
        return db_entities.EnrichmentAssociation

    @staticmethod
    def to_domain(
        db_entity: db_entities.EnrichmentAssociation,
    ) -> EnrichmentAssociation:
        """Map database entity to domain entity."""
        return EnrichmentAssociation(
            enrichment_id=db_entity.enrichment_id,
            entity_type=db_entity.entity_type,
            entity_id=db_entity.entity_id,
            id=db_entity.id,
        )

    @staticmethod
    def to_db(
        domain_entity: EnrichmentAssociation,
    ) -> db_entities.EnrichmentAssociation:
        """Map domain entity to database entity."""
        from datetime import UTC, datetime

        now = datetime.now(UTC)
        db_entity = db_entities.EnrichmentAssociation(
            enrichment_id=domain_entity.enrichment_id,
            entity_type=domain_entity.entity_type,
            entity_id=domain_entity.entity_id,
        )
        if domain_entity.id is not None:
            db_entity.id = domain_entity.id
        # Always set timestamps since domain entity doesn't track them
        db_entity.created_at = now
        db_entity.updated_at = now
        return db_entity

    # A method to return all snippet summary enrichment associations along with the
    # parent snippet association
    async def associations_for_summaries(
        self, summary_enrichment_ids: list[int]
    ) -> list[EnrichmentAssociation]:
        """Get the snippet associations for the given summary enrichments."""
        # 1. Find the associations for the given enrichments
        self._log.info(
            "finding associations for summary enrichments",
            summary_enrichment_ids=summary_enrichment_ids,
        )
        associations = await self.find(
            EnrichmentAssociationQueryBuilder.associations_pointing_to_these_enrichments(
                enrichment_ids=summary_enrichment_ids,
            )
        )
        self._log.info(
            "found associations for summary enrichments",
            associations=associations,
        )

        # 2. Pull out the snippet enrichments from these associations
        snippet_enrichment_ids = [
            int(association.entity_id)
            for association in associations
            if association.entity_type == db_entities.EnrichmentV2.__tablename__
        ]
        self._log.info(
            "found snippet enrichment ids",
            snippet_enrichment_ids=snippet_enrichment_ids,
        )

        # 3. Get the associations that point to these snippet enrichments
        snippet_associations = await self.find(
            QueryBuilder().filter(
                db_entities.EnrichmentAssociation.enrichment_id.key,
                FilterOperator.IN,
                snippet_enrichment_ids,
            )
        )
        self._log.info(
            "found snippet associations",
            snippet_associations=snippet_associations,
        )
        return snippet_associations

    async def associations_for_commit(
        self, commit_sha: str
    ) -> list[EnrichmentAssociation]:
        """Get the snippet associations for the given commit."""
        return await self.find(
            EnrichmentAssociationQueryBuilder.for_enrichment_associations(
                entity_type=db_entities.GitCommit.__tablename__,
                entity_ids=[commit_sha],
            )
        )
