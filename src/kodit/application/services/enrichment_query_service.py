"""Application service for querying enrichments."""

import structlog

from kodit.domain.enrichments.enrichment import EnrichmentV2
from kodit.domain.tracking.resolution_service import TrackableResolutionService
from kodit.domain.tracking.trackable import Trackable
from kodit.infrastructure.sqlalchemy.enrichment_v2_repository import (
    EnrichmentV2Repository,
)


class EnrichmentQueryService:
    """Finds the latest commit with enrichments for a trackable.

    Orchestrates domain services and repositories to fulfill the use case.
    """

    def __init__(
        self,
        trackable_resolution: TrackableResolutionService,
        enrichment_repo: EnrichmentV2Repository,
    ) -> None:
        """Initialize the enrichment query service."""
        self.trackable_resolution = trackable_resolution
        self.enrichment_repo = enrichment_repo
        self.log = structlog.get_logger(__name__)

    async def find_latest_enriched_commit(
        self,
        trackable: Trackable,
        enrichment_type: str | None = None,
        max_commits_to_check: int = 100,
    ) -> str | None:
        """Find the most recent commit with enrichments.

        Args:
            trackable: What to track (branch, tag, or commit)
            enrichment_type: Optional filter for specific enrichment type
            max_commits_to_check: How far back in history to search

        Returns:
            Commit SHA of the most recent commit with enrichments, or None

        """
        # Get candidate commits from the trackable
        candidate_commits = await self.trackable_resolution.resolve_to_commits(
            trackable, max_commits_to_check
        )

        if not candidate_commits:
            return None

        # Check which commits have enrichments
        enrichments = await self.enrichment_repo.enrichments_for_entity_type(
            entity_type="git_commit",
            entity_ids=candidate_commits,
        )

        # Filter by type if specified
        if enrichment_type:
            enrichments = [e for e in enrichments if e.type == enrichment_type]

        # Find the first commit (newest) that has enrichments
        for commit_sha in candidate_commits:
            if any(e.entity_id == commit_sha for e in enrichments):
                return commit_sha

        return None

    async def get_enrichments_for_commit(
        self,
        commit_sha: str,
        enrichment_type: str | None = None,
    ) -> list[EnrichmentV2]:
        """Get all enrichments for a specific commit.

        Args:
            commit_sha: The commit SHA to get enrichments for
            enrichment_type: Optional filter for specific enrichment type

        Returns:
            List of enrichments for the commit

        """
        enrichments = await self.enrichment_repo.enrichments_for_entity_type(
            entity_type="git_commit",
            entity_ids=[commit_sha],
        )

        # Filter by type if specified
        if enrichment_type:
            enrichments = [e for e in enrichments if e.type == enrichment_type]

        return enrichments
