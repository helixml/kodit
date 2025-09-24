"""Performance tests for Git repository service domain classes."""

import asyncio
import cProfile
import pstats
from datetime import UTC, datetime
from pathlib import Path
from typing import Any
from unittest.mock import AsyncMock

import pytest

from kodit.domain.protocols import GitAdapter
from kodit.domain.services.git_repository_service import GitRepositoryScanner


class MockGitAdapterForPerformance(AsyncMock):
    """Mock Git adapter that simulates performance characteristics of a large repo."""

    def __init__(self, num_branches: int = 268, num_commits: int = 17899) -> None:
        super().__init__(spec=GitAdapter)
        self.num_branches = num_branches
        self.num_commits = num_commits

        # Pre-generate data to simulate real repository structure
        self._generate_test_data()

        # Setup method returns
        self.get_all_branches.return_value = self.branch_data
        self.get_all_commits_bulk.return_value = self.all_commits_data
        self.get_branch_commit_shas.side_effect = self._get_branch_commit_shas
        self.get_commit_files.side_effect = self._get_commit_files
        self.get_all_tags.return_value = []

    def _generate_test_data(self) -> None:
        """Generate realistic test data for large repository simulation."""
        # Generate commits with realistic parent relationships
        self.all_commits_data = {}
        self.commit_to_files = {}

        # Create a base commit
        base_commit_sha = f"commit_{0:06d}"
        self.all_commits_data[base_commit_sha] = {
            "sha": base_commit_sha,
            "date": datetime(2020, 1, 1, tzinfo=UTC),
            "message": "Initial commit",
            "parent_sha": "",
            "author_name": "Test Author",
            "author_email": "test@example.com",
        }
        self.commit_to_files[base_commit_sha] = [
            {
                "blob_sha": f"blob_{0:06d}_0",
                "path": "README.md",
                "mime_type": "text/markdown",
                "size": 1024,
            }
        ]

        # Generate the rest of the commits with parent relationships
        for i in range(1, self.num_commits):
            commit_sha = f"commit_{i:06d}"
            # Most commits have the previous commit as parent (linear history)
            # But some have earlier commits to simulate merges and branches
            if i % 50 == 0 and i > 100:  # Simulate merge commits
                parent_sha = f"commit_{i-100:06d}"
            else:
                parent_sha = f"commit_{i-1:06d}"

            self.all_commits_data[commit_sha] = {
                "sha": commit_sha,
                "date": datetime(2020, 1, 1, tzinfo=UTC).replace(day=min(28, (i % 28) + 1)),
                "message": f"Commit #{i}",
                "parent_sha": parent_sha,
                "author_name": "Test Author",
                "author_email": "test@example.com",
            }

            # Each commit touches 1-5 files
            num_files = min(5, (i % 5) + 1)
            files = []
            for f in range(num_files):
                files.append({
                    "blob_sha": f"blob_{i:06d}_{f}",
                    "path": f"src/module_{f % 10}/file_{i % 100}.py",
                    "mime_type": "text/x-python",
                    "size": 512 + (i % 2048),
                })
            self.commit_to_files[commit_sha] = files

        # Generate branches with realistic commit distributions
        self.branch_data = []
        self.branch_to_commits = {}

        # Main branch has most commits
        main_commits = [f"commit_{i:06d}" for i in range(self.num_commits)]
        self.branch_data.append({
            "name": "main",
            "type": "local",
            "head_commit_sha": main_commits[-1],
        })
        self.branch_to_commits["main"] = main_commits

        # Feature branches have varying numbers of commits
        commits_per_branch = max(1, self.num_commits // self.num_branches)

        for i in range(1, self.num_branches):
            branch_name = f"feature/branch_{i:03d}"

            # Each feature branch has a subset of commits
            start_idx = max(0, i * commits_per_branch - 50)  # Some overlap
            end_idx = min(self.num_commits, (i + 1) * commits_per_branch)
            branch_commits = [f"commit_{j:06d}" for j in range(start_idx, end_idx)]

            if branch_commits:  # Only create branch if it has commits
                self.branch_data.append({
                    "name": branch_name,
                    "type": "local",
                    "head_commit_sha": branch_commits[-1],
                })
                self.branch_to_commits[branch_name] = branch_commits

    async def _get_branch_commit_shas(self, local_path: Path, branch_name: str) -> list[str]:
        """Return commit SHAs for a branch."""
        return self.branch_to_commits.get(branch_name, [])

    async def _get_commit_files(self, local_path: Path, commit_sha: str) -> list[dict[str, Any]]:
        """Return files for a commit."""
        return self.commit_to_files.get(commit_sha, [])


@pytest.mark.asyncio
async def test_performance_large_repository_baseline() -> None:
    """Baseline performance test with current implementation."""
    # Simulate the problematic repository size
    mock_adapter = MockGitAdapterForPerformance(num_branches=268, num_commits=17899)
    scanner = GitRepositoryScanner(mock_adapter)
    cloned_path = Path("/tmp/large-test-repo")

    # Profile the scan operation
    profiler = cProfile.Profile()
    profiler.enable()

    start_time = asyncio.get_event_loop().time()
    result = await scanner.scan_repository(cloned_path)
    end_time = asyncio.get_event_loop().time()

    profiler.disable()

    # Print timing results
    execution_time = end_time - start_time

    # Print top time-consuming functions
    stats = pstats.Stats(profiler)
    stats.sort_stats("cumulative")
    stats.print_stats(10)

    # Assert reasonable performance expectations
    # For 268 branches and ~18k commits, this should complete in reasonable time
    assert execution_time < 60.0, f"Scan took too long: {execution_time:.2f}s"
    assert len(result.branches) <= 268
    assert len(result.all_commits) <= 17899


@pytest.mark.asyncio
async def test_performance_bottleneck_analysis() -> None:
    """Detailed analysis of performance bottlenecks in _process_branches_bulk."""
    mock_adapter = MockGitAdapterForPerformance(num_branches=100, num_commits=5000)
    scanner = GitRepositoryScanner(mock_adapter)
    cloned_path = Path("/tmp/test-repo")

    # Profile with method-level detail
    profiler = cProfile.Profile()
    profiler.enable()

    # Get data needed for _process_branches_bulk
    branch_data = await mock_adapter.get_all_branches(cloned_path)
    all_commits_data = await mock_adapter.get_all_commits_bulk(cloned_path)

    # Profile the specific bottleneck method
    branches, commit_cache = await scanner._process_branches_bulk(
        cloned_path, branch_data, all_commits_data
    )

    profiler.disable()

    # Detailed analysis
    stats = pstats.Stats(profiler)
    stats.sort_stats("cumulative")


    # Look specifically for patterns in method calls
    stats.print_stats("_process_branches_bulk")
    stats.print_stats("_create_git_commit_from_data")
    stats.print_stats("get_branch_commit_shas")
    stats.print_stats("get_commit_files")


@pytest.mark.asyncio
async def test_performance_scaling_branches() -> None:
    """Test how performance scales with number of branches."""
    branch_counts = [50, 100, 200, 400]
    results = []

    for num_branches in branch_counts:
        mock_adapter = MockGitAdapterForPerformance(
            num_branches=num_branches,
            num_commits=5000  # Keep commits constant
        )
        scanner = GitRepositoryScanner(mock_adapter)
        cloned_path = Path("/tmp/test-repo")

        start_time = asyncio.get_event_loop().time()
        result = await scanner.scan_repository(cloned_path)
        end_time = asyncio.get_event_loop().time()

        execution_time = end_time - start_time
        results.append((num_branches, execution_time, len(result.branches)))


    # Check if scaling is reasonable (should be roughly linear)
    for i in range(1, len(results)):
        prev_branches, prev_time, _ = results[i-1]
        curr_branches, curr_time, _ = results[i]

        scaling_factor = curr_time / prev_time
        branch_factor = curr_branches / prev_branches

        # Performance should scale roughly linearly with branches
        # Allow some overhead but flag if it's quadratic
        assert scaling_factor < branch_factor * 2, \
            f"Poor scaling: {scaling_factor:.2f}x time for {branch_factor:.2f}x branches"


@pytest.mark.asyncio
async def test_performance_scaling_commits() -> None:
    """Test how performance scales with number of commits."""
    commit_counts = [1000, 2000, 4000, 8000]
    results = []

    for num_commits in commit_counts:
        mock_adapter = MockGitAdapterForPerformance(
            num_branches=100,  # Keep branches constant
            num_commits=num_commits
        )
        scanner = GitRepositoryScanner(mock_adapter)
        cloned_path = Path("/tmp/test-repo")

        start_time = asyncio.get_event_loop().time()
        result = await scanner.scan_repository(cloned_path)
        end_time = asyncio.get_event_loop().time()

        execution_time = end_time - start_time
        results.append((num_commits, execution_time, len(result.all_commits)))


    # Check commit scaling
    for i in range(1, len(results)):
        prev_commits, prev_time, _ = results[i-1]
        curr_commits, curr_time, _ = results[i]

        scaling_factor = curr_time / prev_time
        commit_factor = curr_commits / prev_commits

        # Commit processing should be efficient due to bulk operations
        assert scaling_factor < commit_factor * 1.5, \
            f"Poor commit scaling: {scaling_factor:.2f}x time for {commit_factor:.2f}x commits"


@pytest.mark.asyncio
async def test_performance_memory_usage() -> None:
    """Test memory usage patterns during large repository processing."""
    import tracemalloc

    tracemalloc.start()

    mock_adapter = MockGitAdapterForPerformance(num_branches=268, num_commits=17899)
    scanner = GitRepositoryScanner(mock_adapter)
    cloned_path = Path("/tmp/large-test-repo")

    # Take initial snapshot
    snapshot1 = tracemalloc.take_snapshot()

    await scanner.scan_repository(cloned_path)

    # Take final snapshot
    snapshot2 = tracemalloc.take_snapshot()

    # Analyze memory usage
    top_stats = snapshot2.compare_to(snapshot1, "lineno")


    total_memory_mb = sum(stat.size for stat in top_stats) / 1024 / 1024

    for _stat in top_stats[:5]:
        pass

    tracemalloc.stop()

    # Memory usage should be reasonable for the dataset size
    assert total_memory_mb < 500, f"Excessive memory usage: {total_memory_mb:.1f} MB"
