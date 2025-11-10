"""Service for generating various types of enrichments for commits."""

from collections import defaultdict
from typing import TYPE_CHECKING

import structlog

from kodit.application.services.reporting import ProgressTracker
from kodit.domain.enrichments.architecture.database_schema.database_schema import (
    DatabaseSchemaEnrichment,
)
from kodit.domain.enrichments.architecture.physical.physical import (
    PhysicalArchitectureEnrichment,
)
from kodit.domain.enrichments.enricher import Enricher
from kodit.domain.enrichments.enrichment import CommitEnrichmentAssociation
from kodit.domain.enrichments.history.commit_description.commit_description import (
    CommitDescriptionEnrichment,
)
from kodit.domain.enrichments.request import (
    EnrichmentRequest as GenericEnrichmentRequest,
)
from kodit.domain.enrichments.usage.cookbook import CookbookEnrichment
from kodit.domain.protocols import (
    EnrichmentAssociationRepository,
    EnrichmentV2Repository,
    GitFileRepository,
    GitRepoRepository,
)
from kodit.domain.services.cookbook_context_service import (
    COOKBOOK_SYSTEM_PROMPT,
    COOKBOOK_TASK_PROMPT,
    CookbookContextService,
)
from kodit.domain.services.git_repository_service import GitRepositoryScanner
from kodit.domain.services.physical_architecture_service import (
    ARCHITECTURE_ENRICHMENT_SYSTEM_PROMPT,
    ARCHITECTURE_ENRICHMENT_TASK_PROMPT,
    PhysicalArchitectureService,
)
from kodit.domain.value_objects import LanguageMapping, TaskOperation, TrackableType
from kodit.infrastructure.database_schema.database_schema_detector import (
    DatabaseSchemaDetector,
)
from kodit.infrastructure.slicing.api_doc_extractor import APIDocExtractor
from kodit.infrastructure.sqlalchemy.query import GitFileQueryBuilder

if TYPE_CHECKING:
    from kodit.application.services.enrichment_query_service import (
        EnrichmentQueryService,
    )
    from kodit.domain.entities.git import GitFile

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


class EnrichmentGenerationService:
    """Handles generation of various enrichment types for commits."""

    def __init__(  # noqa: PLR0913
        self,
        repo_repository: GitRepoRepository,
        git_file_repository: GitFileRepository,
        enrichment_v2_repository: EnrichmentV2Repository,
        enrichment_association_repository: EnrichmentAssociationRepository,
        enrichment_query_service: "EnrichmentQueryService",
        scanner: GitRepositoryScanner,
        architecture_service: PhysicalArchitectureService,
        cookbook_context_service: CookbookContextService,
        database_schema_detector: DatabaseSchemaDetector,
        enricher_service: Enricher,
        operation: ProgressTracker,
    ) -> None:
        """Initialize the enrichment generation service."""
        self.repo_repository = repo_repository
        self.git_file_repository = git_file_repository
        self.enrichment_v2_repository = enrichment_v2_repository
        self.enrichment_association_repository = enrichment_association_repository
        self.enrichment_query_service = enrichment_query_service
        self.scanner = scanner
        self.architecture_service = architecture_service
        self.cookbook_context_service = cookbook_context_service
        self.database_schema_detector = database_schema_detector
        self.enricher_service = enricher_service
        self.operation = operation
        self._log = structlog.get_logger(__name__)

    async def create_architecture_enrichment(
        self, repository_id: int, commit_sha: str
    ) -> None:
        """Discover physical architecture and create enrichment."""
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

    async def create_api_docs(self, repository_id: int, commit_sha: str) -> None:
        """Generate API documentation from code."""
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

    async def create_commit_description(
        self, repository_id: int, commit_sha: str
    ) -> None:
        """Generate commit description from diff."""
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

    async def create_database_schema(
        self, repository_id: int, commit_sha: str
    ) -> None:
        """Discover and document database schemas."""
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

    async def create_cookbook(self, repository_id: int, commit_sha: str) -> None:
        """Generate usage cookbook examples."""
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
