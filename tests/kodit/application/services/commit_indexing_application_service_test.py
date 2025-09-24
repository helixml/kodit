"""Tests for the CommitIndexingApplicationService."""

from collections.abc import Callable
from datetime import UTC, datetime
from unittest.mock import AsyncMock, MagicMock

import pytest
from pydantic import AnyUrl
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.application.services.commit_indexing_application_service import (
    CommitIndexingApplicationService,
)
from kodit.application.services.queue_service import QueueService
from kodit.application.services.reporting import ProgressTracker
from kodit.domain.entities.git import (
    GitBranch,
    GitCommit,
    GitFile,
    GitRepo,
    GitTag,
    SnippetV2,
)
from kodit.domain.factories.git_repo_factory import GitRepoFactory
from kodit.domain.services.bm25_service import BM25DomainService
from kodit.domain.services.embedding_service import EmbeddingDomainService
from kodit.domain.services.enrichment_service import EnrichmentDomainService
from kodit.domain.services.git_repository_service import (
    GitRepositoryScanner,
    RepositoryCloner,
)
from kodit.domain.value_objects import Enrichment, EnrichmentType
from kodit.infrastructure.slicing.slicer import Slicer
from kodit.infrastructure.sqlalchemy.embedding_repository import (
    create_embedding_repository,
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
from kodit.infrastructure.sqlalchemy.snippet_v2_repository import (
    create_snippet_v2_repository,
)


@pytest.fixture
def mock_progress_tracker() -> MagicMock:
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
def mock_scanner() -> MagicMock:
    """Create a mock git repository scanner."""
    return MagicMock(spec=GitRepositoryScanner)


@pytest.fixture
def mock_cloner() -> MagicMock:
    """Create a mock repository cloner."""
    return MagicMock(spec=RepositoryCloner)


@pytest.fixture
def mock_slicer() -> MagicMock:
    """Create a mock slicer."""
    return MagicMock(spec=Slicer)


@pytest.fixture
def mock_bm25_service() -> AsyncMock:
    """Create a mock BM25 service."""
    return AsyncMock(spec=BM25DomainService)


@pytest.fixture
def mock_embedding_service() -> AsyncMock:
    """Create a mock embedding service."""
    return AsyncMock(spec=EmbeddingDomainService)


@pytest.fixture
def mock_enrichment_service() -> AsyncMock:
    """Create a mock enrichment service."""
    return AsyncMock(spec=EnrichmentDomainService)


@pytest.fixture
async def commit_indexing_service(  # noqa: PLR0913
    session_factory: Callable[[], AsyncSession],
    mock_progress_tracker: MagicMock,
    mock_scanner: MagicMock,
    mock_cloner: MagicMock,
    mock_slicer: MagicMock,
    mock_bm25_service: AsyncMock,
    mock_embedding_service: AsyncMock,
    mock_enrichment_service: AsyncMock,
) -> CommitIndexingApplicationService:
    """Create a CommitIndexingApplicationService instance for testing."""
    queue_service = QueueService(session_factory=session_factory)
    snippet_v2_repository = create_snippet_v2_repository(
        session_factory=session_factory
    )
    repo_repository = create_git_repo_repository(session_factory=session_factory)
    git_commit_repository = create_git_commit_repository(
        session_factory=session_factory
    )
    git_branch_repository = create_git_branch_repository(
        session_factory=session_factory
    )
    git_tag_repository = create_git_tag_repository(session_factory=session_factory)
    embedding_repository = create_embedding_repository(session_factory=session_factory)

    return CommitIndexingApplicationService(
        snippet_v2_repository=snippet_v2_repository,
        repo_repository=repo_repository,
        git_commit_repository=git_commit_repository,
        git_branch_repository=git_branch_repository,
        git_tag_repository=git_tag_repository,
        operation=mock_progress_tracker,
        scanner=mock_scanner,
        cloner=mock_cloner,
        snippet_repository=snippet_v2_repository,
        slicer=mock_slicer,
        queue=queue_service,
        bm25_service=mock_bm25_service,
        code_search_service=mock_embedding_service,
        text_search_service=mock_embedding_service,
        enrichment_service=mock_enrichment_service,
        embedding_repository=embedding_repository,
    )


async def create_test_repository_with_full_data(
    service: CommitIndexingApplicationService,
) -> tuple[GitRepo, GitCommit, GitBranch, GitTag, list[SnippetV2]]:
    """Create a test repository with commits, branches, tags, and snippets."""
    # Create and save a repository
    repo = GitRepoFactory.create_from_remote_uri(AnyUrl("https://github.com/test/repo"))
    repo = await service.repo_repository.save(repo)

    # Ensure repo has an ID
    if repo.id is None:
        msg = "Repository ID cannot be None"
        raise ValueError(msg)

    # Create and save a commit
    commit = GitCommit(
        commit_sha="abc123def456",
        date=datetime.now(UTC),
        message="Test commit",
        parent_commit_sha=None,
        author="test@example.com",
        files=[],  # Empty for simplicity
    )
    await service.git_commit_repository.save_bulk([commit], repo.id)

    # Create and save a branch
    branch = GitBranch(
        repo_id=repo.id,
        name="main",
        head_commit=commit,
    )
    await service.git_branch_repository.save_bulk([branch], repo.id)

    # Create and save a tag
    tag = GitTag(
        created_at=datetime.now(UTC),
        repo_id=repo.id,
        name="v1.0.0",
        target_commit=commit,
    )
    await service.git_tag_repository.save_bulk([tag], repo.id)

    # Create test files for snippets
    test_file = GitFile(
        created_at=datetime.now(UTC),
        blob_sha="file1sha",
        path="test.py",
        mime_type="text/x-python",
        size=100,
        extension="py",
    )

    # Create and save snippets
    snippets = [
        SnippetV2(
            sha="snippet1sha",
            derives_from=[test_file],
            content="def hello():\n    print('Hello')",
            extension="py",
            enrichments=[
                Enrichment(
                    type=EnrichmentType.SUMMARIZATION, content="A simple hello function"
                )
            ],
        ),
        SnippetV2(
            sha="snippet2sha",
            derives_from=[test_file],
            content="class TestClass:\n    pass",
            extension="py",
            enrichments=[],
        ),
    ]

    # Save snippets and associate them with the commit
    await service.snippet_repository.save_snippets(commit.commit_sha, snippets)

    return repo, commit, branch, tag, snippets


@pytest.mark.asyncio
async def test_delete_repository_with_full_data_succeeds(
    commit_indexing_service: CommitIndexingApplicationService,
) -> None:
    """Test that deleting a repository with all associated data works correctly.

    This test demonstrates that the deletion logic properly handles all foreign key
    dependencies, including snippets and their associations.
    """
    # Create a repository with full data (commits, branches, tags, snippets)
    (
        repo,
        commit,
        _branch,
        _tag,
        _snippets,
    ) = await create_test_repository_with_full_data(commit_indexing_service)

    # Verify the data was created successfully
    assert repo.id is not None
    repo_exists = await commit_indexing_service.repo_repository.get_by_id(repo.id)
    assert repo_exists is not None
    saved_commit = await commit_indexing_service.git_commit_repository.get_by_sha(
        commit.commit_sha
    )
    assert saved_commit is not None
    saved_snippets = (
        await commit_indexing_service.snippet_repository.get_snippets_for_commit(
            commit.commit_sha
        )
    )
    assert len(saved_snippets) == 2

    # The deletion should succeed because proper FK handling is implemented
    success = await commit_indexing_service.delete_git_repository(repo.id)
    assert success is True

    # Verify the repository was actually deleted
    with pytest.raises(ValueError, match="not found"):
        await commit_indexing_service.repo_repository.get_by_id(repo.id)


@pytest.mark.asyncio
async def test_delete_repository_via_process_delete_repo_succeeds(
    commit_indexing_service: CommitIndexingApplicationService,
) -> None:
    """Test deletion using the process_delete_repo method.

    This test demonstrates that the process_delete_repo method properly handles
    deletion order and foreign key constraints.
    """
    # Create a repository with full data
    (
        repo,
        commit,
        _branch,
        _tag,
        _snippets,
    ) = await create_test_repository_with_full_data(commit_indexing_service)

    # Verify the data exists
    assert repo.id is not None
    saved_commit = await commit_indexing_service.git_commit_repository.get_by_sha(
        commit.commit_sha
    )
    assert saved_commit is not None
    saved_snippets = (
        await commit_indexing_service.snippet_repository.get_snippets_for_commit(
            commit.commit_sha
        )
    )
    assert len(saved_snippets) == 2

    # Delete using the application service's process_delete_repo method
    # This should succeed because proper FK handling is implemented
    await commit_indexing_service.process_delete_repo(repo.id)

    # Verify the repository was deleted successfully
    with pytest.raises(ValueError, match="not found"):
        await commit_indexing_service.repo_repository.get_by_id(repo.id)
