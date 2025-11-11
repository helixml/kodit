"""Handler for generating cookbook examples for a commit."""

from collections import defaultdict
from typing import TYPE_CHECKING, Any

import structlog

from kodit.application.services.reporting import ProgressTracker
from kodit.domain.enrichments.enricher import Enricher
from kodit.domain.enrichments.enrichment import CommitEnrichmentAssociation
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
from kodit.domain.value_objects import LanguageMapping, TaskOperation, TrackableType
from kodit.infrastructure.sqlalchemy.query import GitFileQueryBuilder

if TYPE_CHECKING:
    from kodit.application.services.enrichment_query_service import (
        EnrichmentQueryService,
    )
    from kodit.domain.entities.git import GitFile


class CookbookHandler:
    """Handler for generating cookbook examples for a commit."""

    def __init__(  # noqa: PLR0913
        self,
        repo_repository: GitRepoRepository,
        git_file_repository: GitFileRepository,
        cookbook_context_service: CookbookContextService,
        enricher_service: Enricher,
        enrichment_v2_repository: EnrichmentV2Repository,
        enrichment_association_repository: EnrichmentAssociationRepository,
        enrichment_query_service: "EnrichmentQueryService",
        operation: ProgressTracker,
    ) -> None:
        """Initialize the cookbook handler."""
        self.repo_repository = repo_repository
        self.git_file_repository = git_file_repository
        self.cookbook_context_service = cookbook_context_service
        self.enricher_service = enricher_service
        self.enrichment_v2_repository = enrichment_v2_repository
        self.enrichment_association_repository = enrichment_association_repository
        self.enrichment_query_service = enrichment_query_service
        self.operation = operation
        self._log = structlog.get_logger(__name__)

    async def execute(self, payload: dict[str, Any]) -> None:
        """Execute cookbook generation operation."""
        repository_id = payload["repository_id"]
        commit_sha = payload["commit_sha"]

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
