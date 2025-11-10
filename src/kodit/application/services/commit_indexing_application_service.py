"""Application services for commit indexing operations."""

from typing import TYPE_CHECKING

import structlog
from pydantic import AnyUrl

from kodit.application.services.queue_service import QueueService
from kodit.application.services.reporting import ProgressTracker
from kodit.application.services.repository_deletion_service import (
    RepositoryDeletionService,
)
from kodit.application.services.repository_lifecycle_service import (
    RepositoryLifecycleService,
)

if TYPE_CHECKING:
    from kodit.application.services.commit_scanning_service import (
        CommitScanningService,
    )
    from kodit.application.services.enrichment_generation_service import (
        EnrichmentGenerationService,
    )
    from kodit.application.services.enrichment_query_service import (
        EnrichmentQueryService,
    )
    from kodit.application.services.repository_query_service import (
        RepositoryQueryService,
    )
    from kodit.application.services.search_indexing_service import (
        SearchIndexingService,
    )
    from kodit.application.services.snippet_extraction_service import (
        SnippetExtractionService,
    )
from kodit.domain.enrichments.development.snippet.snippet import (
    SnippetEnrichmentSummary,
)
from kodit.domain.enrichments.enricher import Enricher
from kodit.domain.enrichments.enrichment import (
    EnrichmentAssociation,
)
from kodit.domain.enrichments.request import (
    EnrichmentRequest as GenericEnrichmentRequest,
)
from kodit.domain.entities import Task
from kodit.domain.entities.git import (
    GitRepo,
)
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
from kodit.domain.services.cookbook_context_service import (
    CookbookContextService,
)
from kodit.domain.services.embedding_service import EmbeddingDomainService
from kodit.domain.services.git_repository_service import (
    GitRepositoryScanner,
    RepositoryCloner,
)
from kodit.domain.services.physical_architecture_service import (
    PhysicalArchitectureService,
)
from kodit.domain.value_objects import (
    TaskOperation,
    TrackableType,
)
from kodit.infrastructure.database_schema.database_schema_detector import (
    DatabaseSchemaDetector,
)
from kodit.infrastructure.slicing.slicer import Slicer
from kodit.infrastructure.sqlalchemy import entities as db_entities
from kodit.infrastructure.sqlalchemy.embedding_repository import (
    SqlAlchemyEmbeddingRepository,
)

SUMMARIZATION_SYSTEM_PROMPT = """
You are a professional software developer. You will be given a snippet of code.
Please provide a concise explanation of the code.
"""


class CommitIndexingApplicationService:
    """Application service for commit indexing operations."""

    def __init__(  # noqa: PLR0913
        self,
        repo_repository: GitRepoRepository,
        git_commit_repository: GitCommitRepository,
        git_file_repository: GitFileRepository,
        git_branch_repository: GitBranchRepository,
        git_tag_repository: GitTagRepository,
        operation: ProgressTracker,
        scanner: GitRepositoryScanner,
        cloner: RepositoryCloner,
        slicer: Slicer,
        queue: QueueService,
        bm25_service: BM25DomainService,
        code_search_service: EmbeddingDomainService,
        text_search_service: EmbeddingDomainService,
        embedding_repository: SqlAlchemyEmbeddingRepository,
        architecture_service: PhysicalArchitectureService,
        cookbook_context_service: CookbookContextService,
        database_schema_detector: DatabaseSchemaDetector,
        enricher_service: Enricher,
        enrichment_v2_repository: EnrichmentV2Repository,
        enrichment_association_repository: EnrichmentAssociationRepository,
        enrichment_query_service: "EnrichmentQueryService",
        repository_query_service: "RepositoryQueryService",
        repository_lifecycle_service: RepositoryLifecycleService,
        repository_deletion_service: RepositoryDeletionService,
        commit_scanning_service: "CommitScanningService",
        snippet_extraction_service: "SnippetExtractionService",
        search_indexing_service: "SearchIndexingService",
        enrichment_generation_service: "EnrichmentGenerationService",
    ) -> None:
        """Initialize the commit indexing application service."""
        self.repo_repository = repo_repository
        self.git_commit_repository = git_commit_repository
        self.git_file_repository = git_file_repository
        self.git_branch_repository = git_branch_repository
        self.git_tag_repository = git_tag_repository
        self.operation = operation
        self.scanner = scanner
        self.cloner = cloner
        self.slicer = slicer
        self.queue = queue
        self.bm25_service = bm25_service
        self.code_search_service = code_search_service
        self.text_search_service = text_search_service
        self.embedding_repository = embedding_repository
        self.architecture_service = architecture_service
        self.cookbook_context_service = cookbook_context_service
        self.database_schema_detector = database_schema_detector
        self.enrichment_v2_repository = enrichment_v2_repository
        self.enrichment_association_repository = enrichment_association_repository
        self.enricher_service = enricher_service
        self.enrichment_query_service = enrichment_query_service
        self.repository_query_service = repository_query_service
        self.repository_lifecycle_service = repository_lifecycle_service
        self.repository_deletion_service = repository_deletion_service
        self.commit_scanning_service = commit_scanning_service
        self.snippet_extraction_service = snippet_extraction_service
        self.search_indexing_service = search_indexing_service
        self.enrichment_generation_service = enrichment_generation_service
        self._log = structlog.get_logger(__name__)

        # Create task handlers and dispatcher
        from kodit.application.services.task_dispatcher import TaskDispatcher
        from kodit.application.services.task_handlers import (
            CommitTaskHandler,
            RepositoryTaskHandler,
        )

        repository_handler = RepositoryTaskHandler(
            lifecycle_service=repository_lifecycle_service,
            deletion_service=repository_deletion_service,
        )
        commit_handler = CommitTaskHandler(commit_service=self)
        self.task_dispatcher = TaskDispatcher(
            repository_handler=repository_handler,
            commit_handler=commit_handler,
        )

    async def create_git_repository(self, remote_uri: AnyUrl) -> tuple[GitRepo, bool]:
        """Create a new Git repository or get existing one.

        Returns tuple of (repository, created) where created is True if new.
        """
        return await self.repository_lifecycle_service.create_or_get_repository(
            remote_uri
        )

    async def delete_git_repository(self, repo_id: int) -> bool:
        """Delete a Git repository by ID."""
        repo = await self.repo_repository.get(repo_id)
        if not repo:
            return False

        await self.repository_deletion_service.delete_repository(repo_id)
        return True

    async def run_task(self, task: Task) -> None:
        """Run a task by dispatching to appropriate handler."""
        await self.task_dispatcher.dispatch(task)

    async def process_clone_repo(self, repository_id: int) -> None:
        """Clone a repository and enqueue head commit scan."""
        await self.repository_lifecycle_service.clone_repository(repository_id)

    async def process_sync_repo(self, repository_id: int) -> None:
        """Sync a repository by pulling and scanning head commit if changed."""
        await self.repository_lifecycle_service.sync_repository(repository_id)

    async def process_scan_commit(self, repository_id: int, commit_sha: str) -> None:
        """Scan a specific commit and save to database."""
        await self.commit_scanning_service.scan_commit(repository_id, commit_sha)

    async def process_delete_repo(self, repository_id: int) -> None:
        """Delete a repository."""
        await self.repository_deletion_service.delete_repository(repository_id)

    async def process_snippets_for_commit(
        self, repository_id: int, commit_sha: str
    ) -> None:
        """Generate snippets for a repository."""
        # Check if snippets already exist
        if await self.enrichment_query_service.has_snippets_for_commit(commit_sha):
            async with self.operation.create_child(
                operation=TaskOperation.EXTRACT_SNIPPETS_FOR_COMMIT,
                trackable_type=TrackableType.KODIT_REPOSITORY,
                trackable_id=repository_id,
            ) as step:
                await step.skip("Snippets already extracted for commit")
            return

        # Delegate to snippet extraction service
        await self.snippet_extraction_service.extract_snippets(
            repository_id, commit_sha
        )

    async def process_bm25_index(self, repository_id: int, commit_sha: str) -> None:
        """Handle BM25_INDEX task - create keyword index."""
        await self.search_indexing_service.create_bm25_index(repository_id, commit_sha)

    async def process_code_embeddings(
        self, repository_id: int, commit_sha: str
    ) -> None:
        """Handle CODE_EMBEDDINGS task - create code embeddings."""
        await self.search_indexing_service.create_code_embeddings(
            repository_id, commit_sha
        )

    async def process_enrich(self, repository_id: int, commit_sha: str) -> None:
        """Handle ENRICH task - enrich snippets and create text embeddings."""
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

    async def process_summary_embeddings(
        self, repository_id: int, commit_sha: str
    ) -> None:
        """Handle SUMMARY_EMBEDDINGS task - create summary embeddings."""
        await self.search_indexing_service.create_summary_embeddings(
            repository_id, commit_sha
        )

    async def process_architecture_discovery(
        self, repository_id: int, commit_sha: str
    ) -> None:
        """Handle ARCHITECTURE_DISCOVERY task - discover physical architecture."""
        await self.enrichment_generation_service.create_architecture_enrichment(
            repository_id, commit_sha
        )

    async def process_api_docs(self, repository_id: int, commit_sha: str) -> None:
        """Handle API_DOCS task - generate API documentation."""
        await self.enrichment_generation_service.create_api_docs(
            repository_id, commit_sha
        )

    async def process_commit_description(
        self, repository_id: int, commit_sha: str
    ) -> None:
        """Handle COMMIT_DESCRIPTION task - generate commit descriptions."""
        await self.enrichment_generation_service.create_commit_description(
            repository_id, commit_sha
        )

    async def process_database_schema(
        self, repository_id: int, commit_sha: str
    ) -> None:
        """Handle DATABASE_SCHEMA task - discover and document database schemas."""
        await self.enrichment_generation_service.create_database_schema(
            repository_id, commit_sha
        )

    async def process_cookbook(self, repository_id: int, commit_sha: str) -> None:
        """Handle COOKBOOK task - generate usage cookbook examples."""
        await self.enrichment_generation_service.create_cookbook(
            repository_id, commit_sha
        )
