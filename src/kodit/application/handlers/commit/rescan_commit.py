"""Handler for rescanning a specific commit."""

from typing import TYPE_CHECKING, Any

from kodit.application.services.queue_service import QueueService
from kodit.application.services.reporting import ProgressTracker
from kodit.domain.protocols import (
    EnrichmentAssociationRepository,
    EnrichmentV2Repository,
    GitFileRepository,
)
from kodit.domain.services.bm25_service import BM25DomainService
from kodit.domain.value_objects import (
    DeleteRequest,
    PrescribedOperations,
    QueuePriority,
    TaskOperation,
    TrackableType,
)
from kodit.infrastructure.sqlalchemy import entities as db_entities
from kodit.infrastructure.sqlalchemy.embedding_repository import (
    SqlAlchemyEmbeddingRepository,
)
from kodit.infrastructure.sqlalchemy.query import FilterOperator, QueryBuilder

if TYPE_CHECKING:
    from kodit.application.services.enrichment_query_service import (
        EnrichmentQueryService,
    )


class RescanCommitHandler:
    """Handler for rescanning a specific commit."""

    def __init__(  # noqa: PLR0913
        self,
        git_file_repository: GitFileRepository,
        bm25_service: BM25DomainService,
        embedding_repository: SqlAlchemyEmbeddingRepository,
        enrichment_v2_repository: EnrichmentV2Repository,
        enrichment_association_repository: EnrichmentAssociationRepository,
        enrichment_query_service: "EnrichmentQueryService",
        queue: QueueService,
        operation: ProgressTracker,
    ) -> None:
        """Initialize the rescan commit handler."""
        self.git_file_repository = git_file_repository
        self.bm25_service = bm25_service
        self.embedding_repository = embedding_repository
        self.enrichment_v2_repository = enrichment_v2_repository
        self.enrichment_association_repository = enrichment_association_repository
        self.enrichment_query_service = enrichment_query_service
        self.queue = queue
        self.operation = operation

    async def execute(self, payload: dict[str, Any]) -> None:
        """Execute rescan commit operation."""
        repository_id = payload["repository_id"]
        commit_sha = payload["commit_sha"]

        async with self.operation.create_child(
            TaskOperation.RESCAN_COMMIT,
            trackable_type=TrackableType.KODIT_COMMIT,
            trackable_id=repository_id,
        ):
            # Delete snippet enrichments and their indices
            await self._delete_snippet_enrichments(commit_sha)

            # Delete example enrichments and their indices
            await self._delete_example_enrichments(commit_sha)

            # Delete commit-level enrichments
            await self._delete_commit_enrichments(commit_sha)

            # Delete git files for this commit
            await self.git_file_repository.delete_by_commit_sha(commit_sha)

            # Re-trigger the indexing pipeline
            await self.queue.enqueue_tasks(
                tasks=PrescribedOperations.SCAN_AND_INDEX_COMMIT,
                base_priority=QueuePriority.USER_INITIATED,
                payload={
                    "repository_id": repository_id,
                    "commit_sha": commit_sha,
                },
            )

    async def _delete_snippet_enrichments(self, commit_sha: str) -> None:
        """Delete snippet enrichments and their indices for a commit."""
        snippet_enrichments = (
            await self.enrichment_query_service.get_all_snippets_for_commit(commit_sha)
        )
        enrichment_ids = [
            enrichment.id for enrichment in snippet_enrichments if enrichment.id
        ]

        if not enrichment_ids:
            return

        # Delete from BM25 and embedding indices
        snippet_id_strings = [str(sid) for sid in enrichment_ids]
        delete_request = DeleteRequest(snippet_ids=snippet_id_strings)
        await self.bm25_service.delete_documents(delete_request)

        for snippet_id in enrichment_ids:
            await self.embedding_repository.delete_embeddings_by_snippet_id(
                str(snippet_id)
            )

        # Delete enrichment associations for snippets
        await self.enrichment_association_repository.delete_by_query(
            QueryBuilder()
            .filter("entity_type", FilterOperator.EQ, "snippet_v2")
            .filter("entity_id", FilterOperator.IN, snippet_id_strings)
        )

        # Delete the enrichments themselves
        await self.enrichment_v2_repository.delete_by_query(
            QueryBuilder().filter("id", FilterOperator.IN, enrichment_ids)
        )

    async def _delete_example_enrichments(self, commit_sha: str) -> None:
        """Delete example enrichments and their indices for a commit."""
        example_enrichments = (
            await self.enrichment_query_service.get_all_examples_for_commit(commit_sha)
        )
        enrichment_ids = [
            enrichment.id for enrichment in example_enrichments if enrichment.id
        ]

        if not enrichment_ids:
            return

        # Delete from BM25 and embedding indices
        example_id_strings = [str(eid) for eid in enrichment_ids]
        delete_request = DeleteRequest(snippet_ids=example_id_strings)
        await self.bm25_service.delete_documents(delete_request)

        for example_id in enrichment_ids:
            await self.embedding_repository.delete_embeddings_by_snippet_id(
                str(example_id)
            )

        # Delete enrichment associations for examples
        await self.enrichment_association_repository.delete_by_query(
            QueryBuilder()
            .filter("entity_type", FilterOperator.EQ, "example_v2")
            .filter("entity_id", FilterOperator.IN, example_id_strings)
        )

        # Delete the enrichments themselves
        await self.enrichment_v2_repository.delete_by_query(
            QueryBuilder().filter("id", FilterOperator.IN, enrichment_ids)
        )

    async def _delete_commit_enrichments(self, commit_sha: str) -> None:
        """Delete commit-level enrichments for a commit."""
        existing_enrichment_associations = (
            await self.enrichment_association_repository.find(
                QueryBuilder()
                .filter(
                    "entity_type",
                    FilterOperator.EQ,
                    db_entities.GitCommit.__tablename__,
                )
                .filter("entity_id", FilterOperator.EQ, commit_sha)
            )
        )
        enrichment_ids = [a.enrichment_id for a in existing_enrichment_associations]
        if not enrichment_ids:
            return

        # Delete associations first
        await self.enrichment_association_repository.delete_by_query(
            QueryBuilder().filter("enrichment_id", FilterOperator.IN, enrichment_ids)
        )
        # Then delete enrichments
        await self.enrichment_v2_repository.delete_by_query(
            QueryBuilder().filter("id", FilterOperator.IN, enrichment_ids)
        )
