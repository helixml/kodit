import socket
import subprocess
import time
import pytest
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker
from kodit.database import Base
from kodit.embedding.vectorchord_vector_search_service import (
    VectorChordVectorSearchService,
)
from kodit.indexing.indexing_models import Index, Snippet
from kodit.source.source_models import File, Source
from sqlalchemy.ext.asyncio import AsyncEngine
from typing import AsyncGenerator
from sqlalchemy import text
from sqlalchemy.ext.asyncio import create_async_engine

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


def find_free_port() -> int:
    """Find a free port on the machine."""
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.bind(("127.0.0.1", 0))
        return s.getsockname()[1]


@pytest.fixture
async def vectorchord_engine() -> AsyncGenerator[AsyncEngine, None]:
    """Create a test database engine."""

    free_port = find_free_port()

    # Spin up a docker container for the vectorchord database and delete it after the test
    subprocess.run(
        [
            "docker",
            "run",
            "-d",
            "-e",
            "POSTGRES_DB=kodit",
            "-e",
            "POSTGRES_PASSWORD=mysecretpassword",
            "--name",
            "vectorchord",
            "-p",
            f"{free_port}:5432",
            "tensorchord/vchord-suite:pg17-20250601",
        ],
        check=True,
    )

    # Wait for the database to be ready
    while True:
        try:
            engine = create_async_engine(
                f"postgresql+asyncpg://postgres:mysecretpassword@localhost:{free_port}/kodit",
                echo=False,
                future=True,
            )
            async with engine.connect() as conn:
                await conn.execute(text("SELECT 1"))
            break
        except Exception as e:
            time.sleep(1)

    try:
        engine = create_async_engine(
            f"postgresql+asyncpg://postgres:mysecretpassword@localhost:{free_port}/kodit",
            echo=False,
            future=True,
        )

        async with engine.begin() as conn:
            await conn.run_sync(Base.metadata.create_all)

        yield engine

        await engine.dispose()
    finally:
        subprocess.run(["docker", "rm", "-f", "vectorchord"], check=True)


@pytest.fixture
async def vectorchord_session(
    vectorchord_engine: AsyncEngine,
) -> AsyncGenerator[AsyncSession, None]:
    """Create a test database session."""
    async_session = async_sessionmaker(
        vectorchord_engine,
        class_=AsyncSession,
        expire_on_commit=False,
        autocommit=False,
        autoflush=False,
    )

    async with async_session() as session:
        yield session


async def create_dummy_db_file(vectorchord_session: AsyncSession):
    # Create test source
    source = Source(uri="test_source", cloned_path="test_source")
    vectorchord_session.add(source)
    await vectorchord_session.commit()

    # Create test index
    index = Index(source_id=source.id)
    vectorchord_session.add(index)
    await vectorchord_session.commit()

    # Create test files and snippets
    file1 = File(
        source_id=source.id,
        cloned_path="test1.txt",
        mime_type="text/plain",
        uri="test1.txt",
        sha256="hash1",
        size_bytes=100,
    )
    vectorchord_session.add(file1)
    await vectorchord_session.commit()

    return index.id, file1.id


@pytest.fixture
def embedding_provider():
    return HashEmbeddingProvider()


@pytest.fixture
def vector_search_service(
    vectorchord_session: AsyncSession, embedding_provider: EmbeddingProvider
):
    return VectorChordVectorSearchService(
        task_name="test",
        session=vectorchord_session,
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
