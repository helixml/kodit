"""Application services for commit indexing operations."""

from collections import defaultdict
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
from kodit.infrastructure.sqlalchemy.query import FilterOperator, QueryBuilder

if TYPE_CHECKING:
    from kodit.application.services.commit_scanning_service import (
        CommitScanningService,
    )
    from kodit.application.services.enrichment_query_service import (
        EnrichmentQueryService,
    )
    from kodit.application.services.repository_query_service import (
        RepositoryQueryService,
    )
    from kodit.application.services.snippet_extraction_service import (
        SnippetExtractionService,
    )
from kodit.domain.enrichments.architecture.database_schema.database_schema import (
    DatabaseSchemaEnrichment,
)
from kodit.domain.enrichments.architecture.physical.physical import (
    PhysicalArchitectureEnrichment,
)
from kodit.domain.enrichments.development.snippet.snippet import (
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
from kodit.domain.enrichments.usage.cookbook import CookbookEnrichment
from kodit.domain.entities import Task
from kodit.domain.entities.git import (
    GitFile,
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
    COOKBOOK_SYSTEM_PROMPT,
    COOKBOOK_TASK_PROMPT,
    CookbookContextService,
)
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
    Document,
    IndexRequest,
    LanguageMapping,
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
    GitFileQueryBuilder,
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
    User ||--o{{ Order : places
    User {{
        int id PK
        string email
        string name
    }}
    Order {{
        int id PK
        int user_id FK
        datetime created_at
    }}
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

    async def process_cookbook(self, repository_id: int, commit_sha: str) -> None:
        """Handle COOKBOOK task - generate usage cookbook examples."""
        async with self.operation.create_child(
            TaskOperation.CREATE_COOKBOOK_FOR_COMMIT,
            trackable_type=TrackableType.KODIT_REPOSITORY,
            trackable_id=repository_id,
        ) as step:
            # Check if cookbook already exists for this commit
            if await self.enrichment_query_service.has_cookbook_for_commit(commit_sha):
                await step.skip("Cookbook already exists for commit")
                return

            # Get repository path
            repo = await self.repo_repository.get(repository_id)
            if not repo.cloned_path:
                raise ValueError(f"Repository {repository_id} has never been cloned")

            await step.set_total(4)
            await step.set_current(1, "Getting files for cookbook generation")

            # Get files for the commit
            files = await self.git_file_repository.find(
                GitFileQueryBuilder().for_commit_sha(commit_sha)
            )
            if not files:
                await step.skip("No files to generate cookbook from")
                return

            # Group files by language and find primary language
            lang_files_map: dict[str, list[GitFile]] = defaultdict(list)
            for file in files:
                try:
                    lang = LanguageMapping.get_language_for_extension(file.extension)
                except ValueError:
                    continue
                lang_files_map[lang].append(file)

            if not lang_files_map:
                await step.skip("No supported languages found for cookbook")
                return

            # Use the language with the most files as primary
            primary_lang = max(lang_files_map.items(), key=lambda x: len(x[1]))[0]
            primary_lang_files = lang_files_map[primary_lang]

            await step.set_current(2, f"Parsing {primary_lang} code with AST")

            # Parse API structure using AST analyzer
            api_modules = None
            try:
                from kodit.infrastructure.slicing.ast_analyzer import ASTAnalyzer

                analyzer = ASTAnalyzer(primary_lang)
                parsed_files = analyzer.parse_files(primary_lang_files)
                api_modules = analyzer.extract_module_definitions(
                    parsed_files, include_private=False
                )
                # Filter out test modules
                api_modules = [
                    m
                    for m in api_modules
                    if not self._is_test_module_path(m.module_path)
                ]
            except (ValueError, Exception) as e:
                self._log.debug(
                    "Could not parse API structure, continuing without it",
                    language=primary_lang,
                    error=str(e),
                )

            await step.set_current(3, "Gathering repository context for cookbook")

            # Gather context for cookbook generation
            repository_context = await self.cookbook_context_service.gather_context(
                repo.cloned_path, language=primary_lang, api_modules=api_modules
            )

            await step.set_current(4, "Generating cookbook examples with LLM")

            # Generate cookbook through the enricher
            enrichment_request = GenericEnrichmentRequest(
                id=commit_sha,
                text=COOKBOOK_TASK_PROMPT.format(repository_context=repository_context),
                system_prompt=COOKBOOK_SYSTEM_PROMPT,
            )

            enriched_content = ""
            async for response in self.enricher_service.enrich([enrichment_request]):
                enriched_content = response.text

            # Create and save cookbook enrichment
            enrichment = await self.enrichment_v2_repository.save(
                CookbookEnrichment(
                    content=enriched_content,
                )
            )
            if not enrichment or not enrichment.id:
                raise ValueError(
                    f"Failed to save cookbook enrichment for commit {commit_sha}"
                )
            await self.enrichment_association_repository.save(
                CommitEnrichmentAssociation(
                    enrichment_id=enrichment.id,
                    entity_id=commit_sha,
                )
            )

    def _is_test_module_path(self, module_path: str) -> bool:
        """Check if a module path appears to be a test module."""
        module_path_lower = module_path.lower()
        test_indicators = ["test", "tests", "__tests__", "_test", "spec"]
        return any(indicator in module_path_lower for indicator in test_indicators)

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
