"""Tests for the VectorChord BM25 repository with real database."""

import asyncio
import socket
import subprocess
import time
from collections.abc import AsyncGenerator, Generator
from datetime import UTC, datetime

import pytest
from sqlalchemy import text
from sqlalchemy.ext.asyncio import (
    AsyncEngine,
    AsyncSession,
    async_sessionmaker,
    create_async_engine,
)

from kodit.domain.value_objects import (
    Document,
    FileProcessingStatus,
    IndexRequest,
    SearchRequest,
    SearchResult,
)
from kodit.infrastructure.bm25.vectorchord_bm25_repository import (
    VectorChordBM25Repository,
)
from kodit.infrastructure.sqlalchemy.entities import (
    Base,
    File,
    Index,
    Snippet,
    Source,
    SourceType,
)

# Suppress the pytest-asyncio event_loop fixture deprecation warning
pytestmark = [
    pytest.mark.asyncio(loop_scope="module"),
    pytest.mark.filterwarnings("ignore::DeprecationWarning:pytest_asyncio.*"),
]


@pytest.fixture(scope="module")
def event_loop() -> Generator[asyncio.AbstractEventLoop, None, None]:
    """Create an event loop for module-scoped async tests."""
    loop = asyncio.new_event_loop()
    yield loop
    loop.close()


def find_free_port() -> int:
    """Find a free port on the machine."""
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.bind(("127.0.0.1", 0))
        return s.getsockname()[1]


# Global variables to store the database state
_vectorchord_port: int | None = None
_vectorchord_container_name: str | None = None


@pytest.fixture(scope="module")
async def vectorchord_engine() -> AsyncGenerator[AsyncEngine, None]:
    """Create a test database engine for the entire test module."""
    global _vectorchord_port, _vectorchord_container_name  # noqa: PLW0603

    _vectorchord_port = find_free_port()
    _vectorchord_container_name = f"vectorchord_test_{_vectorchord_port}"

    # Spin up a docker container for the vectorchord database
    subprocess.run(  # noqa: S603,ASYNC221
        [  # noqa: S607
            "docker",
            "run",
            "-d",
            "-e",
            "POSTGRES_DB=kodit",
            "-e",
            "POSTGRES_PASSWORD=mysecretpassword",
            "--name",
            _vectorchord_container_name,
            "-p",
            f"{_vectorchord_port}:5432",
            "tensorchord/vchord-suite:pg17-20250601",
        ],
        check=True,
    )

    # Wait for the database to be ready
    while True:
        try:
            engine = create_async_engine(
                f"postgresql+asyncpg://postgres:mysecretpassword@localhost:{_vectorchord_port}/kodit",
                echo=False,
                future=True,
            )
            async with engine.connect() as conn:
                await conn.execute(text("SELECT 1"))
            break
        except Exception:  # noqa: BLE001
            time.sleep(1)  # noqa: ASYNC251

    try:
        engine = create_async_engine(
            f"postgresql+asyncpg://postgres:mysecretpassword@localhost:{_vectorchord_port}/kodit",
            echo=False,
            future=True,
        )

        async with engine.begin() as conn:
            await conn.run_sync(Base.metadata.create_all)

        yield engine

        await engine.dispose()
    finally:
        # Clean up the container at the end of the module
        subprocess.run(  # noqa: S603,ASYNC221
            ["docker", "rm", "-f", _vectorchord_container_name],  # noqa: S607
            check=True,
        )


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
        # Clean up tables after each test
        await session.rollback()
        # Clear all tables by truncating them
        async with vectorchord_engine.begin() as conn:
            # Get all table names and truncate them
            result = await conn.execute(
                text("""
                SELECT tablename FROM pg_tables
                WHERE schemaname = 'public'
                AND tablename NOT LIKE 'pg_%'
                AND tablename NOT LIKE 'information_schema%'
            """)
            )
            tables = [row[0] for row in result.fetchall()]

            # Disable foreign key checks temporarily
            await conn.execute(text("SET session_replication_role = replica"))

            # Truncate all tables
            for table in tables:
                await conn.execute(
                    text(f"TRUNCATE TABLE {table} RESTART IDENTITY CASCADE")
                )

            # Re-enable foreign key checks
            await conn.execute(text("SET session_replication_role = DEFAULT"))


@pytest.fixture
async def test_data(
    vectorchord_session: AsyncSession,
) -> tuple[list[Snippet], VectorChordBM25Repository]:
    """Create test data and repository."""
    # Create test data
    source = Source(uri="test", cloned_path="test", source_type=SourceType.FOLDER)
    vectorchord_session.add(source)
    await vectorchord_session.flush()

    file = File(
        created_at=datetime.now(UTC),
        updated_at=datetime.now(UTC),
        source_id=source.id,
        mime_type="text/plain",
        uri="test.py",
        cloned_path="test",
        sha256="",
        size_bytes=0,
        extension="py",
        file_processing_status=FileProcessingStatus.CLEAN,
    )
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
            content=(
                "Python is a high-level programming language "
                "known for its simplicity and readability."
            ),
            summary="",
        ),
        Snippet(
            file_id=file.id,
            index_id=index.id,
            content=(
                "Python supports multiple programming paradigms including "
                "procedural, object-oriented, and functional programming."
            ),
            summary="",
        ),
        Snippet(
            file_id=file.id,
            index_id=index.id,
            content=(
                "The Python programming language was created by "
                "Guido van Rossum and first released in 1991."
            ),
            summary="",
        ),
        Snippet(
            file_id=file.id,
            index_id=index.id,
            content=(
                "Python is widely used in data science, machine learning, "
                "and artificial intelligence applications."
            ),
            summary="",
        ),
        Snippet(
            file_id=file.id,
            index_id=index.id,
            content=(
                "Python's extensive standard library and third-party packages "
                "make it a versatile language for various applications."
            ),
            summary="",
        ),
    ]

    for snippet in snippets:
        vectorchord_session.add(snippet)
    await vectorchord_session.commit()

    # Initialize repository
    repository = VectorChordBM25Repository(session=vectorchord_session)

    # Index the documents
    await repository.index_documents(
        IndexRequest(
            documents=[Document(snippet_id=s.id, text=s.content) for s in snippets]
        )
    )

    return snippets, repository


@pytest.mark.asyncio
async def test_search_with_none_snippet_ids_returns_all_results(
    test_data: tuple[list[Snippet], VectorChordBM25Repository],
) -> None:
    """Test that search with None snippet_ids returns all results (no filtering)."""
    snippets, repository = test_data

    # Setup
    request = SearchRequest(
        query="Python programming",
        top_k=10,
        snippet_ids=None,  # No filtering
    )

    # Execute
    results = await repository.search(request)

    # Verify
    assert len(results) > 0
    assert all(isinstance(result, SearchResult) for result in results)
    # Should return multiple results since "Python programming" matches multiple snips
    assert len(results) >= 3


@pytest.mark.asyncio
async def test_search_with_empty_snippet_ids_returns_no_results(
    test_data: tuple[list[Snippet], VectorChordBM25Repository],
) -> None:
    """Test that search with empty snippet_ids list returns no results."""
    snippets, repository = test_data

    # Setup
    request = SearchRequest(
        query="Python programming",
        top_k=10,
        snippet_ids=[],  # Empty list - should return no results
    )

    # Execute
    results = await repository.search(request)

    # Verify
    assert len(results) == 0
    assert results == []


@pytest.mark.asyncio
async def test_search_with_filtered_snippet_ids_returns_matching_results(
    test_data: tuple[list[Snippet], VectorChordBM25Repository],
) -> None:
    """Test that search with specific snippet_ids returns only matching results."""
    snippets, repository = test_data

    # Setup - only search in snippets 0 and 2
    request = SearchRequest(
        query="Python programming",
        top_k=10,
        snippet_ids=[snippets[0].id, snippets[2].id],  # Only return snippets 0 and 2
    )

    # Execute
    results = await repository.search(request)

    # Verify
    assert len(results) > 0
    assert all(isinstance(result, SearchResult) for result in results)
    # All returned snippet_ids should be in our filtered list
    returned_snippet_ids = [result.snippet_id for result in results]
    assert all(
        snippet_id in [snippets[0].id, snippets[2].id]
        for snippet_id in returned_snippet_ids
    )


@pytest.mark.asyncio
async def test_search_with_single_snippet_id_returns_one_result(
    test_data: tuple[list[Snippet], VectorChordBM25Repository],
) -> None:
    """Test that search with a single snippet_id returns only that result."""
    snippets, repository = test_data

    # Setup - only search in snippet 2 (which mentions "Guido van Rossum")
    request = SearchRequest(
        query="Guido van Rossum",
        top_k=10,
        snippet_ids=[snippets[2].id],  # Only return snippet 2
    )

    # Execute
    results = await repository.search(request)

    # Verify
    assert len(results) == 1
    assert results[0].snippet_id == snippets[2].id
    assert isinstance(results[0].score, (int, float))


@pytest.mark.asyncio
async def test_search_with_nonexistent_snippet_ids_returns_no_results(
    test_data: tuple[list[Snippet], VectorChordBM25Repository],
) -> None:
    """Test that search with snippet_ids that don't exist returns no results."""
    snippets, repository = test_data

    # Setup
    request = SearchRequest(
        query="Python programming",
        top_k=10,
        snippet_ids=[99999, 100000],  # Non-existent snippet IDs
    )

    # Execute
    results = await repository.search(request)

    # Verify
    assert len(results) == 0
    assert results == []


@pytest.mark.asyncio
async def test_search_with_empty_query_returns_empty_list(
    test_data: tuple[list[Snippet], VectorChordBM25Repository],
) -> None:
    """Test that search with empty query returns empty list."""
    snippets, repository = test_data

    # Setup
    request = SearchRequest(
        query="",  # Empty query
        top_k=10,
        snippet_ids=None,
    )

    # Execute
    results = await repository.search(request)

    # Verify
    assert results == []


@pytest.mark.asyncio
async def test_search_with_whitespace_query_returns_empty_list(
    test_data: tuple[list[Snippet], VectorChordBM25Repository],
) -> None:
    """Test that search with whitespace-only query returns empty list."""
    snippets, repository = test_data

    # Setup
    request = SearchRequest(
        query="   ",  # Whitespace-only query
        top_k=10,
        snippet_ids=None,
    )

    # Execute
    results = await repository.search(request)

    # Verify
    assert results == []


@pytest.mark.asyncio
async def test_search_respects_top_k_limit(
    test_data: tuple[list[Snippet], VectorChordBM25Repository],
) -> None:
    """Test that search respects the top_k limit."""
    snippets, repository = test_data

    # Setup
    request = SearchRequest(
        query="Python",
        top_k=2,  # Limit to 2 results
        snippet_ids=None,
    )

    # Execute
    results = await repository.search(request)

    # Verify
    assert len(results) == 2  # Should be limited by top_k
    assert all(isinstance(result, SearchResult) for result in results)


@pytest.mark.asyncio
async def test_search_result_structure(
    test_data: tuple[list[Snippet], VectorChordBM25Repository],
) -> None:
    """Test that search results have the correct structure."""
    snippets, repository = test_data

    # Setup
    request = SearchRequest(
        query="Guido van Rossum", top_k=1, snippet_ids=[snippets[2].id]
    )

    # Execute
    results = await repository.search(request)

    # Verify
    assert len(results) == 1
    result = results[0]
    assert isinstance(result, SearchResult)
    assert hasattr(result, "snippet_id")
    assert hasattr(result, "score")
    assert result.snippet_id == snippets[2].id
    assert isinstance(result.score, (int, float))


@pytest.mark.asyncio
async def test_search_with_mixed_existing_and_nonexistent_ids(
    test_data: tuple[list[Snippet], VectorChordBM25Repository],
) -> None:
    """Test that search with a mix of existing and non-existent snippet_ids works."""
    snippets, repository = test_data

    # Setup - mix of existing and non-existent IDs
    request = SearchRequest(
        query="Python",
        top_k=10,
        snippet_ids=[
            snippets[0].id,
            99999,
            snippets[2].id,
            100000,
        ],  # Mix of real and fake IDs
    )

    # Execute
    results = await repository.search(request)

    # Verify
    assert len(results) > 0
    # All returned snippet_ids should be in our filtered list (only the real ones)
    returned_snippet_ids = [result.snippet_id for result in results]
    assert all(
        snippet_id in [snippets[0].id, snippets[2].id]
        for snippet_id in returned_snippet_ids
    )


@pytest.mark.asyncio
async def test_search_with_phrase_matching_and_filtering(
    test_data: tuple[list[Snippet], VectorChordBM25Repository],
) -> None:
    """Test that phrase matching works correctly with snippet filtering."""
    snippets, repository = test_data

    # Setup - search for "data science" which should match snippet 3
    request = SearchRequest(
        query="data science",
        top_k=10,
        snippet_ids=[snippets[3].id],  # Only snippet 3 mentions "data science"
    )

    # Execute
    results = await repository.search(request)

    # Verify
    assert len(results) == 1
    assert results[0].snippet_id == snippets[3].id
    assert isinstance(
        results[0].score, (int, float)
    )  # Should have a numeric score (can be negative)


@pytest.mark.asyncio
async def test_search_with_case_insensitive_filtering(
    test_data: tuple[list[Snippet], VectorChordBM25Repository],
) -> None:
    """Test that case insensitive search works with filtering."""
    snippets, repository = test_data

    # Setup - search for "PYTHON" (uppercase) with filtering
    request = SearchRequest(
        query="PYTHON",
        top_k=10,
        snippet_ids=[snippets[0].id, snippets[1].id],  # Only first two snippets
    )

    # Execute
    results = await repository.search(request)

    # Verify
    assert len(results) > 0
    # All returned snippet_ids should be in our filtered list
    returned_snippet_ids = [result.snippet_id for result in results]
    assert all(
        snippet_id in [snippets[0].id, snippets[1].id]
        for snippet_id in returned_snippet_ids
    )


@pytest.mark.asyncio
async def test_search_maintains_ordering_with_different_top_k_values(
    test_data: tuple[list[Snippet], VectorChordBM25Repository],
) -> None:
    """Test that search results maintain consistent ordering with different top_k's."""
    snippets, repository = test_data

    # Search with different top_k values
    results_k1 = await repository.search(
        SearchRequest(query="Python programming language", top_k=1)
    )
    results_k2 = await repository.search(
        SearchRequest(query="Python programming language", top_k=2)
    )
    results_k3 = await repository.search(
        SearchRequest(query="Python programming language", top_k=3)
    )
    results_k5 = await repository.search(
        SearchRequest(query="Python programming language", top_k=5)
    )

    # Verify we got the expected number of results
    assert len(results_k1) == 1
    assert len(results_k2) == 2
    assert len(results_k3) == 3
    assert len(results_k5) == 5  # All snippets since we have 5 total

    # The #1 result should be the same across all queries
    assert results_k1[0].snippet_id == results_k2[0].snippet_id, (
        "Top result should be consistent between k=1 and k=2"
    )
    assert results_k1[0].snippet_id == results_k3[0].snippet_id, (
        "Top result should be consistent between k=1 and k=3"
    )
    assert results_k1[0].snippet_id == results_k5[0].snippet_id, (
        "Top result should be consistent between k=1 and k=5"
    )

    # The #2 result should be the same when k>=2
    assert results_k2[1].snippet_id == results_k3[1].snippet_id, (
        "Second result should be consistent between k=2 and k=3"
    )
    assert results_k2[1].snippet_id == results_k5[1].snippet_id, (
        "Second result should be consistent between k=2 and k=5"
    )

    # The #3 result should be the same when k>=3
    assert results_k3[2].snippet_id == results_k5[2].snippet_id, (
        "Third result should be consistent between k=3 and k=5"
    )

    # Verify scores are in descending order (best matches first)
    for results in [results_k2, results_k3, results_k5]:
        for i in range(len(results) - 1):
            # Note: vectorchord bm25 scores are negative, so lower is better
            assert results[i].score <= results[i + 1].score, (
                f"Results should be sorted in descending order by score at index {i}"
            )


@pytest.mark.asyncio
async def test_search_ordering_with_snippet_id_filter(
    test_data: tuple[list[Snippet], VectorChordBM25Repository],
) -> None:
    """Test that search maintains ordering even when snippet_ids filter is applied."""
    snippets, repository = test_data

    # First, get all results without filtering
    all_results = await repository.search(SearchRequest(query="Python", top_k=10))

    # Get the top 3 snippet IDs from the unfiltered results
    top_3_ids = [r.snippet_id for r in all_results[:3]]

    # Now search with only these top 3 IDs as filter
    filtered_results = await repository.search(
        SearchRequest(query="Python", top_k=10, snippet_ids=top_3_ids)
    )

    # The ordering should be maintained
    assert len(filtered_results) <= 3
    for i, result in enumerate(filtered_results):
        # The snippet ID at position i in filtered results should match
        # the snippet ID at position i in the original results (among the top 3)
        original_position = next(
            j
            for j, r in enumerate(all_results[:3])
            if r.snippet_id == result.snippet_id
        )
        assert original_position == i, (
            f"Result at position {i} has snippet_id {result.snippet_id} "
            f"but was at position {original_position} in unfiltered results"
        )
