"""Handler for creating summary embeddings for a commit."""

from typing import TYPE_CHECKING, Any

from kodit.application.services.reporting import ProgressTracker
from kodit.domain.enrichments.enrichment import EnrichmentV2
from kodit.domain.protocols import (
    EnrichmentAssociationRepository,
    EnrichmentV2Repository,
)
from kodit.domain.services.embedding_service import EmbeddingDomainService
from kodit.domain.value_objects import (
    Document,
    IndexRequest,
    TaskOperation,
    TrackableType,
)
from kodit.infrastructure.sqlalchemy import entities as db_entities
from kodit.infrastructure.sqlalchemy.embedding_repository import (
    SqlAlchemyEmbeddingRepository,
)
from kodit.infrastructure.sqlalchemy.entities import EmbeddingType
from kodit.infrastructure.sqlalchemy.query import (
    EnrichmentAssociationQueryBuilder,
    FilterOperator,
    QueryBuilder,
)

if TYPE_CHECKING:
    from kodit.application.services.enrichment_query_service import (
        EnrichmentQueryService,
    )


class CreateSummaryEmbeddingsHandler:
    """Handler for creating summary embeddings for a commit."""

    def __init__(  # noqa: PLR0913
        self,
        text_search_service: EmbeddingDomainService,
        embedding_repository: SqlAlchemyEmbeddingRepository,
        enrichment_v2_repository: EnrichmentV2Repository,
        enrichment_association_repository: EnrichmentAssociationRepository,
        enrichment_query_service: "EnrichmentQueryService",
        operation: ProgressTracker,
    ) -> None:
        """Initialize the create summary embeddings handler."""
        self.text_search_service = text_search_service
        self.embedding_repository = embedding_repository
        self.enrichment_v2_repository = enrichment_v2_repository
        self.enrichment_association_repository = enrichment_association_repository
        self.enrichment_query_service = enrichment_query_service
        self.operation = operation

    async def execute(self, payload: dict[str, Any]) -> None:
        """Execute create summary embeddings operation."""
        repository_id = payload["repository_id"]
        commit_sha = payload["commit_sha"]

        async with self.operation.create_child(
            TaskOperation.CREATE_SUMMARY_EMBEDDINGS_FOR_COMMIT,
            trackable_type=TrackableType.KODIT_REPOSITORY,
            trackable_id=repository_id,
        ) as step:
            # Get all snippet enrichments for this commit
            all_snippet_enrichments = (
                await self.enrichment_query_service.get_all_snippets_for_commit(
                    commit_sha
                )
            )
            if not all_snippet_enrichments:
                await step.skip("No snippets to create summary embeddings")
                return

            # Get summary enrichments that point to these snippet enrichments
            query = EnrichmentAssociationQueryBuilder.for_enrichment_associations(
                entity_type=db_entities.EnrichmentV2.__tablename__,
                entity_ids=[
                    str(snippet.id) for snippet in all_snippet_enrichments if snippet.id
                ],
            )
            summary_enrichment_associations = (
                await self.enrichment_association_repository.find(query)
            )

            if not summary_enrichment_associations:
                await step.skip("No summary enrichments found for snippets")
                return

            # Get the actual summary enrichments
            summary_enrichments = await self.enrichment_v2_repository.find(
                QueryBuilder().filter(
                    "id",
                    FilterOperator.IN,
                    [
                        association.enrichment_id
                        for association in summary_enrichment_associations
                    ],
                )
            )

            # Check if embeddings already exist for these summaries
            new_summaries = await self._new_snippets_for_type(
                summary_enrichments, EmbeddingType.TEXT
            )
            if not new_summaries:
                await step.skip("All snippets already have text embeddings")
                return

            await step.set_total(len(new_summaries))
            processed = 0

            # Create documents from the summary enrichments
            documents_with_summaries = [
                Document(snippet_id=str(summary.id), text=summary.content)
                for summary in new_summaries
                if summary.id
            ]

            async for result in self.text_search_service.index_documents(
                IndexRequest(documents=documents_with_summaries)
            ):
                processed += len(result)
                await step.set_current(processed, "Creating text embeddings for commit")

    async def _new_snippets_for_type(
        self, all_snippets: list[EnrichmentV2], embedding_type: EmbeddingType
    ) -> list[EnrichmentV2]:
        """Get new snippets for a given type."""
        existing_embeddings = (
            await self.embedding_repository.list_embeddings_by_snippet_ids_and_type(
                [str(s.id) for s in all_snippets], embedding_type
            )
        )
        # TODO(Phil): Can't do this incrementally yet because like the API, we don't
        # have a unified embedding repository
        if existing_embeddings:
            return []
        existing_embeddings_by_snippet_id = {
            embedding.snippet_id: embedding for embedding in existing_embeddings
        }
        return [
            s for s in all_snippets if s.id not in existing_embeddings_by_snippet_id
        ]
