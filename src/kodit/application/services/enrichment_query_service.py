"""Application service for querying enrichments."""

import structlog

from kodit.domain.enrichments.architecture.architecture import (
    ENRICHMENT_TYPE_ARCHITECTURE,
)
from kodit.domain.enrichments.architecture.database_schema.database_schema import (
    ENRICHMENT_SUBTYPE_DATABASE_SCHEMA,
)
from kodit.domain.enrichments.architecture.physical.physical import (
    ENRICHMENT_SUBTYPE_PHYSICAL,
)
from kodit.domain.enrichments.development.development import ENRICHMENT_TYPE_DEVELOPMENT
from kodit.domain.enrichments.development.snippet.snippet import (
    ENRICHMENT_SUBTYPE_SNIPPET,
    ENRICHMENT_SUBTYPE_SNIPPET_SUMMARY,
)
from kodit.domain.enrichments.enrichment import EnrichmentAssociation, EnrichmentV2
from kodit.domain.enrichments.history.commit_description.commit_description import (
    ENRICHMENT_SUBTYPE_COMMIT_DESCRIPTION,
)
from kodit.domain.enrichments.history.history import ENRICHMENT_TYPE_HISTORY
from kodit.domain.enrichments.usage.api_docs import ENRICHMENT_SUBTYPE_API_DOCS
from kodit.domain.enrichments.usage.cookbook import ENRICHMENT_SUBTYPE_COOKBOOK
from kodit.domain.enrichments.usage.usage import ENRICHMENT_TYPE_USAGE
from kodit.domain.protocols import (
    EnrichmentAssociationRepository,
    EnrichmentV2Repository,
)
from kodit.domain.tracking.resolution_service import TrackableResolutionService
from kodit.infrastructure.api.v1.query_params import PaginationParams
from kodit.infrastructure.sqlalchemy import entities as db_entities
from kodit.infrastructure.sqlalchemy.query import (
    EnrichmentAssociationQueryBuilder,
    EnrichmentQueryBuilder,
)


class EnrichmentQueryService:
    """Service for querying enrichments."""

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

    async def associations_for_commit(
        self, commit_sha: str
    ) -> list[EnrichmentAssociation]:
        """Get enrichments for a commit."""
        return await self.enrichment_association_repository.find(
            EnrichmentAssociationQueryBuilder.for_enrichment_associations(
                entity_type=db_entities.GitCommit.__tablename__,
                entity_ids=[commit_sha],
            )
        )

    async def has_enrichments_for_commit(
        self,
        commit_sha: str,
        pagination: PaginationParams,
        enrichment_type: str | None = None,
    ) -> bool:
        """Check if a commit has enrichments."""
        enrichments = await self.all_enrichments_for_commit(
            commit_sha=commit_sha,
            pagination=pagination,
            enrichment_type=enrichment_type,
        )
        return bool(enrichments)

    async def all_enrichments_for_commit(
        self,
        commit_sha: str,
        pagination: PaginationParams,
        enrichment_type: str | None = None,
        enrichment_subtype: str | None = None,
    ) -> dict[EnrichmentV2, list[EnrichmentAssociation]]:
        """Get all enrichments for a specific commit."""
        associations = await self.enrichment_association_repository.find(
            EnrichmentAssociationQueryBuilder().for_commit(commit_sha)
        )
        enrichment_ids = [association.enrichment_id for association in associations]
        query = EnrichmentQueryBuilder().for_ids(enrichment_ids).paginate(pagination)
        if enrichment_type:
            query = query.for_type(enrichment_type)
        if enrichment_subtype:
            query = query.for_subtype(enrichment_subtype)
        enrichments = await self.enrichment_repo.find(query)
        # Find all other associations for these enrichments
        other_associations = await self.enrichment_association_repository.find(
            EnrichmentAssociationQueryBuilder().for_enrichments(enrichments)
        )
        all_associations = set(associations + other_associations)
        return {
            enrichment: [
                association
                for association in all_associations
                if association.enrichment_id == enrichment.id
                or association.entity_id == str(enrichment.id)
            ]
            for enrichment in enrichments
        }

    async def get_all_snippets_for_commit(self, commit_sha: str) -> list[EnrichmentV2]:
        """Get snippet enrichments for a commit."""
        return list(
            await self.all_enrichments_for_commit(
                commit_sha=commit_sha,
                pagination=PaginationParams(page_size=32000),
                enrichment_type=ENRICHMENT_TYPE_DEVELOPMENT,
                enrichment_subtype=ENRICHMENT_SUBTYPE_SNIPPET,
            )
        )

    async def get_summaries_for_commit(self, commit_sha: str) -> list[EnrichmentV2]:
        """Get summary enrichments for a commit."""
        return await self.get_enrichments_for_commit(
            commit_sha,
            enrichment_type=ENRICHMENT_TYPE_DEVELOPMENT,
            enrichment_subtype=ENRICHMENT_SUBTYPE_SNIPPET_SUMMARY,
        )

    async def get_enrichments_for_commit(
        self,
        commit_sha: str,
        enrichment_type: str | None = None,
        enrichment_subtype: str | None = None,
    ) -> list[EnrichmentV2]:
        """Get enrichments for a commit."""
        # Find associations pointing to this commit
        all_associations = await self.enrichment_association_repository.find(
            EnrichmentAssociationQueryBuilder().for_commit(commit_sha)
        )
        query = EnrichmentQueryBuilder().for_ids(
            enrichment_ids=[
                int(association.enrichment_id) for association in all_associations
            ]
        )
        if enrichment_type:
            query = query.for_type(enrichment_type)
        if enrichment_subtype:
            query = query.for_subtype(enrichment_subtype)
        return await self.enrichment_repo.find(query)

    async def get_architecture_docs_for_commit(
        self, commit_sha: str
    ) -> list[EnrichmentV2]:
        """Get architecture documentation enrichments for a commit."""
        return await self.get_enrichments_for_commit(
            commit_sha,
            enrichment_type=ENRICHMENT_TYPE_ARCHITECTURE,
            enrichment_subtype=ENRICHMENT_SUBTYPE_PHYSICAL,
        )

    async def get_api_docs_for_commit(self, commit_sha: str) -> list[EnrichmentV2]:
        """Get API documentation enrichments for a commit."""
        return await self.get_enrichments_for_commit(
            commit_sha,
            enrichment_type=ENRICHMENT_TYPE_USAGE,
            enrichment_subtype=ENRICHMENT_SUBTYPE_API_DOCS,
        )

    async def get_enrichment_entities_from_associations(
        self, associations: list[EnrichmentAssociation]
    ) -> list[EnrichmentV2]:
        """Get enrichments by their associations."""
        return await self.enrichment_repo.find(
            EnrichmentQueryBuilder().for_ids(
                enrichment_ids=[
                    int(association.entity_id) for association in associations
                ]
            )
        )

    async def get_enrichments_by_ids(
        self, enrichment_ids: list[int]
    ) -> list[EnrichmentV2]:
        """Get enrichments by their IDs."""
        return await self.enrichment_repo.find(
            EnrichmentQueryBuilder().for_ids(enrichment_ids=enrichment_ids)
        )

    async def has_snippets_for_commit(self, commit_sha: str) -> bool:
        """Check if a commit has snippet enrichments."""
        snippets = await self.get_all_snippets_for_commit(commit_sha)
        return len(snippets) > 0

    async def has_summaries_for_commit(self, commit_sha: str) -> bool:
        """Check if a commit has summary enrichments."""
        summaries = await self.get_summaries_for_commit(commit_sha)
        return len(summaries) > 0

    async def has_architecture_for_commit(self, commit_sha: str) -> bool:
        """Check if a commit has architecture enrichments."""
        architecture_docs = await self.get_architecture_docs_for_commit(commit_sha)
        return len(architecture_docs) > 0

    async def has_api_docs_for_commit(self, commit_sha: str) -> bool:
        """Check if a commit has API documentation enrichments."""
        api_docs = await self.get_api_docs_for_commit(commit_sha)
        return len(api_docs) > 0

    async def get_commit_description_for_commit(
        self, commit_sha: str
    ) -> list[EnrichmentV2]:
        """Get commit description enrichments for a commit."""
        return await self.get_enrichments_for_commit(
            commit_sha,
            enrichment_type=ENRICHMENT_TYPE_HISTORY,
            enrichment_subtype=ENRICHMENT_SUBTYPE_COMMIT_DESCRIPTION,
        )

    async def has_commit_description_for_commit(self, commit_sha: str) -> bool:
        """Check if a commit has commit description enrichments."""
        commit_descriptions = await self.get_commit_description_for_commit(commit_sha)
        return len(commit_descriptions) > 0

    async def get_database_schema_for_commit(
        self, commit_sha: str
    ) -> list[EnrichmentV2]:
        """Get database schema enrichments for a commit."""
        return await self.get_enrichments_for_commit(
            commit_sha,
            enrichment_type=ENRICHMENT_TYPE_ARCHITECTURE,
            enrichment_subtype=ENRICHMENT_SUBTYPE_DATABASE_SCHEMA,
        )

    async def has_database_schema_for_commit(self, commit_sha: str) -> bool:
        """Check if a commit has database schema enrichments."""
        database_schemas = await self.get_database_schema_for_commit(commit_sha)
        return len(database_schemas) > 0

    async def get_cookbook_for_commit(self, commit_sha: str) -> list[EnrichmentV2]:
        """Get cookbook enrichments for a commit."""
        return await self.get_enrichments_for_commit(
            commit_sha,
            enrichment_type=ENRICHMENT_TYPE_USAGE,
            enrichment_subtype=ENRICHMENT_SUBTYPE_COOKBOOK,
        )

    async def has_cookbook_for_commit(self, commit_sha: str) -> bool:
        """Check if a commit has cookbook enrichments."""
        cookbooks = await self.get_cookbook_for_commit(commit_sha)
        return len(cookbooks) > 0

    async def associations_for_enrichments(
        self, enrichments: list[EnrichmentV2]
    ) -> list[EnrichmentAssociation]:
        """Get enrichment associations for given enrichment IDs."""
        return await self.enrichment_association_repository.find(
            EnrichmentAssociationQueryBuilder()
            .for_enrichments(enrichments)
            .for_enrichment_type()
        )

    async def snippet_associations_from_enrichments(
        self, enrichments: list[EnrichmentV2]
    ) -> list[EnrichmentAssociation]:
        """Get snippet enrichment associations for given enrichments."""
        return await self.enrichment_association_repository.find(
            EnrichmentAssociationQueryBuilder()
            .for_enrichments(enrichments)
            .for_enrichment_type()
        )

    async def snippets_for_summary_enrichments(
        self, summary_enrichments: list[EnrichmentV2]
    ) -> list[EnrichmentV2]:
        """Get snippet enrichment IDs for summary enrichments, preserving order."""
        if not summary_enrichments:
            return []

        # Get associations where enrichment_id points to these summaries
        associations = await self.enrichment_association_repository.find(
            EnrichmentAssociationQueryBuilder()
            .for_enrichments(summary_enrichments)
            .for_enrichment_type()
        )

        all_enrichments = await self.enrichment_repo.find(
            EnrichmentQueryBuilder().for_ids(
                enrichment_ids=[
                    int(association.entity_id) for association in associations
                ]
            )
        )
        snippet_enrichments = [
            e for e in all_enrichments if e.subtype == ENRICHMENT_SUBTYPE_SNIPPET
        ]

        # Re-Sort snippet enrichments to be in the same order as the associations
        original_snippet_ids = [association.entity_id for association in associations]
        return sorted(
            snippet_enrichments,
            key=lambda x: original_snippet_ids.index(str(x.id)),
        )

    async def get_enrichments_pointing_to_enrichments(
        self, target_enrichment_ids: list[int]
    ) -> dict[int, list[EnrichmentV2]]:
        """Get enrichments that point to the given enrichments, grouped by target."""
        # Get associations pointing to these enrichments
        associations = await self.enrichment_association_repository.find(
            EnrichmentAssociationQueryBuilder()
            .for_enrichment_type()
            .for_entity_ids(
                [str(enrichment_id) for enrichment_id in target_enrichment_ids]
            )
        )

        if not associations:
            return {eid: [] for eid in target_enrichment_ids}

        # Get the enrichments referenced by these associations
        enrichment_ids = [a.enrichment_id for a in associations]
        enrichments = await self.enrichment_repo.find(
            EnrichmentQueryBuilder().for_ids(enrichment_ids=enrichment_ids)
        )

        # Create lookup map
        enrichment_map = {e.id: e for e in enrichments if e.id is not None}

        # Group by target enrichment ID
        result: dict[int, list[EnrichmentV2]] = {
            eid: [] for eid in target_enrichment_ids
        }
        for association in associations:
            target_id = int(association.entity_id)
            if target_id in result and association.enrichment_id in enrichment_map:
                result[target_id].append(enrichment_map[association.enrichment_id])

        return result

    async def summary_to_snippet_map(self, summary_ids: list[int]) -> dict[int, int]:
        """Get a map of summary IDs to snippet IDs."""
        # Get the snippet enrichment IDs that these summaries point to
        summary_enrichments = await self.get_enrichments_by_ids(summary_ids)

        # Get all the associations for these summary enrichments
        all_associations = await self.associations_for_enrichments(summary_enrichments)

        # Get all enrichments for these summary associations
        all_snippet_enrichments = await self.get_enrichment_entities_from_associations(
            all_associations
        )
        snippet_type_map = {e.id: e.subtype for e in all_snippet_enrichments}

        # Create a lookup map from summary ID to snippet ID, via the associations,
        # filtering out any snippets that are not summary enrichments
        return {
            assoc.enrichment_id: int(assoc.entity_id)
            for assoc in all_associations
            if snippet_type_map[int(assoc.entity_id)] == ENRICHMENT_SUBTYPE_SNIPPET
        }
