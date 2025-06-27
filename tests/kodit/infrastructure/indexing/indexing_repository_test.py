"""Test the indexing repository."""

from datetime import UTC, datetime

import pytest
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities import File, Source, SourceType
from kodit.infrastructure.indexing.index_repository import SQLAlchemyIndexRepository


@pytest.fixture
def indexing_repository(session: AsyncSession) -> SQLAlchemyIndexRepository:
    """Create a real indexing repository instance."""
    return SQLAlchemyIndexRepository(session)


@pytest.mark.asyncio
async def test_should_allow_multiple_snippets_for_one_file(
    session: AsyncSession,
    indexing_repository: SQLAlchemyIndexRepository,
) -> None:
    """Test all snippets for a file are added to the index."""
    source = Source(
        uri="test_folder", cloned_path="test_folder", source_type=SourceType.FOLDER
    )
    session.add(source)
    await session.commit()

    file = File(
        created_at=datetime.now(UTC),
        updated_at=datetime.now(UTC),
        source_id=source.id,
        mime_type="text/plain",
        uri="test.py",
        cloned_path="test.py",
        sha256="abc123",
        size_bytes=100,
        extension="py",
    )
    session.add(file)
    await session.commit()

    index = await indexing_repository.create_index(source.id)

    snippet1 = {
        "file_id": file.id,
        "index_id": index.id,
        "content": "print('Hello, world!')",
    }
    snippet2 = {
        "file_id": file.id,
        "index_id": index.id,
        "content": "print('Hello, world 2!')",
    }

    await indexing_repository.add_snippet(snippet1)
    await indexing_repository.add_snippet(snippet2)

    snippets = await indexing_repository.get_snippets_for_index(index.id)
    assert len(snippets) == 2


@pytest.mark.asyncio
async def test_should_raise_error_if_some_ids_are_not_present(
    indexing_repository: SQLAlchemyIndexRepository,
) -> None:
    """Test that an error is raised if some IDs are not present."""
    with pytest.raises(ValueError, match="Some IDs are not present: .*"):
        await indexing_repository.list_snippets_by_ids([1, 2, 3])


@pytest.mark.asyncio
async def test_should_return_when_items_are_present(
    session: AsyncSession,
    indexing_repository: SQLAlchemyIndexRepository,
) -> None:
    """Test that an error is raised if some IDs are not present."""
    source = Source(
        uri="test_folder",
        cloned_path="test_folder",
        source_type=SourceType.FOLDER,
    )
    session.add(source)
    await session.commit()

    file = File(
        created_at=datetime.now(UTC),
        updated_at=datetime.now(UTC),
        source_id=source.id,
        mime_type="text/plain",
        uri="test.py",
        cloned_path="test.py",
        sha256="abc123",
        size_bytes=100,
        extension="py",
    )
    session.add(file)
    await session.commit()

    index = await indexing_repository.create_index(source.id)

    snippet1 = {
        "file_id": file.id,
        "index_id": index.id,
        "content": "print('Hello, world!')",
    }
    await indexing_repository.add_snippet(snippet1)
    snippet2 = {
        "file_id": file.id,
        "index_id": index.id,
        "content": "print('Hello, world 2!')",
    }
    await indexing_repository.add_snippet(snippet2)

    # Get the actual snippet IDs from the database
    snippets = await indexing_repository.get_snippets_for_index(index.id)
    snippet1_id = snippets[0].id
    snippet2_id = snippets[1].id

    result = await indexing_repository.list_snippets_by_ids([snippet1_id, snippet2_id])
    assert len(result) == 2
    assert result[0].snippet.content == snippet1["content"]
    assert result[1].snippet.content == snippet2["content"]

    result = await indexing_repository.list_snippets_by_ids([snippet2_id, snippet1_id])
    assert len(result) == 2
    assert result[0].snippet.content == snippet2["content"]
    assert result[1].snippet.content == snippet1["content"]
