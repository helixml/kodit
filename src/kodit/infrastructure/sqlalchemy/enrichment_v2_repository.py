"""EnrichmentV2 repository."""

from collections.abc import Callable, Sequence

import structlog
from sqlalchemy import delete, select
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.enrichments.enrichment import EnrichmentV2
from kodit.infrastructure.mappers.enrichment_mapper import EnrichmentMapper
from kodit.infrastructure.sqlalchemy import entities as db_entities
from kodit.infrastructure.sqlalchemy.unit_of_work import SqlAlchemyUnitOfWork


class EnrichmentV2Repository:
    """Repository for managing enrichments and their associations."""

    def __init__(
        self,
        session_factory: Callable[[], AsyncSession],
    ) -> None:
        """Initialize the repository."""
        self.session_factory = session_factory
        self.mapper = EnrichmentMapper()
        self.log = structlog.get_logger(__name__)

    async def enrichments_for_entity_type(
        self,
        entity_type: str,
        entity_ids: list[str],
    ) -> list[EnrichmentV2]:
        """Get all enrichments for multiple entities of the same type."""
        if not entity_ids:
            return []

        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            stmt = (
                select(
                    db_entities.EnrichmentV2,
                    db_entities.EnrichmentAssociation.entity_id,
                )
                .join(db_entities.EnrichmentAssociation)
                .where(
                    db_entities.EnrichmentAssociation.entity_type == entity_type,
                    db_entities.EnrichmentAssociation.entity_id.in_(entity_ids),
                )
            )

            result = await session.execute(stmt)
            rows = result.all()

            return [
                self.mapper.to_domain(db_enrichment, entity_type, entity_id)
                for db_enrichment, entity_id in rows
            ]

    async def bulk_save_enrichments(
        self,
        enrichments: Sequence[EnrichmentV2],
    ) -> None:
        """Bulk save enrichments with their associations."""
        if not enrichments:
            return

        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            enrichment_records = []
            for enrichment in enrichments:
                db_enrichment = db_entities.EnrichmentV2(
                    type=enrichment.type,
                    subtype=enrichment.subtype,
                    content=enrichment.content,
                )
                session.add(db_enrichment)
                enrichment_records.append((enrichment, db_enrichment))

            await session.flush()

            for enrichment, db_enrichment in enrichment_records:
                db_association = db_entities.EnrichmentAssociation(
                    enrichment_id=db_enrichment.id,
                    entity_type=enrichment.entity_type_key(),
                    entity_id=enrichment.entity_id,
                )
                session.add(db_association)

    async def bulk_delete_enrichments(
        self,
        entity_type: str,
        entity_ids: list[str],
    ) -> None:
        """Bulk delete enrichments for multiple entities of the same type."""
        if not entity_ids:
            return

        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            stmt = select(db_entities.EnrichmentAssociation.enrichment_id).where(
                db_entities.EnrichmentAssociation.entity_type == entity_type,
                db_entities.EnrichmentAssociation.entity_id.in_(entity_ids),
            )
            result = await session.execute(stmt)
            enrichment_ids = result.scalars().all()

            if enrichment_ids:
                await session.execute(
                    delete(db_entities.EnrichmentV2).where(
                        db_entities.EnrichmentV2.id.in_(enrichment_ids)
                    )
                )

    async def delete_enrichment(self, enrichment_id: int) -> bool:
        """Delete a specific enrichment by ID."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            result = await session.execute(
                delete(db_entities.EnrichmentV2).where(
                    db_entities.EnrichmentV2.id == enrichment_id
                )
            )
            return result.rowcount > 0
