"""EnrichmentV2 repository."""

from collections.abc import Callable

from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.enrichments.architecture.architecture import (
    ENRICHMENT_TYPE_ARCHITECTURE,
)
from kodit.domain.enrichments.architecture.physical.physical import (
    ENRICHMENT_SUBTYPE_PHYSICAL,
    PhysicalArchitectureEnrichment,
)
from kodit.domain.enrichments.development.development import ENRICHMENT_TYPE_DEVELOPMENT
from kodit.domain.enrichments.development.snippet.snippet import (
    ENRICHMENT_SUBTYPE_SNIPPET,
    ENRICHMENT_SUBTYPE_SNIPPET_SUMMARY,
    SnippetEnrichment,
    SnippetEnrichmentSummary,
)
from kodit.domain.enrichments.enrichment import EnrichmentV2
from kodit.domain.enrichments.usage.api_docs import (
    ENRICHMENT_SUBTYPE_API_DOCS,
    APIDocEnrichment,
)
from kodit.domain.enrichments.usage.usage import ENRICHMENT_TYPE_USAGE
from kodit.domain.protocols import EnrichmentV2Repository
from kodit.infrastructure.sqlalchemy import entities as db_entities
from kodit.infrastructure.sqlalchemy.repository import SqlAlchemyRepository
from kodit.infrastructure.sqlalchemy.unit_of_work import SqlAlchemyUnitOfWork


def create_enrichment_v2_repository(
    session_factory: Callable[[], AsyncSession],
) -> EnrichmentV2Repository:
    """Create a enrichment v2 repository."""
    return SQLAlchemyEnrichmentV2Repository(session_factory=session_factory)


class SQLAlchemyEnrichmentV2Repository(
    SqlAlchemyRepository[EnrichmentV2, db_entities.EnrichmentV2], EnrichmentV2Repository
):
    """Repository for managing enrichments and their associations."""

    def _get_id(self, entity: EnrichmentV2) -> int | None:
        """Extract ID from domain entity."""
        return entity.id

    @property
    def db_entity_type(self) -> type[db_entities.EnrichmentV2]:
        """The SQLAlchemy model type."""
        return db_entities.EnrichmentV2

    async def save(self, entity: EnrichmentV2) -> EnrichmentV2:
        """Save entity (create new or update existing)."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            entity_id = self._get_id(entity)
            existing_db_entity = await session.get(self.db_entity_type, entity_id)

            if existing_db_entity:
                # Update existing entity
                new_db_entity = self.to_db(entity)
                self._update_db_entity(existing_db_entity, new_db_entity)
                db_entity = existing_db_entity
            else:
                # Create new entity
                db_entity = self.to_db(entity)
                session.add(db_entity)

            await session.flush()
            return self.to_domain(db_entity)

    @staticmethod
    def to_db(domain_entity: EnrichmentV2) -> db_entities.EnrichmentV2:
        """Convert domain enrichment to database entity."""
        enrichment = db_entities.EnrichmentV2(
            type=domain_entity.type,
            subtype=domain_entity.subtype,
            content=domain_entity.content,
        )
        if domain_entity.id is not None:
            enrichment.id = domain_entity.id
        return enrichment

    @staticmethod
    def to_domain(db_entity: db_entities.EnrichmentV2) -> EnrichmentV2:
        """Convert database enrichment to domain entity."""
        # Use the stored type and subtype to determine the correct domain class
        if (
            db_entity.type == ENRICHMENT_TYPE_DEVELOPMENT
            and db_entity.subtype == ENRICHMENT_SUBTYPE_SNIPPET_SUMMARY
        ):
            return SnippetEnrichmentSummary(
                id=db_entity.id,
                content=db_entity.content,
                created_at=db_entity.created_at,
                updated_at=db_entity.updated_at,
            )
        if (
            db_entity.type == ENRICHMENT_TYPE_DEVELOPMENT
            and db_entity.subtype == ENRICHMENT_SUBTYPE_SNIPPET
        ):
            return SnippetEnrichment(
                id=db_entity.id,
                content=db_entity.content,
                created_at=db_entity.created_at,
                updated_at=db_entity.updated_at,
            )
        if (
            db_entity.type == ENRICHMENT_TYPE_USAGE
            and db_entity.subtype == ENRICHMENT_SUBTYPE_API_DOCS
        ):
            return APIDocEnrichment(
                id=db_entity.id,
                content=db_entity.content,
                created_at=db_entity.created_at,
                updated_at=db_entity.updated_at,
            )
        if (
            db_entity.type == ENRICHMENT_TYPE_ARCHITECTURE
            and db_entity.subtype == ENRICHMENT_SUBTYPE_PHYSICAL
        ):
            return PhysicalArchitectureEnrichment(
                id=db_entity.id,
                content=db_entity.content,
                created_at=db_entity.created_at,
                updated_at=db_entity.updated_at,
            )

        raise ValueError(
            f"Unknown enrichment type: {db_entity.type}/{db_entity.subtype}"
        )

    async def get_for_commit(
        self,
        commit_sha: str,
        enrichment_type: str | None = None,
        enrichment_subtype: str | None = None,
    ) -> list[EnrichmentV2]:
        """Get enrichments for a commit, optionally filtered by type/subtype."""
        from kodit.infrastructure.sqlalchemy.query import (
            FilterOperator,
            QueryBuilder,
        )

        # Get associations for this commit
        async with SqlAlchemyUnitOfWork(self.session_factory):
            from kodit.infrastructure.sqlalchemy.enrichment_association_repository import (  # noqa: E501
                SQLAlchemyEnrichmentAssociationRepository,
            )

            assoc_repo = SQLAlchemyEnrichmentAssociationRepository(
                self.session_factory
            )
            associations = await assoc_repo.associations_for_commit(commit_sha)

            if not associations:
                return []

            # Build query for enrichments
            query = QueryBuilder().filter(
                "id", FilterOperator.IN, [a.enrichment_id for a in associations]
            )

            # Add type/subtype filters if specified
            if enrichment_type:
                query = query.filter(
                    db_entities.EnrichmentV2.type.key,
                    FilterOperator.EQ,
                    enrichment_type,
                )
            if enrichment_subtype:
                query = query.filter(
                    db_entities.EnrichmentV2.subtype.key,
                    FilterOperator.EQ,
                    enrichment_subtype,
                )

            return await self.find(query)

    async def get_by_ids(self, enrichment_ids: list[int]) -> list[EnrichmentV2]:
        """Get enrichments by their IDs."""
        if not enrichment_ids:
            return []

        from kodit.infrastructure.sqlalchemy.query import (
            FilterOperator,
            QueryBuilder,
        )

        return await self.find(
            QueryBuilder().filter(
                db_entities.EnrichmentV2.id.key,
                FilterOperator.IN,
                enrichment_ids,
            )
        )

    async def get_pointing_to_enrichments(
        self, target_enrichment_ids: list[int]
    ) -> dict[int, list[EnrichmentV2]]:
        """Get enrichments that point to the given enrichments, grouped by target."""
        if not target_enrichment_ids:
            return {}

        from kodit.infrastructure.sqlalchemy.enrichment_association_repository import (
            SQLAlchemyEnrichmentAssociationRepository,
        )

        # Get associations pointing to these enrichments
        assoc_repo = SQLAlchemyEnrichmentAssociationRepository(self.session_factory)
        associations = await assoc_repo.pointing_to_enrichments(target_enrichment_ids)

        if not associations:
            return {eid: [] for eid in target_enrichment_ids}

        # Get the enrichments referenced by these associations
        enrichment_ids = [a.enrichment_id for a in associations]
        enrichments = await self.get_by_ids(enrichment_ids)

        # Create lookup map
        enrichment_map = {e.id: e for e in enrichments if e.id is not None}

        # Group by target enrichment ID
        result: dict[int, list[EnrichmentV2]] = {
            eid: [] for eid in target_enrichment_ids
        }
        for association in associations:
            target_id = int(association.entity_id)
            if target_id in result and association.enrichment_id in enrichment_map:
                result[target_id].append(enrichment_map[association.enrichment_id])

        return result
