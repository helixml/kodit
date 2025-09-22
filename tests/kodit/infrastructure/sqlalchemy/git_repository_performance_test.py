"""Performance tests for SqlAlchemyGitRepoRepository."""

import asyncio
import time
from collections.abc import Callable
from datetime import UTC, datetime
from pathlib import Path

import pytest
from pydantic import AnyUrl
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities.git import GitBranch, GitCommit, GitFile, GitRepo, GitTag
from kodit.infrastructure.sqlalchemy.git_repository import SqlAlchemyGitRepoRepository


@pytest.fixture
def repository(
    session_factory: Callable[[], AsyncSession],
) -> SqlAlchemyGitRepoRepository:
    """Create a repository with a session factory."""
    return SqlAlchemyGitRepoRepository(session_factory)


def create_large_git_repo(repo_count: int, commits_per_repo: int, files_per_commit: int) -> list[GitRepo]:
    """Create multiple git repositories with configurable scale for performance testing."""
    repos = []

    for repo_idx in range(repo_count):
        # Create files for each commit
        all_commits = []
        for commit_idx in range(commits_per_repo):
            files = []
            for file_idx in range(files_per_commit):
                file = GitFile(
                    created_at=datetime.now(UTC),
                    blob_sha=f"blob_sha_{repo_idx}_{commit_idx}_{file_idx}",
                    path=f"src/module_{file_idx}/file_{file_idx}.py",
                    mime_type="text/x-python",
                    size=1024 + file_idx * 100,
                    extension="py",
                )
                files.append(file)

            commit = GitCommit(
                created_at=datetime.now(UTC),
                updated_at=datetime.now(UTC),
                commit_sha=f"commit_sha_{repo_idx}_{commit_idx}",
                date=datetime.now(UTC),
                message=f"Commit {commit_idx} in repo {repo_idx}",
                parent_commit_sha=f"parent_sha_{repo_idx}_{commit_idx - 1}" if commit_idx > 0 else None,
                files=files,
                author=f"Author {repo_idx}",
            )
            all_commits.append(commit)

        # Create branches pointing to commits
        branches = []
        for branch_idx in range(min(3, commits_per_repo)):  # Max 3 branches per repo
            branch = GitBranch(
                name=f"branch_{branch_idx}",
                head_commit=all_commits[branch_idx],
            )
            branches.append(branch)

        # Create tags
        tags = []
        for tag_idx in range(min(2, commits_per_repo)):  # Max 2 tags per repo
            tag = GitTag(
                created_at=datetime.now(UTC),
                updated_at=datetime.now(UTC),
                name=f"v1.{tag_idx}.0",
                target_commit=all_commits[tag_idx],
            )
            tags.append(tag)

        repo = GitRepo(
            sanitized_remote_uri=AnyUrl(f"https://github.com/test/repo_{repo_idx}.git"),
            remote_uri=AnyUrl(f"https://github.com/test/repo_{repo_idx}.git"),
            cloned_path=f"/tmp/test_repos/repo_{repo_idx}",
            last_scanned_at=datetime.now(UTC),
            commits=all_commits,
            branches=branches,
            tags=tags,
            tracking_branch=branches[0] if branches else None,
        )
        repos.append(repo)

    return repos


class TestGitRepositoryPerformance:
    """Performance tests for git repository operations."""

    @pytest.mark.asyncio
    async def test_get_all_performance_small_scale(
        self, repository: SqlAlchemyGitRepoRepository
    ) -> None:
        """Test get_all performance with small scale data."""
        # Setup: Create 3 repos with 5 commits each, 3 files per commit
        repos = create_large_git_repo(repo_count=3, commits_per_repo=5, files_per_commit=3)

        # Save all repos
        for repo in repos:
            await repository.save(repo)

        # Measure get_all performance
        start_time = time.perf_counter()
        retrieved_repos = await repository.get_all()
        end_time = time.perf_counter()

        elapsed_time = end_time - start_time

        # Assertions
        assert len(retrieved_repos) == 3
        print(f"\nSmall scale get_all took: {elapsed_time:.4f} seconds")
        print(f"Total commits across all repos: {sum(len(r.commits) for r in retrieved_repos)}")
        print(f"Total files across all repos: {sum(len(f) for r in retrieved_repos for c in r.commits for f in [c.files])}")

        # Performance expectation: should complete within reasonable time
        assert elapsed_time < 2.0, f"get_all took too long: {elapsed_time:.4f}s"

    @pytest.mark.asyncio
    async def test_get_all_performance_medium_scale(
        self, repository: SqlAlchemyGitRepoRepository
    ) -> None:
        """Test get_all performance with medium scale data."""
        # Setup: Create 5 repos with 20 commits each, 10 files per commit
        repos = create_large_git_repo(repo_count=5, commits_per_repo=20, files_per_commit=10)

        # Save all repos
        for repo in repos:
            await repository.save(repo)

        # Measure get_all performance
        start_time = time.perf_counter()
        retrieved_repos = await repository.get_all()
        end_time = time.perf_counter()

        elapsed_time = end_time - start_time

        # Assertions
        assert len(retrieved_repos) == 5
        print(f"\nMedium scale get_all took: {elapsed_time:.4f} seconds")
        print(f"Total commits across all repos: {sum(len(r.commits) for r in retrieved_repos)}")
        print(f"Total files across all repos: {sum(len(f) for r in retrieved_repos for c in r.commits for f in [c.files])}")

        # Performance expectation: should complete within reasonable time
        assert elapsed_time < 10.0, f"get_all took too long: {elapsed_time:.4f}s"

    @pytest.mark.asyncio
    async def test_load_complete_repo_performance(
        self, repository: SqlAlchemyGitRepoRepository
    ) -> None:
        """Test _load_complete_repo performance in isolation."""
        from kodit.infrastructure.sqlalchemy.unit_of_work import SqlAlchemyUnitOfWork
        from kodit.infrastructure.sqlalchemy import entities as db_entities
        from sqlalchemy import select

        # Setup: Create 1 large repo with many commits and files
        large_repo = create_large_git_repo(repo_count=1, commits_per_repo=50, files_per_commit=20)[0]

        # Save the repo
        saved_repo = await repository.save(large_repo)

        # Test _load_complete_repo performance
        async with SqlAlchemyUnitOfWork(repository.session_factory) as session:
            # Get the db_repo entity
            stmt = select(db_entities.GitRepo).where(db_entities.GitRepo.id == saved_repo.id)
            db_repo = await session.scalar(stmt)

            # Measure _load_complete_repo performance
            start_time = time.perf_counter()
            loaded_repo = await repository._load_complete_repo(session, db_repo)
            end_time = time.perf_counter()

            elapsed_time = end_time - start_time

            # Assertions
            assert loaded_repo.id == saved_repo.id
            assert len(loaded_repo.commits) == 50
            total_files = sum(len(c.files) for c in loaded_repo.commits)
            assert total_files == 50 * 20  # 50 commits * 20 files each

            print(f"\n_load_complete_repo took: {elapsed_time:.4f} seconds")
            print(f"Loaded {len(loaded_repo.commits)} commits")
            print(f"Loaded {total_files} files")
            print(f"Loaded {len(loaded_repo.branches)} branches")
            print(f"Loaded {len(loaded_repo.tags)} tags")

            # Performance expectation
            assert elapsed_time < 5.0, f"_load_complete_repo took too long: {elapsed_time:.4f}s"

    @pytest.mark.asyncio
    async def test_get_all_vs_individual_load_performance(
        self, repository: SqlAlchemyGitRepoRepository
    ) -> None:
        """Compare performance of get_all vs loading repos individually."""
        # Setup: Create multiple repos
        repos = create_large_git_repo(repo_count=4, commits_per_repo=15, files_per_commit=8)

        # Save all repos
        repo_ids = []
        for repo in repos:
            saved_repo = await repository.save(repo)
            repo_ids.append(saved_repo.id)

        # Method 1: get_all
        start_time = time.perf_counter()
        all_repos_bulk = await repository.get_all()
        bulk_time = time.perf_counter() - start_time

        # Method 2: individual loads
        start_time = time.perf_counter()
        all_repos_individual = []
        for repo_id in repo_ids:
            repo = await repository.get_by_id(repo_id)
            all_repos_individual.append(repo)
        individual_time = time.perf_counter() - start_time

        # Results
        print(f"\nget_all() took: {bulk_time:.4f} seconds")
        print(f"Individual loads took: {individual_time:.4f} seconds")
        print(f"Ratio (individual/bulk): {individual_time/bulk_time:.2f}x")

        # Validate same data
        assert len(all_repos_bulk) == len(all_repos_individual) == 4

        # Performance analysis
        if individual_time > bulk_time:
            print(f"get_all is {individual_time/bulk_time:.2f}x faster than individual loads")
        else:
            print(f"Individual loads are {bulk_time/individual_time:.2f}x faster than get_all")

    @pytest.mark.asyncio
    async def test_real_world_repo_performance(
        self, repository: SqlAlchemyGitRepoRepository
    ) -> None:
        """Test performance with real-world repo size (218 commits, 9 branches)."""
        # Create repo similar to your real scenario: 218 commits, 9 branches, ~25 files per commit
        repo = create_large_git_repo(repo_count=1, commits_per_repo=218, files_per_commit=25)[0]

        # Save the repo
        print(f"\nSaving repo with {len(repo.commits)} commits and {sum(len(c.files) for c in repo.commits)} files...")
        save_start = time.perf_counter()
        saved_repo = await repository.save(repo)
        save_time = time.perf_counter() - save_start
        print(f"Save took: {save_time:.4f} seconds")

        # Test _load_complete_repo performance
        from kodit.infrastructure.sqlalchemy.unit_of_work import SqlAlchemyUnitOfWork
        from kodit.infrastructure.sqlalchemy import entities as db_entities
        from sqlalchemy import select

        async with SqlAlchemyUnitOfWork(repository.session_factory) as session:
            stmt = select(db_entities.GitRepo).where(db_entities.GitRepo.id == saved_repo.id)
            db_repo = await session.scalar(stmt)

            # Measure _load_complete_repo performance
            load_start = time.perf_counter()
            loaded_repo = await repository._load_complete_repo(session, db_repo)
            load_time = time.perf_counter() - load_start

            print(f"\nReal-world repo _load_complete_repo took: {load_time:.4f} seconds")
            print(f"Loaded {len(loaded_repo.commits)} commits")
            print(f"Loaded {sum(len(c.files) for c in loaded_repo.commits)} files")
            print(f"Loaded {len(loaded_repo.branches)} branches")
            print(f"Loaded {len(loaded_repo.tags)} tags")

            # Performance expectation for real-world usage
            if load_time > 1.0:
                print(f"WARNING: Load time {load_time:.4f}s exceeds 1 second threshold!")

            # Test limited loading for comparison
            limited_start = time.perf_counter()
            limited_repo = await repository._load_repo_with_limits(
                session, db_repo, max_commits=50, max_files_per_commit=10
            )
            limited_time = time.perf_counter() - limited_start

            print(f"\nLimited load (50 commits, 10 files/commit) took: {limited_time:.4f} seconds")
            print(f"Limited loaded {len(limited_repo.commits)} commits")
            print(f"Limited loaded {sum(len(c.files) for c in limited_repo.commits)} files")
            print(f"Performance improvement: {(load_time - limited_time)/load_time*100:.1f}%")

    @pytest.mark.asyncio
    async def test_database_query_breakdown(
        self, repository: SqlAlchemyGitRepoRepository
    ) -> None:
        """Break down timing of individual database queries in _load_complete_repo."""
        from kodit.infrastructure.sqlalchemy.unit_of_work import SqlAlchemyUnitOfWork
        from kodit.infrastructure.sqlalchemy import entities as db_entities
        from sqlalchemy import select

        # Setup: Create 1 repo with substantial data
        repo = create_large_git_repo(repo_count=1, commits_per_repo=30, files_per_commit=15)[0]
        saved_repo = await repository.save(repo)

        # Measure individual query times
        async with SqlAlchemyUnitOfWork(repository.session_factory) as session:
            stmt = select(db_entities.GitRepo).where(db_entities.GitRepo.id == saved_repo.id)
            db_repo = await session.scalar(stmt)

            timings = {}

            # Time branches query
            start = time.perf_counter()
            branches = list((await session.scalars(
                select(db_entities.GitBranch).where(db_entities.GitBranch.repo_id == db_repo.id)
            )).all())
            timings['branches'] = time.perf_counter() - start

            # Time commits query
            start = time.perf_counter()
            commits = list((await session.scalars(
                select(db_entities.GitCommit).where(db_entities.GitCommit.repo_id == db_repo.id)
            )).all())
            timings['commits'] = time.perf_counter() - start

            # Time tags query
            start = time.perf_counter()
            tags = list((await session.scalars(
                select(db_entities.GitTag).where(db_entities.GitTag.repo_id == db_repo.id)
            )).all())
            timings['tags'] = time.perf_counter() - start

            # Time files query (potentially the slowest)
            start = time.perf_counter()
            files = list((await session.scalars(
                select(db_entities.GitCommitFile).where(
                    db_entities.GitCommitFile.commit_sha.in_([c.commit_sha for c in commits])
                )
            )).all())
            timings['files'] = time.perf_counter() - start

            # Time tracking branch query
            start = time.perf_counter()
            tracking_branch = await session.scalar(
                select(db_entities.GitTrackingBranch).where(
                    db_entities.GitTrackingBranch.repo_id == db_repo.id
                )
            )
            timings['tracking_branch'] = time.perf_counter() - start

            # Results
            print(f"\nQuery timing breakdown:")
            total_time = sum(timings.values())
            for query_type, timing in sorted(timings.items(), key=lambda x: x[1], reverse=True):
                percentage = (timing / total_time) * 100
                print(f"  {query_type}: {timing:.4f}s ({percentage:.1f}%)")
            print(f"  Total: {total_time:.4f}s")

            # Analysis
            slowest_query = max(timings.items(), key=lambda x: x[1])
            print(f"\nSlowest query: {slowest_query[0]} ({slowest_query[1]:.4f}s)")

            # Data counts
            print(f"\nData loaded:")
            print(f"  Branches: {len(branches)}")
            print(f"  Commits: {len(commits)}")
            print(f"  Tags: {len(tags)}")
            print(f"  Files: {len(files)}")