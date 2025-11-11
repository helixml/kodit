"""Handler for extracting snippets from a commit."""

from collections import defaultdict
from pathlib import Path
from typing import TYPE_CHECKING, Any

import structlog

from kodit.application.services.reporting import ProgressTracker
from kodit.domain.enrichments.development.snippet.snippet import SnippetEnrichment
from kodit.domain.enrichments.enrichment import EnrichmentAssociation
from kodit.domain.entities.git import GitFile, SnippetV2
from kodit.domain.protocols import (
    EnrichmentAssociationRepository,
    EnrichmentV2Repository,
    GitCommitRepository,
    GitRepoRepository,
)
from kodit.domain.services.git_repository_service import GitRepositoryScanner
from kodit.domain.value_objects import LanguageMapping, TaskOperation, TrackableType
from kodit.infrastructure.slicing.slicer import Slicer
from kodit.infrastructure.sqlalchemy import entities as db_entities

if TYPE_CHECKING:
    from kodit.application.services.enrichment_query_service import (
        EnrichmentQueryService,
    )


class ExtractSnippetsHandler:
    """Handler for extracting code snippets from a commit."""

    def __init__(  # noqa: PLR0913
        self,
        repo_repository: GitRepoRepository,
        git_commit_repository: GitCommitRepository,
        scanner: GitRepositoryScanner,
        enrichment_v2_repository: EnrichmentV2Repository,
        enrichment_association_repository: EnrichmentAssociationRepository,
        enrichment_query_service: "EnrichmentQueryService",
        operation: ProgressTracker,
    ) -> None:
        """Initialize the extract snippets handler."""
        self.repo_repository = repo_repository
        self.git_commit_repository = git_commit_repository
        self.scanner = scanner
        self.enrichment_v2_repository = enrichment_v2_repository
        self.enrichment_association_repository = enrichment_association_repository
        self.enrichment_query_service = enrichment_query_service
        self.operation = operation
        self._log = structlog.get_logger(__name__)

    async def execute(self, payload: dict[str, Any]) -> None:
        """Execute extract snippets operation."""
        repository_id = payload["repository_id"]
        commit_sha = payload["commit_sha"]

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
