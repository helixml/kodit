"""SQLAlchemy implementation of SnippetRepositoryV2."""

from collections.abc import Callable

from sqlalchemy import delete, select
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities.git import SnippetV2
from kodit.domain.protocols import SnippetRepositoryV2
from kodit.infrastructure.mappers.git_mapper import GitMapper
from kodit.infrastructure.sqlalchemy import entities as db_entities
from kodit.infrastructure.sqlalchemy.unit_of_work import SqlAlchemyUnitOfWork


def create_snippet_v2_repository(
    session_factory: Callable[[], AsyncSession],
) -> SnippetRepositoryV2:
    """Create a snippet v2 repository."""
    uow = SqlAlchemyUnitOfWork(session_factory=session_factory)
    return SqlAlchemySnippetRepositoryV2(uow)


class SqlAlchemySnippetRepositoryV2(SnippetRepositoryV2):
    """SQLAlchemy implementation of SnippetRepositoryV2."""

    def __init__(self, uow: SqlAlchemyUnitOfWork) -> None:
        """Initialize the repository."""
        self.uow = uow

    @property
    def _mapper(self) -> GitMapper:
        return GitMapper()

    @property
    def _session(self) -> AsyncSession:
        if self.uow.session is None:
            raise RuntimeError("UnitOfWork must be used within async context")
        return self.uow.session

    async def save_snippets(self, commit_sha: str, snippets: list[SnippetV2]) -> None:
        """Batch save snippets for a commit."""
        if not snippets:
            return

        async with self.uow:
            for domain_snippet in snippets:
                # Check if snippet already exists
                db_snippet = await self._session.get(
                    db_entities.SnippetV2, domain_snippet.sha
                )

                if not db_snippet:
                    # Convert to database entity
                    db_snippet = self._mapper.from_domain_snippet_v2(domain_snippet)
                    self._session.add(db_snippet)
                    await self._session.flush()  # Ensure snippet is persisted

                    # Save snippet-file associations (only if snippet is new)
                    for git_file in domain_snippet.derives_from:
                        # Ensure GitFile exists in database
                        db_file = await self._session.get(
                            db_entities.GitFile, git_file.blob_sha
                        )
                        if not db_file:
                            db_file = db_entities.GitFile(
                                blob_sha=git_file.blob_sha,
                                path=git_file.path,
                                mime_type=git_file.mime_type,
                                size=git_file.size,
                                extension=git_file.extension,
                            )
                            self._session.add(db_file)
                            await self._session.flush()

                        # Create file association
                        association = db_entities.SnippetV2File(
                            snippet_sha=db_snippet.sha,
                            file_blob_sha=db_file.blob_sha,
                        )
                        self._session.add(association)

                    # Create commit-snippet association
                    commit_snippet = db_entities.CommitSnippetV2(
                        commit_sha=commit_sha,
                        snippet_sha=db_snippet.sha,
                    )
                    self._session.add(commit_snippet)
                else:
                    # Update enrichments if they have changed
                    current_enrichments = await self._session.scalars(
                        select(db_entities.Enrichment).where(
                            db_entities.Enrichment.snippet_sha == db_snippet.sha
                        )
                    )
                    current_enrichments = list(current_enrichments)
                    current_enrichment_types = {
                        enrichment.type for enrichment in current_enrichments
                    }
                    new_enrichment_types = {
                        enrichment.type for enrichment in domain_snippet.enrichments
                    }
                    if current_enrichment_types != new_enrichment_types:
                        # Delete existing enrichments
                        stmt = delete(db_entities.Enrichment).where(
                            db_entities.Enrichment.snippet_sha == db_snippet.sha
                        )
                        await self._session.execute(stmt)

                        # Re-add enrichments
                        for enrichment in domain_snippet.enrichments:
                            db_enrichment = db_entities.Enrichment(
                                snippet_sha=db_snippet.sha,
                                type=db_entities.EnrichmentType(enrichment.type.value),
                                content=enrichment.content,
                            )
                            self._session.add(db_enrichment)
                        await self._session.flush()

    async def get_snippets_for_commit(self, commit_sha: str) -> list[SnippetV2]:
        """Get all snippets for a specific commit."""
        async with self.uow:
            # Get snippets for the commit through the association table
            snippets_stmt = (
                select(db_entities.SnippetV2)
                .join(
                    db_entities.CommitSnippetV2,
                    db_entities.SnippetV2.sha
                    == db_entities.CommitSnippetV2.snippet_sha,
                )
                .where(db_entities.CommitSnippetV2.commit_sha == commit_sha)
            )
            db_snippets = (await self._session.scalars(snippets_stmt)).all()

            domain_snippets = []
            for db_snippet in db_snippets:
                # Get associated files for this snippet
                files_stmt = (
                    select(db_entities.GitFile)
                    .join(
                        db_entities.SnippetV2File,
                        db_entities.GitFile.blob_sha
                        == db_entities.SnippetV2File.file_blob_sha,
                    )
                    .where(db_entities.SnippetV2File.snippet_sha == db_snippet.sha)
                )
                db_files = (await self._session.scalars(files_stmt)).all()

                # Get enrichments for this snippet
                enrichments_stmt = select(db_entities.Enrichment).where(
                    db_entities.Enrichment.snippet_sha == db_snippet.sha
                )
                db_enrichments = (await self._session.scalars(enrichments_stmt)).all()

                # Convert files to domain entities
                from kodit.domain.entities.git import GitFile

                domain_files = []
                for db_file in db_files:
                    domain_file = GitFile(
                        blob_sha=db_file.blob_sha,
                        path=db_file.path,
                        mime_type=db_file.mime_type,
                        size=db_file.size,
                        extension=db_file.extension,
                    )
                    domain_files.append(domain_file)

                # Convert snippet to domain entity
                domain_snippet = self._mapper.to_domain_snippet_v2(
                    db_snippet, domain_files, list(db_enrichments)
                )
                domain_snippets.append(domain_snippet)

            return domain_snippets

    async def delete_snippets_for_commit(self, commit_sha: str) -> None:
        """Delete all snippet associations for a commit."""
        async with self.uow:
            # Note: We only delete the commit-snippet associations,
            # not the snippets themselves as they might be used by other commits
            stmt = delete(db_entities.CommitSnippetV2).where(
                db_entities.CommitSnippetV2.commit_sha == commit_sha
            )
            await self._session.execute(stmt)
