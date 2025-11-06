"""Debug test to analyze scan performance bottlenecks.

This test adds detailed timing instrumentation to identify where time is being
spent during the repository scanning operation.
"""

import sys
import tempfile
import time
from collections.abc import AsyncGenerator, Callable
from datetime import datetime
from pathlib import Path

import pytest
from pydantic import AnyUrl
from sqlalchemy import text
from sqlalchemy.ext.asyncio import (
    AsyncEngine,
    AsyncSession,
    async_sessionmaker,
    create_async_engine,
)

from kodit.domain.entities.git import GitBranch, RepositoryScanResult
from kodit.domain.factories.git_repo_factory import GitRepoFactory
from kodit.domain.services.git_repository_service import GitRepositoryScanner
from kodit.infrastructure.cloning.git.git_python_adaptor import GitPythonAdapter
from kodit.infrastructure.sqlalchemy.entities import Base
from kodit.infrastructure.sqlalchemy.git_repository import create_git_repo_repository


@pytest.fixture
async def performance_engine() -> AsyncGenerator[AsyncEngine, None]:
    """Create a test database engine for performance tests."""
    with tempfile.TemporaryDirectory() as tmp_dir:
        db_path = Path(tmp_dir) / "performance_test.db"
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


class InstrumentedGitRepositoryScanner(GitRepositoryScanner):
    """Scanner with detailed timing instrumentation."""

    async def scan_repository(
        self, cloned_path: Path, repo_id: int, since_date: datetime | None = None
    ) -> RepositoryScanResult:
        """Scan repository with detailed timing."""
        sys.stderr.write("\n" + "=" * 80 + "\n")
        sys.stderr.write("DETAILED SCAN PERFORMANCE ANALYSIS\n")
        sys.stderr.write("=" * 80 + "\n")

        total_start = time.perf_counter()

        # Step 1: Get all branches
        step_start = time.perf_counter()
        branch_data = await self.git_adapter.get_all_branches(cloned_path)
        step_duration = time.perf_counter() - step_start
        sys.stderr.write(
            f"\n1. Get all branches: {step_duration:.2f}s "
            f"({len(branch_data)} branches)\n"
        )

        # Step 2: Get all commits bulk
        step_start = time.perf_counter()
        all_commits_data = await self.git_adapter.get_all_commits_bulk(
            cloned_path, since_date=since_date
        )
        step_duration = time.perf_counter() - step_start
        sys.stderr.write(
            f"2. Get all commits (bulk): {step_duration:.2f}s "
            f"({len(all_commits_data)} commits)\n"
        )

        # Step 3: Process branches
        step_start = time.perf_counter()
        branches, commit_cache = await self._process_branches_bulk_instrumented(
            cloned_path, branch_data, all_commits_data, repo_id
        )
        step_duration = time.perf_counter() - step_start
        sys.stderr.write(
            f"3. Process branches: {step_duration:.2f}s ({len(branches)} branches)\n"
        )

        # Step 4: Process tags
        step_start = time.perf_counter()
        tags = await self._process_tags(cloned_path, commit_cache, repo_id)
        step_duration = time.perf_counter() - step_start
        sys.stderr.write(f"4. Process tags: {step_duration:.2f}s ({len(tags)} tags)\n")

        # Step 5: Create result
        step_start = time.perf_counter()
        result = self._create_scan_result(branches, commit_cache, tags, [], cloned_path)
        step_duration = time.perf_counter() - step_start
        sys.stderr.write(f"5. Create scan result: {step_duration:.2f}s\n")

        total_duration = time.perf_counter() - total_start
        sys.stderr.write(f"\nTotal scan time: {total_duration:.2f}s\n")
        sys.stderr.write("=" * 80 + "\n")

        return result

    async def _process_branches_bulk_instrumented(
        self,
        cloned_path: Path,
        branch_data: list[dict],
        all_commits_data: dict,
        repo_id: int,
    ) -> tuple[list[GitBranch], dict]:
        """Process branches with detailed timing."""
        from datetime import UTC

        sys.stderr.write("\n   Branch Processing Breakdown:\n")

        # Sub-step 3.1: Create commit cache
        step_start = time.perf_counter()
        branches = []
        commit_cache = {}
        current_time = datetime.now(UTC)

        for commit_sha, commit_data in all_commits_data.items():
            git_commit = self._create_lightweight_git_commit(
                commit_data, current_time, repo_id
            )
            if git_commit:
                commit_cache[commit_sha] = git_commit
        step_duration = time.perf_counter() - step_start
        sys.stderr.write(f"   3.1 Build commit cache: {step_duration:.2f}s\n")

        # Sub-step 3.2: Get all branch head SHAs in bulk (optimized)
        step_start = time.perf_counter()
        branch_names = [branch_info["name"] for branch_info in branch_data]
        branch_head_shas = await self.git_adapter.get_all_branch_head_shas(
            cloned_path, branch_names
        )
        step_duration = time.perf_counter() - step_start
        sys.stderr.write(
            f"   3.2 Get all branch head SHAs (bulk): {step_duration:.2f}s "
            f"({len(branch_names)} branches)\n"
        )

        # Sub-step 3.3: Create branch entities
        step_start = time.perf_counter()
        for branch_info in branch_data:
            branch_name = branch_info["name"]
            head_sha = branch_head_shas.get(branch_name)

            if head_sha and head_sha in commit_cache:
                head_commit = commit_cache[head_sha]
                branch = self._create_branch_entity(
                    repo_id,
                    current_time,
                    branch_name,
                    head_commit.commit_sha,
                )
                branches.append(branch)

        step_duration = time.perf_counter() - step_start
        sys.stderr.write(f"   3.3 Create branch entities: {step_duration:.2f}s\n")

        return branches, commit_cache

    def _create_branch_entity(
        self, repo_id: int, created_at: datetime, name: str, head_commit_sha: str
    ) -> GitBranch:
        """Create a GitBranch entity."""
        return GitBranch(
            repo_id=repo_id,
            created_at=created_at,
            name=name,
            head_commit_sha=head_commit_sha,
        )


@pytest.mark.asyncio
async def test_scan_performance_with_detailed_timing(
    performance_session_factory: Callable[[], AsyncSession],
) -> None:
    """Analyze scan performance with detailed timing breakdown.

    This test identifies performance bottlenecks in the scan operation by
    instrumenting each step with timing measurements.
    """
    # Repository to test
    repo_url = AnyUrl("https://github.com/helixml/helix")

    # Use a pre-cloned repository if available to focus on scan performance
    # This assumes the clone test has already run
    with tempfile.TemporaryDirectory() as tmp_clone_dir:
        clone_dir = Path(tmp_clone_dir)
        git_adapter = GitPythonAdapter()

        # Clone the repository
        from kodit.domain.services.git_repository_service import RepositoryCloner

        sys.stderr.write("\nCloning repository (not timed for this analysis)...\n")
        cloner = RepositoryCloner(git_adapter, clone_dir)
        cloned_path = await cloner.clone_repository(repo_url)

        repo_repository = create_git_repo_repository(
            session_factory=performance_session_factory
        )

        # Create repository entity
        repo = GitRepoFactory.create_from_remote_uri(repo_url)
        repo = await repo_repository.save(repo)
        assert repo.id is not None

        # Create instrumented scanner
        scanner = InstrumentedGitRepositoryScanner(git_adapter)

        # Run scan with detailed timing
        scan_result = await scanner.scan_repository(cloned_path, repo.id)

        # Basic assertions
        assert scan_result.all_commits, "Should have found commits"
        assert scan_result.branches, "Should have found branches"
