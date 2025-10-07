"""Tests for EnrichmentV2Repository."""

from collections.abc import Callable

import pytest
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.enrichments.architecture.physical.physical import (
    PhysicalArchitectureEnrichment,
)
from kodit.domain.enrichments.development.snippet.snippet import SnippetEnrichment
from kodit.infrastructure.sqlalchemy.enrichment_v2_repository import (
    EnrichmentV2Repository,
)


@pytest.fixture
def enrichment_repository(
    session_factory: Callable[[], AsyncSession],
) -> EnrichmentV2Repository:
    """Create an enrichment repository for testing."""
    return EnrichmentV2Repository(session_factory)


async def test_bulk_save_and_get_enrichments(
    enrichment_repository: EnrichmentV2Repository,
    session_factory: Callable[[], AsyncSession],
) -> None:
    """Test saving and retrieving enrichments."""
    snippet_enrichments = [
        SnippetEnrichment(
            entity_id="snippet_sha_1",
            content="This is a helper function for parsing JSON",
        ),
        SnippetEnrichment(
            entity_id="snippet_sha_2",
            content="This function validates user input",
        ),
    ]

    commit_enrichments = [
        PhysicalArchitectureEnrichment(
            entity_id="commit_sha_1",
            content="Added authentication feature",
        ),
        PhysicalArchitectureEnrichment(
            entity_id="commit_sha_2",
            content="Fixed bug in login flow",
        ),
    ]

    all_enrichments = snippet_enrichments + commit_enrichments
    await enrichment_repository.bulk_save_enrichments(all_enrichments)

    async with session_factory() as session:
        enrichment_count = await session.scalar(
            text("SELECT COUNT(*) FROM enrichments_v2")
        )
        association_count = await session.scalar(
            text("SELECT COUNT(*) FROM enrichment_associations")
        )
        assert enrichment_count == 4
        assert association_count == 4

    retrieved_snippets = await enrichment_repository.enrichments_for_entity_type(
        entity_type="snippet_v2",
        entity_ids=["snippet_sha_1", "snippet_sha_2"],
    )
    assert len(retrieved_snippets) == 2
    assert all(isinstance(e, SnippetEnrichment) for e in retrieved_snippets)
    contents = {e.content for e in retrieved_snippets}
    assert "This is a helper function for parsing JSON" in contents
    assert "This function validates user input" in contents

    retrieved_commits = await enrichment_repository.enrichments_for_entity_type(
        entity_type="git_commit",
        entity_ids=["commit_sha_1", "commit_sha_2"],
    )
    assert len(retrieved_commits) == 2
    assert all(isinstance(e, PhysicalArchitectureEnrichment) for e in retrieved_commits)
    contents = {e.content for e in retrieved_commits}
    assert "Added authentication feature" in contents
    assert "Fixed bug in login flow" in contents


async def test_bulk_delete_enrichments(
    enrichment_repository: EnrichmentV2Repository,
    session_factory: Callable[[], AsyncSession],
) -> None:
    """Test bulk deletion of enrichments."""
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

    await enrichment_repository.bulk_save_enrichments(enrichments)

    async with session_factory() as session:
        enrichment_count = await session.scalar(
            text("SELECT COUNT(*) FROM enrichments_v2")
        )
        association_count = await session.scalar(
            text("SELECT COUNT(*) FROM enrichment_associations")
        )
        assert enrichment_count == 3
        assert association_count == 3

    await enrichment_repository.bulk_delete_enrichments(
        entity_type="snippet_v2",
        entity_ids=["snippet_sha_1", "snippet_sha_2"],
    )

    async with session_factory() as session:
        enrichment_count = await session.scalar(
            text("SELECT COUNT(*) FROM enrichments_v2")
        )
        association_count = await session.scalar(
            text("SELECT COUNT(*) FROM enrichment_associations")
        )
        assert enrichment_count == 1
        assert association_count == 1

    remaining = await enrichment_repository.enrichments_for_entity_type(
        entity_type="snippet_v2",
        entity_ids=["snippet_sha_1", "snippet_sha_2"],
    )
    assert len(remaining) == 0

    remaining_commits = await enrichment_repository.enrichments_for_entity_type(
        entity_type="git_commit",
        entity_ids=["commit_sha_1"],
    )
    assert len(remaining_commits) == 1


async def test_get_enrichments_with_empty_entity_ids(
    enrichment_repository: EnrichmentV2Repository,
) -> None:
    """Test that getting enrichments with empty entity IDs returns empty list."""
    result = await enrichment_repository.enrichments_for_entity_type(
        entity_type="snippet_v2",
        entity_ids=[],
    )
    assert result == []


async def test_bulk_save_with_empty_list(
    enrichment_repository: EnrichmentV2Repository,
) -> None:
    """Test that bulk saving with empty list handles gracefully."""
    await enrichment_repository.bulk_save_enrichments([])


async def test_bulk_delete_with_empty_entity_ids(
    enrichment_repository: EnrichmentV2Repository,
) -> None:
    """Test that bulk deleting with empty entity IDs handles gracefully."""
    await enrichment_repository.bulk_delete_enrichments(
        entity_type="snippet_v2",
        entity_ids=[],
    )


async def test_get_enrichments_for_nonexistent_entities(
    enrichment_repository: EnrichmentV2Repository,
) -> None:
    """Test that getting enrichments for non-existent entities returns empty list."""
    result = await enrichment_repository.enrichments_for_entity_type(
        entity_type="snippet_v2",
        entity_ids=["nonexistent_sha"],
    )
    assert result == []


async def test_delete_enrichments_for_nonexistent_entities(
    enrichment_repository: EnrichmentV2Repository,
    session_factory: Callable[[], AsyncSession],
) -> None:
    """Test that deleting enrichments for non-existent entities handles gracefully."""
    await enrichment_repository.bulk_delete_enrichments(
        entity_type="snippet_v2",
        entity_ids=["nonexistent_sha"],
    )

    async with session_factory() as session:
        enrichment_count = await session.scalar(
            text("SELECT COUNT(*) FROM enrichments_v2")
        )
        assert enrichment_count == 0


async def test_get_enrichments_filters_by_entity_type(
    enrichment_repository: EnrichmentV2Repository,
) -> None:
    """Test that getting enrichments correctly filters by entity type."""
    enrichments = [
        SnippetEnrichment(
            entity_id="snippet_sha_1",
            content="Snippet enrichment",
        ),
        PhysicalArchitectureEnrichment(
            entity_id="commit_sha_1",
            content="Commit enrichment",
        ),
    ]

    await enrichment_repository.bulk_save_enrichments(enrichments)

    snippet_results = await enrichment_repository.enrichments_for_entity_type(
        entity_type="snippet_v2",
        entity_ids=["snippet_sha_1", "commit_sha_1"],
    )
    assert len(snippet_results) == 1
    assert isinstance(snippet_results[0], SnippetEnrichment)

    commit_results = await enrichment_repository.enrichments_for_entity_type(
        entity_type="git_commit",
        entity_ids=["snippet_sha_1", "commit_sha_1"],
    )
    assert len(commit_results) == 1
    assert isinstance(commit_results[0], PhysicalArchitectureEnrichment)
