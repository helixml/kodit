from kodit.indexing.indexing_models import Snippet
from kodit.indexing.indexing_repository import IndexRepository
from sqlalchemy.ext.asyncio import AsyncSession
import pytest

from kodit.source.source_models import File, Source


@pytest.fixture
def indexing_repository(session: AsyncSession) -> IndexRepository:
    """Create a real indexing repository instance."""
    return IndexRepository(session)


@pytest.mark.asyncio
async def test_should_allow_multiple_snippets_for_one_file(
    session: AsyncSession,
    indexing_repository: IndexRepository,
) -> None:
    source = Source(uri="test_folder", cloned_path="test_folder")
    session.add(source)
    await session.commit()

    file = File(
        source_id=source.id,
        cloned_path="test.py",
    )
    session.add(file)
    await session.commit()

    index = await indexing_repository.create(source.id)

    snippet1 = Snippet(
        file_id=file.id, index_id=index.id, content="print('Hello, world!')"
    )
    snippet2 = Snippet(
        file_id=file.id, index_id=index.id, content="print('Hello, world 2!')"
    )

    await indexing_repository.add_snippet(snippet1)
    await indexing_repository.add_snippet(snippet2)

    snippets = await indexing_repository.get_all_snippets(index.id)
    assert len(snippets) == 2


@pytest.mark.asyncio
async def test_should_raise_error_if_some_ids_are_not_present(
    indexing_repository: IndexRepository,
) -> None:
    """Test that an error is raised if some IDs are not present."""
    with pytest.raises(ValueError, match="Some IDs are not present: .*"):
        await indexing_repository.list_snippets_by_ids([1, 2, 3])


@pytest.mark.asyncio
async def test_should_return_when_items_are_present(
    session: AsyncSession,
    indexing_repository: IndexRepository,
) -> None:
    """Test that an error is raised if some IDs are not present."""
    source = Source(uri="test_folder", cloned_path="test_folder")
    session.add(source)
    await session.commit()

    file = File(
        source_id=source.id,
        cloned_path="test.py",
    )
    session.add(file)
    await session.commit()

    index = await indexing_repository.create(source.id)

    snippet1 = Snippet(
        file_id=file.id, index_id=index.id, content="print('Hello, world!')"
    )
    await indexing_repository.add_snippet(snippet1)
    snippet2 = Snippet(
        file_id=file.id, index_id=index.id, content="print('Hello, world 2!')"
    )
    await indexing_repository.add_snippet(snippet2)

    snippets = await indexing_repository.list_snippets_by_ids(
        [snippet1.id, snippet2.id]
    )
    assert len(snippets) == 2
    assert snippets[0][1].content == snippet1.content
    assert snippets[1][1].content == snippet2.content

    snippets = await indexing_repository.list_snippets_by_ids(
        [snippet2.id, snippet1.id]
    )
    assert len(snippets) == 2
    assert snippets[0][1].content == snippet2.content
    assert snippets[1][1].content == snippet1.content
