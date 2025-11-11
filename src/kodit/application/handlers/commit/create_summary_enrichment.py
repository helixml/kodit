"""Handler for creating summary enrichments for snippets in a commit."""

from typing import TYPE_CHECKING, Any

from kodit.application.services.reporting import ProgressTracker
from kodit.domain.enrichments.development.snippet.snippet import (
    SnippetEnrichmentSummary,
)
from kodit.domain.enrichments.enricher import Enricher
from kodit.domain.enrichments.enrichment import EnrichmentAssociation
from kodit.domain.enrichments.request import (
    EnrichmentRequest as GenericEnrichmentRequest,
)
from kodit.domain.protocols import (
    EnrichmentAssociationRepository,
    EnrichmentV2Repository,
)
from kodit.domain.value_objects import TaskOperation, TrackableType
from kodit.infrastructure.sqlalchemy import entities as db_entities

if TYPE_CHECKING:
    from kodit.application.services.enrichment_query_service import (
        EnrichmentQueryService,
    )

SUMMARIZATION_SYSTEM_PROMPT = """
You are a professional software developer. You will be given a snippet of code.
Please provide a concise explanation of the code.
"""


class CreateSummaryEnrichmentHandler:
    """Handler for creating summary enrichments for snippets."""

    def __init__(
        self,
        enricher_service: Enricher,
        enrichment_v2_repository: EnrichmentV2Repository,
        enrichment_association_repository: EnrichmentAssociationRepository,
        enrichment_query_service: "EnrichmentQueryService",
        operation: ProgressTracker,
    ) -> None:
        """Initialize the create summary enrichment handler."""
        self.enricher_service = enricher_service
        self.enrichment_v2_repository = enrichment_v2_repository
        self.enrichment_association_repository = enrichment_association_repository
        self.enrichment_query_service = enrichment_query_service
        self.operation = operation

    async def execute(self, payload: dict[str, Any]) -> None:
        """Execute create summary enrichment operation."""
        repository_id = payload["repository_id"]
        commit_sha = payload["commit_sha"]

        async with self.operation.create_child(
            TaskOperation.CREATE_SUMMARY_ENRICHMENT_FOR_COMMIT,
            trackable_type=TrackableType.KODIT_REPOSITORY,
            trackable_id=repository_id,
        ) as step:
            if await self.enrichment_query_service.has_summaries_for_commit(commit_sha):
                await step.skip("Summary enrichments already exist for commit")
                return

            all_snippets = (
                await self.enrichment_query_service.get_all_snippets_for_commit(
                    commit_sha
                )
            )
            if not all_snippets:
                await step.skip("No snippets to enrich")
                return

            # Enrich snippets
            await step.set_total(len(all_snippets))
            snippet_map = {
                str(snippet.id): snippet for snippet in all_snippets if snippet.id
            }

            enrichment_requests = [
                GenericEnrichmentRequest(
                    id=str(snippet_id),
                    text=snippet.content,
                    system_prompt=SUMMARIZATION_SYSTEM_PROMPT,
                )
                for snippet_id, snippet in snippet_map.items()
            ]

            processed = 0
            async for result in self.enricher_service.enrich(enrichment_requests):
                snippet = snippet_map[result.id]
                db_summary = await self.enrichment_v2_repository.save(
                    SnippetEnrichmentSummary(content=result.text)
                )
                if not db_summary.id:
                    raise ValueError(
                        f"Failed to save snippet enrichment for commit {commit_sha}"
                    )
                await self.enrichment_association_repository.save(
                    EnrichmentAssociation(
                        enrichment_id=db_summary.id,
                        entity_type=db_entities.EnrichmentV2.__tablename__,
                        entity_id=str(snippet.id),
                    )
                )
                await self.enrichment_association_repository.save(
                    EnrichmentAssociation(
                        enrichment_id=db_summary.id,
                        entity_type=db_entities.GitCommit.__tablename__,
                        entity_id=commit_sha,
                    )
                )
                processed += 1
                await step.set_current(processed, "Enriching snippets for commit")
