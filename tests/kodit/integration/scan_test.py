"""Simple integration tests for repository scanning."""

import tempfile
from datetime import UTC, datetime, timedelta
from pathlib import Path

import pytest
from pydantic import AnyUrl

from kodit.domain.services.git_repository_service import GitRepositoryScanner
from kodit.infrastructure.cloning.git.git_python_adaptor import GitPythonAdapter


@pytest.mark.asyncio
async def test_full_scan_helix_repo() -> None:
    """Test full scan finds all commits, branches, and tags."""
    repo_url = AnyUrl("https://github.com/helixml/helix")

    with tempfile.TemporaryDirectory() as tmp_dir:
        # Clone
        from git import Repo

        clone_path = Path(tmp_dir) / "helix"
        Repo.clone_from(str(repo_url), clone_path)

        # Scan
        adapter = GitPythonAdapter()
        scanner = GitRepositoryScanner(adapter)
        result = await scanner.scan_repository(clone_path, repo_id=1)

        # Verify we found data
        assert len(result.all_commits) > 19000, "Should find ~19k commits"
        assert len(result.branches) > 250, "Should find 250+ branches"
        assert len(result.all_tags) > 300, "Should find 300+ tags"


@pytest.mark.asyncio
async def test_incremental_scan_only_new_commits() -> None:
    """Test incremental scan only returns new commits after a date."""
    repo_url = AnyUrl("https://github.com/helixml/helix")

    with tempfile.TemporaryDirectory() as tmp_dir:
        # Clone
        from git import Repo

        clone_path = Path(tmp_dir) / "helix"
        Repo.clone_from(str(repo_url), clone_path)

        # Full scan
        adapter = GitPythonAdapter()
        scanner = GitRepositoryScanner(adapter)
        full_result = await scanner.scan_repository(clone_path, repo_id=1)
        full_count = len(full_result.all_commits)

        # Incremental scan with recent date (last 7 days)
        since_date = datetime.now(UTC) - timedelta(days=7)
        inc_result = await scanner.scan_repository(
            clone_path, repo_id=1, since_date=since_date
        )
        inc_count = len(inc_result.all_commits)

        # Should have fewer commits in incremental scan
        assert inc_count < full_count, "Incremental should find fewer commits"
        assert inc_count < 1000, "Should find <1000 commits in last week"


@pytest.mark.asyncio
async def test_scan_result_structure() -> None:
    """Test scan result has expected structure."""
    repo_url = AnyUrl("https://github.com/helixml/helix")

    with tempfile.TemporaryDirectory() as tmp_dir:
        # Clone
        from git import Repo

        clone_path = Path(tmp_dir) / "helix"
        Repo.clone_from(str(repo_url), clone_path)

        # Scan
        adapter = GitPythonAdapter()
        scanner = GitRepositoryScanner(adapter)
        result = await scanner.scan_repository(clone_path, repo_id=1)

        # Verify structure
        assert result.branches
        assert result.all_commits
        assert result.all_tags
        assert result.scan_timestamp

        # Verify branch has main or master
        branch_names = {b.name for b in result.branches}
        assert "main" in branch_names or "master" in branch_names

        # Verify commits have required fields
        first_commit = result.all_commits[0]
        assert first_commit.commit_sha
        assert first_commit.date
        assert first_commit.message
        assert first_commit.author
        assert first_commit.repo_id == 1

        # Verify tags have required fields
        if result.all_tags:
            first_tag = result.all_tags[0]
            assert first_tag.name
            assert first_tag.target_commit_sha
            assert first_tag.repo_id == 1
