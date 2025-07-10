"""Benchmark script for semantic similarity search performance."""

import asyncio
import random
import time
from datetime import UTC, datetime
from pathlib import Path

import structlog
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from kodit.domain.value_objects import FileProcessingStatus
from kodit.infrastructure.sqlalchemy.embedding_repository import (
    SqlAlchemyEmbeddingRepository,
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

log = structlog.get_logger(__name__)


def generate_random_embedding(dim: int = 750) -> list[float]:
    """Generate a random embedding vector of specified dimension."""
    return [random.uniform(-1, 1) for _ in range(dim)]  # noqa: S311


async def setup_test_data(session: AsyncSession, num_embeddings: int = 5000) -> None:
    """Set up test data with random embeddings."""
    # Create a test index
    source = Source(uri="test", cloned_path="test", source_type=SourceType.FOLDER)
    session.add(source)
    await session.commit()
    index = Index(source_id=source.id)
    session.add(index)
    await session.commit()
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
    session.add(file)
    await session.commit()
    snippet = Snippet(
        file_id=file.id,
        index_id=index.id,
        content="This is a test snippet",
        summary="",
    )
    session.add(snippet)
    await session.commit()

    # Create test embeddings
    embeddings = []
    for _ in range(num_embeddings):
        embedding = Embedding()
        embedding.snippet_id = snippet.id
        embedding.type = EmbeddingType.CODE
        embedding.embedding = generate_random_embedding()
        embeddings.append(embedding)

    session.add_all(embeddings)
    await session.commit()


async def run_benchmark(session: AsyncSession) -> None:
    """Run the semantic search benchmark."""
    # Setup test data
    log.info("Setting up test data...")
    await setup_test_data(session)

    # Create repository instance
    repo = SqlAlchemyEmbeddingRepository(session)

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
    async with async_session() as session:
        await run_benchmark(session)

    # Cleanup
    await engine.dispose()


if __name__ == "__main__":
    asyncio.run(main())
