"""Tests for the indexing service module."""

from pathlib import Path

import pytest
from sqlalchemy.exc import IntegrityError
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.indexing.repository import IndexRepository
from kodit.indexing.service import IndexService
from kodit.sources.models import File, Source


@pytest.fixture
def repository(session: AsyncSession) -> IndexRepository:
    """Create a real repository instance with a database session."""
    return IndexRepository(session)


@pytest.fixture
def service(repository: IndexRepository) -> IndexService:
    """Create a service instance with a real repository."""
    return IndexService(repository)


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
    with pytest.raises(IntegrityError):
        await service.create(999)


@pytest.mark.asyncio
async def test_create_index_already_exists(
    service: IndexService, session: AsyncSession
) -> None:
    """Test creating an index that already exists."""
    # Create a test source
    source = Source(uri="test_folder", cloned_path="test_folder")
    session.add(source)
    await session.commit()

    # Create first index
    await service.create(source.id)

    # Try to create second index
    with pytest.raises(IntegrityError, match="UNIQUE constraint failed"):
        await service.create(source.id)


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
    await session.commit()
    file = File(
        source_id=source.id,
        cloned_path=str(test_file),
        mime_type="unknown/unknown",
        uri=str(test_file),
        sha256="",
    )
    session.add(file)

    # Create index
    index = await service.create(source.id)

    # Run the index
    await service.run(index.id)

    # Verify snippets were created
    snippets = await repository.get_snippets_for_index(index.id)
    assert len(snippets) == 1
    assert snippets[0].content == "print('hello')"


@pytest.mark.asyncio
async def test_run_index_not_exists(service: IndexService) -> None:
    """Test running an index that doesn't exist."""
    with pytest.raises(ValueError):
        await service.run(999)
