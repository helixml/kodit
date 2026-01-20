"""Tests for RescanCommitHandler."""

from collections.abc import Callable
from datetime import UTC, datetime
from unittest.mock import AsyncMock, MagicMock

import pytest
from pydantic import AnyUrl
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.application.handlers.commit.rescan_commit import RescanCommitHandler
from kodit.application.services.enrichment_query_service import EnrichmentQueryService
from kodit.application.services.queue_service import QueueService
from kodit.domain.enrichments.development.development import ENRICHMENT_TYPE_DEVELOPMENT
from kodit.domain.enrichments.development.snippet.snippet import (
    ENRICHMENT_SUBTYPE_SNIPPET,
)
from kodit.domain.entities.git import GitCommit, GitFile, GitRepo
from kodit.domain.services.bm25_service import BM25DomainService
from kodit.domain.tracking.resolution_service import TrackableResolutionService
from kodit.domain.value_objects import PrescribedOperations, QueuePriority
from kodit.infrastructure.sqlalchemy import entities as db_entities
from kodit.infrastructure.sqlalchemy.embedding_repository import (
    SqlAlchemyEmbeddingRepository,
)
from kodit.infrastructure.sqlalchemy.enrichment_association_repository import (
    SQLAlchemyEnrichmentAssociationRepository,
)
from kodit.infrastructure.sqlalchemy.enrichment_v2_repository import (
    SQLAlchemyEnrichmentV2Repository,
)
from kodit.infrastructure.sqlalchemy.git_commit_repository import (
    create_git_commit_repository,
)
from kodit.infrastructure.sqlalchemy.git_file_repository import (
    create_git_file_repository,
)
from kodit.infrastructure.sqlalchemy.git_repository import create_git_repo_repository
from kodit.infrastructure.sqlalchemy.query import FilterOperator, QueryBuilder


@pytest.fixture
def mock_progress() -> MagicMock:
    """Create a mock progress tracker."""
    tracker = MagicMock()
    context = AsyncMock()
    context.__aenter__ = AsyncMock(return_value=context)
    context.__aexit__ = AsyncMock(return_value=None)
    tracker.create_child = MagicMock(return_value=context)
    return tracker


@pytest.fixture
def mock_bm25_service() -> MagicMock:
    """Create a mock BM25 service."""
    service = MagicMock(spec=BM25DomainService)
    service.delete_documents = AsyncMock()
    return service


@pytest.fixture
def mock_queue() -> MagicMock:
    """Create a mock queue service."""
    queue = MagicMock(spec=QueueService)
    queue.enqueue_tasks = AsyncMock()
    return queue


@pytest.mark.asyncio
async def test_rescan_commit_deletes_artifacts_and_queues_tasks(
    session_factory: Callable[[], AsyncSession],
    mock_progress: MagicMock,
    mock_bm25_service: MagicMock,
    mock_queue: MagicMock,
) -> None:
    """Test that rescan deletes all artifacts and queues reindexing."""
    # Setup repositories
    repo_repo = create_git_repo_repository(session_factory)
    commit_repo = create_git_commit_repository(session_factory)
    file_repo = create_git_file_repository(session_factory)
    enrichment_repo = SQLAlchemyEnrichmentV2Repository(session_factory)
    assoc_repo = SQLAlchemyEnrichmentAssociationRepository(session_factory)
    embedding_repo = SqlAlchemyEmbeddingRepository(session_factory)

    # Create test data
    repo = await repo_repo.save(GitRepo(
        sanitized_remote_uri=AnyUrl("https://github.com/test/repo"),
        remote_uri=AnyUrl("https://github.com/test/repo.git"),
    ))
    assert repo.id is not None
    commit = await commit_repo.save(GitCommit(
        commit_sha="abc123",
        repo_id=repo.id,
        date=datetime.now(UTC),
        message="Test",
        author="test@example.com",
    ))
    await file_repo.save(GitFile(
        created_at=datetime.now(UTC),
        blob_sha="blob123",
        commit_sha=commit.commit_sha,
        path="test.py",
        mime_type="text/x-python",
        size=100,
        extension="py",
    ))

    # Create enrichment linked to commit
    snippet = await enrichment_repo.save(db_entities.EnrichmentV2(  # type: ignore[arg-type]
        type=ENRICHMENT_TYPE_DEVELOPMENT,
        subtype=ENRICHMENT_SUBTYPE_SNIPPET,
        content="def foo(): pass",
    ))
    await assoc_repo.save(db_entities.EnrichmentAssociation(  # type: ignore[arg-type]
        enrichment_id=snippet.id,
        entity_type=db_entities.GitCommit.__tablename__,
        entity_id=commit.commit_sha,
    ))

    # Create handler
    handler = RescanCommitHandler(
        git_file_repository=file_repo,
        bm25_service=mock_bm25_service,
        embedding_repository=embedding_repo,
        enrichment_v2_repository=enrichment_repo,
        enrichment_association_repository=assoc_repo,
        enrichment_query_service=EnrichmentQueryService(
            trackable_resolution=MagicMock(spec=TrackableResolutionService),
            enrichment_repo=enrichment_repo,
            enrichment_association_repository=assoc_repo,
        ),
        queue=mock_queue,
        operation=mock_progress,
    )

    # Execute rescan
    await handler.execute({"repository_id": repo.id, "commit_sha": commit.commit_sha})

    # Verify files deleted
    files = await file_repo.find(
        QueryBuilder().filter("commit_sha", FilterOperator.EQ, commit.commit_sha)
    )
    assert len(files) == 0

    # Verify enrichment deleted
    enrichments = await enrichment_repo.find(
        QueryBuilder().filter("id", FilterOperator.EQ, snippet.id)
    )
    assert len(enrichments) == 0

    # Verify tasks queued
    mock_queue.enqueue_tasks.assert_called_once_with(
        tasks=PrescribedOperations.SCAN_AND_INDEX_COMMIT,
        base_priority=QueuePriority.USER_INITIATED,
        payload={"repository_id": repo.id, "commit_sha": commit.commit_sha},
    )
