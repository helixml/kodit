"""Handler for deleting a repository."""

from typing import TYPE_CHECKING, Any

from kodit.application.services.reporting import ProgressTracker
from kodit.domain.protocols import (
    EnrichmentAssociationRepository,
    EnrichmentV2Repository,
    GitBranchRepository,
    GitCommitRepository,
    GitFileRepository,
    GitRepoRepository,
    GitTagRepository,
)
from kodit.domain.services.bm25_service import BM25DomainService
from kodit.domain.value_objects import DeleteRequest, TaskOperation, TrackableType
from kodit.infrastructure.sqlalchemy import entities as db_entities
from kodit.infrastructure.sqlalchemy.embedding_repository import (
    SqlAlchemyEmbeddingRepository,
)
from kodit.infrastructure.sqlalchemy.query import FilterOperator, QueryBuilder

if TYPE_CHECKING:
    from kodit.application.services.enrichment_query_service import (
        EnrichmentQueryService,
    )


class DeleteRepositoryHandler:
    """Handler for deleting a repository."""

    def __init__(  # noqa: PLR0913
        self,
        repo_repository: GitRepoRepository,
        git_commit_repository: GitCommitRepository,
        git_file_repository: GitFileRepository,
        git_branch_repository: GitBranchRepository,
        git_tag_repository: GitTagRepository,
        bm25_service: BM25DomainService,
        embedding_repository: SqlAlchemyEmbeddingRepository,
        enrichment_v2_repository: EnrichmentV2Repository,
        enrichment_association_repository: EnrichmentAssociationRepository,
        enrichment_query_service: "EnrichmentQueryService",
        operation: ProgressTracker,
    ) -> None:
        """Initialize the delete repository handler."""
        self.repo_repository = repo_repository
        self.git_commit_repository = git_commit_repository
        self.git_file_repository = git_file_repository
        self.git_branch_repository = git_branch_repository
        self.git_tag_repository = git_tag_repository
        self.bm25_service = bm25_service
        self.embedding_repository = embedding_repository
        self.enrichment_v2_repository = enrichment_v2_repository
        self.enrichment_association_repository = enrichment_association_repository
        self.enrichment_query_service = enrichment_query_service
        self.operation = operation

    async def execute(self, payload: dict[str, Any]) -> None:
        """Execute delete repository operation."""
        repository_id = payload["repository_id"]

        async with self.operation.create_child(
            TaskOperation.DELETE_REPOSITORY,
            trackable_type=TrackableType.KODIT_REPOSITORY,
            trackable_id=repository_id,
        ):
            repo = await self.repo_repository.get(repository_id)
            if not repo:
                raise ValueError(f"Repository {repository_id} not found")

            # Get all commit SHAs for this repository
            commits = await self.git_commit_repository.find(
                QueryBuilder().filter("repo_id", FilterOperator.EQ, repository_id)
            )
            commit_shas = [commit.commit_sha for commit in commits]

            # Delete all enrichments and their indices
            if commit_shas:
                await self._delete_snippet_enrichments_for_commits(commit_shas)
                await self._delete_commit_enrichments(commit_shas)

            # Delete branches, tags, files, commits, and repository
            await self.git_branch_repository.delete_by_repo_id(repository_id)
            await self.git_tag_repository.delete_by_repo_id(repository_id)

            for commit_sha in commit_shas:
                await self.git_file_repository.delete_by_commit_sha(commit_sha)

            await self.git_commit_repository.delete_by_query(
                QueryBuilder().filter("repo_id", FilterOperator.EQ, repository_id)
            )

            if repo.id:
                await self.repo_repository.delete(repo)

    async def _delete_snippet_enrichments_for_commits(
        self, commit_shas: list[str]
    ) -> None:
        """Delete snippet enrichments and their indices for commits."""
        # Get all snippet enrichment IDs for these commits
        all_snippet_enrichment_ids = []
        for commit_sha in commit_shas:
            snippet_enrichments = (
                await self.enrichment_query_service.get_all_snippets_for_commit(
                    commit_sha
                )
            )
            enrichment_ids = [
                enrichment.id for enrichment in snippet_enrichments if enrichment.id
            ]
            all_snippet_enrichment_ids.extend(enrichment_ids)

        if not all_snippet_enrichment_ids:
            return

        # Delete from BM25 and embedding indices
        snippet_id_strings = [str(sid) for sid in all_snippet_enrichment_ids]
        delete_request = DeleteRequest(snippet_ids=snippet_id_strings)
        await self.bm25_service.delete_documents(delete_request)

        for snippet_id in all_snippet_enrichment_ids:
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
            QueryBuilder().filter("id", FilterOperator.IN, all_snippet_enrichment_ids)
        )

    async def _delete_commit_enrichments(self, commit_shas: list[str]) -> None:
        """Delete commit-level enrichments for commits."""
        existing_enrichment_associations = (
            await self.enrichment_association_repository.find(
                QueryBuilder()
                .filter(
                    "entity_type",
                    FilterOperator.EQ,
                    db_entities.GitCommit.__tablename__,
                )
                .filter("entity_id", FilterOperator.IN, commit_shas)
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
