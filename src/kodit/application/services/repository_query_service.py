"""Application service for querying repository information."""

from collections.abc import Callable

import structlog

from kodit.domain.protocols import GitRepoRepository
from kodit.domain.tracking.resolution_service import TrackableResolutionService
from kodit.domain.tracking.trackable import Trackable, TrackableReferenceType
from kodit.infrastructure.api.v1.query_params import PaginationParams


class RepositoryQueryService:
    """Service for querying repository information."""

    def __init__(
        self,
        git_repo_repository: GitRepoRepository,
        trackable_resolution: TrackableResolutionService,
    ) -> None:
        """Initialize the repository query service."""
        self.git_repo_repository = git_repo_repository
        self.trackable_resolution = trackable_resolution
        self.log = structlog.get_logger(__name__)

    async def find_repo_by_url(self, repo_url: str) -> int | None:
        """Find a repository ID by its URL.

        Matches against both remote_uri and sanitized_remote_uri.
        """
        from kodit.infrastructure.sqlalchemy.query import FilterOperator, QueryBuilder

        # Try to find by sanitized_remote_uri first (more common)
        repos = await self.git_repo_repository.find(
            QueryBuilder().filter("sanitized_remote_uri", FilterOperator.EQ, repo_url)
        )

        if repos:
            return repos[0].id

        # Try to find by remote_uri
        repos = await self.git_repo_repository.find(
            QueryBuilder().filter("remote_uri", FilterOperator.EQ, repo_url)
        )

        if repos:
            return repos[0].id

        self.log.warning("Repository not found by URL", repo_url=repo_url)
        return None

    async def find_latest_commit(
        self,
        repo_id: int,
        max_commits_to_check: int = 100,
    ) -> str | None:
        """Find the most recent commit for a repository.

        Uses the repository's tracking_config to determine which branch/tag to check.
        """
        # Get the repository
        repo = await self.git_repo_repository.get(repo_id)
        if not repo:
            self.log.warning("Repository not found", repo_id=repo_id)
            return None

        # Create trackable from repository's tracking config
        trackable = Trackable(
            type=TrackableReferenceType(repo.tracking_config.type),
            identifier=repo.tracking_config.name,
            repo_id=repo_id,
        )

        # Get candidate commits from the trackable
        candidate_commits = await self.trackable_resolution.resolve_to_commits(
            trackable, max_commits_to_check
        )

        if not candidate_commits:
            return None

        # Return the most recent commit
        return candidate_commits[0]

    async def find_latest_enriched_commit(
        self,
        repo_id: int,
        enrichment_type: str | None = None,
        max_commits_to_check: int = 100,
        check_enrichments_fn: Callable | None = None,
    ) -> str | None:
        """Find the most recent commit with enrichments for a repository.

        Uses the repository's tracking_config to determine which branch/tag to check.
        """
        # Get the repository
        repo = await self.git_repo_repository.get(repo_id)
        if not repo:
            self.log.warning("Repository not found", repo_id=repo_id)
            return None

        # Create trackable from repository's tracking config
        trackable = Trackable(
            type=TrackableReferenceType(repo.tracking_config.type),
            identifier=repo.tracking_config.name,
            repo_id=repo_id,
        )

        # Get candidate commits from the trackable
        candidate_commits = await self.trackable_resolution.resolve_to_commits(
            trackable, max_commits_to_check
        )

        if not candidate_commits:
            return None

        # Check which commits have enrichments using the provided function
        if check_enrichments_fn:
            for commit_sha in candidate_commits:
                has_enrichments = await check_enrichments_fn(
                    commit_sha=commit_sha,
                    pagination=PaginationParams(page_size=1),
                    enrichment_type=enrichment_type,
                )
                if has_enrichments:
                    return commit_sha

        return None
