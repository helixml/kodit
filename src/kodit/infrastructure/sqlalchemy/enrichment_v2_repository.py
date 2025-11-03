"""EnrichmentV2 repository."""

from collections.abc import Callable

from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.enrichments.architecture.architecture import (
    ENRICHMENT_TYPE_ARCHITECTURE,
)
from kodit.domain.enrichments.architecture.database_schema.database_schema import (
    ENRICHMENT_SUBTYPE_DATABASE_SCHEMA,
    DatabaseSchemaEnrichment,
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
from kodit.domain.enrichments.history.commit_description.commit_description import (
    ENRICHMENT_SUBTYPE_COMMIT_DESCRIPTION,
    CommitDescriptionEnrichment,
)
from kodit.domain.enrichments.history.history import ENRICHMENT_TYPE_HISTORY
from kodit.domain.enrichments.usage.api_docs import (
    ENRICHMENT_SUBTYPE_API_DOCS,
    APIDocEnrichment,
)
from kodit.domain.enrichments.usage.cookbook import (
    ENRICHMENT_SUBTYPE_COOKBOOK,
    CookbookEnrichment,
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
            # Skip session.get if entity_id is None (new entity not yet persisted)
            existing_db_entity = (
                await session.get(self.db_entity_type, entity_id)
                if entity_id is not None
                else None
            )

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
    def to_domain(db_entity: db_entities.EnrichmentV2) -> EnrichmentV2:  # noqa: PLR0911
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
            db_entity.type == ENRICHMENT_TYPE_USAGE
            and db_entity.subtype == ENRICHMENT_SUBTYPE_COOKBOOK
        ):
            return CookbookEnrichment(
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
        if (
            db_entity.type == ENRICHMENT_TYPE_HISTORY
            and db_entity.subtype == ENRICHMENT_SUBTYPE_COMMIT_DESCRIPTION
        ):
            return CommitDescriptionEnrichment(
                id=db_entity.id,
                content=db_entity.content,
                created_at=db_entity.created_at,
                updated_at=db_entity.updated_at,
            )
        if (
            db_entity.type == ENRICHMENT_TYPE_ARCHITECTURE
            and db_entity.subtype == ENRICHMENT_SUBTYPE_DATABASE_SCHEMA
        ):
            return DatabaseSchemaEnrichment(
                id=db_entity.id,
                content=db_entity.content,
                created_at=db_entity.created_at,
                updated_at=db_entity.updated_at,
            )

        raise ValueError(
            f"Unknown enrichment type: {db_entity.type}/{db_entity.subtype}"
        )
