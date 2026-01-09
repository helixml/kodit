"""Tests for RepositoryStructureHandler."""

import tempfile
from collections.abc import AsyncIterator, Callable
from datetime import UTC, datetime
from pathlib import Path
from unittest.mock import AsyncMock, MagicMock

import git
import pytest
from pydantic import AnyUrl
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.application.handlers.commit.repository_structure import (
    RepositoryStructureHandler,
)
from kodit.application.services.enrichment_query_service import EnrichmentQueryService
from kodit.application.services.reporting import ProgressTracker
from kodit.domain.enrichments.enricher import Enricher
from kodit.domain.enrichments.response import EnrichmentResponse
from kodit.domain.entities.git import GitCommit
from kodit.domain.factories.git_repo_factory import GitRepoFactory
from kodit.domain.services.repository_structure_service import (
    RepositoryStructureService,
)
from kodit.domain.tracking.resolution_service import TrackableResolutionService
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


@pytest.fixture
def mock_progress() -> MagicMock:
    """Create a mock progress tracker."""
    tracker = MagicMock(spec=ProgressTracker)
    ctx = AsyncMock()
    ctx.__aenter__ = AsyncMock(return_value=ctx)
    ctx.__aexit__ = AsyncMock(return_value=None)
    ctx.skip = ctx.set_total = ctx.set_current = AsyncMock()
    tracker.create_child = MagicMock(return_value=ctx)
    return tracker


@pytest.fixture
def mock_enricher() -> MagicMock:
    """Create a mock enricher."""

    async def fake_enrich(requests: list) -> AsyncIterator[EnrichmentResponse]:
        for req in requests:
            yield EnrichmentResponse(id=req.id, text="mock structure output")

    enricher = MagicMock(spec=Enricher)
    enricher.enrich = fake_enrich
    return enricher


@pytest.fixture
async def handler(
    session_factory: Callable[[], AsyncSession],
    mock_progress: MagicMock,
    mock_enricher: MagicMock,
) -> RepositoryStructureHandler:
    """Create handler with real services except LLM."""
    enrichment_v2_repo = create_enrichment_v2_repository(session_factory)
    enrichment_assoc_repo = create_enrichment_association_repository(session_factory)

    trackable_resolution = TrackableResolutionService(
        commit_repo=create_git_commit_repository(session_factory),
        branch_repo=create_git_branch_repository(session_factory),
        tag_repo=create_git_tag_repository(session_factory),
    )

    return RepositoryStructureHandler(
        repo_repository=create_git_repo_repository(session_factory),
        repository_structure_service=RepositoryStructureService(),
        enricher_service=mock_enricher,
        enrichment_v2_repository=enrichment_v2_repo,
        enrichment_association_repository=enrichment_assoc_repo,
        enrichment_query_service=EnrichmentQueryService(
            trackable_resolution=trackable_resolution,
            enrichment_repo=enrichment_v2_repo,
            enrichment_association_repository=enrichment_assoc_repo,
        ),
        operation=mock_progress,
    )


@pytest.mark.asyncio
async def test_creates_enrichment_and_is_idempotent(
    handler: RepositoryStructureHandler,
    session_factory: Callable[[], AsyncSession],
) -> None:
    """Test handler creates enrichment and skips on second run."""
    with tempfile.TemporaryDirectory() as tmpdir:
        repo_path = Path(tmpdir) / "repo"
        repo_path.mkdir()

        git_repo = git.Repo.init(repo_path)
        with git_repo.config_writer() as cw:
            cw.set_value("user", "name", "Test")
            cw.set_value("user", "email", "test@test.com")

        (repo_path / "main.py").write_text("def main(): pass\n")
        git_repo.index.add(["main.py"])
        commit = git_repo.index.commit("init")

        repo = GitRepoFactory.create_from_remote_uri(
            AnyUrl("https://github.com/test/repo.git")
        )
        repo.cloned_path = repo_path
        repo = await create_git_repo_repository(session_factory).save(repo)

        await create_git_commit_repository(session_factory).save(
            GitCommit(
                commit_sha=commit.hexsha,
                repo_id=repo.id,
                message=str(commit.message),
                author=str(commit.author),
                date=datetime.fromtimestamp(commit.committed_date, UTC),
            )
        )

        payload = {"repository_id": repo.id, "commit_sha": commit.hexsha}

        await handler.execute(payload)
        await handler.execute(payload)  # Second run should skip

        enrichments = (
            await handler.enrichment_query_service.get_repository_structure_for_commit(
                commit.hexsha
            )
        )
        assert len(enrichments) == 1
