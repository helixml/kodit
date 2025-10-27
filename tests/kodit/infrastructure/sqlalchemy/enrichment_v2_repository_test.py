"""Tests for EnrichmentV2Repository."""

from collections.abc import Callable

import pytest
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.enrichments.architecture.physical.physical import (
    PhysicalArchitectureEnrichment,
)
from kodit.domain.enrichments.development.snippet.snippet import SnippetEnrichment
from kodit.domain.enrichments.enrichment import EnrichmentAssociation
from kodit.infrastructure.sqlalchemy.enrichment_association_repository import (
    create_enrichment_association_repository,
)
from kodit.infrastructure.sqlalchemy.enrichment_v2_repository import (
    SQLAlchemyEnrichmentV2Repository,
)
from kodit.infrastructure.sqlalchemy.query import FilterOperator, QueryBuilder


@pytest.fixture
def enrichment_repository(
    session_factory: Callable[[], AsyncSession],
) -> SQLAlchemyEnrichmentV2Repository:
    """Create an enrichment repository for testing."""
    return SQLAlchemyEnrichmentV2Repository(session_factory)


async def test_save_and_get_enrichments(
    enrichment_repository: SQLAlchemyEnrichmentV2Repository,
    session_factory: Callable[[], AsyncSession],
) -> None:
    """Test saving and retrieving enrichments using the new paradigm."""
    # Create enrichments
    snippet_enrichment = SnippetEnrichment(
        entity_id="snippet_sha_1",
        content="This is a helper function for parsing JSON",
    )

    commit_enrichment = PhysicalArchitectureEnrichment(
        entity_id="commit_sha_1",
        content="Added authentication feature",
    )

    # Save enrichments
    saved_snippet = await enrichment_repository.save(snippet_enrichment)
    saved_commit = await enrichment_repository.save(commit_enrichment)

    assert saved_snippet.id is not None
    assert saved_commit.id is not None

    # Create associations
    association_repo = create_enrichment_association_repository(session_factory)
    await association_repo.save(
        EnrichmentAssociation(
            enrichment_id=saved_snippet.id,  # type: ignore[arg-type]
            entity_type="snippet_v2",
            entity_id="snippet_sha_1",
        )
    )
    await association_repo.save(
        EnrichmentAssociation(
            enrichment_id=saved_commit.id,  # type: ignore[arg-type]
            entity_type="git_commit",
            entity_id="commit_sha_1",
        )
    )

    # Verify enrichments were created
    async with session_factory() as session:
        enrichment_count = await session.scalar(
            text("SELECT COUNT(*) FROM enrichments_v2")
        )
        association_count = await session.scalar(
            text("SELECT COUNT(*) FROM enrichment_associations")
        )
        assert enrichment_count == 2
        assert association_count == 2

    # Retrieve enrichments by ID
    retrieved_snippet = await enrichment_repository.get(saved_snippet.id)  # type: ignore[arg-type]
    assert retrieved_snippet is not None
    assert retrieved_snippet.content == "This is a helper function for parsing JSON"

    retrieved_commit = await enrichment_repository.get(saved_commit.id)  # type: ignore[arg-type]
    assert retrieved_commit is not None
    assert retrieved_commit.content == "Added authentication feature"


async def test_bulk_save_enrichments(
    enrichment_repository: SQLAlchemyEnrichmentV2Repository,
    session_factory: Callable[[], AsyncSession],
) -> None:
    """Test bulk saving enrichments."""
    enrichments = [
        SnippetEnrichment(
            entity_id="snippet_sha_1",
            content="This is a helper function",
        ),
        SnippetEnrichment(
            entity_id="snippet_sha_2",
            content="This function validates input",
        ),
        PhysicalArchitectureEnrichment(
            entity_id="commit_sha_1",
            content="Added feature",
        ),
    ]

    saved_enrichments = await enrichment_repository.save_bulk(enrichments)  # type: ignore[arg-type]

    assert len(saved_enrichments) == 3
    assert all(e.id is not None for e in saved_enrichments)

    async with session_factory() as session:
        enrichment_count = await session.scalar(
            text("SELECT COUNT(*) FROM enrichments_v2")
        )
        assert enrichment_count == 3


async def test_delete_enrichment(
    enrichment_repository: SQLAlchemyEnrichmentV2Repository,
    session_factory: Callable[[], AsyncSession],
) -> None:
    """Test deleting enrichments."""
    # Create and save enrichment
    enrichment = SnippetEnrichment(
        entity_id="snippet_sha_1",
        content="This is a helper function",
    )
    saved_enrichment = await enrichment_repository.save(enrichment)

    # Create association
    association_repo = create_enrichment_association_repository(session_factory)
    await association_repo.save(
        EnrichmentAssociation(
            enrichment_id=saved_enrichment.id,  # type: ignore[arg-type]
            entity_type="snippet_v2",
            entity_id="snippet_sha_1",
        )
    )

    # Verify it was created
    async with session_factory() as session:
        enrichment_count = await session.scalar(
            text("SELECT COUNT(*) FROM enrichments_v2")
        )
        assert enrichment_count == 1

    # Delete enrichment
    await enrichment_repository.delete(saved_enrichment.id)  # type: ignore[arg-type]

    # Verify it was deleted (cascading should also delete associations)
    async with session_factory() as session:
        enrichment_count = await session.scalar(
            text("SELECT COUNT(*) FROM enrichments_v2")
        )
        association_count = await session.scalar(
            text("SELECT COUNT(*) FROM enrichment_associations")
        )
        assert enrichment_count == 0
        assert association_count == 0


async def test_find_enrichments_by_associations(
    enrichment_repository: SQLAlchemyEnrichmentV2Repository,
    session_factory: Callable[[], AsyncSession],
) -> None:
    """Test finding enrichments through associations."""
    # Create and save enrichments
    enrichments = [
        SnippetEnrichment(
            entity_id="snippet_sha_1",
            content="Snippet enrichment 1",
        ),
        SnippetEnrichment(
            entity_id="snippet_sha_2",
            content="Snippet enrichment 2",
        ),
        PhysicalArchitectureEnrichment(
            entity_id="commit_sha_1",
            content="Commit enrichment",
        ),
    ]

    saved_enrichments = await enrichment_repository.save_bulk(enrichments)  # type: ignore[arg-type]

    # Create associations
    association_repo = create_enrichment_association_repository(session_factory)
    for enrichment in saved_enrichments:
        await association_repo.save(
            EnrichmentAssociation(
                enrichment_id=enrichment.id,  # type: ignore[arg-type]
                entity_type=enrichment.entity_type_key(),
                entity_id=enrichment.entity_id,
            )
        )

    # Find associations for snippet entities
    snippet_associations = await association_repo.find(
        QueryBuilder()
        .filter("entity_type", FilterOperator.EQ, "snippet_v2")
    )
    assert len(snippet_associations) == 2

    # Get enrichments for those associations
    snippet_enrichment_ids = [assoc.enrichment_id for assoc in snippet_associations]
    snippet_enrichments = []
    for enrich_id in snippet_enrichment_ids:
        enrichment = await enrichment_repository.get(enrich_id)
        if enrichment:
            snippet_enrichments.append(enrichment)

    assert len(snippet_enrichments) == 2
    assert all(isinstance(e, SnippetEnrichment) for e in snippet_enrichments)
