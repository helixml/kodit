"""Simple tests for enrichments router."""

from collections.abc import Callable

import pytest
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.enrichments.development.snippet.snippet import SnippetEnrichment
from kodit.infrastructure.sqlalchemy.enrichment_v2_repository import (
    SQLAlchemyEnrichmentV2Repository,
)


@pytest.fixture
async def sample_enrichment(
    session_factory: Callable[[], AsyncSession],
) -> SnippetEnrichment:
    """Create a sample enrichment for testing."""
    repository = SQLAlchemyEnrichmentV2Repository(session_factory)
    enrichment = SnippetEnrichment(content="test snippet content")
    saved = await repository.save(enrichment)
    assert isinstance(saved, SnippetEnrichment)
    return saved


async def test_save_and_get_enrichment(
    session_factory: Callable[[], AsyncSession],
) -> None:
    """Test saving and retrieving an enrichment."""
    repository = SQLAlchemyEnrichmentV2Repository(session_factory)

    # Create and save enrichment
    enrichment = SnippetEnrichment(content="test content")
    saved_enrichment = await repository.save(enrichment)

    assert saved_enrichment.id is not None
    assert saved_enrichment.content == "test content"
    assert saved_enrichment.type == "development"
    assert saved_enrichment.subtype == "snippet"

    # Retrieve enrichment
    retrieved_enrichment = await repository.get(saved_enrichment.id)
    assert retrieved_enrichment is not None
    assert retrieved_enrichment.id == saved_enrichment.id
    assert retrieved_enrichment.content == "test content"


async def test_update_enrichment(
    session_factory: Callable[[], AsyncSession],
    sample_enrichment: SnippetEnrichment,
) -> None:
    """Test updating an enrichment."""
    repository = SQLAlchemyEnrichmentV2Repository(session_factory)

    # Update enrichment content using dataclass replace
    from dataclasses import replace

    updated_enrichment = replace(sample_enrichment, content="updated content")
    saved_enrichment = await repository.save(updated_enrichment)

    assert saved_enrichment.id == sample_enrichment.id
    assert saved_enrichment.content == "updated content"

    # Verify update persisted
    retrieved_enrichment = await repository.get(saved_enrichment.id)
    assert retrieved_enrichment is not None
    assert retrieved_enrichment.content == "updated content"


async def test_delete_enrichment(
    session_factory: Callable[[], AsyncSession],
    sample_enrichment: SnippetEnrichment,
) -> None:
    """Test deleting an enrichment."""
    repository = SQLAlchemyEnrichmentV2Repository(session_factory)

    # Delete enrichment
    await repository.delete(sample_enrichment)

    # Verify deletion - repository.get() raises ValueError when not found
    with pytest.raises(ValueError, match="Entity with id .* not found"):
        await repository.get(sample_enrichment.id)  # type: ignore[arg-type]
