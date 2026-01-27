"""End-to-end test for derives_from population in search results."""

import tempfile
from collections.abc import Callable
from datetime import UTC, datetime
from pathlib import Path
from unittest.mock import MagicMock

import git
import pytest
from pydantic import AnyUrl
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.application.handlers.commit.extract_snippets import ExtractSnippetsHandler
from kodit.application.services.enrichment_query_service import EnrichmentQueryService
from kodit.application.services.reporting import ProgressTracker
from kodit.domain.entities.git import GitCommit
from kodit.domain.factories.git_repo_factory import GitRepoFactory
from kodit.domain.services.git_repository_service import GitRepositoryScanner
from kodit.domain.tracking.resolution_service import TrackableResolutionService
from kodit.infrastructure.cloning.git.git_python_adaptor import GitPythonAdapter
from kodit.infrastructure.sqlalchemy import entities as db_entities
from kodit.infrastructure.sqlalchemy.enrichment_association_repository import (
    create_enrichment_association_repository,
)
from kodit.infrastructure.sqlalchemy.enrichment_v2_repository import (
    create_enrichment_v2_repository,
)
from kodit.infrastructure.sqlalchemy.git_branch_repository import (
    create_git_branch_repository,
)
from kodit.infrastructure.sqlalchemy.git_commit_repository import (
    create_git_commit_repository,
)
from kodit.infrastructure.sqlalchemy.git_file_repository import (
    create_git_file_repository,
)
from kodit.infrastructure.sqlalchemy.git_repository import create_git_repo_repository
from kodit.infrastructure.sqlalchemy.git_tag_repository import (
    create_git_tag_repository,
)
from kodit.infrastructure.sqlalchemy.query import QueryBuilder


@pytest.fixture
def mock_progress() -> MagicMock:
    """Create a mock progress tracker."""
    from unittest.mock import AsyncMock

    tracker = MagicMock(spec=ProgressTracker)
    context_manager = AsyncMock()
    context_manager.__aenter__ = AsyncMock(return_value=context_manager)
    context_manager.__aexit__ = AsyncMock(return_value=None)
    context_manager.skip = AsyncMock()
    context_manager.set_total = AsyncMock()
    context_manager.set_current = AsyncMock()
    tracker.create_child = MagicMock(return_value=context_manager)
    return tracker


@pytest.mark.asyncio
async def test_extract_snippets_creates_file_associations(
    session_factory: Callable[[], AsyncSession],
    mock_progress: MagicMock,
) -> None:
    """Test that extracting snippets creates associations to source files."""
    with tempfile.TemporaryDirectory() as tmpdir:
        repo_path = Path(tmpdir) / "test-repo"
        repo_path.mkdir()

        # Create git repo with Python code
        git_repo = git.Repo.init(repo_path)
        with git_repo.config_writer() as cw:
            cw.set_value("user", "name", "Test User")
            cw.set_value("user", "email", "test@example.com")

        # Create Python file with a function
        python_file = repo_path / "calculator.py"
        python_file.write_text("""
def add(a: int, b: int) -> int:
    \"\"\"Add two numbers together.\"\"\"
    return a + b
""")
        git_repo.index.add(["calculator.py"])
        commit = git_repo.index.commit("Add calculator module")

        # Setup database repositories
        repo_repository = create_git_repo_repository(session_factory)
        commit_repository = create_git_commit_repository(session_factory)
        git_file_repository = create_git_file_repository(session_factory)
        enrichment_v2_repo = create_enrichment_v2_repository(session_factory)
        enrichment_assoc_repo = create_enrichment_association_repository(session_factory)

        # Create and save the repository
        repo = GitRepoFactory.create_from_remote_uri(
            AnyUrl("https://github.com/test/derives-from-test.git")
        )
        repo.cloned_path = repo_path
        repo = await repo_repository.save(repo)
        assert repo.id is not None

        # Save commit
        db_commit = GitCommit(
            commit_sha=commit.hexsha,
            repo_id=repo.id,
            message=str(commit.message),
            author=str(commit.author),
            date=datetime.fromtimestamp(commit.committed_date, UTC),
        )
        await commit_repository.save(db_commit)

        # Save the git file to the database
        git_adapter = GitPythonAdapter()
        files_data = await git_adapter.get_commit_files(repo_path, commit.hexsha)
        for file_data in files_data:
            db_file = db_entities.GitCommitFile(
                commit_sha=commit.hexsha,
                path=file_data["path"],
                blob_sha=file_data["blob_sha"],
                mime_type=file_data.get("mime_type", "application/octet-stream"),
                extension=Path(file_data["path"]).suffix.lstrip("."),
                size=file_data.get("size", 0),
                created_at=datetime.now(UTC),
            )
            await git_file_repository.save(db_file)

        # Create services
        git_scanner = GitRepositoryScanner(git_adapter)
        trackable_resolution = TrackableResolutionService(
            commit_repo=commit_repository,
            branch_repo=create_git_branch_repository(session_factory),
            tag_repo=create_git_tag_repository(session_factory),
        )
        enrichment_query_service = EnrichmentQueryService(
            trackable_resolution=trackable_resolution,
            enrichment_repo=enrichment_v2_repo,
            enrichment_association_repository=enrichment_assoc_repo,
        )

        # Create handler and extract snippets
        handler = ExtractSnippetsHandler(
            repo_repository=repo_repository,
            git_commit_repository=commit_repository,
            scanner=git_scanner,
            enrichment_v2_repository=enrichment_v2_repo,
            enrichment_association_repository=enrichment_assoc_repo,
            enrichment_query_service=enrichment_query_service,
            operation=mock_progress,
        )

        await handler.execute({
            "repository_id": repo.id,
            "commit_sha": commit.hexsha,
        })

        # Get all enrichment associations
        all_associations = await enrichment_assoc_repo.find(QueryBuilder())

        # Should have commit associations
        commit_associations = [
            a for a in all_associations
            if a.entity_type == db_entities.GitCommit.__tablename__
        ]
        assert len(commit_associations) > 0, "Should have commit associations"

        # Should also have file associations (derives_from)
        file_associations = [
            a for a in all_associations
            if a.entity_type == db_entities.GitCommitFile.__tablename__
        ]
        assert len(file_associations) > 0, (
            "Should have file associations for derives_from. "
            f"Found {len(all_associations)} associations total: "
            f"{[(a.entity_type, a.entity_id) for a in all_associations]}"
        )
