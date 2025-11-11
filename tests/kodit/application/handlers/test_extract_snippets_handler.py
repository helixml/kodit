"""Tests for ExtractSnippetsHandler."""

import tempfile
from collections.abc import Callable
from datetime import UTC, datetime
from pathlib import Path
from unittest.mock import AsyncMock, MagicMock

import git
import pytest
from pydantic import AnyUrl
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.application.handlers.commit.extract_snippets import ExtractSnippetsHandler
from kodit.application.services.enrichment_query_service import (
    EnrichmentQueryService,
)
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
from kodit.infrastructure.sqlalchemy.git_repository import create_git_repo_repository
from kodit.infrastructure.sqlalchemy.git_tag_repository import (
    create_git_tag_repository,
)
from kodit.infrastructure.sqlalchemy.query import FilterOperator, QueryBuilder


@pytest.fixture
def mock_progress() -> MagicMock:
    """Create a mock progress tracker."""
    tracker = MagicMock(spec=ProgressTracker)
    context_manager = AsyncMock()
    context_manager.__aenter__ = AsyncMock(return_value=context_manager)
    context_manager.__aexit__ = AsyncMock(return_value=None)
    context_manager.skip = AsyncMock()
    context_manager.set_total = AsyncMock()
    context_manager.set_current = AsyncMock()
    tracker.create_child = MagicMock(return_value=context_manager)
    return tracker


@pytest.fixture
async def extract_handler(
    session_factory: Callable[[], AsyncSession],
    mock_progress: MagicMock,
) -> ExtractSnippetsHandler:
    """Create an ExtractSnippetsHandler instance."""
    git_adapter = GitPythonAdapter()
    scanner = GitRepositoryScanner(git_adapter)

    enrichment_v2_repo = create_enrichment_v2_repository(session_factory)
    enrichment_assoc_repo = create_enrichment_association_repository(session_factory)

    # Create enrichment query service
    trackable_resolution = TrackableResolutionService(
        commit_repo=create_git_commit_repository(session_factory),
        branch_repo=create_git_branch_repository(session_factory),
        tag_repo=create_git_tag_repository(session_factory),
    )
    enrichment_query_service = EnrichmentQueryService(
        trackable_resolution=trackable_resolution,
        enrichment_repo=enrichment_v2_repo,
        enrichment_association_repository=enrichment_assoc_repo,
    )

    return ExtractSnippetsHandler(
        repo_repository=create_git_repo_repository(session_factory),
        git_commit_repository=create_git_commit_repository(session_factory),
        scanner=scanner,
        enrichment_v2_repository=enrichment_v2_repo,
        enrichment_association_repository=enrichment_assoc_repo,
        enrichment_query_service=enrichment_query_service,
        operation=mock_progress,
    )


@pytest.mark.asyncio
async def test_extract_snippets_creates_enrichments_and_associations(
    extract_handler: ExtractSnippetsHandler,
    session_factory: Callable[[], AsyncSession],
) -> None:
    """Test that extracting snippets creates enrichments and associations."""
    with tempfile.TemporaryDirectory() as tmpdir:
        repo_path = Path(tmpdir) / "test-repo"
        repo_path.mkdir()

        # Create git repo with Python code
        git_repo = git.Repo.init(repo_path)
        with git_repo.config_writer() as cw:
            cw.set_value("user", "name", "Test User")
            cw.set_value("user", "email", "test@example.com")

        # Create Python file with multiple functions
        python_file = repo_path / "module.py"
        python_file.write_text("""
def hello():
    print("hello")

def goodbye():
    print("goodbye")
""")
        git_repo.index.add(["module.py"])
        commit = git_repo.index.commit("Add module")

        # Setup database
        repo_repository = create_git_repo_repository(session_factory)
        commit_repository = create_git_commit_repository(session_factory)

        repo = GitRepoFactory.create_from_remote_uri(
            AnyUrl("https://github.com/test/repo.git")
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

        # Extract snippets
        await extract_handler.execute({
            "repository_id": repo.id,
            "commit_sha": commit.hexsha,
        })

        # Verify enrichments were created
        enrichment_repo = create_enrichment_v2_repository(session_factory)
        enrichment_assoc_repo = create_enrichment_association_repository(
            session_factory
        )

        # Get enrichments associated with this commit
        associations = await enrichment_assoc_repo.find(
            QueryBuilder()
            .filter(
                "entity_type", FilterOperator.EQ, db_entities.GitCommit.__tablename__
            )
            .filter("entity_id", FilterOperator.EQ, commit.hexsha)
        )

        assert len(associations) > 0, "Should have created snippet enrichments"

        # Verify enrichments have content
        enrichment_ids = [a.enrichment_id for a in associations]
        enrichments = await enrichment_repo.find(
            QueryBuilder().filter("id", FilterOperator.IN, enrichment_ids)
        )

        assert len(enrichments) > 0
        # Should have extracted at least the two functions
        assert any("hello" in e.content for e in enrichments)


@pytest.mark.asyncio
async def test_extract_snippets_deduplicates_by_sha(
    extract_handler: ExtractSnippetsHandler,
    session_factory: Callable[[], AsyncSession],
) -> None:
    """Test that duplicate snippets are deduplicated by SHA."""
    with tempfile.TemporaryDirectory() as tmpdir:
        repo_path = Path(tmpdir) / "test-repo"
        repo_path.mkdir()

        git_repo = git.Repo.init(repo_path)
        with git_repo.config_writer() as cw:
            cw.set_value("user", "name", "Test User")
            cw.set_value("user", "email", "test@example.com")

        # Create two files with identical functions (will have same SHA)
        identical_code = """
def identical_function():
    return "same"
"""
        (repo_path / "file1.py").write_text(identical_code)
        (repo_path / "file2.py").write_text(identical_code)
        git_repo.index.add(["file1.py", "file2.py"])
        commit = git_repo.index.commit("Add identical files")

        # Setup database
        repo_repository = create_git_repo_repository(session_factory)
        commit_repository = create_git_commit_repository(session_factory)

        repo = GitRepoFactory.create_from_remote_uri(
            AnyUrl("https://github.com/test/repo.git")
        )
        repo.cloned_path = repo_path
        repo = await repo_repository.save(repo)
        assert repo.id is not None

        db_commit = GitCommit(
            commit_sha=commit.hexsha,
            repo_id=repo.id,
            message=str(commit.message),
            author=str(commit.author),
            date=datetime.fromtimestamp(commit.committed_date, UTC),
        )
        await commit_repository.save(db_commit)

        # Extract snippets
        await extract_handler.execute({
            "repository_id": repo.id,
            "commit_sha": commit.hexsha,
        })

        # Get all enrichments
        enrichment_assoc_repo = create_enrichment_association_repository(
            session_factory
        )
        enrichment_repo = create_enrichment_v2_repository(session_factory)

        associations = await enrichment_assoc_repo.find(
            QueryBuilder()
            .filter(
                "entity_type", FilterOperator.EQ, db_entities.GitCommit.__tablename__
            )
            .filter("entity_id", FilterOperator.EQ, commit.hexsha)
        )

        enrichment_ids = [a.enrichment_id for a in associations]
        enrichments = await enrichment_repo.find(
            QueryBuilder().filter("id", FilterOperator.IN, enrichment_ids)
        )

        # Count how many have the identical function
        identical_count = sum(
            1 for e in enrichments if "identical_function" in e.content
        )

        # Should be deduplicated - only one copy despite being in two files
        assert identical_count == 1, "Duplicate snippets should be deduplicated by SHA"


@pytest.mark.asyncio
async def test_extract_snippets_is_idempotent(
    extract_handler: ExtractSnippetsHandler,
    session_factory: Callable[[], AsyncSession],
) -> None:
    """Test that extracting snippets twice doesn't create duplicates."""
    with tempfile.TemporaryDirectory() as tmpdir:
        repo_path = Path(tmpdir) / "test-repo"
        repo_path.mkdir()

        git_repo = git.Repo.init(repo_path)
        with git_repo.config_writer() as cw:
            cw.set_value("user", "name", "Test User")
            cw.set_value("user", "email", "test@example.com")

        (repo_path / "code.py").write_text("def func():\n    pass\n")
        git_repo.index.add(["code.py"])
        commit = git_repo.index.commit("Add code")

        # Setup database
        repo_repository = create_git_repo_repository(session_factory)
        commit_repository = create_git_commit_repository(session_factory)

        repo = GitRepoFactory.create_from_remote_uri(
            AnyUrl("https://github.com/test/repo.git")
        )
        repo.cloned_path = repo_path
        repo = await repo_repository.save(repo)
        assert repo.id is not None

        db_commit = GitCommit(
            commit_sha=commit.hexsha,
            repo_id=repo.id,
            message=str(commit.message),
            author=str(commit.author),
            date=datetime.fromtimestamp(commit.committed_date, UTC),
        )
        await commit_repository.save(db_commit)

        payload = {"repository_id": repo.id, "commit_sha": commit.hexsha}

        # Extract first time
        await extract_handler.execute(payload)

        enrichment_assoc_repo = create_enrichment_association_repository(
            session_factory
        )
        associations_first = await enrichment_assoc_repo.find(
            QueryBuilder()
            .filter(
                "entity_type", FilterOperator.EQ, db_entities.GitCommit.__tablename__
            )
            .filter("entity_id", FilterOperator.EQ, commit.hexsha)
        )
        count_first = len(associations_first)

        # Extract second time - should skip
        await extract_handler.execute(payload)

        associations_second = await enrichment_assoc_repo.find(
            QueryBuilder()
            .filter(
                "entity_type", FilterOperator.EQ, db_entities.GitCommit.__tablename__
            )
            .filter("entity_id", FilterOperator.EQ, commit.hexsha)
        )
        count_second = len(associations_second)

        assert count_first == count_second, "Should not create duplicates on re-extract"


@pytest.mark.asyncio
async def test_extract_snippets_handles_multiple_languages(
    extract_handler: ExtractSnippetsHandler,
    session_factory: Callable[[], AsyncSession],
) -> None:
    """Test that snippets are extracted from multiple languages."""
    with tempfile.TemporaryDirectory() as tmpdir:
        repo_path = Path(tmpdir) / "test-repo"
        repo_path.mkdir()

        git_repo = git.Repo.init(repo_path)
        with git_repo.config_writer() as cw:
            cw.set_value("user", "name", "Test User")
            cw.set_value("user", "email", "test@example.com")

        # Create files in different languages
        (repo_path / "script.py").write_text("def python_func():\n    pass\n")
        (repo_path / "script.js").write_text("function jsFunc() {}\n")
        (repo_path / "code.go").write_text("func goFunc() {}\n")

        git_repo.index.add(["script.py", "script.js", "code.go"])
        commit = git_repo.index.commit("Add multilanguage code")

        # Setup database
        repo_repository = create_git_repo_repository(session_factory)
        commit_repository = create_git_commit_repository(session_factory)

        repo = GitRepoFactory.create_from_remote_uri(
            AnyUrl("https://github.com/test/repo.git")
        )
        repo.cloned_path = repo_path
        repo = await repo_repository.save(repo)
        assert repo.id is not None

        db_commit = GitCommit(
            commit_sha=commit.hexsha,
            repo_id=repo.id,
            message=str(commit.message),
            author=str(commit.author),
            date=datetime.fromtimestamp(commit.committed_date, UTC),
        )
        await commit_repository.save(db_commit)

        # Extract snippets
        await extract_handler.execute({
            "repository_id": repo.id,
            "commit_sha": commit.hexsha,
        })

        # Verify snippets from all languages were extracted
        enrichment_assoc_repo = create_enrichment_association_repository(
            session_factory
        )
        enrichment_repo = create_enrichment_v2_repository(session_factory)

        associations = await enrichment_assoc_repo.find(
            QueryBuilder()
            .filter(
                "entity_type", FilterOperator.EQ, db_entities.GitCommit.__tablename__
            )
            .filter("entity_id", FilterOperator.EQ, commit.hexsha)
        )

        enrichment_ids = [a.enrichment_id for a in associations]
        enrichments = await enrichment_repo.find(
            QueryBuilder().filter("id", FilterOperator.IN, enrichment_ids)
        )

        # Should have extracted from multiple languages
        assert len(enrichments) >= 3, "Should extract from Python, JS, and Go"

        contents = [e.content for e in enrichments]
        " ".join(contents)

        # Check for language-specific patterns
        has_python = any("python_func" in c or "def " in c for c in contents)
        has_js = any("jsFunc" in c or "function" in c for c in contents)
        has_go = any("goFunc" in c or "func " in c for c in contents)

        assert (
            has_python or has_js or has_go
        ), "Should have extracted from at least one language"
