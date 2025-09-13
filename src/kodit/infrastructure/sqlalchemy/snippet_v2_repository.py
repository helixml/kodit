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
            # First, delete any existing snippets for this commit
            await self.delete_snippets_for_commit(commit_sha)

            # Save each snippet
            for domain_snippet in snippets:
                # Convert to database entity
                db_snippet = self._mapper.from_domain_snippet_v2(
                    domain_snippet, commit_sha
                )
                self._session.add(db_snippet)
                await self._session.flush()  # Get the ID

                # Save snippet-file associations
                for git_file in domain_snippet.derives_from:
                    # Ensure GitFile exists in database
                    existing_file = await self._session.get(
                        db_entities.GitFile, git_file.blob_sha
                    )
                    if not existing_file:
                        db_file = db_entities.GitFile(
                            blob_sha=git_file.blob_sha,
                            path=git_file.path,
                            mime_type=git_file.mime_type,
                            size=git_file.size,
                        )
                        self._session.add(db_file)
                        await self._session.flush()

                    # Create association
                    association = db_entities.SnippetV2File(
                        snippet_id=db_snippet.id,
                        file_blob_sha=git_file.blob_sha,
                    )
                    self._session.add(association)

    async def get_snippets_for_commit(self, commit_sha: str) -> list[SnippetV2]:
        """Get all snippets for a specific commit."""
        async with self.uow:
            # Get snippets for the commit
            snippets_stmt = select(db_entities.SnippetV2).where(
                db_entities.SnippetV2.commit_sha == commit_sha
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
                    .where(db_entities.SnippetV2File.snippet_id == db_snippet.id)
                )
                db_files = (await self._session.scalars(files_stmt)).all()

                # Convert files to domain entities
                from kodit.domain.entities.git import GitFile
                domain_files = []
                for db_file in db_files:
                    domain_file = GitFile(
                        blob_sha=db_file.blob_sha,
                        path=db_file.path,
                        mime_type=db_file.mime_type,
                        size=db_file.size,
                    )
                    domain_files.append(domain_file)

                # Convert snippet to domain entity
                domain_snippet = self._mapper.to_domain_snippet_v2(
                    db_snippet, domain_files
                )
                domain_snippets.append(domain_snippet)

            return domain_snippets

    async def delete_snippets_for_commit(self, commit_sha: str) -> None:
        """Delete all snippets and their associations for a commit."""
        # Get snippet IDs for this commit
        snippets_stmt = select(db_entities.SnippetV2.id).where(
            db_entities.SnippetV2.commit_sha == commit_sha
        )
        snippet_ids = (await self._session.scalars(snippets_stmt)).all()

        if snippet_ids:
            # Delete snippet-file associations
            assoc_stmt = delete(db_entities.SnippetV2File).where(
                db_entities.SnippetV2File.snippet_id.in_(snippet_ids)
            )
            await self._session.execute(assoc_stmt)

            # Delete snippets
            snippets_del_stmt = delete(db_entities.SnippetV2).where(
                db_entities.SnippetV2.commit_sha == commit_sha
            )
            await self._session.execute(snippets_del_stmt)
