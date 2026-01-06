"""Handler for extracting examples from a commit."""

from pathlib import Path
from typing import TYPE_CHECKING, Any

import structlog

from kodit.application.services.reporting import ProgressTracker
from kodit.domain.enrichments.development.example.example import ExampleEnrichment
from kodit.domain.enrichments.enrichment import EnrichmentAssociation
from kodit.domain.entities.git import GitFile
from kodit.domain.protocols import (
    EnrichmentAssociationRepository,
    EnrichmentV2Repository,
    GitCommitRepository,
    GitRepoRepository,
)
from kodit.domain.services.git_repository_service import GitRepositoryScanner
from kodit.domain.value_objects import LanguageMapping, TaskOperation, TrackableType
from kodit.infrastructure.example_extraction.discovery import ExampleDiscovery
from kodit.infrastructure.example_extraction.parser import ParserFactory
from kodit.infrastructure.sqlalchemy import entities as db_entities

if TYPE_CHECKING:
    from kodit.application.services.enrichment_query_service import (
        EnrichmentQueryService,
    )


class ExtractExamplesHandler:
    """Handler for extracting code examples from a commit."""

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
        """Initialize the extract examples handler."""
        self.repo_repository = repo_repository
        self.git_commit_repository = git_commit_repository
        self.scanner = scanner
        self.enrichment_v2_repository = enrichment_v2_repository
        self.enrichment_association_repository = enrichment_association_repository
        self.enrichment_query_service = enrichment_query_service
        self.operation = operation
        self._log = structlog.get_logger(__name__)
        self.discovery = ExampleDiscovery()

    async def execute(self, payload: dict[str, Any]) -> None:
        """Execute extract examples operation."""
        repository_id = payload["repository_id"]
        commit_sha = payload["commit_sha"]

        async with self.operation.create_child(
            operation=TaskOperation.EXTRACT_EXAMPLES_FOR_COMMIT,
            trackable_type=TrackableType.KODIT_REPOSITORY,
            trackable_id=repository_id,
        ) as step:
            if await self.enrichment_query_service.has_examples_for_commit(commit_sha):
                await step.skip("Examples already extracted for commit")
                return

            commit = await self.git_commit_repository.get(commit_sha)
            repo = await self.repo_repository.get(repository_id)
            if not repo.cloned_path:
                raise ValueError(f"Repository {repository_id} has never been cloned")

            files_data = await self.scanner.git_adapter.get_commit_files(
                repo.cloned_path, commit_sha
            )

            files = [
                GitFile(
                    commit_sha=commit.commit_sha,
                    created_at=file_data.get("created_at", commit.date),
                    blob_sha=file_data["blob_sha"],
                    path=str(repo.cloned_path / file_data["path"]),
                    mime_type=file_data.get("mime_type", "application/octet-stream"),
                    size=file_data.get("size", 0),
                    extension=Path(file_data["path"]).suffix.lstrip("."),
                )
                for file_data in files_data
            ]

            example_candidates = [
                f for f in files if self.discovery.is_example_candidate(f.path)
            ]

            examples: list[str] = []
            await step.set_total(len(example_candidates))

            for i, file in enumerate(example_candidates):
                await step.set_current(i, f"Processing {Path(file.path).name}")

                if self.discovery.is_documentation_file(file.path):
                    # Concatenate all code blocks from this documentation file
                    example = self._extract_from_documentation(file)
                    if example:
                        examples.append(example)
                else:
                    example = self._extract_full_file(file)
                    if example:
                        examples.append(example)

            unique_examples = list(set(examples))

            commit_short = commit.commit_sha[:8]
            self._log.info(
                f"Extracted {len(examples)} examples, "
                f"deduplicated to {len(unique_examples)} for {commit_short}"
            )

            saved_enrichments = await self.enrichment_v2_repository.save_bulk(
                [
                    ExampleEnrichment(content=self._sanitize_content(content))
                    for content in unique_examples
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
                f"Saved {len(saved_enrichments)} example enrichments and "
                f"{len(saved_associations)} associations for commit {commit_sha}"
            )

    def _extract_from_documentation(self, file: GitFile) -> str | None:
        """Extract and concatenate all code blocks from documentation file."""
        parser = ParserFactory.create(Path(file.path).suffix)
        if not parser:
            return None

        try:
            with Path(file.path).open() as f:
                content = f.read()
        except OSError as e:
            self._log.warning(f"Failed to read file {file.path}", error=str(e))
            return None

        blocks = parser.parse(content)
        if not blocks:
            return None

        # Concatenate all code blocks with separators
        code_parts = [block.content for block in blocks]
        return "\n\n".join(code_parts)

    def _extract_full_file(self, file: GitFile) -> str | None:
        """Extract full file content as an example."""
        try:
            LanguageMapping.get_language_for_extension(file.extension)
        except ValueError:
            return None

        try:
            with Path(file.path).open() as f:
                return f.read()
        except OSError as e:
            self._log.warning(f"Failed to read file {file.path}", error=str(e))
            return None

    def _sanitize_content(self, content: str) -> str:
        """Remove null bytes and other problematic characters for PostgreSQL UTF-8."""
        # Remove null bytes (0x00) which PostgreSQL UTF-8 doesn't allow
        return content.replace("\x00", "")
