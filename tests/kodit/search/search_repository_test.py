import pytest
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.indexing.indexing_models import Index, Snippet
from kodit.search.search_repository import SearchRepository
from kodit.source.source_models import File, Source


@pytest.fixture
def repository(session: AsyncSession) -> SearchRepository:
    """Create a repository instance with a real database session."""
    return SearchRepository(session)


# Test that list_snippets_by_ids returns a list in the same order it was passed
@pytest.mark.asyncio
async def test_list_snippets_by_ids_order(
    repository: SearchRepository, session: AsyncSession
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
    assert results[0][1].id == test_ids[0]
    assert results[1][1].id == test_ids[1]
    assert results[2][1].id == test_ids[2]


@pytest.mark.asyncio
async def test_list_semantic_results(
    repository: SearchRepository, session: AsyncSession
) -> None:
    """Test that list_semantic_results returns results sorted by similarity."""
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

    # Create test embeddings
    from kodit.embedding.embedding_models import Embedding, EmbeddingType

    # Create embeddings that are increasingly similar to the query
    embeddings = [
        [0.1, 0.2, 0.3],  # Less similar
        [0.2, 0.3, 0.4],  # More similar
        [0.3, 0.4, 0.5],  # Most similar
    ]

    for snippet, embedding in zip(snippets, embeddings):
        db_embedding = Embedding(
            snippet_id=snippet.id, type=EmbeddingType.CODE, embedding=embedding
        )
        session.add(db_embedding)
    await session.commit()

    # Test query embedding that should be most similar to the last embedding
    query_embedding = [0.35, 0.45, 0.55]

    # Get results
    results = await repository.list_semantic_results(
        EmbeddingType.CODE, query_embedding, top_k=2
    )

    # Verify results
    assert len(results) == 2
    # Results should be sorted by similarity (highest first)
    assert results[0][0] == snippets[2].id  # Most similar
    assert results[1][0] == snippets[1].id  # Second most similar
    # Verify similarity scores are in descending order
    assert results[0][1] > results[1][1]
