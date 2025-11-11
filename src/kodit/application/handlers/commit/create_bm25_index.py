"""Handler for creating BM25 index for a commit."""

from typing import TYPE_CHECKING, Any

from kodit.application.services.reporting import ProgressTracker
from kodit.domain.services.bm25_service import BM25DomainService
from kodit.domain.value_objects import (
    Document,
    IndexRequest,
    TaskOperation,
    TrackableType,
)

if TYPE_CHECKING:
    from kodit.application.services.enrichment_query_service import (
        EnrichmentQueryService,
    )


class CreateBM25IndexHandler:
    """Handler for creating BM25 keyword index for a commit."""

    def __init__(
        self,
        bm25_service: BM25DomainService,
        enrichment_query_service: "EnrichmentQueryService",
        operation: ProgressTracker,
    ) -> None:
        """Initialize the create BM25 index handler."""
        self.bm25_service = bm25_service
        self.enrichment_query_service = enrichment_query_service
        self.operation = operation

    async def execute(self, payload: dict[str, Any]) -> None:
        """Execute create BM25 index operation."""
        repository_id = payload["repository_id"]
        commit_sha = payload["commit_sha"]

        async with self.operation.create_child(
            TaskOperation.CREATE_BM25_INDEX_FOR_COMMIT,
            trackable_type=TrackableType.KODIT_REPOSITORY,
            trackable_id=repository_id,
        ):
            existing_enrichments = (
                await self.enrichment_query_service.get_all_snippets_for_commit(
                    commit_sha
                )
            )
            await self.bm25_service.index_documents(
                IndexRequest(
                    documents=[
                        Document(snippet_id=str(snippet.id), text=snippet.content)
                        for snippet in existing_enrichments
                        if snippet.id
                    ]
                )
            )
