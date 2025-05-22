import pytest
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.indexing.models import Index, Snippet
from kodit.retreival.repository import RetrievalRepository
from kodit.sources.models import File, Source


@pytest.fixture
def repository(session: AsyncSession) -> RetrievalRepository:
    """Create a repository instance with a real database session."""
    return RetrievalRepository(session)


# Test that list_snippets_by_ids returns a list in the same order it was passed
@pytest.mark.asyncio
async def test_list_snippets_by_ids_order(
    repository: RetrievalRepository, session: AsyncSession
) -> None:
    """Test that list_snippets_by_ids returns snippets in the same order as input IDs."""
    # Create test source and file
    source = Source(uri="test_source", cloned_path="test_source")
    session.add(source)
    await session.commit()

    file = File(
        source_id=source.id,
        cloned_path="test.txt",
        mime_type="text/plain",
        uri="test.txt",
        sha256="hash1",
        size_bytes=100,
    )
    session.add(file)
    await session.commit()

    index = Index(source_id=source.id)
    session.add(index)
    await session.commit()

    # Create test snippets
    snippets = []
    for i in range(3):
        snippet = Snippet(index_id=index.id, file_id=file.id, content=f"content {i}")
        session.add(snippet)
        snippets.append(snippet)
    await session.commit()

    # Test with IDs in different order than creation
    test_ids = [snippets[2].id, snippets[0].id, snippets[1].id]
    results = await repository.list_snippets_by_ids(test_ids)
    print(results)
    # Verify results are in same order as input IDs
    assert len(results) == 3
    assert results[0].id == test_ids[0]
    assert results[1].id == test_ids[1]
    assert results[2].id == test_ids[2]
