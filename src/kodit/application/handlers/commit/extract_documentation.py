"""Handler for extracting documentation from a commit."""

from pathlib import Path
from typing import TYPE_CHECKING, Any, ClassVar

import structlog

from kodit.application.services.reporting import ProgressTracker
from kodit.domain.enrichments.enrichment import EnrichmentAssociation
from kodit.domain.enrichments.usage.documentation import DocumentationEnrichment
from kodit.domain.entities.git import GitFile
from kodit.domain.protocols import (
    EnrichmentAssociationRepository,
    EnrichmentV2Repository,
    GitCommitRepository,
    GitRepoRepository,
)
from kodit.domain.services.git_repository_service import GitRepositoryScanner
from kodit.domain.value_objects import TaskOperation, TrackableType
from kodit.infrastructure.sqlalchemy import entities as db_entities

if TYPE_CHECKING:
    from kodit.application.services.enrichment_query_service import (
        EnrichmentQueryService,
    )


class DocumentationChunker:
    """Chunks documentation files into smaller pieces for embedding."""

    DOCUMENTATION_EXTENSIONS: ClassVar[set[str]] = {
        ".md",
        ".markdown",
        ".rst",
        ".adoc",
        ".asciidoc",
        ".txt",
    }

    MAX_CHUNK_SIZE: ClassVar[int] = 2000
    CHUNK_OVERLAP: ClassVar[int] = 200

    def is_documentation_file(self, file_path: str) -> bool:
        """Check if file is a documentation file."""
        return Path(file_path).suffix.lower() in self.DOCUMENTATION_EXTENSIONS

    def chunk_text(self, text: str) -> list[str]:
        """Split text into overlapping chunks."""
        if len(text) <= self.MAX_CHUNK_SIZE:
            return [text] if text.strip() else []

        chunks = []
        start = 0
        while start < len(text):
            end = start + self.MAX_CHUNK_SIZE

            # Try to break at paragraph boundary
            if end < len(text):
                # Look for paragraph break within range
                para_break = text.rfind("\n\n", start, end)
                if para_break > start + self.MAX_CHUNK_SIZE // 2:
                    end = para_break + 2

            chunk = text[start:end].strip()
            if chunk:
                chunks.append(chunk)

            # Move start with overlap
            start = end - self.CHUNK_OVERLAP
            if start >= len(text):
                break

        return chunks


class ExtractDocumentationHandler:
    """Handler for extracting documentation from a commit."""

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
        """Initialize the extract documentation handler."""
        self.repo_repository = repo_repository
        self.git_commit_repository = git_commit_repository
        self.scanner = scanner
        self.enrichment_v2_repository = enrichment_v2_repository
        self.enrichment_association_repository = enrichment_association_repository
        self.enrichment_query_service = enrichment_query_service
        self.operation = operation
        self._log = structlog.get_logger(__name__)
        self.chunker = DocumentationChunker()

    async def execute(self, payload: dict[str, Any]) -> None:
        """Execute extract documentation operation."""
        repository_id = payload["repository_id"]
        commit_sha = payload["commit_sha"]

        async with self.operation.create_child(
            operation=TaskOperation.EXTRACT_DOCUMENTATION_FOR_COMMIT,
            trackable_type=TrackableType.KODIT_REPOSITORY,
            trackable_id=repository_id,
        ) as step:
            if await self.enrichment_query_service.has_documentation_for_commit(
                commit_sha
            ):
                await step.skip("Documentation already extracted for commit")
                return

            commit = await self.git_commit_repository.get(commit_sha)
            repo = await self.repo_repository.get(repository_id)
            if not repo.cloned_path:
                raise ValueError(f"Repository {repository_id} has never been cloned")

            files_data = await self.scanner.git_adapter.get_commit_file_data(
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

            doc_files = [
                f for f in files if self.chunker.is_documentation_file(f.path)
            ]

            chunks: list[str] = []
            await step.set_total(len(doc_files))

            for i, file in enumerate(doc_files):
                await step.set_current(i, f"Processing {Path(file.path).name}")

                file_chunks = self._extract_chunks(file)
                chunks.extend(file_chunks)

            unique_chunks = list(set(chunks))

            commit_short = commit.commit_sha[:8]
            self._log.info(
                f"Extracted {len(chunks)} documentation chunks, "
                f"deduplicated to {len(unique_chunks)} for {commit_short}"
            )

            saved_enrichments = await self.enrichment_v2_repository.save_bulk(
                [
                    DocumentationEnrichment(content=self._sanitize_content(content))
                    for content in unique_chunks
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
                f"Saved {len(saved_enrichments)} documentation enrichments and "
                f"{len(saved_associations)} associations for commit {commit_sha}"
            )

    def _extract_chunks(self, file: GitFile) -> list[str]:
        """Extract and chunk content from documentation file."""
        try:
            with Path(file.path).open() as f:
                content = f.read()
        except OSError as e:
            self._log.warning(f"Failed to read file {file.path}", error=str(e))
            return []

        return self.chunker.chunk_text(content)

    def _sanitize_content(self, content: str) -> str:
        """Remove null bytes and other problematic characters for PostgreSQL UTF-8."""
        return content.replace("\x00", "")
