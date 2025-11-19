"""Unit tests for example extraction logic."""

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
async def test_examples_directory_one_file_per_example(
    extract_handler: ExtractExamplesHandler,
    session_factory: Callable[[], AsyncSession],
) -> None:
    """Test that each file in examples directory becomes a single example.

    Requirements:
    - One Python file directly under examples/ -> one example
    - One Python file in examples/subfolder/ -> one example
    - Total: 2 examples, each containing the full file content
    """
    with tempfile.TemporaryDirectory() as tmpdir:
        repo_path = Path(tmpdir) / "test-repo"
        repo_path.mkdir()

        repo = git.Repo.init(repo_path)
        repo.config_writer().set_value("user", "name", "Test User").release()
        repo.config_writer().set_value("user", "email", "test@example.com").release()

        # Create examples directory with one file
        examples_dir = repo_path / "examples"
        examples_dir.mkdir()

        example1 = examples_dir / "hello.py"
        example1.write_text(
            'def hello():\n    """Say hello."""\n    print("Hello, World!")\n'
        )

        # Create subdirectory with another example
        subdir = examples_dir / "advanced"
        subdir.mkdir()
        example2 = subdir / "greet.py"
        example2.write_text(
            'def greet(name: str):\n    """Greet someone."""\n'
            '    print(f"Hello, {name}!")\n'
        )

        repo.index.add(
            [
                str(example1.relative_to(repo_path)),
                str(example2.relative_to(repo_path)),
            ]
        )
        commit = repo.index.commit("Add examples")

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

        # Should have exactly 2 examples (one per file)
        assert len(enrichments) == 2, f"Expected 2 examples, got {len(enrichments)}"

        # Check that each example contains the full file content
        contents = [e.content for e in enrichments]

        # One should contain the hello.py content
        hello_example = [c for c in contents if 'print("Hello, World!")' in c]
        assert len(hello_example) == 1, "Should have one example with hello.py content"
        assert "def hello():" in hello_example[0]
        assert '"""Say hello."""' in hello_example[0]

        # One should contain the greet.py content
        greet_example = [c for c in contents if "def greet(name: str):" in c]
        assert len(greet_example) == 1, "Should have one example with greet.py content"
        assert '"""Greet someone."""' in greet_example[0]
        assert 'print(f"Hello, {name}!")' in greet_example[0]


@pytest.mark.asyncio
async def test_markdown_multiple_code_blocks_become_one_example(
    extract_handler: ExtractExamplesHandler,
    session_factory: Callable[[], AsyncSession],
) -> None:
    """Test that all code blocks in a markdown file are concatenated into one example.

    Requirements:
    - One markdown file with multiple code blocks
    - Should produce one example containing all code blocks concatenated
    - Should NOT include the markdown text, only the code
    """
    with tempfile.TemporaryDirectory() as tmpdir:
        repo_path = Path(tmpdir) / "test-repo"
        repo_path.mkdir()

        repo = git.Repo.init(repo_path)
        repo.config_writer().set_value("user", "name", "Test User").release()
        repo.config_writer().set_value("user", "email", "test@example.com").release()

        readme = repo_path / "README.md"
        readme.write_text(
            """# Example Usage

This library provides greeting functions.

## Basic Hello

```python
def hello():
    print("Hello, World!")
```

## Personalized Greeting

```python
def greet(name: str):
    print(f"Hello, {name}!")
```

## Usage

```python
hello()
greet("Alice")
```

That's all folks!
"""
        )

        repo.index.add([str(readme.relative_to(repo_path))])
        commit = repo.index.commit("Add README with examples")

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

        # Should have exactly 1 example (all code blocks concatenated)
        assert len(enrichments) == 1, f"Expected 1 example, got {len(enrichments)}"

        content = enrichments[0].content

        # Should contain all three code blocks
        assert 'print("Hello, World!")' in content
        assert 'print(f"Hello, {name}!")' in content
        assert 'greet("Alice")' in content

        # Should NOT contain the markdown text
        assert "This library provides" not in content
        assert "## Basic Hello" not in content
        assert "That's all folks!" not in content

        # All code should be present
        assert "def hello():" in content
        assert "def greet(name: str):" in content
        assert "hello()" in content
