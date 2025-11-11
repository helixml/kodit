"""Handler for generating API documentation for a commit."""

from collections import defaultdict
from typing import TYPE_CHECKING, Any

from kodit.application.services.reporting import ProgressTracker
from kodit.domain.enrichments.enrichment import CommitEnrichmentAssociation
from kodit.domain.protocols import (
    EnrichmentAssociationRepository,
    EnrichmentV2Repository,
    GitFileRepository,
    GitRepoRepository,
)
from kodit.domain.value_objects import LanguageMapping, TaskOperation, TrackableType
from kodit.infrastructure.slicing.api_doc_extractor import APIDocExtractor
from kodit.infrastructure.sqlalchemy.query import GitFileQueryBuilder

if TYPE_CHECKING:
    from kodit.application.services.enrichment_query_service import (
        EnrichmentQueryService,
    )
    from kodit.domain.entities.git import GitFile


class ApiDocsHandler:
    """Handler for generating API documentation for a commit."""

    def __init__(  # noqa: PLR0913
        self,
        repo_repository: GitRepoRepository,
        git_file_repository: GitFileRepository,
        enrichment_v2_repository: EnrichmentV2Repository,
        enrichment_association_repository: EnrichmentAssociationRepository,
        enrichment_query_service: "EnrichmentQueryService",
        operation: ProgressTracker,
    ) -> None:
        """Initialize the API docs handler."""
        self.repo_repository = repo_repository
        self.git_file_repository = git_file_repository
        self.enrichment_v2_repository = enrichment_v2_repository
        self.enrichment_association_repository = enrichment_association_repository
        self.enrichment_query_service = enrichment_query_service
        self.operation = operation

    async def execute(self, payload: dict[str, Any]) -> None:
        """Execute API docs generation operation."""
        repository_id = payload["repository_id"]
        commit_sha = payload["commit_sha"]

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
