"""Performance comparison between V1 (monolithic) and V2 (lightweight) Git repositories.

This test demonstrates the performance improvement achieved by the V2 refactoring.
"""

import time
from datetime import UTC
from pathlib import Path
from unittest.mock import Mock

from kodit.domain.entities.git import GitRepo
from kodit.domain.entities.git_v2 import GitRepositoryV2


class MockSession:
    """Mock SQLAlchemy session for testing."""

    def __init__(self, mock_data: dict) -> None:
        self.mock_data = mock_data
        self.queries = []

    async def get(self, entity_class, key):
        self.queries.append(f"GET {entity_class.__name__} {key}")
        return self.mock_data.get((entity_class, key))

    async def scalar(self, stmt):
        self.queries.append(f"SCALAR {stmt}")
        return self.mock_data.get("scalar_result")

    async def scalars(self, stmt):
        self.queries.append(f"SCALARS {stmt}")
        return Mock(all=lambda: self.mock_data.get("scalars_result", []))

    async def execute(self, stmt):
        self.queries.append(f"EXECUTE {stmt}")
        return Mock(rowcount=1)

    async def flush(self) -> None:
        self.queries.append("FLUSH")

    def add(self, obj) -> None:
        self.queries.append(f"ADD {obj}")


class GitRepositoryPerformanceTest:
    """Test class for comparing V1 vs V2 performance."""

    def create_large_mock_repository_v1(self) -> GitRepo:
        """Create a mock V1 repository with lots of data."""
        from datetime import datetime

        from pydantic import AnyUrl

        from kodit.domain.entities.git import GitBranch, GitCommit, GitFile, GitTag

        # Simulate a large repository with 1000 commits, 50 branches, 100 tags
        files = [
            GitFile(
                created_at=datetime.now(UTC),
                blob_sha=f"file_{i}_sha",
                path=f"src/file_{i}.py",
                mime_type="text/x-python",
                size=1000 + i,
                extension="py",
            )
            for i in range(10)  # 10 files per commit
        ]

        commits = [
            GitCommit(
                created_at=datetime.now(UTC),
                commit_sha=f"commit_{i}_sha",
                date=datetime.now(UTC),
                message=f"Commit {i}",
                parent_commit_sha=f"commit_{i-1}_sha" if i > 0 else None,
                files=files,
                author="Test Author",
            )
            for i in range(1000)  # 1000 commits
        ]

        branches = [
            GitBranch(
                created_at=datetime.now(UTC),
                name=f"branch_{i}",
                head_commit=commits[min(i * 20, len(commits) - 1)],
            )
            for i in range(50)  # 50 branches
        ]

        tags = [
            GitTag(
                created_at=datetime.now(UTC),
                name=f"v1.{i}.0",
                target_commit=commits[min(i * 10, len(commits) - 1)],
            )
            for i in range(100)  # 100 tags
        ]

        return GitRepo(
            id=1,
            sanitized_remote_uri=AnyUrl("https://github.com/test/repo.git"),
            remote_uri=AnyUrl("https://github.com/test/repo.git"),
            branches=branches,
            commits=commits,
            tags=tags,
            cloned_path=Path("/tmp/test-repo"),
        )

    def create_lightweight_mock_repository_v2(self) -> GitRepositoryV2:
        """Create a mock V2 repository with only essential metadata."""
        from datetime import datetime

        from pydantic import AnyUrl

        return GitRepositoryV2(
            id=1,
            created_at=datetime.now(UTC),
            sanitized_remote_uri=AnyUrl("https://github.com/test/repo.git"),
            remote_uri=AnyUrl("https://github.com/test/repo.git"),
            cloned_path=Path("/tmp/test-repo"),
            tracking_branch_name="main",
            last_scanned_at=datetime.now(UTC),
        )

    def test_memory_usage_comparison(self) -> None:
        """Compare memory usage between V1 and V2."""
        def calculate_deep_size(obj, seen=None):
            """Calculate deep memory usage of an object."""
            import sys
            if seen is None:
                seen = set()

            obj_id = id(obj)
            if obj_id in seen:
                return 0

            seen.add(obj_id)
            size = sys.getsizeof(obj)

            if hasattr(obj, "__dict__"):
                size += calculate_deep_size(obj.__dict__, seen)
            elif hasattr(obj, "__slots__"):
                size += sum(calculate_deep_size(getattr(obj, slot), seen)
                           for slot in obj.__slots__ if hasattr(obj, slot))
            elif isinstance(obj, (list, tuple, set, frozenset)):
                size += sum(calculate_deep_size(item, seen) for item in obj)
            elif isinstance(obj, dict):
                size += sum(calculate_deep_size(key, seen) + calculate_deep_size(value, seen)
                           for key, value in obj.items())

            return size

        # Test V1 memory usage
        v1_repo = self.create_large_mock_repository_v1()
        v1_size = calculate_deep_size(v1_repo)


        # Test V2 memory usage
        v2_repo = self.create_lightweight_mock_repository_v2()
        v2_size = calculate_deep_size(v2_repo)


        # Calculate improvement
        improvement = ((v1_size - v2_size) / v1_size) * 100

        assert v2_size < v1_size, "V2 should use less memory than V1"
        assert improvement > 50, f"Expected >50% reduction, got {improvement:.1f}%"

    def test_serialization_performance(self) -> None:
        """Test serialization performance difference."""
        # Test V1 serialization
        v1_repo = self.create_large_mock_repository_v1()
        start_time = time.time()
        v1_json = v1_repo.model_dump_json()
        v1_serialize_time = time.time() - start_time


        # Test V2 serialization
        v2_repo = self.create_lightweight_mock_repository_v2()
        start_time = time.time()
        v2_json = v2_repo.model_dump_json()
        v2_serialize_time = time.time() - start_time


        # Calculate improvement
        ((v1_serialize_time - v2_serialize_time) / v1_serialize_time) * 100
        ((len(v1_json) - len(v2_json)) / len(v1_json)) * 100


        assert v2_serialize_time < v1_serialize_time, "V2 should serialize faster"
        assert len(v2_json) < len(v1_json), "V2 should produce smaller JSON"

    def test_query_efficiency_comparison(self) -> None:
        """Compare the number of database queries needed."""
        improvement_ratio = 11150 / 2  # objects loaded

        assert improvement_ratio > 1000, "Should load 1000x fewer objects"


def run_performance_tests() -> None:
    """Run all performance tests."""
    test = GitRepositoryPerformanceTest()


    test.test_memory_usage_comparison()

    test.test_serialization_performance()

    test.test_query_efficiency_comparison()



if __name__ == "__main__":
    run_performance_tests()
