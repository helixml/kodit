import pytest

from kodit.embedding.embedding_provider.embedding_provider import EmbeddingProvider
from kodit.embedding.embedding_provider.hash_embedding_provider import (
    HashEmbeddingProvider,
)
from kodit.embedding.embedding_repository import EmbeddingRepository
from kodit.embedding.local_vector_search_service import LocalVectorSearchService
from kodit.embedding.vector_search_service import (
    VectorSearchRequest,
    VectorSearchResponse,
)
from kodit.embedding.embedding_models import EmbeddingType
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.indexing.indexing_models import Index, Snippet
from kodit.source.source_models import File, Source


async def create_dummy_db_file(session: AsyncSession):
    # Create test source
    source = Source(uri="test_source", cloned_path="test_source")
    session.add(source)
    await session.commit()

    # Create test index
    index = Index(source_id=source.id)
    session.add(index)
    await session.commit()

    # Create test files and snippets
    file1 = File(
        source_id=source.id,
        cloned_path="test1.txt",
        mime_type="text/plain",
        uri="test1.txt",
        sha256="hash1",
        size_bytes=100,
    )
    session.add(file1)
    await session.commit()

    return index.id, file1.id


@pytest.fixture
def embedding_provider():
    return HashEmbeddingProvider()


@pytest.fixture
async def embedding_repository(session: AsyncSession):
    # Create the dummy snippets, files, index and source
    return EmbeddingRepository(session=session)


@pytest.fixture
def vector_search_service(
    embedding_repository: EmbeddingRepository, embedding_provider: EmbeddingProvider
):
    return LocalVectorSearchService(
        embedding_repository=embedding_repository,
        embedding_provider=embedding_provider,
    )


@pytest.mark.asyncio
async def test_retrieve_documents(
    vector_search_service: LocalVectorSearchService, session: AsyncSession
):
    index_id, file_id = await create_dummy_db_file(session)

    snippet1 = Snippet(
        index_id=index_id, file_id=file_id, content="python programming language"
    )
    snippet2 = Snippet(
        index_id=index_id, file_id=file_id, content="javascript web development"
    )
    snippet3 = Snippet(
        index_id=index_id, file_id=file_id, content="java enterprise applications"
    )
    session.add(snippet1)
    session.add(snippet2)
    session.add(snippet3)
    await session.commit()

    test_data = [
        VectorSearchRequest(snippet_id=snippet1.id, text=snippet1.content),
        VectorSearchRequest(snippet_id=snippet2.id, text=snippet2.content),
        VectorSearchRequest(snippet_id=snippet3.id, text=snippet3.content),
    ]
    await vector_search_service.index(test_data)

    results = await vector_search_service.retrieve(
        "python programming language", top_k=2
    )

    assert len(results) == 2
    assert all(isinstance(r, VectorSearchResponse) for r in results)
    assert all(0 <= r.score <= 1 for r in results)
    # The first result should be the most relevant (python-related)
    assert results[0].snippet_id == snippet1.id


@pytest.mark.asyncio
async def test_retrieve_with_empty_index(
    vector_search_service: LocalVectorSearchService,
):
    results = await vector_search_service.retrieve("test query")
    assert len(results) == 0


@pytest.mark.asyncio
async def test_retrieve_with_custom_top_k(
    vector_search_service: LocalVectorSearchService,
    session: AsyncSession,
):
    index_id, file_id = await create_dummy_db_file(session)

    snippet1 = Snippet(
        index_id=index_id, file_id=file_id, content="python programming language"
    )
    snippet2 = Snippet(
        index_id=index_id, file_id=file_id, content="javascript web development"
    )
    snippet3 = Snippet(
        index_id=index_id, file_id=file_id, content="java enterprise applications"
    )
    session.add(snippet1)
    session.add(snippet2)
    session.add(snippet3)
    await session.commit()

    test_data = [
        VectorSearchRequest(snippet_id=snippet1.id, text=snippet1.content),
        VectorSearchRequest(snippet_id=snippet2.id, text=snippet2.content),
        VectorSearchRequest(snippet_id=snippet3.id, text=snippet3.content),
    ]
    await vector_search_service.index(test_data)

    results = await vector_search_service.retrieve("test", top_k=1)

    assert len(results) == 1
