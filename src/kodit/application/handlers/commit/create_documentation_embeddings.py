"""Handler for creating embeddings for documentation in a commit."""

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


class CreateDocumentationEmbeddingsHandler:
    """Handler for creating embeddings for documentation."""

    def __init__(
        self,
        documentation_search_service: EmbeddingDomainService,
        embedding_repository: SqlAlchemyEmbeddingRepository,
        enrichment_query_service: "EnrichmentQueryService",
        operation: ProgressTracker,
    ) -> None:
        """Initialize the create documentation embeddings handler."""
        self.documentation_search_service = documentation_search_service
        self.embedding_repository = embedding_repository
        self.enrichment_query_service = enrichment_query_service
        self.operation = operation

    async def execute(self, payload: dict[str, Any]) -> None:
        """Execute create documentation embeddings operation."""
        repository_id = payload["repository_id"]
        commit_sha = payload["commit_sha"]

        async with self.operation.create_child(
            TaskOperation.CREATE_DOCUMENTATION_EMBEDDINGS_FOR_COMMIT,
            trackable_type=TrackableType.KODIT_REPOSITORY,
            trackable_id=repository_id,
        ) as step:
            existing_enrichments = (
                await self.enrichment_query_service.get_all_documentation_for_commit(
                    commit_sha
                )
            )

            new_docs = await self._new_docs_for_type(
                existing_enrichments, EmbeddingType.DOCUMENTATION
            )
            if not new_docs:
                await step.skip("All documentation already has embeddings")
                return

            await step.set_total(len(new_docs))
            processed = 0
            documents = [
                Document(snippet_id=str(doc.id), text=doc.content)
                for doc in new_docs
                if doc.id
            ]
            async for result in self.documentation_search_service.index_documents(
                IndexRequest(documents=documents)
            ):
                processed += len(result)
                await step.set_current(
                    processed, "Creating embeddings for documentation"
                )

    async def _new_docs_for_type(
        self, all_docs: list[EnrichmentV2], embedding_type: EmbeddingType
    ) -> list[EnrichmentV2]:
        """Get documentation that doesn't have embeddings yet."""
        existing_embeddings = (
            await self.embedding_repository.list_embeddings_by_snippet_ids_and_type(
                [str(e.id) for e in all_docs], embedding_type
            )
        )
        if existing_embeddings:
            return []
        existing_embeddings_by_doc_id = {
            embedding.snippet_id: embedding for embedding in existing_embeddings
        }
        return [
            e for e in all_docs if e.id not in existing_embeddings_by_doc_id
        ]
