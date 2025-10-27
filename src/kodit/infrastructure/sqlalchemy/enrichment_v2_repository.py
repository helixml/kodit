"""EnrichmentV2 repository."""

from collections.abc import Callable

import structlog
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.enrichments.enrichment import EnrichmentV2
from kodit.domain.protocols import EnrichmentV2Repository
from kodit.infrastructure.mappers.enrichment_mapper import EnrichmentMapper
from kodit.infrastructure.sqlalchemy import entities as db_entities
from kodit.infrastructure.sqlalchemy.repository import SqlAlchemyRepository


def create_enrichment_v2_repository(
    session_factory: Callable[[], AsyncSession],
) -> EnrichmentV2Repository:
    """Create a enrichment v2 repository."""
    return SQLAlchemyEnrichmentV2Repository(session_factory=session_factory)


class SQLAlchemyEnrichmentV2Repository(
    SqlAlchemyRepository[EnrichmentV2, db_entities.EnrichmentV2], EnrichmentV2Repository
):
    """Repository for managing enrichments and their associations."""

    def __init__(
        self,
        session_factory: Callable[[], AsyncSession],
    ) -> None:
        """Initialize the repository."""
        self.session_factory = session_factory
        self.mapper = EnrichmentMapper()
        self.log = structlog.get_logger(__name__)

    def _get_id(self, entity: EnrichmentV2) -> int | None:
        """Extract ID from domain entity."""
        return entity.id

    @property
    def db_entity_type(self) -> type[db_entities.EnrichmentV2]:
        """The SQLAlchemy model type."""
        return db_entities.EnrichmentV2

    def to_domain(self, db_entity: db_entities.EnrichmentV2) -> EnrichmentV2:
        """Map database entity to domain entity."""
        return self.mapper.to_domain(db_entity, "", "")

    def to_db(self, domain_entity: EnrichmentV2) -> db_entities.EnrichmentV2:
        """Map domain entity to database entity."""
        return db_entities.EnrichmentV2(
            type=domain_entity.type,
            subtype=domain_entity.subtype,
            content=domain_entity.content,
        )
