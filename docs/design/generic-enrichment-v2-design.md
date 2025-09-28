# Generic Enrichment V2 Design Document

## Overview

This document outlines the implementation plan for a generic enrichment system (EnrichmentV2) that replaces the current snippet-specific enrichment model. The new design allows enrichments to be associated with any entity type (snippets, commits, files, etc.) through a polymorphic association pattern.

## Current State

The existing enrichment system is tightly coupled to snippets:

```python
# Domain (value_objects.py)
class Enrichment:
    type: EnrichmentType
    content: str

# Infrastructure (entities.py)
class Enrichment(Base, CommonMixin):
    snippet_sha: Mapped[str] = mapped_column(ForeignKey("snippets_v2.sha"))
    type: Mapped[EnrichmentType]
    content: Mapped[str]
```

**Limitations:**

- Cannot enrich commits, files, or other entities
- No extensibility for entity-specific enrichment behavior
- Tight coupling between enrichment and snippet

## Design Goals

1. **Generic Association**: Enrichments can be attached to any entity
2. **Type Safety**: Maintain strong typing for entity-specific enrichments
3. **Extensibility**: Easy to add new enrichment types
4. **Backward Compatibility**: Minimal disruption to existing code
5. **Clean Architecture**: Maintain DDD boundaries between domain and infrastructure

## Architecture

### Layer 1: Database Schema (Infrastructure)

#### SQLAlchemy Entities

The database schema will be managed by Alembic migrations. The SQLAlchemy entities define the structure:

```python
# src/kodit/infrastructure/sqlalchemy/entities.py

class EnrichmentV2(Base, CommonMixin):
    """Generic enrichment entity."""

    __tablename__ = "enrichments_v2"

    content: Mapped[str] = mapped_column(UnicodeText, nullable=False)
    type: Mapped[str] = mapped_column(String(50), nullable=False, index=True)


class EnrichmentAssociation(Base, CommonMixin):
    """Polymorphic association between enrichments and entities."""

    __tablename__ = "enrichment_associations"

    enrichment_id: Mapped[int] = mapped_column(
        ForeignKey("enrichments_v2.id", ondelete="CASCADE"),
        nullable=False,
        index=True
    )
    entity_type: Mapped[str] = mapped_column(
        String(50),
        nullable=False,
        index=True
    )
    entity_id: Mapped[str] = mapped_column(
        String(255),
        nullable=False,
        index=True
    )

    __table_args__ = (
        UniqueConstraint(
            "entity_type",
            "entity_id",
            "enrichment_id",
            name="uix_entity_enrichment"
        ),
        Index("idx_entity_lookup", "entity_type", "entity_id"),
    )
```

**Design Rationale:**

- **Separate tables**: Allows enrichments to exist independently and be shared (future enhancement)
- **Generic string fields**: `entity_type` and `entity_id` provide maximum flexibility
- **String type field**: Avoids enum limitations, allows dynamic enrichment types
- **Cascade delete**: When enrichment is deleted, associations are automatically cleaned up
- **Composite unique constraint**: Prevents duplicate enrichments for same entity

### Layer 2: Repository (Infrastructure)

```python
# src/kodit/infrastructure/sqlalchemy/enrichment_v2_repository.py

from typing import TypeVar, Generic

T = TypeVar('T')


class EnrichmentV2Repository:
    """Repository for managing enrichments and their associations."""

    def __init__(
        self,
        session_factory: Callable[[], AsyncSession],
        mapper: EnrichmentMapper
    ) -> None:
        self.session_factory = session_factory
        self.mapper = mapper

    async def get_enrichments(
        self,
        entity_type: str,
        entity_ids: list[str]
    ) -> list[domain_entities.EnrichmentV2]:
        """Get all enrichments for multiple entities of the same type."""
        if not entity_ids:
            return []

        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            stmt = (
                select(db_entities.EnrichmentV2)
                .join(db_entities.EnrichmentAssociation)
                .where(
                    db_entities.EnrichmentAssociation.entity_type == entity_type,
                    db_entities.EnrichmentAssociation.entity_id.in_(entity_ids)
                )
            )
            result = await session.execute(stmt)
            db_enrichments = result.scalars().all()

            return [
                self.mapper.to_domain(db_enrichment)
                for db_enrichment in db_enrichments
            ]

    async def bulk_save_enrichments(
        self,
        enrichments: list[domain_entities.EnrichmentV2]
    ) -> None:
        """Bulk save enrichments with their associations.

        Critical for performance during indexing operations.
        """
        if not enrichments:
            return

        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            # Prepare enrichment data
            enrichment_records = []
            for enrichment in enrichments:
                db_enrichment = db_entities.EnrichmentV2(
                    content=enrichment.content,
                    type=enrichment.type
                )
                session.add(db_enrichment)
                enrichment_records.append((enrichment, db_enrichment))

            # Flush to get IDs assigned
            await session.flush()

            # Create associations
            for enrichment, db_enrichment in enrichment_records:
                db_association = db_entities.EnrichmentAssociation(
                    enrichment_id=db_enrichment.id,
                    entity_type=enrichment.entity_type_key(),
                    entity_id=enrichment.entity_id
                )
                session.add(db_association)

    async def bulk_delete_enrichments(
        self,
        entity_type: str,
        entity_ids: list[str]
    ) -> None:
        """Bulk delete enrichments for multiple entities of the same type."""
        if not entity_ids:
            return

        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            # Get enrichment IDs for the entities
            stmt = (
                select(db_entities.EnrichmentAssociation.enrichment_id)
                .where(
                    db_entities.EnrichmentAssociation.entity_type == entity_type,
                    db_entities.EnrichmentAssociation.entity_id.in_(entity_ids)
                )
            )
            result = await session.execute(stmt)
            enrichment_ids = result.scalars().all()

            # Delete enrichments (cascade will delete associations)
            if enrichment_ids:
                await session.execute(
                    delete(db_entities.EnrichmentV2)
                    .where(db_entities.EnrichmentV2.id.in_(enrichment_ids))
                )
```

**Design Rationale:**

- **Self-describing enrichments**: Know their type via `entity_type_key()` and have `entity_id` field
- **Simple API**: Just pass list of enrichments, no tuples needed
- **Bulk operations**: Save, get, and delete all support bulk operations for performance
- **Efficient operations**: All methods handle multiple entities of same type in one query
- **Cascade delete**: Deleting enrichment automatically removes associations via foreign key

### Layer 3: Mapper (Infrastructure)

```python
# src/kodit/infrastructure/mappers/enrichment_mapper.py

from kodit.domain.entities.git import SnippetV2, GitCommit


class EnrichmentMapper:
    """Maps between domain enrichment entities and database entities."""

    def to_database(
        self,
        domain_enrichment: domain_entities.EnrichmentV2
    ) -> db_entities.EnrichmentV2:
        """Convert domain enrichment to database entity."""
        return db_entities.EnrichmentV2(
            content=domain_enrichment.content,
            type=domain_enrichment.type
        )

    def to_domain(
        self,
        db_enrichment: db_entities.EnrichmentV2
    ) -> domain_entities.EnrichmentV2:
        """Convert database enrichment to domain entity."""
        return domain_entities.EnrichmentV2(
            id=db_enrichment.id,
            content=db_enrichment.content,
            type=db_enrichment.type,
            created_at=db_enrichment.created_at,
            updated_at=db_enrichment.updated_at
        )
```

**Design Rationale:**

- **Simple mapping**: Just converts between domain and database representations
- **Stateless**: Pure functions with no side effects

### Layer 4: Enrichment Domain Entities

```python
# src/kodit/domain/entities/enrichment.py

from abc import ABC, abstractmethod
from dataclasses import dataclass
from datetime import datetime


@dataclass
class EnrichmentV2(ABC):
    """Generic enrichment that can be attached to any entity."""

    entity_id: str
    content: str = ""
    created_at: datetime | None = None
    updated_at: datetime | None = None

    @abstractmethod
    def entity_type_key(self) -> str:
        """Return the entity type key this enrichment is for."""
        pass


@dataclass
class SnippetEnrichment(EnrichmentV2):
    """Enrichment specific to code snippets."""

    def entity_type_key(self) -> str:
        return "snippet_v2"


@dataclass
class CommitEnrichment(EnrichmentV2):
    """Enrichment specific to commits."""

    def entity_type_key(self) -> str:
        return "git_commit"
```

**Design Rationale:**

- **Abstract base class**: Forces subclasses to implement type key
- **Entity ID stored**: Enrichment owns the entity reference (required field)
- **Self-describing**: Each enrichment knows both its type and entity ID
- **Type safety**: Cannot instantiate base class directly

## Implementation Plan

### Phase 1: Database Layer (1-2 days)

1. Add new SQLAlchemy entities to `entities.py`
2. Run `alembic revision --autogenerate -m "Add generic enrichment v2 tables"`
3. Review and adjust generated migration
4. Write migration tests

### Phase 2: Enrichment Domain Layer (1 day)

1. Create `src/kodit/domain/entities/enrichment.py`
2. Add `EnrichmentV2` abstract base class with `entity_type_key()` abstract method
3. Add `SnippetEnrichment` subclass (returns "snippet_v2")
4. Add `CommitEnrichment` subclass (returns "git_commit")
5. Write domain entity tests

### Phase 3: Mapper Layer (1 day)

1. Create `src/kodit/infrastructure/mappers/enrichment_mapper.py`
2. Implement `to_database()` conversion method
3. Implement `to_domain()` conversion method
4. Write mapper unit tests

### Phase 4: Repository Layer (2 days)

1. Create `src/kodit/infrastructure/sqlalchemy/enrichment_v2_repository.py`
2. Implement `get_enrichments(entity_type, entity_ids)` - bulk get for multiple entities
3. Implement `bulk_save_enrichments(list[EnrichmentV2])` method
4. Implement `bulk_delete_enrichments(entity_type, entity_ids)` method
5. Write repository integration tests

## Testing Strategy

### Repository Integration Tests

Test the repository methods against a real database. Database configuration is available in `config.py`.

**Test: `bulk_save_enrichments()`, `get_enrichments()`, and `bulk_delete_enrichments()`**

1. Create multiple `SnippetEnrichment` and `CommitEnrichment` instances with different entity IDs
2. Call `bulk_save_enrichments()` to save them
3. Call `get_enrichments(entity_type, list_of_entity_ids)` to retrieve them in bulk
4. Call `bulk_delete_enrichments(entity_type, list_of_entity_ids)` to delete some enrichments
5. Call `get_enrichments()` again to verify deletion
6. Verify:
   - Enrichments are saved to `enrichments_v2` table
   - Associations are created in `enrichment_associations` table with correct entity_type and entity_id
   - Retrieved enrichments match what was saved
   - Bulk get retrieves enrichments for all specified entity IDs
   - Bulk delete removes enrichments and their associations
   - After deletion, get returns empty list for deleted entities

## References

- Current enrichment: `src/kodit/infrastructure/sqlalchemy/entities.py:496`
- Domain enrichment: `src/kodit/domain/value_objects.py:36`
- Snippet mapper: `src/kodit/infrastructure/mappers/snippet_mapper.py`
