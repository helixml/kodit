"""Tests for the indexing service module."""

from pathlib import Path
import tempfile
from typing import Any, Generator

import pytest
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.bm25.local_bm25 import BM25Service
from kodit.config import AppContext
from kodit.embedding.embedding_provider.local_embedding_provider import (
    TINY,
    LocalEmbeddingProvider,
)
from kodit.embedding.embedding_repository import EmbeddingRepository
from kodit.embedding.local_vector_search_service import LocalVectorSearchService
from kodit.embedding.vector_search_service import (
    VectorSearchService,
)
from kodit.enrichment.enrichment_service import NullEnrichmentService
from kodit.indexing.indexing_repository import IndexRepository
from kodit.indexing.indexing_service import IndexService
from kodit.source.source_models import File, Source
from kodit.source.source_repository import SourceRepository
from kodit.source.source_service import SourceService


@pytest.fixture
def repository(session: AsyncSession) -> IndexRepository:
    """Create a real repository instance with a database session."""
    return IndexRepository(session)


@pytest.fixture
def source_repository(session: AsyncSession) -> SourceRepository:
    """Create a real source repository instance with a database session."""
    return SourceRepository(session)


@pytest.fixture
def source_service(
    tmp_path: Path, source_repository: SourceRepository
) -> SourceService:
    """Create a real source service instance."""
    return SourceService(tmp_path, source_repository)


@pytest.fixture
def embedding_service(session: AsyncSession) -> VectorSearchService:
    """Create a real embedding service instance."""
    return LocalVectorSearchService(
        embedding_repository=EmbeddingRepository(session),
        embedding_provider=LocalEmbeddingProvider(TINY),
    )


@pytest.fixture
def service(
    app_context: AppContext,
    repository: IndexRepository,
    source_service: SourceService,
    embedding_service: VectorSearchService,
) -> IndexService:
    """Create a real service instance with a database session."""
    keyword_search_provider = BM25Service(app_context.get_data_dir())
    return IndexService(
        repository=repository,
        source_service=source_service,
        keyword_search_provider=keyword_search_provider,
        code_search_service=embedding_service,
        text_search_service=embedding_service,
        enrichment_service=NullEnrichmentService(),
    )


@pytest.mark.asyncio
async def test_create_index(
    service: IndexService, repository: IndexRepository, session: AsyncSession
) -> None:
    """Test creating a new index through the service."""
    # Create a test source
    source = Source(uri="test_folder", cloned_path="test_folder")
    session.add(source)
    await session.commit()

    index = await service.create(source.id)

    assert index.id is not None
    assert index.created_at is not None

    # Verify the index was created in the database
    db_index = await repository.get_by_id(index.id)
    assert db_index is not None
    assert db_index.source_id == source.id

    # Verify it's listed
    indexes = await service.list_indexes()
    assert len(indexes) == 1
    assert indexes[0].id == index.id


@pytest.mark.asyncio
async def test_create_index_source_not_found(service: IndexService) -> None:
    """Test creating an index for a non-existent source."""
    with pytest.raises(ValueError, match="Source not found: 999"):
        await service.create(999)


@pytest.mark.asyncio
async def test_run_index(
    repository: IndexRepository,
    service: IndexService,
    session: AsyncSession,
    tmp_path: Path,
) -> None:
    """Test running an index through the service."""
    # Create test files
    test_dir = tmp_path / "test_folder"
    test_dir.mkdir()
    test_file = test_dir / "test.py"
    test_file.write_text("print('hello')")

    # Create test source
    source = Source(uri=str(test_dir), cloned_path=str(test_dir))
    session.add(source)
    await session.commit()

    # Create test files
    file = File(
        source_id=source.id,
        cloned_path=str(test_file),
        mime_type="text/x-python",
        uri=str(test_file),
        sha256="",
    )
    session.add(file)
    file = File(
        source_id=source.id,
        cloned_path=str(test_file),
        mime_type="unknown/unknown",  # This file will be ignored
        uri=str(test_file),
        sha256="",
    )
    session.add(file)
    await session.commit()

    # Create index
    index = await service.create(source.id)

    # Run the index
    await service.run(index.id)

    # Verify snippets were created
    snippets = await repository.get_snippets_for_index(index.id)
    assert len(snippets) == 1
    assert "print('hello')" in snippets[0].content

    # Try to create second index, should be the same index
    new_index = await service.create(source.id)
    assert index.id == new_index.id

    # Try to run the index again, should be fine
    await service.run(index.id)

    # Check that number of snippets is still 1 (because there is only one snippet)
    # I.e. if the file wasn't detected at the same, or was not deleted, then it
    # could create a second, duplicate snippet.
    snippets = await repository.get_snippets_for_index(index.id)
    assert len(snippets) == 1


@pytest.mark.asyncio
async def test_run_index_not_exists(service: IndexService) -> None:
    """Test running an index that doesn't exist."""
    with pytest.raises(ValueError, match="Index not found: 999"):
        await service.run(999)
