"""Benchmark script for semantic similarity search performance."""

import asyncio
import random
import time
from collections.abc import Callable
from datetime import UTC, datetime
from pathlib import Path

import structlog
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from kodit.domain.value_objects import FileProcessingStatus
from kodit.infrastructure.sqlalchemy.embedding_repository import (
    create_embedding_repository,
)
from kodit.infrastructure.sqlalchemy.entities import (
    Embedding,
    EmbeddingType,
    File,
    Index,
    Snippet,
    Source,
    SourceType,
)
from kodit.infrastructure.sqlalchemy.unit_of_work import SqlAlchemyUnitOfWork

log = structlog.get_logger(__name__)


def generate_random_embedding(dim: int = 750) -> list[float]:
    """Generate a random embedding vector of specified dimension."""
    return [random.uniform(-1, 1) for _ in range(dim)]  # noqa: S311


async def setup_test_data(
    session_factory: Callable[[], AsyncSession], num_embeddings: int = 5000
) -> None:
    """Set up test data with random embeddings."""
    uow = SqlAlchemyUnitOfWork(session_factory=session_factory)
    async with uow:
        # Create a test index
        source = Source(uri="test", cloned_path="test", source_type=SourceType.FOLDER)
        uow.session.add(source)
        await uow.commit()
        index = Index(source_id=source.id)
        uow.session.add(index)
        await uow.commit()
        now = datetime.now(UTC)
        file = File(
            created_at=now,
            updated_at=now,
            source_id=source.id,
            mime_type="text/plain",
            uri="test",
            cloned_path="test",
            sha256="abc123",
            size_bytes=100,
            extension="txt",
            file_processing_status=FileProcessingStatus.CLEAN,
        )
        uow.session.add(file)
        await uow.commit()
        snippet = Snippet(
            file_id=file.id,
            index_id=index.id,
            content="This is a test snippet",
            summary="",
        )
        uow.session.add(snippet)
        await uow.commit()

        # Create test embeddings
        embeddings = []
        for _ in range(num_embeddings):
            embedding = Embedding()
            embedding.snippet_id = snippet.id
            embedding.type = EmbeddingType.CODE
            embedding.embedding = generate_random_embedding()
            embeddings.append(embedding)

        uow.session.add_all(embeddings)
        await uow.commit()


async def run_benchmark(session_factory: Callable[[], AsyncSession]) -> None:
    """Run the semantic search benchmark."""
    # Setup test data
    log.info("Setting up test data...")
    await setup_test_data(session_factory=session_factory)

    # Create repository instance
    repo = create_embedding_repository(session_factory=session_factory)

    # Generate a test query embedding
    query_embedding = generate_random_embedding()

    # Run the benchmark
    num_runs = 10
    total_time = float(0)
    results = []  # Initialize results list

    log.info("Running warm-up query...")
    # Warm up
    await repo.list_semantic_results(
        embedding_type=EmbeddingType.CODE, embedding=query_embedding, top_k=10
    )

    log.info("Running benchmark queries...", num_runs=num_runs)

    # Actual benchmark
    for i in range(num_runs):
        start_time = time.perf_counter()
        results = await repo.list_semantic_results(
            embedding_type=EmbeddingType.CODE, embedding=query_embedding, top_k=10
        )
        end_time = time.perf_counter()
        run_time = end_time - start_time
        total_time += run_time
        log.info("Run", run_number=i + 1, num_runs=num_runs, run_time=run_time * 1000)

    # Calculate average time per run
    avg_time = total_time / num_runs

    log.info(
        "Semantic Search Performance Results",
        num_runs=num_runs,
        total_time=total_time,
        avg_time=avg_time * 1000,
    )

    # Print sample results
    log.info("Sample query returned results", num_results=len(results))
    if results:  # Add safety check
        log.info("First result score", score=results[0][1])


async def main() -> None:
    """Run the benchmark."""
    # Remove the database file if it exists
    if Path("benchmark.db").exists():
        Path("benchmark.db").unlink()

    # Create async engine and session
    engine = create_async_engine("sqlite+aiosqlite:///benchmark.db")

    # Create tables
    async with engine.begin() as conn:
        await conn.run_sync(Source.metadata.create_all)
        await conn.run_sync(File.metadata.create_all)
        await conn.run_sync(Index.metadata.create_all)
        await conn.run_sync(Snippet.metadata.create_all)
        await conn.run_sync(Embedding.metadata.create_all)

    # Create session factory
    async_session = async_sessionmaker(engine, expire_on_commit=False)

    # Run benchmark
    await run_benchmark(session_factory=async_session)

    # Cleanup
    await engine.dispose()


if __name__ == "__main__":
    asyncio.run(main())
