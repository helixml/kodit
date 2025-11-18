"""Handler for creating summary embeddings for examples in a commit."""

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


class CreateExampleSummaryEmbeddingsHandler:
    """Handler for creating summary embeddings for examples."""

    def __init__(  # noqa: PLR0913
        self,
        text_search_service: EmbeddingDomainService,
        embedding_repository: SqlAlchemyEmbeddingRepository,
        enrichment_v2_repository: EnrichmentV2Repository,
        enrichment_association_repository: EnrichmentAssociationRepository,
        enrichment_query_service: "EnrichmentQueryService",
        operation: ProgressTracker,
    ) -> None:
        """Initialize the create example summary embeddings handler."""
        self.text_search_service = text_search_service
        self.embedding_repository = embedding_repository
        self.enrichment_v2_repository = enrichment_v2_repository
        self.enrichment_association_repository = enrichment_association_repository
        self.enrichment_query_service = enrichment_query_service
        self.operation = operation

    async def execute(self, payload: dict[str, Any]) -> None:
        """Execute create example summary embeddings operation."""
        repository_id = payload["repository_id"]
        commit_sha = payload["commit_sha"]

        async with self.operation.create_child(
            TaskOperation.CREATE_EXAMPLE_SUMMARY_EMBEDDINGS_FOR_COMMIT,
            trackable_type=TrackableType.KODIT_REPOSITORY,
            trackable_id=repository_id,
        ) as step:
            all_example_enrichments = (
                await self.enrichment_query_service.get_all_examples_for_commit(
                    commit_sha
                )
            )
            if not all_example_enrichments:
                await step.skip("No examples to create summary embeddings")
                return

            query = EnrichmentAssociationQueryBuilder.for_enrichment_associations(
                entity_type=db_entities.EnrichmentV2.__tablename__,
                entity_ids=[
                    str(example.id) for example in all_example_enrichments if example.id
                ],
            )
            summary_enrichment_associations = (
                await self.enrichment_association_repository.find(query)
            )

            if not summary_enrichment_associations:
                await step.skip("No summary enrichments found for examples")
                return

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

            new_summaries = await self._new_examples_for_type(
                summary_enrichments, EmbeddingType.TEXT
            )
            if not new_summaries:
                await step.skip("All examples already have text embeddings")
                return

            await step.set_total(len(new_summaries))
            processed = 0

            documents_with_summaries = [
                Document(snippet_id=str(summary.id), text=summary.content)
                for summary in new_summaries
                if summary.id
            ]

            async for result in self.text_search_service.index_documents(
                IndexRequest(documents=documents_with_summaries)
            ):
                processed += len(result)
                await step.set_current(
                    processed, "Creating text embeddings for example summaries"
                )

    async def _new_examples_for_type(
        self, all_examples: list[EnrichmentV2], embedding_type: EmbeddingType
    ) -> list[EnrichmentV2]:
        """Get new examples for a given type."""
        existing_embeddings = (
            await self.embedding_repository.list_embeddings_by_snippet_ids_and_type(
                [str(e.id) for e in all_examples], embedding_type
            )
        )
        if existing_embeddings:
            return []
        existing_embeddings_by_example_id = {
            embedding.snippet_id: embedding for embedding in existing_embeddings
        }
        return [
            e for e in all_examples if e.id not in existing_embeddings_by_example_id
        ]
