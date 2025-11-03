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
from kodit.domain.enrichments.architecture.physical.physical import (
    PhysicalArchitectureEnrichment,
)
from kodit.domain.enrichments.enrichment import EnrichmentAssociation
from kodit.domain.entities.git import (
    GitCommit,
    GitFile,
    GitRepo,
    SnippetV2,
)
from kodit.domain.factories.git_repo_factory import GitRepoFactory
from kodit.domain.services.bm25_service import BM25DomainService
from kodit.domain.services.embedding_service import EmbeddingDomainService
from kodit.domain.services.git_repository_service import (
    GitRepositoryScanner,
    RepositoryCloner,
)
from kodit.domain.services.physical_architecture_service import (
    PhysicalArchitectureService,
)
from kodit.domain.value_objects import Enrichment
from kodit.infrastructure.slicing.slicer import Slicer
from kodit.infrastructure.sqlalchemy import entities as db_entities
from kodit.infrastructure.sqlalchemy.embedding_repository import (
    create_embedding_repository,
)
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
async def commit_indexing_service(
    session_factory: Callable[[], AsyncSession],
    mock_progress_tracker: MagicMock,
) -> CommitIndexingApplicationService:
    """Create a CommitIndexingApplicationService instance for testing."""
    queue_service = QueueService(session_factory=session_factory)
    repo_repository = create_git_repo_repository(session_factory=session_factory)
    git_commit_repository = create_git_commit_repository(
        session_factory=session_factory
    )
    git_branch_repository = create_git_branch_repository(
        session_factory=session_factory
    )
    git_file_repository = create_git_file_repository(session_factory=session_factory)
    git_tag_repository = create_git_tag_repository(session_factory=session_factory)
    embedding_repository = create_embedding_repository(session_factory=session_factory)
    enrichment_v2_repository = create_enrichment_v2_repository(
        session_factory=session_factory
    )
    enrichment_association_repository = create_enrichment_association_repository(
        session_factory=session_factory
    )

    return CommitIndexingApplicationService(
        repo_repository=repo_repository,
        git_commit_repository=git_commit_repository,
        git_branch_repository=git_branch_repository,
        git_tag_repository=git_tag_repository,
        git_file_repository=git_file_repository,
        operation=mock_progress_tracker,
        scanner=AsyncMock(spec=GitRepositoryScanner),
        cloner=MagicMock(spec=RepositoryCloner),
        slicer=MagicMock(spec=Slicer),
        queue=queue_service,
        bm25_service=AsyncMock(spec=BM25DomainService),
        code_search_service=AsyncMock(spec=EmbeddingDomainService),
        text_search_service=AsyncMock(spec=EmbeddingDomainService),
        embedding_repository=embedding_repository,
        architecture_service=AsyncMock(spec=PhysicalArchitectureService),
        cookbook_context_service=MagicMock(),
        database_schema_detector=MagicMock(),
        enrichment_v2_repository=enrichment_v2_repository,
        enrichment_association_repository=enrichment_association_repository,
        enricher_service=AsyncMock(),
        enrichment_query_service=AsyncMock(),
    )


async def create_test_repository_with_data(
    service: CommitIndexingApplicationService,
) -> tuple[GitRepo, GitCommit, list[SnippetV2]]:
    """Create a test repository with commits and snippets."""
    # Create and save a repository
    repo = GitRepoFactory.create_from_remote_uri(AnyUrl("https://github.com/test/repo"))
    repo = await service.repo_repository.save(repo)

    if repo.id is None:
        msg = "Repository ID cannot be None"
        raise ValueError(msg)

    # Create and save a commit
    commit = GitCommit(
        commit_sha="abc123def456",
        repo_id=repo.id,
        date=datetime.now(UTC),
        message="Test commit",
        parent_commit_sha=None,
        author="test@example.com",
    )
    await service.git_commit_repository.save_bulk([commit])

    # Create test file for snippets
    test_file = GitFile(
        created_at=datetime.now(UTC),
        blob_sha="file1sha",
        commit_sha="abc123def456",
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
                Enrichment(type="summarization", content="A simple hello function")
            ],
        ),
    ]

    # Note: Snippets are now stored as enrichments, not directly saved
    # This test helper creates the structure but enrichments would be created separately
    return repo, commit, snippets


@pytest.mark.asyncio
async def test_delete_repository_with_data_succeeds(
    commit_indexing_service: CommitIndexingApplicationService,
) -> None:
    """Test that deleting a repository with associated data works correctly."""
    # Create a repository with data
    repo, commit, snippets = await create_test_repository_with_data(
        commit_indexing_service
    )

    # Verify the data was created successfully
    assert repo.id is not None
    repo_exists = await commit_indexing_service.repo_repository.get(repo.id)
    assert repo_exists is not None

    saved_commit = await commit_indexing_service.git_commit_repository.get(
        commit.commit_sha
    )
    assert saved_commit is not None

    # Note: Snippet verification would now be done through enrichment_v2_repository
    # For now, we just verify commit was saved

    test_enrichment = PhysicalArchitectureEnrichment(
        content="test content",
    )
    # Save enrichment first
    saved_enrichment = await commit_indexing_service.enrichment_v2_repository.save(
        test_enrichment
    )
    # Then create association
    await commit_indexing_service.enrichment_association_repository.save(
        EnrichmentAssociation(
            enrichment_id=saved_enrichment.id,  # type: ignore[arg-type]
            entity_type=db_entities.GitCommit.__tablename__,
            entity_id=commit.commit_sha,
        )
    )

    # Verify enrichment association was created
    from kodit.infrastructure.sqlalchemy.query import FilterOperator, QueryBuilder

    associations = await commit_indexing_service.enrichment_association_repository.find(
        QueryBuilder()
        .filter("entity_type", FilterOperator.EQ, db_entities.GitCommit.__tablename__)
        .filter("entity_id", FilterOperator.EQ, commit.commit_sha)
    )
    assert len(associations) == 1

    # Delete the repository
    success = await commit_indexing_service.delete_git_repository(repo.id)
    assert success is True

    # Verify the repository was actually deleted
    with pytest.raises(ValueError, match="not found"):
        await commit_indexing_service.repo_repository.get(repo.id)

    # Verify enrichment associations were deleted
    assoc_repo = commit_indexing_service.enrichment_association_repository
    associations_after = await assoc_repo.find(
        QueryBuilder()
        .filter("entity_type", FilterOperator.EQ, db_entities.GitCommit.__tablename__)
        .filter("entity_id", FilterOperator.EQ, commit.commit_sha)
    )
    assert len(associations_after) == 0


@pytest.mark.asyncio
async def test_delete_nonexistent_repository_raises_error(
    commit_indexing_service: CommitIndexingApplicationService,
) -> None:
    """Test that deleting a non-existent repository raises ValueError."""
    # Try to delete a repository that doesn't exist - should raise ValueError
    with pytest.raises(ValueError, match="not found"):
        await commit_indexing_service.delete_git_repository(99999)
