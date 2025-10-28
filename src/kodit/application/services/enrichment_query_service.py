"""Application service for querying enrichments."""

import structlog

from kodit.domain.enrichments.enrichment import EnrichmentV2
from kodit.domain.protocols import (
    EnrichmentAssociationRepository,
    EnrichmentV2Repository,
)
from kodit.domain.tracking.resolution_service import TrackableResolutionService
from kodit.domain.tracking.trackable import Trackable
from kodit.infrastructure.sqlalchemy import entities as db_entities
from kodit.infrastructure.sqlalchemy.query import FilterOperator, QueryBuilder


class EnrichmentQueryService:
    """Finds the latest commit with enrichments for a trackable.

    Orchestrates domain services and repositories to fulfill the use case.
    """

    def __init__(
        self,
        trackable_resolution: TrackableResolutionService,
        enrichment_repo: EnrichmentV2Repository,
        enrichment_association_repository: EnrichmentAssociationRepository,
    ) -> None:
        """Initialize the enrichment query service."""
        self.trackable_resolution = trackable_resolution
        self.enrichment_repo = enrichment_repo
        self.enrichment_association_repository = enrichment_association_repository
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
        existing_associations = await self.enrichment_association_repository.find(
            QueryBuilder()
            .filter(
                "entity_type", FilterOperator.EQ, db_entities.GitCommit.__tablename__
            )
            .filter("entity_id", FilterOperator.IN, candidate_commits)
        )
        existing_enrichments = await self.enrichment_repo.find(
            QueryBuilder().filter(
                "id",
                FilterOperator.IN,
                [a.enrichment_id for a in existing_associations],
            )
        )

        if len(existing_associations) != len(existing_enrichments):
            raise ValueError("Mismatch between enrichment associations and enrichments")

        # Filter by type if specified
        if enrichment_type:
            existing_associations = [
                a
                for a, e in zip(
                    existing_associations, existing_enrichments, strict=True
                )
                if e.type == enrichment_type
            ]

        # Find the first commit (newest) that has enrichments
        for commit_sha in candidate_commits:
            if any(e.entity_id == commit_sha for e in existing_associations):
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
        enrichments = await self.enrichment_repo.find(
            QueryBuilder()
            .filter("entity_type", FilterOperator.EQ, "git_commit")
            .filter("entity_id", FilterOperator.EQ, commit_sha)
        )

        # Filter by type if specified
        if enrichment_type:
            enrichments = [e for e in enrichments if e.type == enrichment_type]

        return enrichments
