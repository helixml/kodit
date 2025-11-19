"""Tests for ExtractExamplesHandler."""

import tempfile
from collections.abc import Callable
from datetime import UTC, datetime
from pathlib import Path
from unittest.mock import AsyncMock, MagicMock

import git
import pytest
from pydantic import AnyUrl
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.application.handlers.commit.extract_examples import ExtractExamplesHandler
from kodit.application.services.enrichment_query_service import (
    EnrichmentQueryService,
)
from kodit.application.services.reporting import ProgressTracker
from kodit.domain.enrichments.development.example.example import (
    ENRICHMENT_SUBTYPE_EXAMPLE,
)
from kodit.domain.entities.git import GitCommit
from kodit.domain.factories.git_repo_factory import GitRepoFactory
from kodit.domain.services.git_repository_service import GitRepositoryScanner
from kodit.domain.tracking.resolution_service import TrackableResolutionService
from kodit.infrastructure.cloning.git.git_python_adaptor import GitPythonAdapter
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
from kodit.infrastructure.sqlalchemy.git_tag_repository import create_git_tag_repository
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
) -> ExtractExamplesHandler:
    """Create an ExtractExamplesHandler instance."""
    git_adapter = GitPythonAdapter()
    scanner = GitRepositoryScanner(git_adapter)

    enrichment_v2_repo = create_enrichment_v2_repository(session_factory)
    enrichment_assoc_repo = create_enrichment_association_repository(session_factory)

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

    return ExtractExamplesHandler(
        repo_repository=create_git_repo_repository(session_factory),
        git_commit_repository=create_git_commit_repository(session_factory),
        scanner=scanner,
        enrichment_v2_repository=enrichment_v2_repo,
        enrichment_association_repository=enrichment_assoc_repo,
        enrichment_query_service=enrichment_query_service,
        operation=mock_progress,
    )


@pytest.mark.asyncio
async def test_extract_examples_from_directory(
    extract_handler: ExtractExamplesHandler,
    session_factory: Callable[[], AsyncSession],
) -> None:
    """Test extracting examples from example directory."""
    with tempfile.TemporaryDirectory() as tmpdir:
        repo_path = Path(tmpdir) / "test-repo"
        repo_path.mkdir()

        repo = git.Repo.init(repo_path)
        repo.config_writer().set_value("user", "name", "Test User").release()
        repo.config_writer().set_value("user", "email", "test@example.com").release()

        examples_dir = repo_path / "examples"
        examples_dir.mkdir()

        example_file = examples_dir / "hello.py"
        example_file.write_text("def hello():\n    print('Hello, World!')\n")

        repo.index.add([str(example_file.relative_to(repo_path))])
        commit = repo.index.commit("Add example")

        repo_repository = create_git_repo_repository(session_factory)
        commit_repository = create_git_commit_repository(session_factory)

        db_repo = GitRepoFactory.create_from_remote_uri(
            AnyUrl("https://github.com/test/repo.git")
        )
        db_repo.cloned_path = repo_path
        db_repo = await repo_repository.save(db_repo)
        assert db_repo.id is not None

        db_commit = GitCommit(
            commit_sha=commit.hexsha,
            repo_id=db_repo.id,
            message=str(commit.message),
            author=str(commit.author),
            date=datetime.fromtimestamp(commit.committed_date, UTC),
        )
        await commit_repository.save(db_commit)

        await extract_handler.execute(
            {"repository_id": db_repo.id, "commit_sha": commit.hexsha}
        )

        enrichment_repo = create_enrichment_v2_repository(session_factory)
        enrichments = await enrichment_repo.find(
            QueryBuilder().filter(
                "subtype", FilterOperator.EQ, ENRICHMENT_SUBTYPE_EXAMPLE
            )
        )

        assert len(enrichments) == 1
        assert "Hello, World!" in enrichments[0].content


@pytest.mark.asyncio
async def test_extract_examples_from_markdown(
    extract_handler: ExtractExamplesHandler,
    session_factory: Callable[[], AsyncSession],
) -> None:
    """Test extracting code blocks from Markdown documentation."""
    with tempfile.TemporaryDirectory() as tmpdir:
        repo_path = Path(tmpdir) / "test-repo"
        repo_path.mkdir()

        repo = git.Repo.init(repo_path)
        repo.config_writer().set_value("user", "name", "Test User").release()
        repo.config_writer().set_value("user", "email", "test@example.com").release()

        readme_file = repo_path / "README.md"
        readme_file.write_text(
            """# Example Usage

Here's how to use this library:

```python
from mylib import greeter

greeter.hello()
```
"""
        )

        repo.index.add([str(readme_file.relative_to(repo_path))])
        commit = repo.index.commit("Add README with example")

        repo_repository = create_git_repo_repository(session_factory)
        commit_repository = create_git_commit_repository(session_factory)

        db_repo = GitRepoFactory.create_from_remote_uri(
            AnyUrl("https://github.com/test/repo.git")
        )
        db_repo.cloned_path = repo_path
        db_repo = await repo_repository.save(db_repo)
        assert db_repo.id is not None

        db_commit = GitCommit(
            commit_sha=commit.hexsha,
            repo_id=db_repo.id,
            message=str(commit.message),
            author=str(commit.author),
            date=datetime.fromtimestamp(commit.committed_date, UTC),
        )
        await commit_repository.save(db_commit)

        await extract_handler.execute(
            {"repository_id": db_repo.id, "commit_sha": commit.hexsha}
        )

        enrichment_repo = create_enrichment_v2_repository(session_factory)
        enrichments = await enrichment_repo.find(
            QueryBuilder().filter(
                "subtype", FilterOperator.EQ, ENRICHMENT_SUBTYPE_EXAMPLE
            )
        )

        assert len(enrichments) == 1
        assert "greeter.hello()" in enrichments[0].content
        assert "from mylib import greeter" in enrichments[0].content


@pytest.mark.asyncio
async def test_extract_examples_is_idempotent(
    extract_handler: ExtractExamplesHandler,
    session_factory: Callable[[], AsyncSession],
) -> None:
    """Test that extracting examples twice doesn't create duplicates."""
    with tempfile.TemporaryDirectory() as tmpdir:
        repo_path = Path(tmpdir) / "test-repo"
        repo_path.mkdir()

        repo = git.Repo.init(repo_path)
        repo.config_writer().set_value("user", "name", "Test User").release()
        repo.config_writer().set_value("user", "email", "test@example.com").release()

        examples_dir = repo_path / "examples"
        examples_dir.mkdir()
        (examples_dir / "test.py").write_text("print('test')")

        repo.index.add(["examples/test.py"])
        commit = repo.index.commit("Add example")

        repo_repository = create_git_repo_repository(session_factory)
        commit_repository = create_git_commit_repository(session_factory)

        db_repo = GitRepoFactory.create_from_remote_uri(
            AnyUrl("https://github.com/test/repo.git")
        )
        db_repo.cloned_path = repo_path
        db_repo = await repo_repository.save(db_repo)
        assert db_repo.id is not None

        db_commit = GitCommit(
            commit_sha=commit.hexsha,
            repo_id=db_repo.id,
            message=str(commit.message),
            author=str(commit.author),
            date=datetime.fromtimestamp(commit.committed_date, UTC),
        )
        await commit_repository.save(db_commit)

        await extract_handler.execute(
            {"repository_id": db_repo.id, "commit_sha": commit.hexsha}
        )
        await extract_handler.execute(
            {"repository_id": db_repo.id, "commit_sha": commit.hexsha}
        )

        enrichment_repo = create_enrichment_v2_repository(session_factory)
        enrichments = await enrichment_repo.find(
            QueryBuilder().filter(
                "subtype", FilterOperator.EQ, ENRICHMENT_SUBTYPE_EXAMPLE
            )
        )

        assert len(enrichments) == 1
