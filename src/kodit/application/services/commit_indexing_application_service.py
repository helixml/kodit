"""Application services for commit indexing operations."""

from typing import TYPE_CHECKING

import structlog
from pydantic import AnyUrl

if TYPE_CHECKING:
    from kodit.application.services.commit_processing_services import (
        CommitProcessingServices,
    )
    from kodit.application.services.domain_services import DomainServices
    from kodit.application.services.infrastructure_services import (
        InfrastructureServices,
    )
    from kodit.application.services.repository_management_services import (
        RepositoryManagementServices,
    )
    from kodit.application.services.repository_services import RepositoryServices
from kodit.domain.enrichments.development.snippet.snippet import (
    SnippetEnrichmentSummary,
)
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
from kodit.domain.value_objects import (
    TaskOperation,
    TrackableType,
)
from kodit.infrastructure.sqlalchemy import entities as db_entities

SUMMARIZATION_SYSTEM_PROMPT = """
You are a professional software developer. You will be given a snippet of code.
Please provide a concise explanation of the code.
"""


class CommitIndexingApplicationService:
    """Application service for commit indexing operations."""

    def __init__(
        self,
        repositories: "RepositoryServices",
        domain_services: "DomainServices",
        infrastructure: "InfrastructureServices",
        commit_processing: "CommitProcessingServices",
        repository_management: "RepositoryManagementServices",
    ) -> None:
        """Initialize the commit indexing application service."""
        # Store service bundles
        self.repositories = repositories
        self.domain_services = domain_services
        self.infrastructure = infrastructure
        self.commit_processing = commit_processing
        self.repository_management = repository_management

        # Convenience accessors from repository_management bundle
        self.repository_query_service = repository_management.query
        self.repository_lifecycle_service = repository_management.lifecycle
        self.repository_deletion_service = repository_management.deletion

        # Convenience accessors for backward compatibility
        self.repo_repository = repositories.repo
        self.git_commit_repository = repositories.git_commit
        self.git_file_repository = repositories.git_file
        self.git_branch_repository = repositories.git_branch
        self.git_tag_repository = repositories.git_tag
        self.operation = infrastructure.operation
        self.scanner = domain_services.scanner
        self.cloner = domain_services.cloner
        self.slicer = domain_services.slicer
        self.queue = infrastructure.queue
        self.bm25_service = domain_services.bm25
        self.code_search_service = domain_services.code_search
        self.text_search_service = domain_services.text_search
        self.embedding_repository = repositories.embedding
        self.architecture_service = domain_services.architecture
        self.cookbook_context_service = domain_services.cookbook_context
        self.database_schema_detector = domain_services.database_schema_detector
        self.enrichment_v2_repository = repositories.enrichment_v2
        self.enrichment_association_repository = repositories.enrichment_association
        self.enricher_service = domain_services.enricher
        self.enrichment_query_service = commit_processing.enrichment_query
        self.commit_scanning_service = commit_processing.commit_scanning
        self.snippet_extraction_service = commit_processing.snippet_extraction
        self.search_indexing_service = commit_processing.search_indexing
        self.enrichment_generation_service = commit_processing.enrichment_generation

        self._log = structlog.get_logger(__name__)

        # Create task handlers and dispatcher
        from kodit.application.services.task_dispatcher import TaskDispatcher
        from kodit.application.services.task_handlers import (
            CommitTaskHandler,
            RepositoryTaskHandler,
        )

        repository_handler = RepositoryTaskHandler(
            lifecycle_service=self.repository_lifecycle_service,
            deletion_service=self.repository_deletion_service,
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
