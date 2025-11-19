"""Handler for creating code embeddings for examples in a commit."""

from typing import TYPE_CHECKING, Any

from kodit.application.services.reporting import ProgressTracker
from kodit.domain.enrichments.enrichment import EnrichmentV2
from kodit.domain.services.embedding_service import EmbeddingDomainService
from kodit.domain.value_objects import (
    Document,
    IndexRequest,
    TaskOperation,
    TrackableType,
)
from kodit.infrastructure.sqlalchemy.embedding_repository import (
    SqlAlchemyEmbeddingRepository,
)
from kodit.infrastructure.sqlalchemy.entities import EmbeddingType

if TYPE_CHECKING:
    from kodit.application.services.enrichment_query_service import (
        EnrichmentQueryService,
    )


class CreateExampleCodeEmbeddingsHandler:
    """Handler for creating code embeddings for examples."""

    def __init__(
        self,
        code_search_service: EmbeddingDomainService,
        embedding_repository: SqlAlchemyEmbeddingRepository,
        enrichment_query_service: "EnrichmentQueryService",
        operation: ProgressTracker,
    ) -> None:
        """Initialize the create example code embeddings handler."""
        self.code_search_service = code_search_service
        self.embedding_repository = embedding_repository
        self.enrichment_query_service = enrichment_query_service
        self.operation = operation

    async def execute(self, payload: dict[str, Any]) -> None:
        """Execute create example code embeddings operation."""
        repository_id = payload["repository_id"]
        commit_sha = payload["commit_sha"]

        async with self.operation.create_child(
            TaskOperation.CREATE_EXAMPLE_CODE_EMBEDDINGS_FOR_COMMIT,
            trackable_type=TrackableType.KODIT_REPOSITORY,
            trackable_id=repository_id,
        ) as step:
            existing_enrichments = (
                await self.enrichment_query_service.get_all_examples_for_commit(
                    commit_sha
                )
            )

            new_examples = await self._new_examples_for_type(
                existing_enrichments, EmbeddingType.CODE
            )
            if not new_examples:
                await step.skip("All examples already have code embeddings")
                return

            await step.set_total(len(new_examples))
            processed = 0
            documents = [
                Document(snippet_id=str(example.id), text=example.content)
                for example in new_examples
                if example.id
            ]
            async for result in self.code_search_service.index_documents(
                IndexRequest(documents=documents)
            ):
                processed += len(result)
                await step.set_current(
                    processed, "Creating code embeddings for examples"
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
