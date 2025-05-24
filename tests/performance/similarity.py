"""Benchmark script for semantic similarity search performance."""

import asyncio
from pathlib import Path
import random
import time
from typing import List

from sqlalchemy.ext.asyncio import AsyncSession, create_async_engine, async_sessionmaker

from kodit.embedding.embedding_models import Embedding, EmbeddingType
from kodit.indexing.indexing_models import Index, Snippet
from kodit.search.search_repository import SearchRepository
from kodit.source.source_models import File, Source


def generate_random_embedding(dim: int = 750) -> List[float]:
    """Generate a random embedding vector of specified dimension."""
    return [random.uniform(-1, 1) for _ in range(dim)]


async def setup_test_data(session: AsyncSession, num_embeddings: int = 5000) -> None:
    """Set up test data with random embeddings."""
    # Create a test index
    source = Source(uri="test", cloned_path="test")
    session.add(source)
    await session.commit()
    index = Index(source_id=source.id)
    session.add(index)
    await session.commit()
    file = File(
        uri="test",
        cloned_path="test",
        source_id=source.id,
    )
    session.add(file)
    await session.commit()
    snippet = Snippet(
        file_id=file.id,
        index_id=index.id,
        content="This is a test snippet",
    )
    session.add(snippet)
    await session.commit()

    # Create test embeddings
    embeddings = []
    for i in range(num_embeddings):
        embedding = Embedding(
            snippet_id=snippet.id,
            type=EmbeddingType.CODE,
            embedding=generate_random_embedding(),
        )
        embeddings.append(embedding)

    session.add_all(embeddings)
    await session.commit()


async def run_benchmark(session: AsyncSession) -> None:
    """Run the semantic search benchmark."""
    # Setup test data
    print("Setting up test data...")
    await setup_test_data(session)

    # Create repository instance
    repo = SearchRepository(session)

    # Generate a test query embedding
    query_embedding = generate_random_embedding()

    # Run the benchmark
    num_runs = 10
    total_time = 0
    results = []  # Initialize results list

    print("Running warm-up query...")
    # Warm up
    await repo.list_semantic_results(
        embedding_type=EmbeddingType.CODE, embedding=query_embedding, top_k=10
    )

    print(f"\nRunning {num_runs} benchmark queries...")

    # Actual benchmark
    for i in range(num_runs):
        start_time = time.perf_counter()
        results = await repo.list_semantic_results(
            embedding_type=EmbeddingType.CODE, embedding=query_embedding, top_k=10
        )
        end_time = time.perf_counter()
        run_time = end_time - start_time
        total_time += run_time
        print(f"\nRun {i + 1}/{num_runs}: {run_time * 1000:.2f}ms")

    # Calculate average time per run
    avg_time = total_time / num_runs

    print(f"\nSemantic Search Performance Results:")
    print(f"Number of runs: {num_runs}")
    print(f"Total execution time: {total_time:.2f} seconds")
    print(f"Average time per query: {avg_time * 1000:.2f} ms")

    # Print sample results
    print(f"\nSample query returned {len(results)} results")
    if results:  # Add safety check
        print(f"First result score: {results[0][1]:.4f}")


async def main():
    """Main entry point for the benchmark."""
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
