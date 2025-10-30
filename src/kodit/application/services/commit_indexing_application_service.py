"""Application services for commit indexing operations."""

from collections import defaultdict
from pathlib import Path
from typing import TYPE_CHECKING

import structlog
from pydantic import AnyUrl

from kodit.application.services.queue_service import QueueService
from kodit.application.services.reporting import ProgressTracker

if TYPE_CHECKING:
    from kodit.application.services.enrichment_query_service import (
        EnrichmentQueryService,
    )
from kodit.domain.enrichments.architecture.database_schema.database_schema import (
    DatabaseSchemaEnrichment,
)
from kodit.domain.enrichments.architecture.physical.physical import (
    PhysicalArchitectureEnrichment,
)
from kodit.domain.enrichments.development.snippet.snippet import (
    SnippetEnrichment,
    SnippetEnrichmentSummary,
)
from kodit.domain.enrichments.enricher import Enricher
from kodit.domain.enrichments.enrichment import (
    CommitEnrichmentAssociation,
    EnrichmentAssociation,
    EnrichmentV2,
)
from kodit.domain.enrichments.history.commit_description.commit_description import (
    CommitDescriptionEnrichment,
)
from kodit.domain.enrichments.request import (
    EnrichmentRequest as GenericEnrichmentRequest,
)
from kodit.domain.entities import Task
from kodit.domain.entities.git import GitFile, GitRepo, SnippetV2, TrackingType
from kodit.domain.factories.git_repo_factory import GitRepoFactory
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
from kodit.domain.services.embedding_service import EmbeddingDomainService
from kodit.domain.services.git_repository_service import (
    GitRepositoryScanner,
    RepositoryCloner,
)
from kodit.domain.services.physical_architecture_service import (
    ARCHITECTURE_ENRICHMENT_SYSTEM_PROMPT,
    ARCHITECTURE_ENRICHMENT_TASK_PROMPT,
    PhysicalArchitectureService,
)
from kodit.domain.value_objects import (
    DeleteRequest,
    Document,
    IndexRequest,
    LanguageMapping,
    PrescribedOperations,
    QueuePriority,
    TaskOperation,
    TrackableType,
)
from kodit.infrastructure.database_schema.database_schema_detector import (
    DatabaseSchemaDetector,
)
from kodit.infrastructure.slicing.api_doc_extractor import APIDocExtractor
from kodit.infrastructure.slicing.slicer import Slicer
from kodit.infrastructure.sqlalchemy import entities as db_entities
from kodit.infrastructure.sqlalchemy.embedding_repository import (
    SqlAlchemyEmbeddingRepository,
)
from kodit.infrastructure.sqlalchemy.entities import EmbeddingType
from kodit.infrastructure.sqlalchemy.query import (
    EnrichmentAssociationQueryBuilder,
    FilterOperator,
    GitFileQueryBuilder,
    QueryBuilder,
)

SUMMARIZATION_SYSTEM_PROMPT = """
You are a professional software developer. You will be given a snippet of code.
Please provide a concise explanation of the code.
"""

COMMIT_DESCRIPTION_SYSTEM_PROMPT = """
You are a professional software developer. You will be given a git commit diff.
Please provide a concise description of what changes were made and why.
"""

DATABASE_SCHEMA_SYSTEM_PROMPT = """
You are an expert database architect and documentation specialist.
Your task is to create clear, visual documentation of database schemas.
"""

DATABASE_SCHEMA_TASK_PROMPT = """
You will be provided with a database schema discovery report.
Please create comprehensive database schema documentation.

<schema_report>
{schema_report}
</schema_report>

**Return the following:**

## Entity List

For each table/entity, write one line:
- **[Table Name]**: [brief description of what it stores]

## Mermaid ERD

Create a Mermaid Entity Relationship Diagram showing:
- All entities (tables)
- Key relationships between entities (if apparent from names or common patterns)
- Use standard ERD notation

Example format:
```mermaid
erDiagram
    User ||--o{ Order : places
    User {
        int id PK
        string email
        string name
    }
    Order {
        int id PK
        int user_id FK
        datetime created_at
    }
```

If specific field details aren't available, show just the entity boxes and
relationships.

## Key Observations

Answer these questions in 1-2 sentences each:
1. What is the primary data model pattern (e.g., user-centric,
   event-sourced, multi-tenant)?
2. What migration strategy is being used?
3. Are there any notable database design patterns or concerns?

## Rules:
- Be concise and focus on the high-level structure
- Infer reasonable relationships from table names when explicit information
  isn't available
- If no database schema is found, state that clearly
- Keep entity descriptions to 10 words or less
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
        database_schema_detector: DatabaseSchemaDetector,
        enricher_service: Enricher,
        enrichment_v2_repository: EnrichmentV2Repository,
        enrichment_association_repository: EnrichmentAssociationRepository,
        enrichment_query_service: "EnrichmentQueryService",
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
        self.database_schema_detector = database_schema_detector
        self.enrichment_v2_repository = enrichment_v2_repository
        self.enrichment_association_repository = enrichment_association_repository
        self.enricher_service = enricher_service
        self.enrichment_query_service = enrichment_query_service
        self._log = structlog.get_logger(__name__)

    async def create_git_repository(self, remote_uri: AnyUrl) -> GitRepo:
        """Create a new Git repository."""
        async with self.operation.create_child(
            TaskOperation.CREATE_REPOSITORY,
            trackable_type=TrackableType.KODIT_REPOSITORY,
        ):
            repo = GitRepoFactory.create_from_remote_uri(remote_uri)
            repo = await self.repo_repository.save(repo)
            await self.queue.enqueue_tasks(
                tasks=PrescribedOperations.CREATE_NEW_REPOSITORY,
                base_priority=QueuePriority.USER_INITIATED,
                payload={"repository_id": repo.id},
            )
            return repo

    async def delete_git_repository(self, repo_id: int) -> bool:
        """Delete a Git repository by ID."""
        repo = await self.repo_repository.get(repo_id)
        if not repo:
            return False

        # Use the proper deletion process that handles all dependencies
        await self.process_delete_repo(repo_id)
        return True

    # TODO(Phil): Make this polymorphic
    async def run_task(self, task: Task) -> None:  # noqa: PLR0912, C901
        """Run a task."""
        if task.type.is_repository_operation():
            repo_id = task.payload["repository_id"]
            if not repo_id:
                raise ValueError("Repository ID is required")
            if task.type == TaskOperation.CLONE_REPOSITORY:
                await self.process_clone_repo(repo_id)
            elif task.type == TaskOperation.SCAN_REPOSITORY:
                await self.process_scan_repo(repo_id)
            elif task.type == TaskOperation.DELETE_REPOSITORY:
                await self.process_delete_repo(repo_id)
            else:
                raise ValueError(f"Unknown task type: {task.type}")
        elif task.type.is_commit_operation():
            repository_id = task.payload["repository_id"]
            if not repository_id:
                raise ValueError("Repository ID is required")
            commit_sha = task.payload["commit_sha"]
            if not commit_sha:
                raise ValueError("Commit SHA is required")
            if task.type == TaskOperation.EXTRACT_SNIPPETS_FOR_COMMIT:
                await self.process_snippets_for_commit(repository_id, commit_sha)
            elif task.type == TaskOperation.CREATE_BM25_INDEX_FOR_COMMIT:
                await self.process_bm25_index(repository_id, commit_sha)
            elif task.type == TaskOperation.CREATE_CODE_EMBEDDINGS_FOR_COMMIT:
                await self.process_code_embeddings(repository_id, commit_sha)
            elif task.type == TaskOperation.CREATE_SUMMARY_ENRICHMENT_FOR_COMMIT:
                await self.process_enrich(repository_id, commit_sha)
            elif task.type == TaskOperation.CREATE_SUMMARY_EMBEDDINGS_FOR_COMMIT:
                await self.process_summary_embeddings(repository_id, commit_sha)
            elif task.type == TaskOperation.CREATE_ARCHITECTURE_ENRICHMENT_FOR_COMMIT:
                await self.process_architecture_discovery(repository_id, commit_sha)
            elif task.type == TaskOperation.CREATE_PUBLIC_API_DOCS_FOR_COMMIT:
                await self.process_api_docs(repository_id, commit_sha)
            elif task.type == TaskOperation.CREATE_COMMIT_DESCRIPTION_FOR_COMMIT:
                await self.process_commit_description(repository_id, commit_sha)
            elif task.type == TaskOperation.CREATE_DATABASE_SCHEMA_FOR_COMMIT:
                await self.process_database_schema(repository_id, commit_sha)
            else:
                raise ValueError(f"Unknown task type: {task.type}")
        else:
            raise ValueError(f"Unknown task type: {task.type}")

    async def process_clone_repo(self, repository_id: int) -> None:
        """Clone a repository."""
        async with self.operation.create_child(
            TaskOperation.CLONE_REPOSITORY,
            trackable_type=TrackableType.KODIT_REPOSITORY,
            trackable_id=repository_id,
        ):
            repo = await self.repo_repository.get(repository_id)
            repo.cloned_path = await self.cloner.clone_repository(repo.remote_uri)
            await self.repo_repository.save(repo)

    async def process_scan_repo(self, repository_id: int) -> None:
        """Scan a repository."""
        async with self.operation.create_child(
            TaskOperation.SCAN_REPOSITORY,
            trackable_type=TrackableType.KODIT_REPOSITORY,
            trackable_id=repository_id,
        ) as step:
            await step.set_total(7)
            repo = await self.repo_repository.get(repository_id)
            if not repo.cloned_path:
                raise ValueError(f"Repository {repository_id} has never been cloned")

            # Scan the repository to get all metadata
            await step.set_current(0, "Scanning repository")
            scan_result = await self.scanner.scan_repository(
                repo.cloned_path, repository_id
            )

            # Update repo with scan result (this sets num_commits, num_branches, etc.)
            await step.set_current(1, "Updating repository with scan result")
            repo.update_with_scan_result(scan_result)
            await self.repo_repository.save(repo)

            await step.set_current(2, "Saving commits")
            await self.git_commit_repository.save_bulk(scan_result.all_commits)

            await step.set_current(3, "Saving files")
            await self.git_file_repository.save_bulk(scan_result.all_files)

            await step.set_current(4, "Saving branches")
            if scan_result.branches:
                await self.git_branch_repository.save_bulk(
                    scan_result.branches,
                )

            await step.set_current(5, "Saving tags")
            if scan_result.all_tags:
                await self.git_tag_repository.save_bulk(
                    scan_result.all_tags,
                )

            await step.set_current(6, "Enqueuing commit indexing tasks")
            if not repo.tracking_config.name:
                raise ValueError(f"Repository {repository_id} has no tracking branch")
            if repo.tracking_config.type == TrackingType.BRANCH.value:
                branch = await self.git_branch_repository.get_by_name(
                    repo.tracking_config.name, repository_id
                )
                commit_sha = branch.head_commit_sha
            elif repo.tracking_config.type == TrackingType.TAG.value:
                tag = await self.git_tag_repository.get_by_name(
                    repo.tracking_config.name, repository_id
                )
                commit_sha = tag.target_commit_sha
            elif repo.tracking_config.type == TrackingType.COMMIT_SHA.value:
                commit_sha = repo.tracking_config.name
            else:
                raise ValueError(f"Unknown tracking type: {repo.tracking_config.type}")

            await self.queue.enqueue_tasks(
                tasks=PrescribedOperations.INDEX_COMMIT,
                base_priority=QueuePriority.USER_INITIATED,
                payload={"commit_sha": commit_sha, "repository_id": repository_id},
            )

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

    async def process_delete_repo(self, repository_id: int) -> None:
        """Delete a repository."""
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

    async def process_snippets_for_commit(
        self, repository_id: int, commit_sha: str
    ) -> None:
        """Generate snippets for a repository."""
        async with self.operation.create_child(
            operation=TaskOperation.EXTRACT_SNIPPETS_FOR_COMMIT,
            trackable_type=TrackableType.KODIT_REPOSITORY,
            trackable_id=repository_id,
        ) as step:
            # Find existing snippet enrichments for this commit
            if await self.enrichment_query_service.has_snippets_for_commit(commit_sha):
                await step.skip("Snippets already extracted for commit")
                return

            commit = await self.git_commit_repository.get(commit_sha)

            # Load files on demand for snippet extraction (performance optimization)
            # Instead of using commit.files (which may be empty), load files directly
            repo = await self.repo_repository.get(repository_id)
            if not repo.cloned_path:
                raise ValueError(f"Repository {repository_id} has never been cloned")

            files_data = await self.scanner.git_adapter.get_commit_file_data(
                repo.cloned_path, commit_sha
            )

            # Create GitFile entities with absolute paths for the slicer
            files = []
            for file_data in files_data:
                # Extract extension from file path
                file_path = Path(file_data["path"])
                extension = file_path.suffix.lstrip(".")

                # Create absolute path for the slicer to read
                absolute_path = str(repo.cloned_path / file_data["path"])

                git_file = GitFile(
                    commit_sha=commit.commit_sha,
                    created_at=file_data.get("created_at", commit.date),
                    blob_sha=file_data["blob_sha"],
                    path=absolute_path,  # Use absolute path for file reading
                    mime_type=file_data.get("mime_type", "application/octet-stream"),
                    size=file_data.get("size", 0),
                    extension=extension,
                )
                files.append(git_file)

            # Create a set of languages to extract snippets for
            extensions = {file.extension for file in files}
            lang_files_map: dict[str, list[GitFile]] = defaultdict(list)
            for ext in extensions:
                try:
                    lang = LanguageMapping.get_language_for_extension(ext)
                    lang_files_map[lang].extend(
                        file for file in files if file.extension == ext
                    )
                except ValueError as e:
                    self._log.debug("Skipping", error=str(e))
                    continue

            # Extract snippets
            all_snippets: list[SnippetV2] = []
            slicer = Slicer()
            await step.set_total(len(lang_files_map.keys()))
            for i, (lang, lang_files) in enumerate(lang_files_map.items()):
                await step.set_current(i, f"Extracting snippets for {lang}")
                snippets = slicer.extract_snippets_from_git_files(
                    lang_files, language=lang
                )
                all_snippets.extend(snippets)

            # Deduplicate snippets by SHA before saving to prevent constraint violations
            unique_snippets: dict[str, SnippetV2] = {}
            for snippet in all_snippets:
                unique_snippets[snippet.sha] = snippet

            deduplicated_snippets = list(unique_snippets.values())

            commit_short = commit.commit_sha[:8]
            self._log.info(
                f"Extracted {len(all_snippets)} snippets, "
                f"deduplicated to {len(deduplicated_snippets)} for {commit_short}"
            )

            saved_enrichments = await self.enrichment_v2_repository.save_bulk(
                [
                    SnippetEnrichment(content=snippet.content)
                    for snippet in deduplicated_snippets
                ]
            )
            saved_associations = await self.enrichment_association_repository.save_bulk(
                [
                    EnrichmentAssociation(
                        enrichment_id=enrichment.id,
                        entity_type=db_entities.GitCommit.__tablename__,
                        entity_id=commit_sha,
                    )
                    for enrichment in saved_enrichments
                    if enrichment.id
                ]
            )
            self._log.info(
                f"Saved {len(saved_enrichments)} snippet enrichments and "
                f"{len(saved_associations)} associations for commit {commit_sha}"
            )

    async def process_bm25_index(self, repository_id: int, commit_sha: str) -> None:
        """Handle BM25_INDEX task - create keyword index."""
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

    async def process_code_embeddings(
        self, repository_id: int, commit_sha: str
    ) -> None:
        """Handle CODE_EMBEDDINGS task - create code embeddings."""
        async with self.operation.create_child(
            TaskOperation.CREATE_CODE_EMBEDDINGS_FOR_COMMIT,
            trackable_type=TrackableType.KODIT_REPOSITORY,
            trackable_id=repository_id,
        ) as step:
            existing_enrichments = (
                await self.enrichment_query_service.get_all_snippets_for_commit(
                    commit_sha
                )
            )

            new_snippets = await self._new_snippets_for_type(
                existing_enrichments, EmbeddingType.CODE
            )
            if not new_snippets:
                await step.skip("All snippets already have code embeddings")
                return

            await step.set_total(len(new_snippets))
            processed = 0
            documents = [
                Document(snippet_id=str(snippet.id), text=snippet.content)
                for snippet in new_snippets
                if snippet.id
            ]
            async for result in self.code_search_service.index_documents(
                IndexRequest(documents=documents)
            ):
                processed += len(result)
                await step.set_current(processed, "Creating code embeddings for commit")

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

    async def process_architecture_discovery(
        self, repository_id: int, commit_sha: str
    ) -> None:
        """Handle ARCHITECTURE_DISCOVERY task - discover physical architecture."""
        async with self.operation.create_child(
            TaskOperation.CREATE_ARCHITECTURE_ENRICHMENT_FOR_COMMIT,
            trackable_type=TrackableType.KODIT_REPOSITORY,
            trackable_id=repository_id,
        ) as step:
            await step.set_total(3)

            # Check if architecture enrichment already exists for this commit
            if await self.enrichment_query_service.has_architecture_for_commit(
                commit_sha
            ):
                await step.skip("Architecture enrichment already exists for commit")
                return

            # Get repository path
            repo = await self.repo_repository.get(repository_id)
            if not repo.cloned_path:
                raise ValueError(f"Repository {repository_id} has never been cloned")

            await step.set_current(1, "Discovering physical architecture")

            # Discover architecture
            architecture_narrative = (
                await self.architecture_service.discover_architecture(repo.cloned_path)
            )

            await step.set_current(2, "Enriching architecture notes with LLM")

            # Enrich the architecture narrative through the enricher
            enrichment_request = GenericEnrichmentRequest(
                id=commit_sha,
                text=ARCHITECTURE_ENRICHMENT_TASK_PROMPT.format(
                    architecture_narrative=architecture_narrative,
                ),
                system_prompt=ARCHITECTURE_ENRICHMENT_SYSTEM_PROMPT,
            )

            enriched_content = ""
            async for response in self.enricher_service.enrich([enrichment_request]):
                enriched_content = response.text

            # Create and save architecture enrichment with enriched content
            enrichment = await self.enrichment_v2_repository.save(
                PhysicalArchitectureEnrichment(
                    content=enriched_content,
                )
            )
            if not enrichment or not enrichment.id:
                raise ValueError(
                    f"Failed to save architecture enrichment for commit {commit_sha}"
                )
            await self.enrichment_association_repository.save(
                CommitEnrichmentAssociation(
                    enrichment_id=enrichment.id,
                    entity_id=commit_sha,
                )
            )

            await step.set_current(3, "Architecture enrichment completed")

    async def process_api_docs(self, repository_id: int, commit_sha: str) -> None:
        """Handle API_DOCS task - generate API documentation."""
        async with self.operation.create_child(
            TaskOperation.CREATE_PUBLIC_API_DOCS_FOR_COMMIT,
            trackable_type=TrackableType.KODIT_REPOSITORY,
            trackable_id=repository_id,
        ) as step:
            # Check if API docs already exist for this commit
            if await self.enrichment_query_service.has_api_docs_for_commit(commit_sha):
                await step.skip("API docs already exist for commit")
                return

            # Get repository for metadata
            repo = await self.repo_repository.get(repository_id)
            if not repo:
                raise ValueError(f"Repository {repository_id} not found")
            str(repo.sanitized_remote_uri)

            files = await self.git_file_repository.find(
                GitFileQueryBuilder().for_commit_sha(commit_sha)
            )
            if not files:
                await step.skip("No files to extract API docs from")
                return

            # Group files by language
            lang_files_map: dict[str, list[GitFile]] = defaultdict(list)
            for file in files:
                try:
                    lang = LanguageMapping.get_language_for_extension(file.extension)
                except ValueError:
                    continue
                lang_files_map[lang].append(file)

            all_enrichments = []
            extractor = APIDocExtractor()

            await step.set_total(len(lang_files_map))
            for i, (lang, lang_files) in enumerate(lang_files_map.items()):
                await step.set_current(i, f"Extracting API docs for {lang}")
                enrichments = extractor.extract_api_docs(
                    files=lang_files,
                    language=lang,
                    include_private=False,
                )
                all_enrichments.extend(enrichments)

            # Save all enrichments
            if all_enrichments:
                saved_enrichments = await self.enrichment_v2_repository.save_bulk(
                    all_enrichments  # type: ignore[arg-type]
                )
                await self.enrichment_association_repository.save_bulk(
                    [
                        CommitEnrichmentAssociation(
                            enrichment_id=enrichment.id,
                            entity_id=commit_sha,
                        )
                        for enrichment in saved_enrichments
                        if enrichment.id
                    ]
                )

    async def process_commit_description(
        self, repository_id: int, commit_sha: str
    ) -> None:
        """Handle COMMIT_DESCRIPTION task - generate commit descriptions."""
        async with self.operation.create_child(
            TaskOperation.CREATE_COMMIT_DESCRIPTION_FOR_COMMIT,
            trackable_type=TrackableType.KODIT_REPOSITORY,
            trackable_id=repository_id,
        ) as step:
            # Check if commit description already exists for this commit
            if await self.enrichment_query_service.has_commit_description_for_commit(
                commit_sha
            ):
                await step.skip("Commit description already exists for commit")
                return

            # Get repository path
            repo = await self.repo_repository.get(repository_id)
            if not repo.cloned_path:
                raise ValueError(f"Repository {repository_id} has never been cloned")

            await step.set_total(3)
            await step.set_current(1, "Getting commit diff")

            # Get the diff for this commit
            diff = await self.scanner.git_adapter.get_commit_diff(
                repo.cloned_path, commit_sha
            )

            if not diff or len(diff.strip()) == 0:
                await step.skip("No diff found for commit")
                return

            await step.set_current(2, "Enriching commit description with LLM")

            # Enrich the diff through the enricher
            enrichment_request = GenericEnrichmentRequest(
                id=commit_sha,
                text=diff,
                system_prompt=COMMIT_DESCRIPTION_SYSTEM_PROMPT,
            )

            enriched_content = ""
            async for response in self.enricher_service.enrich([enrichment_request]):
                enriched_content = response.text

            # Create and save commit description enrichment
            enrichment = await self.enrichment_v2_repository.save(
                CommitDescriptionEnrichment(
                    content=enriched_content,
                )
            )
            if not enrichment or not enrichment.id:
                raise ValueError(
                    f"Failed to save commit description enrichment for commit "
                    f"{commit_sha}"
                )
            await self.enrichment_association_repository.save(
                CommitEnrichmentAssociation(
                    enrichment_id=enrichment.id,
                    entity_id=commit_sha,
                )
            )

            await step.set_current(3, "Commit description enrichment completed")

    async def process_database_schema(
        self, repository_id: int, commit_sha: str
    ) -> None:
        """Handle DATABASE_SCHEMA task - discover and document database schemas."""
        async with self.operation.create_child(
            TaskOperation.CREATE_DATABASE_SCHEMA_FOR_COMMIT,
            trackable_type=TrackableType.KODIT_REPOSITORY,
            trackable_id=repository_id,
        ) as step:
            # Check if database schema already exists for this commit
            if await self.enrichment_query_service.has_database_schema_for_commit(
                commit_sha
            ):
                await step.skip("Database schema already exists for commit")
                return

            # Get repository path
            repo = await self.repo_repository.get(repository_id)
            if not repo.cloned_path:
                raise ValueError(f"Repository {repository_id} has never been cloned")

            await step.set_total(3)
            await step.set_current(1, "Discovering database schemas")

            # Discover database schemas
            schema_report = await self.database_schema_detector.discover_schemas(
                repo.cloned_path
            )

            if "No database schemas detected" in schema_report:
                await step.skip("No database schemas found in repository")
                return

            await step.set_current(2, "Enriching schema documentation with LLM")

            # Enrich the schema report through the enricher
            enrichment_request = GenericEnrichmentRequest(
                id=commit_sha,
                text=DATABASE_SCHEMA_TASK_PROMPT.format(schema_report=schema_report),
                system_prompt=DATABASE_SCHEMA_SYSTEM_PROMPT,
            )

            enriched_content = ""
            async for response in self.enricher_service.enrich([enrichment_request]):
                enriched_content = response.text

            # Create and save database schema enrichment
            enrichment = await self.enrichment_v2_repository.save(
                DatabaseSchemaEnrichment(
                    content=enriched_content,
                )
            )
            if not enrichment or not enrichment.id:
                raise ValueError(
                    f"Failed to save database schema enrichment for commit {commit_sha}"
                )
            await self.enrichment_association_repository.save(
                CommitEnrichmentAssociation(
                    enrichment_id=enrichment.id,
                    entity_id=commit_sha,
                )
            )

            await step.set_current(3, "Database schema enrichment completed")

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
