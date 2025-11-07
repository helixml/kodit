"""Benchmark test for file saving performance during initial indexing.

This test measures the performance of saving file metadata to the database,
which is a known bottleneck during initial repository indexing.
"""

import sys
import tempfile
import time
from collections.abc import Callable
from datetime import UTC, datetime
from pathlib import Path

import pytest
import pytest_asyncio
from pydantic import AnyUrl
from sqlalchemy import text
from sqlalchemy.ext.asyncio import (
    AsyncEngine,
    AsyncSession,
    async_sessionmaker,
    create_async_engine,
)

from kodit.domain.entities.git import GitCommit, GitFile
from kodit.domain.factories.git_repo_factory import GitRepoFactory
from kodit.domain.protocols import GitFileRepository
from kodit.domain.services.git_repository_service import GitRepositoryScanner
from kodit.infrastructure.cloning.git.git_python_adaptor import GitPythonAdapter
from kodit.infrastructure.sqlalchemy.entities import Base
from kodit.infrastructure.sqlalchemy.git_file_repository import (
    create_git_file_repository,
)
from kodit.infrastructure.sqlalchemy.git_repository import create_git_repo_repository


@pytest_asyncio.fixture  # type: ignore[misc]
async def performance_engine():  # noqa: ANN201
    """Create a test database engine for performance tests."""
    with tempfile.TemporaryDirectory() as tmp_dir:
        db_path = Path(tmp_dir) / "file_performance_test.db"
        engine = create_async_engine(
            f"sqlite+aiosqlite:///{db_path}",
            echo=False,
            future=True,
        )

        async with engine.begin() as conn:
            await conn.execute(text("PRAGMA foreign_keys = ON"))
            await conn.run_sync(Base.metadata.create_all)

        yield engine

        async with engine.begin() as conn:
            await conn.run_sync(Base.metadata.drop_all)

        await engine.dispose()


@pytest.fixture
def performance_session_factory(
    performance_engine: AsyncEngine,
) -> Callable[[], AsyncSession]:
    """Create a test database session factory for performance tests."""
    return async_sessionmaker(
        performance_engine, class_=AsyncSession, expire_on_commit=False
    )


class InstrumentedFileSavingService:
    """Service with detailed timing instrumentation for file saving."""

    def __init__(
        self,
        scanner: GitRepositoryScanner,
        file_repo: GitFileRepository,
    ) -> None:
        """Initialize service."""
        self.scanner = scanner
        self.file_repo = file_repo

    async def process_files_with_timing(
        self, cloned_path: Path, commit_shas: list[str], batch_size: int = 100
    ) -> dict[str, float]:
        """Process files with detailed timing breakdown."""
        timing = {
            "total_time": 0.0,
            "git_extraction_time": 0.0,
            "db_save_time": 0.0,
            "batches_processed": 0,
            "total_files": 0,
            "avg_git_per_batch": 0.0,
            "avg_db_per_batch": 0.0,
        }

        total_start = time.perf_counter()
        total_batches = (len(commit_shas) + batch_size - 1) // batch_size

        sys.stderr.write("\n" + "=" * 80 + "\n")
        sys.stderr.write("FILE SAVING PERFORMANCE ANALYSIS\n")
        sys.stderr.write("=" * 80 + "\n")
        sys.stderr.write(
            f"Processing {len(commit_shas)} commits in {total_batches} batches\n"
        )
        sys.stderr.write(f"Batch size: {batch_size}\n\n")

        git_times = []
        db_times = []

        for i in range(0, len(commit_shas), batch_size):
            batch = commit_shas[i : i + batch_size]
            batch_num = i // batch_size + 1

            # Time: Git extraction
            git_start = time.perf_counter()
            files = await self.scanner.process_files_for_commits_batch(
                cloned_path, batch
            )
            git_duration = time.perf_counter() - git_start
            git_times.append(git_duration)
            timing["git_extraction_time"] += git_duration

            # Time: Database save
            db_start = time.perf_counter()
            if files:
                await self.file_repo.save_bulk(files)
                timing["total_files"] += len(files)
            db_duration = time.perf_counter() - db_start
            db_times.append(db_duration)
            timing["db_save_time"] += db_duration

            # Progress output
            if batch_num % 10 == 0 or batch_num == total_batches:
                sys.stderr.write(
                    f"Batch {batch_num}/{total_batches}: "
                    f"{len(files)} files | "
                    f"Git: {git_duration:.2f}s | "
                    f"DB: {db_duration:.2f}s\n"
                )

            timing["batches_processed"] += 1

        timing["total_time"] = time.perf_counter() - total_start
        timing["avg_git_per_batch"] = (
            sum(git_times) / len(git_times) if git_times else 0
        )
        timing["avg_db_per_batch"] = (
            sum(db_times) / len(db_times) if db_times else 0
        )

        # Summary output
        sys.stderr.write("\n" + "-" * 80 + "\n")
        sys.stderr.write("SUMMARY:\n")
        sys.stderr.write(f"  Total files processed: {timing['total_files']}\n")
        sys.stderr.write(f"  Total batches: {timing['batches_processed']}\n")
        sys.stderr.write(f"  Total time: {timing['total_time']:.2f}s\n")
        sys.stderr.write(
            f"  Git extraction time: {timing['git_extraction_time']:.2f}s "
            f"({timing['git_extraction_time']/timing['total_time']*100:.1f}%)\n"
        )
        sys.stderr.write(
            f"  DB save time: {timing['db_save_time']:.2f}s "
            f"({timing['db_save_time']/timing['total_time']*100:.1f}%)\n"
        )
        sys.stderr.write(f"  Avg git per batch: {timing['avg_git_per_batch']:.2f}s\n")
        sys.stderr.write(f"  Avg DB per batch: {timing['avg_db_per_batch']:.2f}s\n")

        if timing["total_files"] > 0:
            git_rate = timing["total_files"] / timing["git_extraction_time"]
            db_rate = timing["total_files"] / timing["db_save_time"]
            overall_rate = timing["total_files"] / timing["total_time"]
            sys.stderr.write(f"  Files/second (git): {git_rate:.1f}\n")
            sys.stderr.write(f"  Files/second (db): {db_rate:.1f}\n")
            sys.stderr.write(f"  Files/second (overall): {overall_rate:.1f}\n")
        sys.stderr.write("=" * 80 + "\n")

        return timing


@pytest.mark.asyncio
async def test_file_saving_performance_ray_repo(
    performance_session_factory: Callable[[], AsyncSession],
) -> None:
    """Measure file saving performance for Ray repository.

    This test benchmarks the critical file saving path during initial indexing,
    which processes file metadata for all commits and saves to the database.
    """
    repo_url = AnyUrl("https://github.com/ray-project/ray")

    with tempfile.TemporaryDirectory() as tmp_clone_dir:
        clone_dir = Path(tmp_clone_dir)
        git_adapter = GitPythonAdapter()

        # Clone the repository
        from kodit.domain.services.git_repository_service import RepositoryCloner

        sys.stderr.write("\nCloning ray-project/ray repository...\n")
        clone_start = time.perf_counter()
        cloner = RepositoryCloner(git_adapter, clone_dir)
        cloned_path = await cloner.clone_repository(repo_url)
        clone_duration = time.perf_counter() - clone_start
        sys.stderr.write(f"Clone completed in {clone_duration:.2f}s\n")

        # Setup repositories
        repo_repository = create_git_repo_repository(
            session_factory=performance_session_factory
        )
        file_repository = create_git_file_repository(
            session_factory=performance_session_factory
        )
        from kodit.infrastructure.sqlalchemy.git_commit_repository import (
            create_git_commit_repository,
        )

        commit_repository = create_git_commit_repository(
            session_factory=performance_session_factory
        )

        # Create repository entity
        repo = GitRepoFactory.create_from_remote_uri(repo_url)
        repo = await repo_repository.save(repo)
        assert repo.id is not None

        # Get commits directly from git
        sys.stderr.write("\nGetting commit list...\n")
        scanner = GitRepositoryScanner(git_adapter)
        all_commits_data = await git_adapter.get_all_commits_bulk(cloned_path)
        sys.stderr.write(f"Found {len(all_commits_data)} commits\n")

        # Create commit entities
        commits = []
        current_time = datetime.now(UTC)
        for commit_sha, commit_data in all_commits_data.items():
            # Format author string
            author_name = commit_data.get("author_name", "")
            author_email = commit_data.get("author_email", "")
            if author_name and author_email:
                author = f"{author_name} <{author_email}>"
            else:
                author = author_name or "Unknown"

            commits.append(
                GitCommit(
                    created_at=current_time,
                    commit_sha=commit_sha,
                    repo_id=repo.id,
                    date=commit_data["date"],
                    message=commit_data["message"],
                    parent_commit_sha=commit_data["parent_sha"],
                    author=author,
                )
            )

        # Save commits to satisfy foreign key constraint
        await commit_repository.save_bulk(commits)

        # Now benchmark the file processing and saving
        instrumented_service = InstrumentedFileSavingService(
            scanner=scanner,
            file_repo=file_repository,
        )

        commit_shas = [commit.commit_sha for commit in commits]
        timing = await instrumented_service.process_files_with_timing(
            cloned_path, commit_shas, batch_size=100
        )

        # Assertions to validate the test
        assert timing["total_files"] > 0, "Should have processed files"
        assert timing["batches_processed"] > 0, "Should have processed batches"
        assert timing["db_save_time"] > 0, "Should have spent time saving to DB"
        assert (
            timing["git_extraction_time"] > 0
        ), "Should have spent time extracting from git"

        # Print performance metrics for analysis
        sys.stderr.write("\n" + "=" * 80 + "\n")
        sys.stderr.write("PERFORMANCE METRICS:\n")
        ratio = timing["db_save_time"] / timing["git_extraction_time"]
        sys.stderr.write(f"  DB save is {ratio:.1f}x ")
        comparison = (
            "slower than git extraction\n"
            if timing["db_save_time"] > timing["git_extraction_time"]
            else "faster than git extraction\n"
        )
        sys.stderr.write(comparison)

        # Calculate approximate time for full processing
        files_per_commit = (
            timing["total_files"] / len(commit_shas) if commit_shas else 0
        )
        sys.stderr.write(f"  Avg files per commit: {files_per_commit:.1f}\n")
        sys.stderr.write("=" * 80 + "\n")


@pytest.mark.asyncio
async def test_file_saving_performance_synthetic_data(
    performance_session_factory: Callable[[], AsyncSession],
) -> None:
    """Test file saving performance with synthetic data.

    This test creates synthetic file data to isolate and measure
    pure database save performance without git operations.
    """
    from kodit.domain.entities.git import GitCommit
    from kodit.infrastructure.sqlalchemy.git_commit_repository import (
        create_git_commit_repository,
    )

    file_repository = create_git_file_repository(
        session_factory=performance_session_factory
    )
    commit_repository = create_git_commit_repository(
        session_factory=performance_session_factory
    )

    sys.stderr.write("\n" + "=" * 80 + "\n")
    sys.stderr.write("SYNTHETIC FILE SAVING BENCHMARK\n")
    sys.stderr.write("=" * 80 + "\n")

    # Create synthetic files
    num_commits = 1000
    files_per_commit = 50
    total_files = num_commits * files_per_commit

    sys.stderr.write(f"Creating {total_files} synthetic files ")
    sys.stderr.write(f"({num_commits} commits x {files_per_commit} files)\n\n")

    # First create commits (needed for foreign key)
    sys.stderr.write("Creating commits...")
    synthetic_commits = []
    synthetic_files: list[GitFile] = []
    current_time = datetime.now(UTC)

    for commit_idx in range(num_commits):
        commit_sha = f"{'0' * 32}{commit_idx:08x}"
        synthetic_commits.append(
            GitCommit(
                repo_id=1,
                commit_sha=commit_sha,
                created_at=current_time,
                date=current_time,
                message=f"Synthetic commit {commit_idx}",
                author=f"author{commit_idx}@example.com",
            )
        )
        synthetic_files.extend(
            GitFile(
                commit_sha=commit_sha,
                created_at=current_time,
                blob_sha=f"blob{'0' * 32}{file_idx:08x}",
                path=f"src/module{commit_idx}/file{file_idx}.py",
                mime_type="text/x-python",
                size=1024 + file_idx,
                extension=".py",
            )
            for file_idx in range(files_per_commit)
        )

    # Create repo for commits
    repo_repository = create_git_repo_repository(
        session_factory=performance_session_factory
    )
    repo = GitRepoFactory.create_from_remote_uri(AnyUrl("https://github.com/test/synthetic"))
    repo = await repo_repository.save(repo)

    # Update commits with actual repo_id
    for commit in synthetic_commits:
        assert repo.id is not None
        commit.repo_id = repo.id

    await commit_repository.save_bulk(synthetic_commits)
    sys.stderr.write(" Done\n")

    # Benchmark bulk save with different batch sizes
    for batch_size in [100, 500, 1000]:
        sys.stderr.write(f"\nTesting batch size: {batch_size}\n")

        save_start = time.perf_counter()
        batches_saved = 0
        files_saved = 0

        for i in range(0, len(synthetic_files), batch_size):
            batch = synthetic_files[i : i + batch_size]
            await file_repository.save_bulk(batch)
            batches_saved += 1
            files_saved += len(batch)

            if batches_saved % 10 == 0:
                sys.stderr.write(
                    f"  Saved {batches_saved} batches, {files_saved} files\n"
                )

        save_duration = time.perf_counter() - save_start

        sys.stderr.write(f"  Total time: {save_duration:.2f}s\n")
        sys.stderr.write(f"  Files/second: {total_files/save_duration:.1f}\n")
        sys.stderr.write(f"  Seconds/batch: {save_duration/batches_saved:.3f}s\n")

        # Clean up for next test
        from kodit.infrastructure.sqlalchemy.query import QueryBuilder
        await file_repository.delete_by_query(QueryBuilder())

    sys.stderr.write("\n" + "=" * 80 + "\n")
