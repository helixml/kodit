import socket
import subprocess
import time
from kodit.bm25.keyword_search_service import BM25Document
from kodit.bm25.vectorchord_bm25 import VectorChordBM25
import pytest
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker

from kodit.database import Base
from kodit.indexing.indexing_models import Index, Snippet
from kodit.source.source_models import File, Source
from sqlalchemy.ext.asyncio import AsyncEngine
from typing import AsyncGenerator
from sqlalchemy import text
from sqlalchemy.ext.asyncio import create_async_engine


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


@pytest.mark.asyncio
async def test_vectorchord_repository_bm25_search(vectorchord_session: AsyncSession):
    """Test the BM25 search capabilities of VectorChordRepository."""
    # Create test data
    source = Source(uri="test", cloned_path="test")
    vectorchord_session.add(source)
    await vectorchord_session.flush()

    file = File(source_id=source.id, cloned_path="test")
    vectorchord_session.add(file)
    await vectorchord_session.flush()

    index = Index(source_id=source.id)
    vectorchord_session.add(index)
    await vectorchord_session.flush()

    # Create snippets with varied content to test different aspects of BM25
    snippets = [
        Snippet(
            file_id=file.id,
            index_id=index.id,
            content="Python is a high-level programming language known for its simplicity and readability.",
        ),
        Snippet(
            file_id=file.id,
            index_id=index.id,
            content="Python supports multiple programming paradigms including procedural, object-oriented, and functional programming.",
        ),
        Snippet(
            file_id=file.id,
            index_id=index.id,
            content="The Python programming language was created by Guido van Rossum and first released in 1991.",
        ),
        Snippet(
            file_id=file.id,
            index_id=index.id,
            content="Python is widely used in data science, machine learning, and artificial intelligence applications.",
        ),
        Snippet(
            file_id=file.id,
            index_id=index.id,
            content="Python's extensive standard library and third-party packages make it a versatile language for various applications.",
        ),
    ]

    for snippet in snippets:
        vectorchord_session.add(snippet)
    await vectorchord_session.commit()

    # Initialize repository
    r = VectorChordBM25(session=vectorchord_session)

    # Index the documents
    await r.index([BM25Document(snippet_id=s.id, text=s.content) for s in snippets])

    # Test 1: Basic keyword search
    results = await r.retrieve("Python programming", top_k=3)
    assert len(results) == 3

    # Test 2: Phrase search
    results = await r.retrieve("Guido van Rossum", top_k=10)
    assert len(results) == 1
    assert results[0].snippet_id == snippets[2].id

    # Test 3: Multiple word search with different frequencies
    results = await r.retrieve("Python data science", top_k=1)
    assert len(results) == 1
    assert results[0].snippet_id == snippets[3].id

    # Test 4: Edge case - empty query
    results = await r.retrieve("", top_k=10)
    assert len(results) == 0

    # Test 5: Case insensitivity
    results = await r.retrieve("PYTHON", top_k=10)
    assert len(results) > 0

    # Test 6: Partial word matching
    results = await r.retrieve("program", top_k=10)
    assert len(results) == 3

    # Test 7: Delete documents
    await r.delete([s.id for s in snippets])
    results = await r.retrieve("Python programming", top_k=10)
    assert len(results) == 0
